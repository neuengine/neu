package render

import (
	"context"
	"slices"
	"testing"

	"github.com/neuengine/neu/pkg/math"
	"github.com/neuengine/neu/pkg/task"
)

// identityVP yields the canonical clip-cube frustum [-1,1]^3 (see buildFrustum
// Gribb–Hartmann derivation).
func identityVP() math.Mat4 {
	return math.Mat4{
		{X: 1}, {Y: 1}, {Z: 1}, {W: 1},
	}
}

// buildGroup makes n objects alternating inside/outside the unit-cube frustum.
// Even DataIndex → inside (visible), odd → far outside (culled).
func buildGroup(n int) *VisibilityGroup {
	vg := NewVisibilityGroup()
	for i := range n {
		var c math.Vec3
		if i%2 == 1 {
			c = math.Vec3{X: 50, Y: 50, Z: 50} // well outside [-1,1]^3
		}
		vg.Add(RenderObject{
			DataIndex: i,
			Enabled:   true,
			Group:     1,
			Bounds:    math.AABBFromCenterSize(c, math.Vec3{X: 0.1, Y: 0.1, Z: 0.1}),
		})
	}
	return vg
}

func expectedEven(n int) []int {
	out := make([]int, 0, n/2+1)
	for i := range n {
		if i%2 == 0 {
			out = append(out, i)
		}
	}
	return out
}

// TestVisibilityGroup_ParallelEqualsSequential: the parallel cull (ForBatched)
// must produce byte-identical VisibleObjects to the sequential reference, and
// match the geometric expectation. Run under -race (C-005).
func TestVisibilityGroup_ParallelEqualsSequential(t *testing.T) {
	const n = 10_000
	pool, _ := task.NewTaskPools(task.TaskPoolConfig{})
	defer pool.Shutdown(context.Background())

	par := buildGroup(n)
	seq := buildGroup(n)

	vPar := &RenderView{ViewProjection: identityVP(), CullingMask: 1}
	vSeq := &RenderView{ViewProjection: identityVP(), CullingMask: 1}

	par.TryCollect(vPar, pool, 1) // parallel
	seq.TryCollect(vSeq, nil, 1)  // sequential reference

	want := expectedEven(n)
	if !slices.Equal(vPar.VisibleObjects, want) {
		t.Fatalf("parallel visible (len %d) != expected even set (len %d)",
			len(vPar.VisibleObjects), len(want))
	}
	if !slices.Equal(vPar.VisibleObjects, vSeq.VisibleObjects) {
		t.Fatal("parallel result differs from sequential reference")
	}
}

// TestVisibilityGroup_FrameSkip: a view already collected this frame is a
// no-op (l1-render-core §4.9 frame-counter tracking).
func TestVisibilityGroup_FrameSkip(t *testing.T) {
	vg := buildGroup(8)
	v := &RenderView{ViewProjection: identityVP(), CullingMask: 1}
	vg.TryCollect(v, nil, 5)
	first := slices.Clone(v.VisibleObjects)

	// Mutate an object, re-collect at the SAME frame → must be skipped.
	vg.objects[0].Enabled = false
	vg.TryCollect(v, nil, 5)
	if !slices.Equal(v.VisibleObjects, first) {
		t.Fatal("TryCollect did not skip an already-collected frame")
	}
	// Advancing the frame re-collects (object 0 now disabled → gone).
	vg.TryCollect(v, nil, 6)
	if slices.Contains(v.VisibleObjects, 0) {
		t.Fatal("disabled object 0 still visible after re-collect")
	}
}

// TestVisibilityGroup_CullingMask: an object whose Group shares no bit with
// the view CullingMask is excluded (l1-render-core §4.11).
func TestVisibilityGroup_CullingMask(t *testing.T) {
	vg := NewVisibilityGroup()
	vg.Add(RenderObject{DataIndex: 0, Enabled: true, Group: 0b01,
		Bounds: math.AABBFromCenterSize(math.Vec3{}, math.Vec3{X: 0.1, Y: 0.1, Z: 0.1})})
	vg.Add(RenderObject{DataIndex: 1, Enabled: true, Group: 0b10,
		Bounds: math.AABBFromCenterSize(math.Vec3{}, math.Vec3{X: 0.1, Y: 0.1, Z: 0.1})})

	v := &RenderView{ViewProjection: identityVP(), CullingMask: 0b01}
	vg.TryCollect(v, nil, 1)
	if !slices.Equal(v.VisibleObjects, []int{0}) {
		t.Fatalf("mask cull = %v, want [0] (group 0b10 must be excluded)", v.VisibleObjects)
	}
}

// BenchmarkFrustumCullSoA pins the SoA cull KERNEL (sequential, nil pool) at
// 0 B/op 0 allocs/op (C-027 / l2-render-core §Performance). The SoA design's
// allocation-free claim is about the contiguous data path + reused buffers;
// the parallel ForBatched path adds task.ForBatched's documented O(numWorkers)
// per-call dispatch cost (a Phase-3 concern, not the SoA kernel) and is
// correctness/race-verified by TestVisibilityGroup_ParallelEqualsSequential.
func BenchmarkFrustumCullSoA(b *testing.B) {
	const n = 10_000
	vg := buildGroup(n)
	v := &RenderView{ViewProjection: identityVP(), CullingMask: 1}
	vg.TryCollect(v, nil, 1) // frame 1 (not skipped) — warms visible/VisibleObjects buffers

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.LastFrameCollected = 0 // force re-collect; buffers already sized
		vg.TryCollect(v, nil, uint64(i)+1)
	}
}
