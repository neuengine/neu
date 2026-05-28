package lighting

import (
	"context"
	"testing"

	internalrender "github.com/neuengine/neu/internal/render"
	pkgmath "github.com/neuengine/neu/pkg/math"
	gpu "github.com/neuengine/neu/pkg/render"
	"github.com/neuengine/neu/pkg/task"
)

// ─── ClusterLights: outside tile → zero froxels ───────────────────────────────

func TestClusterLights_OutsideTile(t *testing.T) {
	t.Parallel()
	grid := NewClusterGrid(4, 4, 1)
	// Light at world-pos (100, 0, 0): with identity VP, NDC = (100, 0, 0) — far
	// outside every tile's NDC range [-1, 1].
	lights := []LightRef{
		{Sphere: pkgmath.Sphere{Center: pkgmath.Vec3{X: 100}, Radius: 0.01}, Kind: LightKindPoint},
	}
	ClusterLights(grid, pkgmath.Mat4Identity(), lights, nil)

	for i, f := range grid.Cells {
		if len(f.Lights) != 0 {
			t.Errorf("cell %d (tx=%d,ty=%d): got %d lights, want 0",
				i, f.TileX, f.TileY, len(f.Lights))
		}
	}
}

func TestClusterLights_LightBehindCamera(t *testing.T) {
	t.Parallel()
	grid := NewClusterGrid(4, 4, 1)
	// Right-handed perspective: camera looks down -Z, so +Z is behind it.
	// For point (0,0,+200,1): clip.W = (-1)*200 = -200 ≤ 0 → rejected.
	vp := pkgmath.Mat4Perspective(1.0, 1.0, 0.1, 100)
	lights := []LightRef{
		{Sphere: pkgmath.Sphere{Center: pkgmath.Vec3{X: 0, Y: 0, Z: 200}, Radius: 0.1}, Kind: LightKindPoint},
	}
	ClusterLights(grid, vp, lights, nil)
	for i, f := range grid.Cells {
		if len(f.Lights) != 0 {
			t.Errorf("cell %d: behind-camera light should contribute 0 froxels, got %d", i, len(f.Lights))
		}
	}
}

func TestClusterLights_InsideFrustum(t *testing.T) {
	t.Parallel()
	grid := NewClusterGrid(4, 4, 1)
	// Light at NDC origin with radius > sqrt(2) covers all 16 tiles.
	lights := []LightRef{
		{Sphere: pkgmath.Sphere{Center: pkgmath.Vec3{}, Radius: 2.0}, Kind: LightKindPoint},
	}
	ClusterLights(grid, pkgmath.Mat4Identity(), lights, nil)

	for i, f := range grid.Cells {
		if len(f.Lights) != 1 {
			t.Errorf("cell %d: got %d lights, want 1", i, len(f.Lights))
		}
	}
}

func TestClusterLights_DirectionalAffectsAll(t *testing.T) {
	t.Parallel()
	grid := NewClusterGrid(4, 4, 1)
	lights := []LightRef{
		{Kind: LightKindDirectional}, // no sphere needed — affects all froxels
	}
	ClusterLights(grid, pkgmath.Mat4Identity(), lights, nil)

	for i, f := range grid.Cells {
		if len(f.Lights) != 1 {
			t.Errorf("cell %d: directional light should affect all tiles; got %d", i, len(f.Lights))
		}
	}
}

// ─── Parallel ≡ sequential ────────────────────────────────────────────────────

func TestClusterLights_ParallelEqualsSequential(t *testing.T) {
	t.Parallel()
	n := uint(2)
	pool, _ := task.NewTaskPools(task.TaskPoolConfig{ComputeThreads: &n})
	defer pool.Shutdown(context.Background()) //nolint:errcheck

	lights := []LightRef{
		{Sphere: pkgmath.Sphere{Center: pkgmath.Vec3{}, Radius: 0.5}, Kind: LightKindPoint},
		{Sphere: pkgmath.Sphere{Center: pkgmath.Vec3{X: 100}, Radius: 0.01}, Kind: LightKindPoint},
		{Kind: LightKindDirectional},
	}
	vp := pkgmath.Mat4Identity()

	seqGrid := NewClusterGrid(4, 4, 1)
	ClusterLights(seqGrid, vp, lights, nil)

	parGrid := NewClusterGrid(4, 4, 1)
	ClusterLights(parGrid, vp, lights, pool)

	for i := range seqGrid.Cells {
		if len(seqGrid.Cells[i].Lights) != len(parGrid.Cells[i].Lights) {
			t.Errorf("cell %d: seq len=%d par len=%d",
				i, len(seqGrid.Cells[i].Lights), len(parGrid.Cells[i].Lights))
		}
	}
}

// ─── ShadowPass graph ordering (INV-4) ───────────────────────────────────────

func TestShadowGraphOrdering(t *testing.T) {
	t.Parallel()
	g := &internalrender.RenderGraph{}
	casters := []ShadowCaster{
		{ShadowMapRID: gpu.MakeRID(gpu.KindTexture, 1, 0), Kind: LightKindSpot},
		{ShadowMapRID: gpu.MakeRID(gpu.KindTexture, 2, 0), Kind: LightKindPoint},
		{ShadowMapRID: gpu.MakeRID(gpu.KindTexture, 3, 0), Kind: LightKindDirectional},
	}
	BuildShadowPasses(g, casters)

	if err := g.Build(nil); err != nil {
		t.Fatalf("Build: %v", err)
	}

	order := g.Order()
	// BuildShadowPasses adds casters[0..n-1] then the lighting pass at index n.
	lightingIdx := len(casters)

	lightingPos := -1
	for pos, idx := range order {
		if idx == lightingIdx {
			lightingPos = pos
			break
		}
	}
	if lightingPos < 0 {
		t.Fatal("lighting pass not found in graph order")
	}

	// Every shadow pass must precede the lighting pass (INV-4).
	for pos, idx := range order {
		if idx < len(casters) && pos >= lightingPos {
			t.Errorf("shadow pass[%d] at order pos %d should be before lighting at %d",
				idx, pos, lightingPos)
		}
	}
}

func TestShadowGraphOrdering_NoCasters(t *testing.T) {
	t.Parallel()
	g := &internalrender.RenderGraph{}
	BuildShadowPasses(g, nil) // no-op
	// Graph has no passes — Build should succeed with empty order.
	if err := g.Build(nil); err != nil {
		t.Fatalf("Build on empty graph: %v", err)
	}
	if len(g.Order()) != 0 {
		t.Errorf("expected empty order, got len=%d", len(g.Order()))
	}
}

// ─── Benchmark ────────────────────────────────────────────────────────────────

// BenchmarkClusterLights measures the steady-state sequential cluster kernel
// (pool=nil) where Froxel.Lights slice capacity is pre-warmed — no append
// allocations occur. ForBatched dispatch overhead is separately documented.
func BenchmarkClusterLights(b *testing.B) {
	grid := NewClusterGrid(4, 4, 1)
	lights := []LightRef{
		{Sphere: pkgmath.Sphere{Center: pkgmath.Vec3{}, Radius: 0.5}, Kind: LightKindPoint},
		{Sphere: pkgmath.Sphere{Center: pkgmath.Vec3{X: 0.3, Y: 0.2}, Radius: 0.3}, Kind: LightKindPoint},
		{Kind: LightKindDirectional},
	}
	vp := pkgmath.Mat4Identity()

	ClusterLights(grid, vp, lights, nil) // warm up: pre-populate Lights slice capacity

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		ClusterLights(grid, vp, lights, nil)
	}
}
