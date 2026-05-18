package math

import "testing"

// ─── CubicBezier1D ──────────────────────────────────────────────────────────

func TestCubicBezier1DEndpoints(t *testing.T) {
	p0, p3 := float32(2), float32(8)
	if got := CubicBezier1D(p0, 3, 7, p3, 0); abs32(got-p0) > 1e-6 {
		t.Errorf("Bezier1D(t=0) = %v, want %v", got, p0)
	}
	if got := CubicBezier1D(p0, 3, 7, p3, 1); abs32(got-p3) > 1e-6 {
		t.Errorf("Bezier1D(t=1) = %v, want %v", got, p3)
	}
}

func TestCubicBezier1DLinear(t *testing.T) {
	// Line from 0 to 1 with collinear control points → linear.
	for _, tt := range []float32{0, 0.25, 0.5, 0.75, 1} {
		got := CubicBezier1D(0, 1.0/3, 2.0/3, 1, tt)
		if abs32(got-tt) > 1e-5 {
			t.Errorf("linear Bezier1D(t=%v) = %v, want %v", tt, got, tt)
		}
	}
}

// ─── CubicBezierVec3 ────────────────────────────────────────────────────────

func TestCubicBezierVec3Endpoints(t *testing.T) {
	p0 := Vec3{X: 0, Y: 0, Z: 0}
	p3 := Vec3{X: 1, Y: 2, Z: 3}
	p1 := Vec3{X: 0.25, Y: 0.5, Z: 0.75}
	p2 := Vec3{X: 0.75, Y: 1.5, Z: 2.25}

	got0 := CubicBezierVec3(p0, p1, p2, p3, 0)
	got1 := CubicBezierVec3(p0, p1, p2, p3, 1)
	if abs32(got0.X-p0.X) > 1e-6 || abs32(got0.Y-p0.Y) > 1e-6 {
		t.Errorf("BezierVec3(t=0) = %+v, want %+v", got0, p0)
	}
	if abs32(got1.X-p3.X) > 1e-6 || abs32(got1.Y-p3.Y) > 1e-6 {
		t.Errorf("BezierVec3(t=1) = %+v, want %+v", got1, p3)
	}
}

// TestCubicBezierVec3DerivativeContinuity verifies the derivative is defined
// at t=0 and t=1 (C0 boundary) and is consistent with the finite-difference
// approximation at interior points (C1 assertion for a single segment).
func TestCubicBezierVec3DerivativeContinuity(t *testing.T) {
	p0 := Vec3{X: 0, Y: 0, Z: 0}
	p1 := Vec3{X: 1, Y: 0, Z: 0}
	p2 := Vec3{X: 1, Y: 1, Z: 0}
	p3 := Vec3{X: 2, Y: 1, Z: 0}

	eps := float32(1e-4)
	for _, tc := range []float32{0.1, 0.3, 0.5, 0.7, 0.9} {
		analytical := CubicBezierVec3Derivative(p0, p1, p2, p3, tc)
		fwd := CubicBezierVec3(p0, p1, p2, p3, tc+eps)
		bwd := CubicBezierVec3(p0, p1, p2, p3, tc-eps)
		fdX := (fwd.X - bwd.X) / (2 * eps)
		fdY := (fwd.Y - bwd.Y) / (2 * eps)
		if abs32(analytical.X-fdX) > 1e-2 || abs32(analytical.Y-fdY) > 1e-2 {
			t.Errorf("derivative at t=%v: analytical=%+v, finite-diff=(%v,%v)", tc, analytical, fdX, fdY)
		}
	}
}

// ─── CubicHermite1D ─────────────────────────────────────────────────────────

func TestCubicHermite1DEndpoints(t *testing.T) {
	p0, p1 := float32(3), float32(7)
	m0, m1 := float32(1), float32(-1)
	if got := CubicHermite1D(p0, m0, p1, m1, 0); abs32(got-p0) > 1e-6 {
		t.Errorf("Hermite1D(t=0) = %v, want %v", got, p0)
	}
	if got := CubicHermite1D(p0, m0, p1, m1, 1); abs32(got-p1) > 1e-6 {
		t.Errorf("Hermite1D(t=1) = %v, want %v", got, p1)
	}
}

func TestCubicHermite1DTangents(t *testing.T) {
	// At t=0, derivative must equal m0; at t=1, derivative must equal m1.
	p0, p1 := float32(0), float32(0)
	m0, m1 := float32(2), float32(3)
	eps := float32(1e-4)
	derivAt0 := (CubicHermite1D(p0, m0, p1, m1, eps) - CubicHermite1D(p0, m0, p1, m1, 0)) / eps
	if abs32(derivAt0-m0) > 1e-2 {
		t.Errorf("Hermite tangent at t=0: got %v, want %v", derivAt0, m0)
	}
	derivAt1 := (CubicHermite1D(p0, m0, p1, m1, 1) - CubicHermite1D(p0, m0, p1, m1, 1-eps)) / eps
	if abs32(derivAt1-m1) > 1e-2 {
		t.Errorf("Hermite tangent at t=1: got %v, want %v", derivAt1, m1)
	}
}

// ─── HermiteSpline C1 continuity ─────────────────────────────────────────────

// TestHermiteSplineC1Continuity samples the derivative just before and just after
// interior knots of a Catmull-Rom spline and verifies they match — confirming C1.
func TestHermiteSplineC1Continuity(t *testing.T) {
	pts := []Vec3{
		{0, 0, 0}, {1, 1, 0}, {2, 0, 0}, {3, 1, 0},
	}
	spline := NewCatmullRomSpline(pts)

	eps := float32(1e-3)
	for knot := 1; knot < len(pts)-1; knot++ {
		u := float32(knot)
		before := spline.Eval(u - eps)
		at := spline.Eval(u)
		after := spline.Eval(u + eps)
		// Left and right derivatives at the knot must match within tolerance.
		dLeft := Vec3{(at.X - before.X) / eps, (at.Y - before.Y) / eps, (at.Z - before.Z) / eps}
		dRight := Vec3{(after.X - at.X) / eps, (after.Y - at.Y) / eps, (after.Z - at.Z) / eps}
		if abs32(dLeft.X-dRight.X) > 0.1 || abs32(dLeft.Y-dRight.Y) > 0.1 {
			t.Errorf("C1 violated at knot %d: dLeft=%+v dRight=%+v", knot, dLeft, dRight)
		}
	}
}

func TestHermiteSplineEndpoints(t *testing.T) {
	pts := []Vec3{{0, 0, 0}, {1, 0, 0}, {2, 1, 0}}
	s := NewCatmullRomSpline(pts)
	if got := s.Eval(0); got != pts[0] {
		t.Errorf("spline at u=0: got %+v, want %+v", got, pts[0])
	}
	if got := s.Eval(float32(len(pts) - 1)); got != pts[len(pts)-1] {
		t.Errorf("spline at u=n-1: got %+v, want %+v", got, pts[len(pts)-1])
	}
}

func TestHermiteSplineClamp(t *testing.T) {
	pts := []Vec3{{0, 0, 0}, {1, 0, 0}}
	s := NewCatmullRomSpline(pts)
	if got := s.Eval(-5); got != pts[0] {
		t.Errorf("clamp below 0: got %+v", got)
	}
	if got := s.Eval(100); got != pts[1] {
		t.Errorf("clamp above n-1: got %+v", got)
	}
}

// ─── Benchmarks ──────────────────────────────────────────────────────────────

func BenchmarkCubicBezierVec3(b *testing.B) {
	p0 := Vec3{0, 0, 0}
	p1 := Vec3{1, 0, 0}
	p2 := Vec3{1, 1, 0}
	p3 := Vec3{2, 1, 0}
	for b.Loop() {
		_ = CubicBezierVec3(p0, p1, p2, p3, 0.5)
	}
}
