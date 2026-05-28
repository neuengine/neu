package tween

import (
	"math"
	"testing"
)

func TestLerp_Float32(t *testing.T) {
	if got := Lerp(float32(0), float32(10), 0.5); got != 5 {
		t.Errorf("Lerp(0,10,0.5) = %v, want 5", got)
	}
	if got := Lerp(float32(0), float32(10), 0); got != 0 {
		t.Errorf("Lerp(0,10,0) = %v, want 0", got)
	}
	if got := Lerp(float32(0), float32(10), 1); got != 10 {
		t.Errorf("Lerp(0,10,1) = %v, want 10", got)
	}
}

func TestLerp_Clamped(t *testing.T) {
	if got := Lerp(float32(0), float32(10), 2); got != 10 {
		t.Errorf("Lerp(0,10,2) should clamp to 10, got %v", got)
	}
	if got := Lerp(float32(0), float32(10), -1); got != 0 {
		t.Errorf("Lerp(0,10,-1) should clamp to 0, got %v", got)
	}
}

func TestLerp_Vec3(t *testing.T) {
	a := [3]float32{0, 0, 0}
	b := [3]float32{2, 4, 6}
	got := Lerp(a, b, 0.5)
	want := [3]float32{1, 2, 3}
	if got != want {
		t.Errorf("Lerp Vec3 = %v, want %v", got, want)
	}
}

func TestLerpAny_Mismatch(t *testing.T) {
	_, ok := LerpAny(float32(0), [2]float32{1, 2}, 0.5)
	if ok {
		t.Error("LerpAny with mismatched types should return false")
	}
}

func TestLerpAny_UnsupportedType(t *testing.T) {
	_, ok := LerpAny("hello", "world", 0.5)
	if ok {
		t.Error("LerpAny with string types should return false")
	}
}

func TestEasing_Endpoints(t *testing.T) {
	easings := []struct {
		name string
		fn   EasingFn
	}{
		{"Linear", Linear},
		{"EaseInQuad", EaseInQuad},
		{"EaseOutQuad", EaseOutQuad},
		{"EaseInOutQuad", EaseInOutQuad},
		{"EaseInCubic", EaseInCubic},
		{"EaseOutCubic", EaseOutCubic},
		{"EaseInSine", EaseInSine},
		{"EaseOutSine", EaseOutSine},
		{"EaseOutBounce", EaseOutBounce},
		{"EaseInBounce", EaseInBounce},
	}
	for _, e := range easings {
		if got := e.fn(0); math.Abs(got) > 1e-6 {
			t.Errorf("%s(0) = %v, want 0", e.name, got)
		}
		if got := e.fn(1); math.Abs(got-1) > 1e-6 {
			t.Errorf("%s(1) = %v, want 1", e.name, got)
		}
	}
}

func TestEaseInOutQuad_Symmetry(t *testing.T) {
	// EaseInOutQuad must satisfy f(t) + f(1-t) == 1 (symmetric).
	fn := EaseInOutQuad
	for _, t0 := range []float64{0.1, 0.25, 0.4} {
		got := fn(t0) + fn(1-t0)
		if math.Abs(got-1) > 1e-9 {
			t.Errorf("EaseInOutQuad symmetry at t=%v: f(t)+f(1-t) = %v, want 1", t0, got)
		}
	}
}

func TestTween_ZeroValue(t *testing.T) {
	var tw Tween
	if tw.Duration != 0 || tw.Elapsed != 0 {
		t.Error("zero Tween should have zero Duration and Elapsed")
	}
	if tw.LoopMode != LoopOnce {
		t.Errorf("zero LoopMode = %v, want LoopOnce", tw.LoopMode)
	}
	if tw.TimeDimension != Virtual {
		t.Errorf("zero TimeDimension = %v, want Virtual", tw.TimeDimension)
	}
}

// BenchmarkLerpVec3 verifies 0-alloc steady state (C-027).
func BenchmarkLerpVec3(b *testing.B) {
	a := [3]float32{1, 2, 3}
	bv := [3]float32{4, 5, 6}
	b.ReportAllocs()
	for b.Loop() {
		_ = Lerp(a, bv, 0.5)
	}
}
