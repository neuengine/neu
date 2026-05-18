package math

import (
	"testing"
)

// ─── Dir3 ────────────────────────────────────────────────────────────────────

func TestDir3NewUnit(t *testing.T) {
	d, ok := NewDir3(Vec3{3, 4, 0})
	if !ok {
		t.Fatal("NewDir3 of non-zero vector must succeed")
	}
	if !approxEq(d.Vec().Length(), 1) {
		t.Fatalf("Dir3 must be unit: length=%v", d.Vec().Length())
	}
}

func TestDir3NewZeroFails(t *testing.T) {
	_, ok := NewDir3(Vec3{})
	if ok {
		t.Fatal("NewDir3 of zero vector must fail")
	}
}

func TestDir3Unchecked(t *testing.T) {
	d := NewDir3Unchecked(Vec3{0, 1, 0})
	if d.Vec() != (Vec3{Y: 1}) {
		t.Fatalf("Unchecked: got %v", d.Vec())
	}
}

// ─── Rot2 / Isometry2D ──────────────────────────────────────────────────────

func TestRot2Rotate90(t *testing.T) {
	r := Rot2FromDegrees(90)
	v := Vec2{1, 0}
	got := r.Rotate(v)
	want := Vec2{0, 1}
	if !approxEq(got.X, want.X) || !approxEq(got.Y, want.Y) {
		t.Fatalf("Rot2(90°) X→Y: got %v want %v", got, want)
	}
}

func TestIsometry2DRoundtrip(t *testing.T) {
	iso := Isometry2D{
		Rotation:    Rot2FromDegrees(45),
		Translation: Vec2{3, -2},
	}
	p := Vec2{1, 1}
	got := iso.Inverse().TransformPoint(iso.TransformPoint(p))
	if !approxEq(got.X, p.X) || !approxEq(got.Y, p.Y) {
		t.Fatalf("Iso2D inverse roundtrip: got %v want %v", got, p)
	}
}

func TestIsometry2DMul(t *testing.T) {
	a := Isometry2D{Rotation: Rot2FromRadians(0), Translation: Vec2{1, 0}}
	b := Isometry2D{Rotation: Rot2FromRadians(0), Translation: Vec2{0, 1}}
	ab := a.Mul(b)
	got := ab.TransformPoint(Vec2{})
	want := Vec2{1, 1}
	if !approxEq(got.X, want.X) || !approxEq(got.Y, want.Y) {
		t.Fatalf("Iso2D Mul: got %v want %v", got, want)
	}
}

// ─── Isometry3D ──────────────────────────────────────────────────────────────

func TestIsometry3DRoundtrip(t *testing.T) {
	iso := Isometry3D{
		Rotation:    QuatFromAxisAngle(Vec3{0, 1, 0}, pi32/3),
		Translation: Vec3{1, 2, -1},
	}
	p := Vec3{3, 0, 0}
	got := iso.Inverse().TransformPoint(iso.TransformPoint(p))
	if !approxVec3(got, p) {
		t.Fatalf("Iso3D inverse roundtrip: got %v want %v", got, p)
	}
}

func TestIsometry3DMulCompose(t *testing.T) {
	a := Isometry3D{Rotation: QuatIdentity(), Translation: Vec3{1, 0, 0}}
	b := Isometry3D{Rotation: QuatIdentity(), Translation: Vec3{0, 1, 0}}
	p := Vec3{0, 0, 0}
	got := a.Mul(b).TransformPoint(p)
	want := Vec3{1, 1, 0}
	if !approxVec3(got, want) {
		t.Fatalf("Iso3D Mul: got %v want %v", got, want)
	}
}

// ─── Affine3 TRS ─────────────────────────────────────────────────────────────

func TestAffine3IdentityTransformPoint(t *testing.T) {
	p := Vec3{3, -1, 2}
	got := Affine3Identity().TransformPoint(p)
	if !approxVec3(got, p) {
		t.Fatalf("identity TransformPoint: got %v want %v", got, p)
	}
}

func TestAffine3TransformPoint(t *testing.T) {
	a := Affine3{
		Translation: Vec3{1, 2, 3},
		Rotation:    QuatIdentity(),
		Scale:       Vec3{1, 1, 1},
	}
	p := Vec3{0, 0, 0}
	got := a.TransformPoint(p)
	want := Vec3{1, 2, 3}
	if !approxVec3(got, want) {
		t.Fatalf("TransformPoint translation only: got %v want %v", got, want)
	}
}

func TestAffine3TransformVectorIgnoresTranslation(t *testing.T) {
	a := Affine3{
		Translation: Vec3{10, 10, 10},
		Rotation:    QuatIdentity(),
		Scale:       Vec3{1, 1, 1},
	}
	v := Vec3{1, 0, 0}
	got := a.TransformVector(v)
	if !approxVec3(got, v) {
		t.Fatalf("TransformVector must ignore translation: got %v", got)
	}
}

func TestAffine3InverseRoundtrip(t *testing.T) {
	a := Affine3{
		Translation: Vec3{1, -2, 3},
		Rotation:    QuatFromAxisAngle(Vec3{0, 1, 0}, pi32/4),
		Scale:       Vec3{2, 2, 2},
	}
	p := Vec3{5, 3, -1}
	got := a.Inverse().TransformPoint(a.TransformPoint(p))
	if !approxVec3(got, p) {
		t.Fatalf("Affine3 inverse roundtrip: got %v want %v", got, p)
	}
}

func TestAffine3ToMat4FromMat4Roundtrip(t *testing.T) {
	a := Affine3{
		Translation: Vec3{2, -1, 4},
		Rotation:    QuatFromAxisAngle(Vec3{1, 0, 0}, pi32/6),
		Scale:       Vec3{3, 3, 3},
	}
	b := Affine3FromMat4(a.ToMat4())
	p := Vec3{1, 1, 1}
	got := b.TransformPoint(p)
	want := a.TransformPoint(p)
	if !approxVec3(got, want) {
		t.Fatalf("ToMat4/FromMat4 roundtrip: got %v want %v", got, want)
	}
}

// ─── RayAABB ─────────────────────────────────────────────────────────────────

func TestRayAABBHit(t *testing.T) {
	dir, _ := NewDir3(Vec3{0, 0, 1})
	r := Ray3D{Origin: Vec3{0, 0, -5}, Direction: dir}
	b := AABB{Min: Vec3{-1, -1, 0}, Max: Vec3{1, 1, 2}}
	t0, ok := RayAABB(r, b)
	if !ok {
		t.Fatal("ray along +Z must hit AABB in front")
	}
	if !approxEq(t0, 5) {
		t.Fatalf("RayAABB hit distance: got %v want 5", t0)
	}
}

func TestRayAABBMiss(t *testing.T) {
	dir, _ := NewDir3(Vec3{0, 0, 1})
	r := Ray3D{Origin: Vec3{5, 0, -5}, Direction: dir}
	b := AABB{Min: Vec3{-1, -1, 0}, Max: Vec3{1, 1, 2}}
	_, ok := RayAABB(r, b)
	if ok {
		t.Fatal("ray offset in X must miss AABB")
	}
}

func TestRayAABBBehind(t *testing.T) {
	dir, _ := NewDir3(Vec3{0, 0, 1})
	r := Ray3D{Origin: Vec3{0, 0, 10}, Direction: dir}
	b := AABB{Min: Vec3{-1, -1, 0}, Max: Vec3{1, 1, 2}}
	_, ok := RayAABB(r, b)
	if ok {
		t.Fatal("ray aimed away from AABB (behind it) must miss")
	}
}

func TestRayAABBInsideOrigin(t *testing.T) {
	dir, _ := NewDir3(Vec3{1, 0, 0})
	r := Ray3D{Origin: Vec3{0, 0, 0}, Direction: dir}
	b := AABB{Min: Vec3{-2, -2, -2}, Max: Vec3{2, 2, 2}}
	_, ok := RayAABB(r, b)
	if !ok {
		t.Fatal("ray origin inside AABB must hit (exit)")
	}
}

// ─── RaySphere ───────────────────────────────────────────────────────────────

func TestRaySphereHit(t *testing.T) {
	dir, _ := NewDir3(Vec3{0, 0, 1})
	r := Ray3D{Origin: Vec3{0, 0, -5}, Direction: dir}
	s := Sphere{Center: Vec3{0, 0, 0}, Radius: 1}
	t0, ok := RaySphere(r, s)
	if !ok {
		t.Fatal("RaySphere center hit: expected hit")
	}
	// Entry at z=-1: distance from z=-5 is 4.
	if !approxEq(t0, 4) {
		t.Fatalf("RaySphere hit distance: got %v want 4", t0)
	}
}

func TestRaySphereMiss(t *testing.T) {
	dir, _ := NewDir3(Vec3{0, 0, 1})
	r := Ray3D{Origin: Vec3{5, 0, -5}, Direction: dir}
	s := Sphere{Center: Vec3{0, 0, 0}, Radius: 1}
	_, ok := RaySphere(r, s)
	if ok {
		t.Fatal("RaySphere far miss: expected no hit")
	}
}

// ─── AABBAABB ────────────────────────────────────────────────────────────────

func TestAABBAABBOverlap(t *testing.T) {
	a := AABB{Min: Vec3{0, 0, 0}, Max: Vec3{2, 2, 2}}
	b := AABB{Min: Vec3{1, 1, 1}, Max: Vec3{3, 3, 3}}
	if !AABBAABB(a, b) {
		t.Fatal("overlapping AABBs must intersect")
	}
}

func TestAABBAABBNoOverlap(t *testing.T) {
	a := AABB{Min: Vec3{0, 0, 0}, Max: Vec3{1, 1, 1}}
	b := AABB{Min: Vec3{2, 2, 2}, Max: Vec3{3, 3, 3}}
	if AABBAABB(a, b) {
		t.Fatal("separated AABBs must not intersect")
	}
}

func TestAABBAABBTouching(t *testing.T) {
	a := AABB{Min: Vec3{0, 0, 0}, Max: Vec3{1, 1, 1}}
	b := AABB{Min: Vec3{1, 0, 0}, Max: Vec3{2, 1, 1}}
	if !AABBAABB(a, b) {
		t.Fatal("face-touching AABBs must intersect")
	}
}

// ─── SphereSphere ────────────────────────────────────────────────────────────

func TestSphereSphereOverlap(t *testing.T) {
	a := Sphere{Center: Vec3{0, 0, 0}, Radius: 2}
	b := Sphere{Center: Vec3{1, 0, 0}, Radius: 2}
	if !SphereSphere(a, b) {
		t.Fatal("overlapping spheres must intersect")
	}
}

func TestSphereSphereNoOverlap(t *testing.T) {
	a := Sphere{Center: Vec3{0, 0, 0}, Radius: 1}
	b := Sphere{Center: Vec3{5, 0, 0}, Radius: 1}
	if SphereSphere(a, b) {
		t.Fatal("separated spheres must not intersect")
	}
}

// ─── AABBContains ────────────────────────────────────────────────────────────

func TestAABBContainsInside(t *testing.T) {
	b := AABB{Min: Vec3{-1, -1, -1}, Max: Vec3{1, 1, 1}}
	if !AABBContains(b, Vec3{0, 0, 0}) {
		t.Fatal("center must be inside AABB")
	}
}

func TestAABBContainsOutside(t *testing.T) {
	b := AABB{Min: Vec3{-1, -1, -1}, Max: Vec3{1, 1, 1}}
	if AABBContains(b, Vec3{2, 0, 0}) {
		t.Fatal("exterior point must not be inside AABB")
	}
}

func TestAABBContainsOnSurface(t *testing.T) {
	b := AABB{Min: Vec3{0, 0, 0}, Max: Vec3{1, 1, 1}}
	if !AABBContains(b, Vec3{1, 0, 0}) {
		t.Fatal("point on surface must be inside AABB")
	}
}

// ─── FrustumAABB ─────────────────────────────────────────────────────────────

// buildBoxFrustum returns a frustum formed by 6 axis-aligned planes around [-1,1]^3.
func buildBoxFrustum() Frustum {
	pX, _ := PlaneFromNormalPoint(Vec3{-1, 0, 0}, Vec3{1, 0, 0}) // right plane
	nX, _ := PlaneFromNormalPoint(Vec3{1, 0, 0}, Vec3{-1, 0, 0}) // left plane
	pY, _ := PlaneFromNormalPoint(Vec3{0, -1, 0}, Vec3{0, 1, 0}) // top plane
	nY, _ := PlaneFromNormalPoint(Vec3{0, 1, 0}, Vec3{0, -1, 0}) // bottom plane
	pZ, _ := PlaneFromNormalPoint(Vec3{0, 0, -1}, Vec3{0, 0, 1}) // far plane
	nZ, _ := PlaneFromNormalPoint(Vec3{0, 0, 1}, Vec3{0, 0, -1}) // near plane
	return Frustum{Planes: [6]Plane{pX, nX, pY, nY, pZ, nZ}}
}

func TestFrustumAABBInside(t *testing.T) {
	f := buildBoxFrustum()
	b := AABB{Min: Vec3{-0.5, -0.5, -0.5}, Max: Vec3{0.5, 0.5, 0.5}}
	if !FrustumAABB(f, b) {
		t.Fatal("AABB fully inside frustum must pass")
	}
}

func TestFrustumAABBOutside(t *testing.T) {
	f := buildBoxFrustum()
	b := AABB{Min: Vec3{5, 5, 5}, Max: Vec3{6, 6, 6}}
	if FrustumAABB(f, b) {
		t.Fatal("AABB fully outside frustum must be culled")
	}
}

// ─── Benchmarks ──────────────────────────────────────────────────────────────

func BenchmarkRayAABB(b *testing.B) {
	dir, _ := NewDir3(Vec3{0.1, 0.2, 1})
	r := Ray3D{Origin: Vec3{0, 0, -10}, Direction: dir}
	box := AABB{Min: Vec3{-1, -1, 0}, Max: Vec3{1, 1, 2}}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = RayAABB(r, box)
	}
}

func BenchmarkRaySphere(b *testing.B) {
	dir, _ := NewDir3(Vec3{0, 0, 1})
	r := Ray3D{Origin: Vec3{0, 0, -5}, Direction: dir}
	s := Sphere{Center: Vec3{0, 0, 0}, Radius: 1}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = RaySphere(r, s)
	}
}
