package math

import (
	"math"
	"testing"
)

func approxQuat(a, b Quat) bool {
	// quaternions q and -q represent the same rotation
	dot := a.X*b.X + a.Y*b.Y + a.Z*b.Z + a.W*b.W
	if dot < 0 {
		dot = -dot
	}
	return math.Abs(float64(dot-1)) < float64(tol)
}

func TestQuatIdentity(t *testing.T) {
	q := QuatIdentity()
	v := Vec3{1, 2, 3}
	got := q.MulVec3(v)
	if !approxVec3(got, v) {
		t.Fatalf("identity rotation must not change vector: got %v", got)
	}
}

func TestQuatNormalized(t *testing.T) {
	q := QuatFromAxisAngle(Vec3{1, 1, 1}, pi32/3)
	length := math.Sqrt(float64(q.X*q.X + q.Y*q.Y + q.Z*q.Z + q.W*q.W))
	if math.Abs(length-1) > float64(tol) {
		t.Fatalf("QuatFromAxisAngle must be unit: length=%v", length)
	}
}

func TestQuatRotate90AroundZ(t *testing.T) {
	q := QuatFromAxisAngle(Vec3{0, 0, 1}, pi32/2)
	x := Vec3{1, 0, 0}
	got := q.MulVec3(x)
	want := Vec3{0, 1, 0}
	if !approxVec3(got, want) {
		t.Fatalf("90° around Z: X→Y expected, got %v", got)
	}
}

func TestQuatMulComposition(t *testing.T) {
	qx := QuatFromAxisAngle(Vec3{1, 0, 0}, pi32/2)
	qy := QuatFromAxisAngle(Vec3{0, 1, 0}, pi32/2)
	// Compose then rotate, vs rotate twice.
	v := Vec3{0, 0, 1}
	got1 := qy.Mul(qx).MulVec3(v)
	got2 := qy.MulVec3(qx.MulVec3(v))
	if !approxVec3(got1, got2) {
		t.Fatalf("Mul composition mismatch: %v vs %v", got1, got2)
	}
}

func TestQuatInverse(t *testing.T) {
	q := QuatFromAxisAngle(Vec3{1, 0, 0}, pi32/3)
	qi := q.Inverse()
	// q * q^-1 = identity
	composed := q.Mul(qi)
	if !approxQuat(composed, QuatIdentity()) {
		t.Fatalf("q * q.Inverse must be identity: got %v", composed)
	}
}

func TestQuatSlerp(t *testing.T) {
	q0 := QuatIdentity()
	q1 := QuatFromAxisAngle(Vec3{0, 1, 0}, pi32/2)
	// At t=0 should be q0, at t=1 should be q1
	if !approxQuat(q0.Slerp(q1, 0), q0) {
		t.Fatal("Slerp(t=0) must equal q0")
	}
	if !approxQuat(q0.Slerp(q1, 1), q1) {
		t.Fatal("Slerp(t=1) must equal q1")
	}
	// Midpoint: half angle
	mid := q0.Slerp(q1, 0.5)
	qHalf := QuatFromAxisAngle(Vec3{0, 1, 0}, pi32/4)
	if !approxQuat(mid, qHalf) {
		t.Fatalf("Slerp midpoint: got %v, want %v", mid, qHalf)
	}
}

func TestQuatAngleBetween(t *testing.T) {
	q0 := QuatIdentity()
	q1 := QuatFromAxisAngle(Vec3{0, 1, 0}, pi32/2)
	angle := q0.AngleBetween(q1)
	if !approxEq(angle, pi32/2) {
		t.Fatalf("AngleBetween 90°: got %v, want pi/2", angle)
	}
}

func TestQuatFromRotationArc(t *testing.T) {
	from := Vec3{1, 0, 0}
	to := Vec3{0, 1, 0}
	q := QuatFromRotationArc(from, to)
	got := q.MulVec3(from)
	if !approxVec3(got, to) {
		t.Fatalf("RotationArc(X→Y): got %v, want %v", got, to)
	}
}

func TestQuatFromRotationArcAntiParallel(t *testing.T) {
	from := Vec3{1, 0, 0}
	to := Vec3{-1, 0, 0}
	q := QuatFromRotationArc(from, to)
	got := q.MulVec3(from)
	if !approxVec3(got, to) {
		t.Fatalf("RotationArc anti-parallel: got %v, want %v", got, to)
	}
}

func TestQuatFromEulerRoundtrip(t *testing.T) {
	orders := []EulerOrder{EulerXYZ, EulerXZY, EulerYXZ, EulerYZX, EulerZXY, EulerZYX}
	angles := [][3]float32{
		{0.1, 0.2, 0.3},
		{-0.5, 0.4, -0.2},
		{0.7, -0.1, 0.6},
	}
	for _, order := range orders {
		for _, abc := range angles {
			a, b, c := abc[0], abc[1], abc[2]
			q := QuatFromEuler(order, a, b, c)
			ea, eb, ec := q.ToEuler(order)
			q2 := QuatFromEuler(order, ea, eb, ec)
			if !approxQuat(q, q2) {
				t.Errorf("order=%d a=%v b=%v c=%v: roundtrip mismatch: %v → %v", order, a, b, c, q, q2)
			}
		}
	}
}

func TestQuatNormalizationInvariant(t *testing.T) {
	q := Quat{1, 2, 3, 4}.Normalize()
	for range 100 {
		q = q.Mul(QuatFromAxisAngle(Vec3{0, 0, 1}, 0.01))
	}
	length := math.Sqrt(float64(q.X*q.X + q.Y*q.Y + q.Z*q.Z + q.W*q.W))
	if math.Abs(length-1) > float64(tol) {
		t.Fatalf("Quat after 100 Mul iterations: length=%v, want 1", length)
	}
}

// ─── Benchmarks ──────────────────────────────────────────────────────────────

func BenchmarkQuatMul(b *testing.B) {
	q := QuatFromAxisAngle(Vec3{1, 0, 0}, 0.1)
	o := QuatFromAxisAngle(Vec3{0, 1, 0}, 0.2)
	b.ReportAllocs()
	for b.Loop() {
		q = q.Mul(o)
	}
	_ = q
}

func BenchmarkQuatMulVec3(b *testing.B) {
	q := QuatFromAxisAngle(Vec3{0, 1, 0}, 0.3)
	v := Vec3{1, 2, 3}
	b.ReportAllocs()
	for b.Loop() {
		v = q.MulVec3(v)
	}
	_ = v
}

func BenchmarkQuatSlerp(b *testing.B) {
	q0 := QuatIdentity()
	q1 := QuatFromAxisAngle(Vec3{0, 1, 0}, pi32/2)
	b.ReportAllocs()
	t := float32(0)
	for b.Loop() {
		t += 0.001
		_ = q0.Slerp(q1, t)
	}
}
