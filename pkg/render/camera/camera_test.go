package camera

import (
	"errors"
	"math"
	"testing"

	pkgmath "github.com/neuengine/neu/pkg/math"
)

// ─── Projection tests ─────────────────────────────────────────────────────────

func TestProjection_Perspective(t *testing.T) {
	t.Parallel()
	p := PerspectiveProjection{FovY: math.Pi / 3, Aspect: 16.0 / 9.0, Near: 0.1, Far: 1000}
	mat, err := p.Matrix()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Projection matrix should be non-zero.
	if mat == (pkgmath.Mat4{}) {
		t.Fatal("perspective matrix is zero")
	}
}

func TestProjection_PerspectiveNearPlaneGuard(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		near float32
	}{
		{"zero near", 0},
		{"negative near", -1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := PerspectiveProjection{FovY: math.Pi / 3, Aspect: 1, Near: tc.near, Far: 100}
			_, err := p.Matrix()
			if !errors.Is(err, ErrInvalidNearPlane) {
				t.Fatalf("got %v, want ErrInvalidNearPlane", err)
			}
		})
	}
}

func TestProjection_Orthographic(t *testing.T) {
	t.Parallel()
	p := OrthographicProjection{Left: -5, Right: 5, Bottom: -5, Top: 5, Near: -100, Far: 100}
	mat, err := p.Matrix()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mat == (pkgmath.Mat4{}) {
		t.Fatal("orthographic matrix is zero")
	}
}

func TestProjection_OrthoDegenerate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		p    OrthographicProjection
	}{
		{"zero width", OrthographicProjection{Left: 1, Right: 1, Bottom: -1, Top: 1}},
		{"zero height", OrthographicProjection{Left: -1, Right: 1, Bottom: 1, Top: 1}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := tc.p.Matrix()
			if !errors.Is(err, ErrDegenerateOrtho) {
				t.Fatalf("got %v, want ErrDegenerateOrtho", err)
			}
		})
	}
}

func BenchmarkProjMatrix(b *testing.B) {
	p := PerspectiveProjection{FovY: math.Pi / 3, Aspect: 16.0 / 9.0, Near: 0.1, Far: 1000}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = p.Matrix()
	}
}

// ─── FrustumFromViewProj tests ────────────────────────────────────────────────

func TestFrustumExtract_PlanesNormalized(t *testing.T) {
	t.Parallel()
	p := PerspectiveProjection{FovY: math.Pi / 3, Aspect: 16.0 / 9.0, Near: 0.1, Far: 1000}
	proj, _ := p.Matrix()
	view := pkgmath.Mat4Identity() // identity view (camera at origin)
	vp := proj.MulMat(view)

	f := FrustumFromViewProj(vp)
	for i, pl := range f.Planes {
		// Each normal should be unit length (within floating-point tolerance).
		n := pl.Normal.Vec()
		length := n.Length()
		if length < 0.99 || length > 1.01 {
			t.Errorf("plane[%d] normal length = %.4f, want ≈ 1.0", i, length)
		}
	}
}

func TestFrustumExtract_PointInsideFrustum(t *testing.T) {
	t.Parallel()
	p := PerspectiveProjection{FovY: math.Pi / 2, Aspect: 1, Near: 1, Far: 100}
	proj, _ := p.Matrix()
	vp := proj.MulMat(pkgmath.Mat4Identity())

	f := FrustumFromViewProj(vp)

	// AABB centred at (0, 0, -5) — clearly inside a perspective frustum
	// looking along -Z with near=1, far=100.
	inside := pkgmath.AABB{
		Min: pkgmath.Vec3{X: -0.1, Y: -0.1, Z: -6},
		Max: pkgmath.Vec3{X: 0.1, Y: 0.1, Z: -4},
	}
	if !IntersectsAabb(f, inside) {
		t.Error("AABB inside frustum reported as outside")
	}
}

func TestFrustumExtract_NoFalseNegatives(t *testing.T) {
	t.Parallel()
	// Property test: AABBs known to be inside a simple axis-aligned frustum
	// must always pass the intersection test.
	p := PerspectiveProjection{FovY: math.Pi / 2, Aspect: 1, Near: 0.1, Far: 50}
	proj, _ := p.Matrix()
	vp := proj.MulMat(pkgmath.Mat4Identity())
	f := FrustumFromViewProj(vp)

	// Small boxes at various depths along -Z (forward direction).
	depths := []float32{1, 5, 10, 25, 49}
	for _, d := range depths {
		box := pkgmath.AABB{
			Min: pkgmath.Vec3{X: -0.01, Y: -0.01, Z: -(d + 0.1)},
			Max: pkgmath.Vec3{X: 0.01, Y: 0.01, Z: -(d - 0.1)},
		}
		if !IntersectsAabb(f, box) {
			t.Errorf("AABB at depth %.1f inside frustum reported as outside", d)
		}
	}
}

// ─── Camera bundle smoke test ─────────────────────────────────────────────────

func TestBundles(t *testing.T) {
	t.Parallel()
	c3 := Camera3D()
	if len(c3) < 2 {
		t.Fatal("Camera3D() should return at least 2 components")
	}
	c2 := Camera2D()
	if len(c2) < 2 {
		t.Fatal("Camera2D() should return at least 2 components")
	}
	// Camera component should be active.
	cam := c3[0].(Camera)
	if !cam.IsActive {
		t.Error("Camera3D camera should be active by default")
	}
}
