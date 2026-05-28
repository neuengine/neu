package mesh

import (
	"math"
	"testing"
)

// getF32x3 extracts a (x,y,z) triple from a Float32x3 attribute buffer.
func getF32x3(buf []byte, vi int) (x, y, z float32) {
	o := vi * 12
	x = math.Float32frombits(uint32(buf[o]) | uint32(buf[o+1])<<8 | uint32(buf[o+2])<<16 | uint32(buf[o+3])<<24)
	o += 4
	y = math.Float32frombits(uint32(buf[o]) | uint32(buf[o+1])<<8 | uint32(buf[o+2])<<16 | uint32(buf[o+3])<<24)
	o += 4
	z = math.Float32frombits(uint32(buf[o]) | uint32(buf[o+1])<<8 | uint32(buf[o+2])<<16 | uint32(buf[o+3])<<24)
	return
}

// checkNormalsUnit asserts every normal in a Float32x3 buffer is unit-length ≤ tol.
func checkNormalsUnit(t *testing.T, normBuf []byte, nVerts int, tol float64) {
	t.Helper()
	for i := range nVerts {
		x, y, z := getF32x3(normBuf, i)
		l := math.Sqrt(float64(x)*float64(x) + float64(y)*float64(y) + float64(z)*float64(z))
		if math.Abs(l-1) > tol {
			t.Errorf("vertex %d normal length = %.6f (want 1 ± %.6f)", i, l, tol)
		}
	}
}

// meshAABB computes a tight AABB from a Float32x3 position buffer.
func meshAABB(buf []byte, nVerts int) (minX, minY, minZ, maxX, maxY, maxZ float32) {
	minX, minY, minZ = math.MaxFloat32, math.MaxFloat32, math.MaxFloat32
	maxX, maxY, maxZ = -math.MaxFloat32, -math.MaxFloat32, -math.MaxFloat32
	for i := range nVerts {
		x, y, z := getF32x3(buf, i)
		if x < minX {
			minX = x
		}
		if y < minY {
			minY = y
		}
		if z < minZ {
			minZ = z
		}
		if x > maxX {
			maxX = x
		}
		if y > maxY {
			maxY = y
		}
		if z > maxZ {
			maxZ = z
		}
	}
	return
}

const normTol = 1e-5

// ─── TestPrimitive ────────────────────────────────────────────────────────────

func TestPrimitiveCube(t *testing.T) {
	t.Parallel()
	const size = float32(1)
	m := Cube(size)

	// 6 faces × 4 vertices = 24 vertices; 6 faces × 6 indices = 36 indices.
	if got := m.VertexCount(); got != 24 {
		t.Fatalf("VertexCount = %d, want 24", got)
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	ib := m.Indices()
	if ib == nil {
		t.Fatal("Cube: no index buffer")
	}
	// u32 index buffer
	nIdx := len(ib.Data) / 4
	if nIdx != 36 {
		t.Fatalf("index count = %d, want 36", nIdx)
	}

	// AABB should span [-size, +size] on all axes.
	attrs := m.Attributes()
	pos := attrs[AttrPosition]
	minX, minY, minZ, maxX, maxY, maxZ := meshAABB(pos.Data, m.VertexCount())
	const eps = 1e-5
	for _, pair := range [][2]float32{{minX, -size}, {maxX, size}, {minY, -size}, {maxY, size}, {minZ, -size}, {maxZ, size}} {
		if diff := pair[0] - pair[1]; diff < -eps || diff > eps {
			t.Errorf("AABB value %.4f, want %.4f", pair[0], pair[1])
		}
	}

	// Normals unit length.
	norm := attrs[AttrNormal]
	checkNormalsUnit(t, norm.Data, m.VertexCount(), normTol)
}

func TestPrimitiveSphere(t *testing.T) {
	t.Parallel()
	const (
		radius  = float32(2)
		sectors = 16
		stacks  = 8
	)
	m := Sphere(radius, sectors, stacks)
	if err := m.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	// Golden vertex count: (stacks+1) × (sectors+1).
	wantVerts := (stacks + 1) * (sectors + 1)
	if got := m.VertexCount(); got != wantVerts {
		t.Fatalf("VertexCount = %d, want %d", got, wantVerts)
	}

	// Normals unit length.
	attrs := m.Attributes()
	checkNormalsUnit(t, attrs[AttrNormal].Data, m.VertexCount(), normTol)

	// All position points should be within radius±eps of origin.
	const posEps = 1e-4
	pos := attrs[AttrPosition]
	for i := range m.VertexCount() {
		x, y, z := getF32x3(pos.Data, i)
		r := math.Sqrt(float64(x)*float64(x) + float64(y)*float64(y) + float64(z)*float64(z))
		if math.Abs(r-float64(radius)) > posEps {
			t.Errorf("vertex %d distance from origin = %.6f, want %.6f", i, r, radius)
			break
		}
	}
}

func TestPrimitivePlane(t *testing.T) {
	t.Parallel()
	m := Plane(10, 2) // 3×3 grid = 9 quads, but 3 subdivisions → (2+1)² = 9 verts per axis? Let me recalc.
	// subdivisions=2 → n=3 cells per axis → (n+1)²=(4)²=16 verts; n²=9 quads × 6 = 54 idx
	if err := m.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	const wantVerts = 16
	if got := m.VertexCount(); got != wantVerts {
		t.Fatalf("VertexCount = %d, want %d", got, wantVerts)
	}
	// Normals should all point up (+Y).
	attrs := m.Attributes()
	for i := range m.VertexCount() {
		_, ny, _ := getF32x3(attrs[AttrNormal].Data, i)
		if math.Abs(float64(ny)-1) > normTol {
			t.Errorf("vertex %d normal Y = %.4f, want 1.0", i, ny)
		}
	}
}

func TestPrimitiveCylinder(t *testing.T) {
	t.Parallel()
	m := Cylinder(1, 2, 8)
	if err := m.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestPrimitiveCapsule(t *testing.T) {
	t.Parallel()
	m := Capsule(0.5, 2, 8, 4)
	if err := m.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestPrimitiveTorus(t *testing.T) {
	t.Parallel()
	m := Torus(1, 0.3, 16, 8)
	if err := m.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	// Golden vertex count: (majorSectors+1) × (minorSectors+1) = 17 × 9 = 153.
	wantVerts := (16 + 1) * (8 + 1)
	if got := m.VertexCount(); got != wantVerts {
		t.Fatalf("VertexCount = %d, want %d", got, wantVerts)
	}
}

// BenchmarkSphereGen pins steady-state zero allocs for the sphere generator.
func BenchmarkSphereGen(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = Sphere(1, 32, 16)
	}
}
