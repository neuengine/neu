// Package lighting implements tile/froxel light clustering and shadow-pass
// render-graph construction (l2-materials-and-lighting-go.md, Bootstrap 0.1.0).
package lighting

import (
	pkgmath "github.com/neuengine/neu/pkg/math"
	gpu "github.com/neuengine/neu/pkg/render"
	"github.com/neuengine/neu/pkg/task"
)

const clusterBatchSize = 4 // froxels per ForBatched work unit

// LightKind identifies a light's type for clustering and shadow decisions.
type LightKind uint8

const (
	LightKindPoint LightKind = iota
	LightKindSpot
	LightKindDirectional // infinite range; affects every froxel in the view
	LightKindAmbient
)

// LightRef is a culling-ready light descriptor for the cluster pass.
// Sphere is the world-space bounding sphere used for Point/Spot tile overlap.
type LightRef struct {
	Sphere    pkgmath.Sphere
	ShadowRID gpu.RID // pre-allocated shadow map; IsNil() == no shadow
	Kind      LightKind
}

// Froxel is a single tile+depth-slice bin in the cluster grid.
// TileX/Y/Z are pre-computed at construction so the parallel ForBatched
// callback can compute NDC bounds without deriving global cell indices.
type Froxel struct {
	Lights []int
	TileX  int
	TileY  int
	TileZ  int
}

// ClusterGrid is a 3-D grid of Froxels (Nx × Ny × Nz).
// Reuse across frames: ClusterLights calls Reset() internally, which trims
// Lights to length zero while preserving backing-array capacity (C-027).
type ClusterGrid struct {
	Cells []Froxel
	Nx    int
	Ny    int
	Nz    int
}

// NewClusterGrid allocates a ClusterGrid and pre-computes tile coordinates.
func NewClusterGrid(nx, ny, nz int) *ClusterGrid {
	cells := make([]Froxel, nx*ny*nz)
	for z := range nz {
		for y := range ny {
			for x := range nx {
				cells[z*ny*nx+y*nx+x] = Froxel{TileX: x, TileY: y, TileZ: z}
			}
		}
	}
	return &ClusterGrid{Nx: nx, Ny: ny, Nz: nz, Cells: cells}
}

// Reset clears per-cell light index lists without releasing backing arrays (C-027).
func (g *ClusterGrid) Reset() {
	for i := range g.Cells {
		g.Cells[i].Lights = g.Cells[i].Lights[:0]
	}
}

// ClusterLights bins lights into froxels using the given view-projection matrix.
//
//   - pool == nil: sequential execution, 0 allocs/op in steady state (benchmark path).
//   - pool != nil: parallel via task.ForBatched — O(workers) dispatch overhead per call.
//
// Disjoint-write: each worker batch owns non-overlapping cells → no mutex needed.
func ClusterLights(grid *ClusterGrid, vp pkgmath.Mat4, lights []LightRef, pool *task.ComputePool) {
	grid.Reset()
	if pool == nil {
		for i := range grid.Cells {
			assignCell(&grid.Cells[i], lights, vp, grid.Nx, grid.Ny)
		}
		return
	}
	task.ForBatched(pool, grid.Cells, clusterBatchSize, func(batch []Froxel) {
		for i := range batch {
			assignCell(&batch[i], lights, vp, grid.Nx, grid.Ny)
		}
	})
}

// assignCell fills one froxel with the indices of lights that overlap its tile.
func assignCell(f *Froxel, lights []LightRef, vp pkgmath.Mat4, nx, ny int) {
	for li, l := range lights {
		if tileOverlapsLight(f, l, vp, nx, ny) {
			f.Lights = append(f.Lights, li)
		}
	}
}

// tileOverlapsLight tests whether a light's sphere overlaps the given froxel.
//
// Directional and ambient lights affect all froxels unconditionally.
// For Point/Spot, the sphere center is projected to clip space and a
// conservative 2-D circle-vs-AABB distance test is applied in NDC space.
func tileOverlapsLight(f *Froxel, l LightRef, vp pkgmath.Mat4, nx, ny int) bool {
	if l.Kind == LightKindDirectional || l.Kind == LightKindAmbient {
		return true
	}

	c := l.Sphere.Center
	clip := vp.MulVec4(pkgmath.Vec4{X: c.X, Y: c.Y, Z: c.Z, W: 1})
	if clip.W <= 0 {
		return false // light is behind the camera
	}

	invW := 1 / clip.W
	ndcX := clip.X * invW
	ndcY := clip.Y * invW
	rNdc := l.Sphere.Radius * invW // conservative NDC-space radius

	// Tile NDC bounds for a uniform grid: tile t spans [-1+t*2/n, -1+(t+1)*2/n].
	stepX := float32(2) / float32(nx)
	tileXMin := -1 + float32(f.TileX)*stepX
	tileXMax := tileXMin + stepX

	stepY := float32(2) / float32(ny)
	tileYMin := -1 + float32(f.TileY)*stepY
	tileYMax := tileYMin + stepY

	// Squared distance from NDC center to the tile AABB (2D circle-vs-AABB).
	dx := max(tileXMin-ndcX, ndcX-tileXMax, float32(0))
	dy := max(tileYMin-ndcY, ndcY-tileYMax, float32(0))
	return dx*dx+dy*dy <= rNdc*rNdc
}
