package animation

import (
	"math"
	"testing"

	pkganimation "github.com/neuengine/neu/pkg/animation"
)

// TestSampleKeyframes_Step verifies step interpolation holds the previous value.
func TestSampleKeyframes_Step(t *testing.T) {
	k := pkganimation.Keyframes{
		Times:  []float32{0, 1, 2},
		Values: []float32{0, 10, 20},
		Interp: pkganimation.InterpolationStep,
	}
	cases := []struct{ t, want float32 }{
		{0, 0}, {0.9, 0}, {1.0, 10}, {1.5, 10}, {2.0, 20},
	}
	for _, c := range cases {
		got := sampleKeyframes(k, c.t)
		if len(got) == 0 || math.Abs(float64(got[0]-c.want)) > 1e-5 {
			t.Errorf("Step t=%v: got %v, want %v", c.t, got, c.want)
		}
	}
}

// TestSampleKeyframes_Linear verifies linear interpolation between keyframes.
func TestSampleKeyframes_Linear(t *testing.T) {
	k := pkganimation.Keyframes{
		Times:  []float32{0, 2},
		Values: []float32{0, 10},
		Interp: pkganimation.InterpolationLinear,
	}
	got := sampleKeyframes(k, 1) // t=1 = halfway
	if len(got) == 0 {
		t.Fatal("empty result")
	}
	if math.Abs(float64(got[0]-5)) > 1e-5 {
		t.Errorf("Linear t=1: got %v, want 5", got[0])
	}
}

// TestSampleKeyframes_Linear_Vec3 verifies multi-component linear interpolation.
func TestSampleKeyframes_Linear_Vec3(t *testing.T) {
	k := pkganimation.Keyframes{
		Times:  []float32{0, 1},
		Values: []float32{0, 0, 0, 6, 8, 10}, // stride=3
		Interp: pkganimation.InterpolationLinear,
	}
	got := sampleKeyframes(k, 0.5)
	want := []float32{3, 4, 5}
	for i, w := range want {
		if math.Abs(float64(got[i]-w)) > 1e-5 {
			t.Errorf("Vec3 Linear [%d]: got %v, want %v", i, got[i], w)
		}
	}
}

// TestSampleKeyframes_Determinism verifies INV-1: same input → same output.
func TestSampleKeyframes_Determinism(t *testing.T) {
	k := pkganimation.Keyframes{
		Times:  []float32{0, 0.5, 1},
		Values: []float32{0, 5, 10},
		Interp: pkganimation.InterpolationLinear,
	}
	const n = 20
	first := sampleKeyframes(k, 0.3)
	for range n {
		got := sampleKeyframes(k, 0.3)
		if len(got) != len(first) || (len(got) > 0 && math.Abs(float64(got[0]-first[0])) > 1e-7) {
			t.Errorf("non-deterministic: got %v, want %v", got, first)
		}
	}
}

// TestSampleKeyframes_Empty verifies empty keyframes return nil.
func TestSampleKeyframes_Empty(t *testing.T) {
	k := pkganimation.Keyframes{}
	if got := sampleKeyframes(k, 0.5); got != nil {
		t.Errorf("empty keyframes: got %v, want nil", got)
	}
}

// TestSampleKeyframes_BeyondEnd clamps to last value.
func TestSampleKeyframes_BeyondEnd(t *testing.T) {
	k := pkganimation.Keyframes{
		Times:  []float32{0, 1},
		Values: []float32{0, 10},
		Interp: pkganimation.InterpolationLinear,
	}
	got := sampleKeyframes(k, 5) // beyond duration
	if len(got) == 0 || math.Abs(float64(got[0]-10)) > 1e-5 {
		t.Errorf("beyond end: got %v, want 10", got)
	}
}

// TestSlerpQuat verifies quaternion slerp via the identity→90° case.
func TestSlerpQuat(t *testing.T) {
	// Identity quaternion: [0,0,0,1]
	a := []float32{0, 0, 0, 1}
	// 90° rotation around Z: [0, 0, sin(45°), cos(45°)]
	s45 := float32(math.Sqrt2 / 2)
	b := []float32{0, 0, s45, s45}

	got := slerpQuat(a, b, 0.5)
	// Midpoint: 45° around Z → [0, 0, sin(22.5°), cos(22.5°)]
	s225 := float32(math.Sin(math.Pi / 8))
	c225 := float32(math.Cos(math.Pi / 8))
	want := []float32{0, 0, s225, c225}

	for i := range 4 {
		if math.Abs(float64(got[i]-want[i])) > 1e-5 {
			t.Errorf("slerpQuat[%d]: got %v, want %v", i, got[i], want[i])
		}
	}
}
