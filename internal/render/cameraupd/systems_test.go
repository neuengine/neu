package cameraupd

import (
	"testing"

	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/hierarchy"
	ecworld "github.com/neuengine/neu/internal/ecs/world"
	pkgmath "github.com/neuengine/neu/pkg/math"
	"github.com/neuengine/neu/pkg/render/camera"
	"github.com/neuengine/neu/pkg/task"
)

// newTestWorld constructs a test World with required components pre-registered.
func newTestWorld() *ecworld.World {
	w := ecworld.NewWorld()
	ecworld.RegisterComponent[camera.Visibility](w)
	ecworld.RegisterComponent[camera.InheritedVisibility](w)
	ecworld.RegisterComponent[camera.ViewVisibility](w)
	ecworld.RegisterComponent[camera.Camera](w)
	ecworld.RegisterComponent[camera.PerspectiveProjection](w)
	ecworld.RegisterComponent[camera.OrthographicProjection](w)
	ecworld.RegisterComponent[hierarchy.ChildOf](w)
	ecworld.RegisterComponent[hierarchy.Children](w)
	ecworld.RegisterComponent[pkgmath.AABB](w)
	ecworld.RegisterComponent[StoredFrustum](w)
	return w
}

func mustGet[T any](t *testing.T, w *ecworld.World, e entity.Entity) *T {
	t.Helper()
	v, ok := ecworld.Get[T](w, e)
	if !ok {
		t.Fatalf("entity %v missing component %T", e, (*T)(nil))
	}
	return v
}

// ─── Visibility propagation ───────────────────────────────────────────────────

func TestVisibilityPropagation_ParentHiddenHidesDescendants(t *testing.T) {
	t.Parallel()
	w := newTestWorld()

	parent := w.Spawn(component.Data{Value: camera.Visibility(camera.VisibilityHidden)})
	child := w.Spawn(
		component.Data{Value: camera.Visibility(camera.VisibilityInherited)},
		component.Data{Value: hierarchy.ChildOf{Parent: parent}},
	)
	grandchild := w.Spawn(
		component.Data{Value: camera.Visibility(camera.VisibilityVisible)},
		component.Data{Value: hierarchy.ChildOf{Parent: child}},
	)
	_ = grandchild

	propagateVisibility(w)

	if iv := mustGet[camera.InheritedVisibility](t, w, parent); iv.Visible {
		t.Error("parent: InheritedVisibility should be false (Hidden)")
	}
	if iv := mustGet[camera.InheritedVisibility](t, w, child); iv.Visible {
		t.Error("child: InheritedVisibility should be false (parent hidden)")
	}
	if iv := mustGet[camera.InheritedVisibility](t, w, grandchild); iv.Visible {
		t.Error("grandchild: InheritedVisibility should be false (ancestor hidden)")
	}
}

func TestVisibilityPropagation_VisibleParentInheritedChild(t *testing.T) {
	t.Parallel()
	w := newTestWorld()

	parent := w.Spawn(component.Data{Value: camera.Visibility(camera.VisibilityVisible)})
	child := w.Spawn(
		component.Data{Value: camera.Visibility(camera.VisibilityInherited)},
		component.Data{Value: hierarchy.ChildOf{Parent: parent}},
	)

	propagateVisibility(w)

	if iv := mustGet[camera.InheritedVisibility](t, w, parent); !iv.Visible {
		t.Error("parent should be visible")
	}
	if iv := mustGet[camera.InheritedVisibility](t, w, child); !iv.Visible {
		t.Error("child should inherit parent's visible state")
	}
}

func TestVisibilityPropagation_DFSTable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		parentVis      camera.Visibility
		childVis       camera.Visibility
		grandchildVis  camera.Visibility
		wantParent     bool
		wantChild      bool
		wantGrandchild bool
	}{
		{
			name:           "all inherited from visible root",
			parentVis:      camera.VisibilityVisible,
			childVis:       camera.VisibilityInherited,
			grandchildVis:  camera.VisibilityInherited,
			wantParent:     true,
			wantChild:      true,
			wantGrandchild: true,
		},
		{
			name:           "hidden root hides everything",
			parentVis:      camera.VisibilityHidden,
			childVis:       camera.VisibilityVisible,
			grandchildVis:  camera.VisibilityVisible,
			wantParent:     false,
			wantChild:      false,
			wantGrandchild: false,
		},
		{
			name:           "hidden child hides grandchild",
			parentVis:      camera.VisibilityVisible,
			childVis:       camera.VisibilityHidden,
			grandchildVis:  camera.VisibilityVisible,
			wantParent:     true,
			wantChild:      false,
			wantGrandchild: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := newTestWorld()
			parent := w.Spawn(component.Data{Value: tc.parentVis})
			child := w.Spawn(
				component.Data{Value: tc.childVis},
				component.Data{Value: hierarchy.ChildOf{Parent: parent}},
			)
			grandchild := w.Spawn(
				component.Data{Value: tc.grandchildVis},
				component.Data{Value: hierarchy.ChildOf{Parent: child}},
			)

			propagateVisibility(w)

			check := func(e entity.Entity, want bool, name string) {
				iv, ok := ecworld.Get[camera.InheritedVisibility](w, e)
				if !ok {
					t.Errorf("%s: missing InheritedVisibility", name)
					return
				}
				if iv.Visible != want {
					t.Errorf("%s: InheritedVisibility.Visible = %v, want %v", name, iv.Visible, want)
				}
			}
			check(parent, tc.wantParent, "parent")
			check(child, tc.wantChild, "child")
			check(grandchild, tc.wantGrandchild, "grandchild")
		})
	}
}

// ─── SortedActiveCameras (INV-4) ──────────────────────────────────────────────

func TestSortedActiveCameras_DeterministicOrder(t *testing.T) {
	t.Parallel()
	w := newTestWorld()

	for _, order := range []int32{2, 0, 1} {
		w.Spawn(component.Data{Value: camera.Camera{Order: order, IsActive: true}})
	}
	// Inactive camera excluded.
	w.Spawn(component.Data{Value: camera.Camera{Order: -1, IsActive: false}})

	sorted := SortedActiveCameras(w)
	if len(sorted) != 3 {
		t.Fatalf("SortedActiveCameras returned %d cameras, want 3", len(sorted))
	}
	for i := range 3 {
		cam := mustGet[camera.Camera](t, w, sorted[i])
		if cam.Order != int32(i) {
			t.Errorf("camera[%d].Order = %d, want %d", i, cam.Order, i)
		}
	}
}

func TestSortedActiveCameras_EqualOrderTiebreakByID(t *testing.T) {
	t.Parallel()
	w := newTestWorld()

	const n = 10
	for range n {
		w.Spawn(component.Data{Value: camera.Camera{Order: 0, IsActive: true}})
	}

	sorted := SortedActiveCameras(w)
	if len(sorted) != n {
		t.Fatalf("got %d cameras, want %d", len(sorted), n)
	}
	for i := 1; i < len(sorted); i++ {
		if sorted[i-1].ID() >= sorted[i].ID() {
			t.Errorf("camera[%d] ID %d >= camera[%d] ID %d (not sorted by ID)", i-1, sorted[i-1].ID(), i, sorted[i].ID())
		}
	}

	// Must be identical on second call.
	sorted2 := SortedActiveCameras(w)
	for i := range sorted {
		if sorted[i] != sorted2[i] {
			t.Errorf("sort is non-deterministic at index %d", i)
		}
	}
}

// ─── FrustumCull ─────────────────────────────────────────────────────────────

func makePerspFrustum() pkgmath.Frustum {
	proj := pkgmath.Mat4Perspective(1.5708, 1, 1, 100)
	return camera.FrustumFromViewProj(proj.MulMat(pkgmath.Mat4Identity()))
}

func makePool() *task.ComputePool {
	pool, _ := task.NewTaskPools(task.TaskPoolConfig{})
	return pool
}

func shutdownPool(t *testing.T, pool *task.ComputePool) {
	t.Helper()
	if err := pool.Shutdown(t.Context()); err != nil {
		t.Errorf("pool shutdown: %v", err)
	}
}

func TestCullFrustum_InsideFrustum(t *testing.T) {
	t.Parallel()
	w := newTestWorld()
	pool := makePool()
	defer shutdownPool(t, pool)

	w.Spawn(
		component.Data{Value: camera.Camera{IsActive: true}},
		component.Data{Value: StoredFrustum{F: makePerspFrustum()}},
	)
	inside := w.Spawn(
		component.Data{Value: camera.InheritedVisibility{Visible: true}},
		component.Data{Value: pkgmath.AABB{
			Min: pkgmath.Vec3{X: -0.1, Y: -0.1, Z: -5},
			Max: pkgmath.Vec3{X: 0.1, Y: 0.1, Z: -4},
		}},
	)

	cullFrustum(w, pool)

	vv := mustGet[camera.ViewVisibility](t, w, inside)
	if !vv.Visible {
		t.Error("entity inside frustum should have ViewVisibility=true")
	}
}

func TestCullFrustum_HierarchyHiddenSkipsCull(t *testing.T) {
	t.Parallel()
	w := newTestWorld()
	pool := makePool()
	defer shutdownPool(t, pool)

	w.Spawn(
		component.Data{Value: camera.Camera{IsActive: true}},
		component.Data{Value: StoredFrustum{F: makePerspFrustum()}},
	)
	hidden := w.Spawn(
		component.Data{Value: camera.InheritedVisibility{Visible: false}},
		component.Data{Value: pkgmath.AABB{
			Min: pkgmath.Vec3{X: -0.1, Y: -0.1, Z: -5},
			Max: pkgmath.Vec3{X: 0.1, Y: 0.1, Z: -4},
		}},
	)

	cullFrustum(w, pool)

	vv := mustGet[camera.ViewVisibility](t, w, hidden)
	if vv.Visible {
		t.Error("hierarchy-hidden entity should have ViewVisibility=false")
	}
}

func TestCullFrustum_ParallelEqualToSequential(t *testing.T) {
	const n = 10_000
	w := newTestWorld()
	pool := makePool()
	defer shutdownPool(t, pool)

	f := makePerspFrustum()
	w.Spawn(
		component.Data{Value: camera.Camera{IsActive: true}},
		component.Data{Value: StoredFrustum{F: f}},
	)

	entities := make([]entity.Entity, n)
	for i := range n {
		z := float32(-(float32(i%50) + 1))
		entities[i] = w.Spawn(
			component.Data{Value: camera.InheritedVisibility{Visible: true}},
			component.Data{Value: pkgmath.AABB{
				Min: pkgmath.Vec3{X: -0.1, Y: -0.1, Z: z - 0.1},
				Max: pkgmath.Vec3{X: 0.1, Y: 0.1, Z: z + 0.1},
			}},
		)
	}

	cullFrustum(w, pool)

	for i, e := range entities {
		aabb, _ := ecworld.Get[pkgmath.AABB](w, e)
		seqVisible := pkgmath.FrustumAABB(f, *aabb)
		vv, ok := ecworld.Get[camera.ViewVisibility](w, e)
		if !ok {
			t.Fatalf("entity %d missing ViewVisibility", i)
		}
		if vv.Visible != seqVisible {
			t.Errorf("entity %d: parallel=%v, sequential=%v", i, vv.Visible, seqVisible)
			break
		}
	}
}
