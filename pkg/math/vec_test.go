package math

import (
	"math"
	"testing"
)

const tol = 1e-6

func approxEq(a, b float32) bool { return math.Abs(float64(a-b)) < float64(tol) }

func approxVec3(a, b Vec3) bool {
	return approxEq(a.X, b.X) && approxEq(a.Y, b.Y) && approxEq(a.Z, b.Z)
}

func TestVec3Add(t *testing.T) {
	a := Vec3{1, 2, 3}
	b := Vec3{4, 5, 6}
	got := a.Add(b)
	want := Vec3{5, 7, 9}
	if got != want {
		t.Fatalf("Add: got %v, want %v", got, want)
	}
}

func TestVec3AddCommutative(t *testing.T) {
	a, b := Vec3{1, 2, 3}, Vec3{7, -1, 4}
	if a.Add(b) != b.Add(a) {
		t.Fatal("Add must be commutative")
	}
}

func TestVec3Dot(t *testing.T) {
	a := Vec3{1, 0, 0}
	b := Vec3{0, 1, 0}
	if a.Dot(b) != 0 {
		t.Fatal("orthogonal vectors must have dot=0")
	}
	if a.Dot(a) != 1 {
		t.Fatal("unit vector self-dot must be 1")
	}
}

func TestVec3Cross(t *testing.T) {
	x := Vec3{1, 0, 0}
	y := Vec3{0, 1, 0}
	got := x.Cross(y)
	want := Vec3{0, 0, 1}
	if got != want {
		t.Fatalf("X×Y: got %v, want %v", got, want)
	}
}

func TestVec3CrossAntiCommutative(t *testing.T) {
	a, b := Vec3{1, 2, 3}, Vec3{4, 5, 6}
	if !approxVec3(a.Cross(b), b.Cross(a).Neg()) {
		t.Fatal("cross product must be anti-commutative")
	}
}

func TestVec3Normalize(t *testing.T) {
	v := Vec3{3, 4, 0}
	n := v.Normalize()
	length := n.Length()
	if !approxEq(length, 1) {
		t.Fatalf("Normalize: length = %v, want 1", length)
	}
}

func TestVec3NormalizeZero(t *testing.T) {
	got := Vec3{}.Normalize()
	if got != (Vec3{}) {
		t.Fatalf("normalize(zero): got %v, want zero", got)
	}
}

func TestVec3LengthSquared(t *testing.T) {
	v := Vec3{1, 2, 3}
	if !approxEq(v.LengthSquared(), v.Length()*v.Length()) {
		t.Fatal("LengthSquared must equal Length squared")
	}
}

func TestVec3Lerp(t *testing.T) {
	a, b := Vec3{0, 0, 0}, Vec3{2, 2, 2}
	got := a.Lerp(b, 0.5)
	want := Vec3{1, 1, 1}
	if got != want {
		t.Fatalf("Lerp(0.5): got %v, want %v", got, want)
	}
}

func TestVec3DistanceTo(t *testing.T) {
	a, b := Vec3{0, 0, 0}, Vec3{3, 4, 0}
	if !approxEq(a.DistanceTo(b), 5) {
		t.Fatalf("DistanceTo: got %v, want 5", a.DistanceTo(b))
	}
}

func TestVec3MinMax(t *testing.T) {
	a, b := Vec3{1, 5, 2}, Vec3{3, 2, 6}
	if a.Min(b) != (Vec3{1, 2, 2}) {
		t.Fatalf("Min: got %v", a.Min(b))
	}
	if a.Max(b) != (Vec3{3, 5, 6}) {
		t.Fatalf("Max: got %v", a.Max(b))
	}
}

func TestVec3Clamp(t *testing.T) {
	v := Vec3{-1, 0.5, 2}
	lo := Vec3{0, 0, 0}
	hi := Vec3{1, 1, 1}
	got := v.Clamp(lo, hi)
	want := Vec3{0, 0.5, 1}
	if got != want {
		t.Fatalf("Clamp: got %v, want %v", got, want)
	}
}

func TestVec3MulComponent(t *testing.T) {
	a := Vec3{2, 3, 4}
	b := Vec3{5, 6, 7}
	got := a.MulComp(b)
	want := Vec3{10, 18, 28}
	if got != want {
		t.Fatalf("MulComp: got %v, want %v", got, want)
	}
}

func TestVec3NamedConstants(t *testing.T) {
	if Vec3Up() != (Vec3{Y: 1}) {
		t.Fatal("Vec3Up wrong")
	}
	if Vec3Forward() != (Vec3{Z: -1}) {
		t.Fatal("Vec3Forward wrong (-Z expected)")
	}
}

func TestVec2FullCycle(t *testing.T) {
	a := Vec2{3, 4}
	if !approxEq(a.Length(), 5) {
		t.Fatalf("Vec2.Length: got %v, want 5", a.Length())
	}
	n := a.Normalize()
	if !approxEq(n.Length(), 1) {
		t.Fatalf("Vec2.Normalize: length=%v", n.Length())
	}
}

func TestVec4Dot(t *testing.T) {
	a := Vec4{1, 0, 0, 0}
	b := Vec4{0, 1, 0, 0}
	if a.Dot(b) != 0 {
		t.Fatal("Vec4 orthogonal dot must be 0")
	}
}

// ─── Benchmarks (must be 0 allocs) ──────────────────────────────────────────

func BenchmarkVec3Add(b *testing.B) {
	v := Vec3{1, 2, 3}
	o := Vec3{4, 5, 6}
	b.ReportAllocs()
	for b.Loop() {
		v = v.Add(o)
	}
	_ = v
}

func BenchmarkVec3Normalize(b *testing.B) {
	v := Vec3{1, 2, 3}
	b.ReportAllocs()
	for b.Loop() {
		v = v.Normalize()
	}
	_ = v
}

func BenchmarkVec3Cross(b *testing.B) {
	a := Vec3{1, 2, 3}
	bb := Vec3{4, 5, 6}
	b.ReportAllocs()
	for b.Loop() {
		a = a.Cross(bb)
	}
	_ = a
}
