package render

import (
	"github.com/neuengine/neu/pkg/math"
	"github.com/neuengine/neu/pkg/task"
)

// RenderGroup is a layer bitmask. An object is considered for a view only if
// object.Group & view.CullingMask != 0 (l1-render-core §4.11) — e.g. a
// minimap camera that sees only terrain + markers.
type RenderGroup uint32

// Matches reports whether g shares any bit with mask.
func (g RenderGroup) Matches(mask RenderGroup) bool { return g&mask != 0 }

// RenderObject is a renderer-owned proxy — NOT an ECS entity
// (l1-render-core §4.10). Features create these during Extract; the renderer
// manages their lifecycle independently of ECS structural changes. Per-object
// payload lives in the SoA [RenderDataHolder] at DataIndex.
type RenderObject struct {
	DataIndex int
	Enabled   bool
	Group     RenderGroup
	Bounds    math.AABB
}

// RenderView is a viewpoint to cull against. LastFrameCollected lets a view
// skip redundant work when its state has not advanced (l1-render-core §4.9
// frame-counter tracking). VisibleObjects is reused across frames (0-alloc).
type RenderView struct {
	VisibleObjects     []int
	LastFrameCollected uint64
	ViewProjection     math.Mat4
	CullingMask        RenderGroup
}

// VisibilityGroup owns a set of render objects and culls them against views
// (l1-render-core §4.11). The per-object visible buffer is reused so steady-
// state culling is allocation-free (C-027).
type VisibilityGroup struct {
	objects []RenderObject
	visible []bool // index == RenderObject.DataIndex; disjoint writes (lock-free)
}

// NewVisibilityGroup returns an empty group.
func NewVisibilityGroup() *VisibilityGroup { return &VisibilityGroup{} }

// Add appends obj. Its DataIndex must be unique within the group (the cull
// uses it as a disjoint write slot).
func (vg *VisibilityGroup) Add(obj RenderObject) { vg.objects = append(vg.objects, obj) }

// Objects exposes the backing slice (diagnostics/tests; do not append).
func (vg *VisibilityGroup) Objects() []RenderObject { return vg.objects }

// buildFrustum extracts six inward-pointing planes from a column-major
// view-projection matrix (Gribb–Hartmann). math.Mat4 is [4]Vec4 columns, so
// matrix entry (row r, col c) = vp[c].<component r>; the classic row
// combinations are reconstructed from those components.
//
// Render-core owns this per l1-render-core §4.11 (BuildFrustum is part of the
// visibility-cull flow). Camera T-4C01 will add a package-level
// FrustumFromViewProj; future consolidation is noted, not blocking.
func buildFrustum(vp math.Mat4) math.Frustum {
	// rowR = (vp[0].R, vp[1].R, vp[2].R, vp[3].R)
	type row struct{ x, y, z, w float32 }
	r0 := row{vp[0].X, vp[1].X, vp[2].X, vp[3].X}
	r1 := row{vp[0].Y, vp[1].Y, vp[2].Y, vp[3].Y}
	r2 := row{vp[0].Z, vp[1].Z, vp[2].Z, vp[3].Z}
	r3 := row{vp[0].W, vp[1].W, vp[2].W, vp[3].W}

	mk := func(a, b, c, d float32) math.Plane {
		n := math.Vec3{X: a, Y: b, Z: c}
		l := n.Length()
		if l == 0 {
			return math.Plane{} // degenerate VP — pass-through (no false negative)
		}
		dir, ok := math.NewDir3(n)
		if !ok {
			return math.Plane{}
		}
		return math.Plane{Normal: dir, Distance: d / l}
	}
	// Inward planes: left/right/bottom/top/near/far.
	return math.Frustum{Planes: [6]math.Plane{
		mk(r3.x+r0.x, r3.y+r0.y, r3.z+r0.z, r3.w+r0.w), // left
		mk(r3.x-r0.x, r3.y-r0.y, r3.z-r0.z, r3.w-r0.w), // right
		mk(r3.x+r1.x, r3.y+r1.y, r3.z+r1.z, r3.w+r1.w), // bottom
		mk(r3.x-r1.x, r3.y-r1.y, r3.z-r1.z, r3.w-r1.w), // top
		mk(r3.x+r2.x, r3.y+r2.y, r3.z+r2.z, r3.w+r2.w), // near
		mk(r3.x-r2.x, r3.y-r2.y, r3.z-r2.z, r3.w-r2.w), // far
	}}
}

// cullPasses reports whether obj is visible to a view with cullMask under f.
func cullPasses(obj RenderObject, cullMask RenderGroup, f math.Frustum) bool {
	return obj.Enabled && obj.Group.Matches(cullMask) && math.FrustumAABB(f, obj.Bounds)
}

// cullBatch tests one contiguous slice of objects, writing each result to its
// DISJOINT slot vg.visible[DataIndex] (lock-free — distinct indices never
// alias). A plain method (not a closure) so the sequential call site allocates
// nothing; f is passed by value (stack).
func (vg *VisibilityGroup) cullBatch(batch []RenderObject, cullMask RenderGroup, f math.Frustum) {
	for i := range batch {
		o := batch[i]
		vg.visible[o.DataIndex] = cullPasses(o, cullMask, f)
	}
}

// TryCollect culls the group's objects into v.VisibleObjects for the given
// frame. If v.LastFrameCollected >= frame the view is already current and the
// call is a no-op (l1-render-core §4.9). With a non-nil pool the frustum tests
// run in parallel via task.ForBatched; each goroutine writes a DISJOINT slot
// (visible[DataIndex]) so no synchronisation is needed, then a single-threaded
// compaction yields output identical to the sequential reference. nil pool →
// sequential (same result).
func (vg *VisibilityGroup) TryCollect(v *RenderView, pool *task.ComputePool, frame uint64) {
	if v.LastFrameCollected >= frame {
		return
	}
	f := buildFrustum(v.ViewProjection)

	// Size the disjoint visibility buffer to max DataIndex+1.
	maxIdx := -1
	for i := range vg.objects {
		if d := vg.objects[i].DataIndex; d > maxIdx {
			maxIdx = d
		}
	}
	need := maxIdx + 1
	if cap(vg.visible) < need {
		vg.visible = make([]bool, need)
	} else {
		vg.visible = vg.visible[:need]
		clear(vg.visible)
	}

	const batchSize = 256
	if pool != nil && len(vg.objects) >= batchSize {
		// Parallel path: the closure is unavoidable (ForBatched takes a func)
		// and escapes by design — its per-call cost is O(numWorkers), owned
		// by task.ForBatched's documented contract, not the SoA kernel.
		cullMask := v.CullingMask
		task.ForBatched(pool, vg.objects, batchSize, func(batch []RenderObject) {
			vg.cullBatch(batch, cullMask, f)
		})
	} else {
		// Sequential SoA kernel: a direct method call, no closure → no escape,
		// zero allocation (C-027). This is the unit BenchmarkFrustumCullSoA
		// pins for the 0-alloc guarantee.
		vg.cullBatch(vg.objects, v.CullingMask, f)
	}

	// Deterministic compaction in ascending DataIndex order.
	v.VisibleObjects = v.VisibleObjects[:0]
	for idx := range need {
		if vg.visible[idx] {
			v.VisibleObjects = append(v.VisibleObjects, idx)
		}
	}
	v.LastFrameCollected = frame
}
