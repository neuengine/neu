package tween

import (
	"math"
	"testing"

	pkgtween "github.com/neuengine/neu/pkg/tween"
)

// testTarget is a simple struct used as the animation target.
type testTarget struct {
	X float32
	V [3]float32
}

func makeAccessor(t *testing.T, path string) *writeAccessor {
	t.Helper()
	acc, err := NewWriteAccessor(path)
	if err != nil {
		t.Fatalf("NewWriteAccessor(%q): %v", path, err)
	}
	return acc
}

func TestAdvanceTween_LinearToEnd(t *testing.T) {
	tw := &pkgtween.Tween{
		StartValue: float32(0),
		EndValue:   float32(10),
		Duration:   1,
		LoopMode:   pkgtween.LoopOnce,
	}
	target := &testTarget{}
	acc := makeAccessor(t, "X")

	done, err := AdvanceTween(tw, acc, target, 0.5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("should not be done at t=0.5")
	}
	if math.Abs(float64(target.X-5)) > 1e-5 {
		t.Errorf("X at t=0.5: got %v, want 5", target.X)
	}

	done, err = AdvanceTween(tw, acc, target, 0.5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("should be done at elapsed=1.0")
	}
	if math.Abs(float64(target.X-10)) > 1e-5 {
		t.Errorf("X at t=1.0: got %v, want 10", target.X)
	}
}

func TestAdvanceTween_DegenerateZeroDuration(t *testing.T) {
	tw := &pkgtween.Tween{
		StartValue: float32(0),
		EndValue:   float32(7),
		Duration:   0,
		LoopMode:   pkgtween.LoopOnce,
	}
	target := &testTarget{}
	acc := makeAccessor(t, "X")

	done, err := AdvanceTween(tw, acc, target, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("degenerate tween should be done immediately")
	}
	if math.Abs(float64(target.X-7)) > 1e-5 {
		t.Errorf("X = %v, want 7", target.X)
	}
}

func TestAdvanceTween_TypeMismatch(t *testing.T) {
	tw := &pkgtween.Tween{
		StartValue: float32(0),
		EndValue:   [3]float32{1, 2, 3}, // wrong type
		Duration:   1,
	}
	target := &testTarget{}
	acc := makeAccessor(t, "X")

	done, err := AdvanceTween(tw, acc, target, 0.5)
	if err == nil {
		t.Error("expected type mismatch error")
	}
	if !done {
		t.Error("mismatch should report done to signal removal")
	}
}

func TestAdvanceTween_Loop(t *testing.T) {
	tw := &pkgtween.Tween{
		StartValue: float32(0),
		EndValue:   float32(10),
		Duration:   1,
		LoopMode:   pkgtween.Loop,
	}
	target := &testTarget{}
	acc := makeAccessor(t, "X")

	// Advance past 1 full loop.
	done, _ := AdvanceTween(tw, acc, target, 1.5)
	if done {
		t.Error("Loop tween should never return done")
	}
	// After 1.5 cycles, t ≈ 0.5 → X ≈ 5.
	if math.Abs(float64(target.X-5)) > 1e-4 {
		t.Errorf("Loop at 1.5s: X = %v, want ~5", target.X)
	}
}

func TestAdvanceTween_PingPong(t *testing.T) {
	tw := &pkgtween.Tween{
		StartValue: float32(0),
		EndValue:   float32(10),
		Duration:   1,
		LoopMode:   pkgtween.PingPong,
	}
	target := &testTarget{}
	acc := makeAccessor(t, "X")

	// At t=1.5 in PingPong: beat 1 → reverse → t=0.5 of reverse → X=5.
	AdvanceTween(tw, acc, target, 1.5)
	if math.Abs(float64(target.X-5)) > 1e-4 {
		t.Errorf("PingPong at 1.5s: X = %v, want ~5", target.X)
	}
}

func TestSplitPath(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"X", []string{"X"}},
		{"Transform.Translation", []string{"Transform", "Translation"}},
		{"A.B.C", []string{"A", "B", "C"}},
	}
	for _, c := range cases {
		got := splitPath(c.in)
		if len(got) != len(c.want) {
			t.Errorf("splitPath(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		for i, s := range c.want {
			if got[i] != s {
				t.Errorf("splitPath(%q)[%d] = %q, want %q", c.in, i, got[i], s)
			}
		}
	}
}
