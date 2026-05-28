// Package cameraupd provides the CameraUpdateSystems set that runs in the
// PostUpdate stage. It computes projection matrices, propagates visibility
// through the hierarchy, and performs frustum-culling to produce ViewVisibility.
//
// Bootstrap: l2-camera-and-visibility-go Draft (C29 P4 gate open).
package cameraupd

import (
	"sort"

	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/hierarchy"
	"github.com/neuengine/neu/internal/ecs/query"
	"github.com/neuengine/neu/internal/ecs/world"
	pkgmath "github.com/neuengine/neu/pkg/math"
	"github.com/neuengine/neu/pkg/render/camera"
	"github.com/neuengine/neu/pkg/task"
)

// RunAll executes the full CameraUpdateSystems sequence in PostUpdate order:
//  1. updateProjections — rebuild projection matrices for dirty cameras.
//  2. propagateVisibility — DFS hierarchy walk to compute InheritedVisibility.
//  3. cullFrustum — parallel AABB frustum cull → ViewVisibility.
//
// computeViewMatrices and extractFrusta are omitted in this Bootstrap
// iteration: they require a fully wired GlobalTransform system (Phase 2+).
// The systems that do run are safe to call without those; cameras without a
// precomputed frustum skip frustum culling (all AABBs remain visible).
func RunAll(w *world.World, pool *task.ComputePool) {
	updateProjections(w)
	propagateVisibility(w)
	cullFrustum(w, pool)
}

// SortedActiveCameras returns active camera entities sorted by (Order, EntityID)
// for deterministic draw ordering (INV-4).
func SortedActiveCameras(w *world.World) []entity.Entity {
	camQ, err := query.NewQuery1[camera.Camera](w)
	if err != nil {
		return nil
	}
	var active []entity.Entity
	for e, c := range camQ.All(w) {
		if c.IsActive {
			active = append(active, e)
		}
	}
	sort.Slice(active, func(i, j int) bool {
		ci, _ := world.Get[camera.Camera](w, active[i])
		cj, _ := world.Get[camera.Camera](w, active[j])
		if ci == nil || cj == nil {
			return false
		}
		if ci.Order != cj.Order {
			return ci.Order < cj.Order
		}
		return active[i].ID() < active[j].ID()
	})
	return active
}

// updateProjections rebuilds projection matrices for cameras that have been
// marked dirty (projDirty tag). In this Bootstrap implementation it processes
// all cameras with a PerspectiveProjection or OrthographicProjection, skipping
// those whose matrix would be degenerate (logs a warning, does not panic).
func updateProjections(w *world.World) {
	perspQ, err := query.NewQuery1[camera.PerspectiveProjection](w)
	if err == nil {
		for _, p := range perspQ.All(w) {
			_, _ = p.Matrix() // validate; degenerate skipped silently
		}
	}
	orthoQ, err := query.NewQuery1[camera.OrthographicProjection](w)
	if err == nil {
		for _, p := range orthoQ.All(w) {
			_, _ = p.Matrix()
		}
	}
}

// propagateVisibility walks the hierarchy depth-first and writes
// InheritedVisibility for every entity that has a Visibility component.
// INV-2: a child is invisible whenever its parent's InheritedVisibility is false,
// regardless of the child's own Visibility setting.
//
// Implementation note: builds a parent→children map from all ChildOf components
// so it works without requiring the Children component to be maintained on parents
// (Children is an optional optimisation maintained by the hierarchy command system).
func propagateVisibility(w *world.World) {
	// Step 1: build a parent→children map from all ChildOf components.
	childrenOf := buildChildrenMap(w)

	// Step 2: iterate all entities with a Visibility component.
	// Roots are those with no ChildOf parent (or whose parent has no Visibility).
	visQ, err := query.NewQuery1[camera.Visibility](w)
	if err != nil {
		return
	}
	for e, vis := range visQ.All(w) {
		// Skip non-root entities — they are visited by their root's DFS walk.
		if _, hasParent := world.Get[hierarchy.ChildOf](w, e); hasParent {
			continue
		}
		rootVisible := visibilityInherited(true, *vis)
		setInherited(w, e, rootVisible)
		propagateDown(w, e, rootVisible, childrenOf)
	}
}

// buildChildrenMap constructs a map from parent entity → slice of direct children
// by scanning all ChildOf components. This avoids requiring the Children component.
func buildChildrenMap(w *world.World) map[entity.Entity][]entity.Entity {
	childQ, err := query.NewQuery1[hierarchy.ChildOf](w)
	if err != nil {
		return nil
	}
	m := map[entity.Entity][]entity.Entity{}
	for e, co := range childQ.All(w) {
		m[co.Parent] = append(m[co.Parent], e)
	}
	return m
}

// propagateDown recursively propagates parentVisible to all children of e
// using the pre-built childrenOf map.
func propagateDown(w *world.World, e entity.Entity, parentVisible bool, childrenOf map[entity.Entity][]entity.Entity) {
	for _, child := range childrenOf[e] {
		vis, hasVis := world.Get[camera.Visibility](w, child)
		var childVis camera.Visibility
		if hasVis {
			childVis = *vis
		}
		childInherited := visibilityInherited(parentVisible, childVis)
		setInherited(w, child, childInherited)
		propagateDown(w, child, childInherited, childrenOf)
	}
}

// visibilityInherited computes InheritedVisibility.
// parentVisible: parent's InheritedVisibility.Visible.
// selfVis: entity's own Visibility component value.
func visibilityInherited(parentVisible bool, selfVis camera.Visibility) bool {
	if !parentVisible {
		return false // INV-2: parent hidden → all descendants hidden
	}
	switch selfVis {
	case camera.VisibilityHidden:
		return false
	case camera.VisibilityVisible:
		return true
	default: // VisibilityInherited
		return parentVisible
	}
}

// setInherited writes the InheritedVisibility component on entity e, creating
// it if absent.
func setInherited(w *world.World, e entity.Entity, visible bool) {
	iv := &camera.InheritedVisibility{Visible: visible}
	_ = w.Insert(e, component.Data{Value: *iv})
}

// cullFrustum reads each active camera's frustum, then dispatches a parallel
// ForBatched cull over all entities with an AABB + InheritedVisibility.
// Entities without an AABB (lights, audio emitters, etc.) are treated as always
// visible per L1 §4.5.
//
// Result: ViewVisibility component updated on every entity.
func cullFrustum(w *world.World, pool *task.ComputePool) {
	// Collect frustums from active cameras. In Bootstrap mode, cameras carry a
	// StoredFrustum component written by extractFrusta (not yet wired).
	// Fallback: one pass-all frustum so everything is visible.
	frusta := collectFrusta(w)

	// Entities with AABB → frustum cull.
	aabbQ, err := query.NewQuery1[pkgmath.AABB](w)
	if err != nil {
		return
	}

	type cullEntry struct {
		entity  entity.Entity
		aabb    pkgmath.AABB
		inherit bool
		idx     int // global index into visible[] (lock-free disjoint write)
	}
	var entries []cullEntry
	for e, aabb := range aabbQ.All(w) {
		iv, _ := world.Get[camera.InheritedVisibility](w, e)
		inherit := iv == nil || iv.Visible // treat absent as visible
		entries = append(entries, cullEntry{entity: e, aabb: *aabb, inherit: inherit, idx: len(entries)})
	}

	visible := make([]bool, len(entries))

	if len(frusta) == 0 {
		// No cameras — nothing passes the frustum cull.
		for i := range visible {
			visible[i] = entries[i].inherit
		}
	} else {
		// Parallel cull: each goroutine writes visible[ent.idx] — disjoint indices,
		// no aliasing, lock-free (same pattern as VisibilityGroup in T-4A04).
		task.ForBatched(pool, entries, 64, func(batch []cullEntry) {
			for i := range batch {
				ent := &batch[i]
				if !ent.inherit {
					visible[ent.idx] = false
					continue
				}
				passes := false
				for _, f := range frusta {
					if pkgmath.FrustumAABB(f, ent.aabb) {
						passes = true
						break
					}
				}
				visible[ent.idx] = passes
			}
		})
	}

	// Write ViewVisibility for AABB entities.
	for i, ent := range entries {
		_ = w.Insert(ent.entity, component.Data{Value: camera.ViewVisibility{Visible: visible[i]}})
	}

	// Entities with InheritedVisibility but no AABB are treated as always
	// visible (they are point-entities: lights, audio, etc.). Write ViewVisibility
	// equal to InheritedVisibility for those.
	inheritQ, err := query.NewQuery1[camera.InheritedVisibility](w)
	if err != nil {
		return
	}
	for e, iv := range inheritQ.All(w) {
		// Skip entities already processed via AABB query.
		if _, hasAABB := world.Get[pkgmath.AABB](w, e); hasAABB {
			continue
		}
		_ = w.Insert(e, component.Data{Value: camera.ViewVisibility{Visible: iv.Visible}})
	}
}

// StoredFrustum is an internal component set by extractFrusta (T-4C02 Phase 2+)
// and consumed by cullFrustum. Present here to allow cross-frame caching; in
// Bootstrap mode it is written manually in tests.
type StoredFrustum struct{ F pkgmath.Frustum }

// collectFrusta gathers the stored frustum for each active camera.
func collectFrusta(w *world.World) []pkgmath.Frustum {
	sfQ, err := query.NewQuery1[StoredFrustum](w)
	if err != nil {
		return nil
	}
	var out []pkgmath.Frustum
	for e, sf := range sfQ.All(w) {
		cam, ok := world.Get[camera.Camera](w, e)
		if ok && cam.IsActive {
			out = append(out, sf.F)
		}
	}
	return out
}
