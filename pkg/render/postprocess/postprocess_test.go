package postprocess

import (
	"math"
	"testing"

	pkgmath "github.com/neuengine/neu/pkg/math"
)

func v3(x, y, z float32) pkgmath.Vec3 { return pkgmath.Vec3{X: x, Y: y, Z: z} }

// ─── EffectSlot ───────────────────────────────────────────────────────────────

func TestEffectSlot_IsHDR(t *testing.T) {
	t.Parallel()
	hdr := []EffectSlot{SlotSSAO, SlotSSR, SlotBloom, SlotDepthOfField,
		SlotMotionBlur, SlotChromaticAberration, SlotFilmGrain, SlotColorGrading}
	for _, s := range hdr {
		if !s.IsHDR() {
			t.Errorf("%v should be HDR", s)
		}
	}
	for _, s := range []EffectSlot{SlotTonemapping, SlotSpatialAA} {
		if s.IsHDR() {
			t.Errorf("%v should NOT be HDR", s)
		}
	}
}

func TestEffectSlot_CanonicalOrder(t *testing.T) {
	t.Parallel()
	all := []EffectSlot{SlotSSAO, SlotSSR, SlotBloom, SlotDepthOfField,
		SlotMotionBlur, SlotChromaticAberration, SlotFilmGrain, SlotColorGrading,
		SlotTonemapping, SlotSpatialAA}
	for i := 1; i < len(all); i++ {
		if all[i] <= all[i-1] {
			t.Errorf("slot order broken at %d: %v <= %v", i, all[i], all[i-1])
		}
	}
}

// ─── PostProcessStack ─────────────────────────────────────────────────────────

func TestPostProcessStack_EnableDisable(t *testing.T) {
	t.Parallel()
	var s PostProcessStack
	s.Enable(SlotBloom).Enable(SlotTonemapping)
	if !s.IsEnabled(SlotBloom) {
		t.Error("Bloom should be enabled")
	}
	if s.IsEnabled(SlotSpatialAA) {
		t.Error("SpatialAA should not be enabled")
	}
	s.Disable(SlotBloom)
	if s.IsEnabled(SlotBloom) {
		t.Error("Bloom should be disabled")
	}
}

func TestPostProcessStack_EnabledSlotsCanonicalOrder(t *testing.T) {
	t.Parallel()
	var s PostProcessStack
	// Enable in reverse order — EnabledSlots must return canonical (enum) order.
	s.Enable(SlotSpatialAA).Enable(SlotBloom).Enable(SlotTonemapping)
	slots := s.EnabledSlots()
	want := []EffectSlot{SlotBloom, SlotTonemapping, SlotSpatialAA}
	if len(slots) != len(want) {
		t.Fatalf("len = %d, want %d", len(slots), len(want))
	}
	for i, w := range want {
		if slots[i] != w {
			t.Errorf("slots[%d] = %v, want %v", i, slots[i], w)
		}
	}
}

// ─── Tonemapper.Apply ─────────────────────────────────────────────────────────

func approxEq(a, b, tol float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

func TestTonemapper_Reinhard(t *testing.T) {
	t.Parallel()
	type tc struct{ r, g, b, exp, wr, wg, wb float32 }
	cases := []tc{
		{1, 1, 1, 1.0, 0.5, 0.5, 0.5},
		{0, 0, 0, 1.0, 0, 0, 0},
		{2, 2, 2, 1.0, 2.0 / 3, 2.0 / 3, 2.0 / 3},
	}
	for _, c := range cases {
		out := TonemapReinhard.Apply(v3(c.r, c.g, c.b), c.exp)
		if !approxEq(out.X, c.wr, 1e-4) || !approxEq(out.Y, c.wg, 1e-4) || !approxEq(out.Z, c.wb, 1e-4) {
			t.Errorf("Reinhard(%v,%v,%v, exp=%v) = (%v,%v,%v), want (%v,%v,%v)",
				c.r, c.g, c.b, c.exp, out.X, out.Y, out.Z, c.wr, c.wg, c.wb)
		}
	}
}

func TestTonemapper_ACES_ReferenceValues(t *testing.T) {
	t.Parallel()
	type tc struct{ v, exp, want float32 }
	cases := []tc{
		{1.0, 1.0, 0.67330},  // Narkowicz 2015: x=0.6 → 0.9216/1.3688
		{0.5, 1.0, 0.43845},  // x=0.3 → 0.2349/0.5357
		{0.0, 1.0, 0.0},      // black → black
	}
	const tol = float32(5e-4)
	for _, c := range cases {
		out := TonemapACES.Apply(v3(c.v, c.v, c.v), c.exp)
		if !approxEq(out.X, c.want, tol) {
			t.Errorf("ACES(%v, exp=%v) = %v, want %v (±%v)", c.v, c.exp, out.X, c.want, tol)
		}
	}
}

func TestTonemapper_OutputClamped(t *testing.T) {
	t.Parallel()
	for _, op := range []Tonemapper{TonemapReinhard, TonemapACES, TonemapAgX} {
		out := op.Apply(v3(1000, 1000, 1000), 1.0)
		if out.X > 1+1e-4 || out.Y > 1+1e-4 || out.Z > 1+1e-4 ||
			out.X < -1e-4 || out.Y < -1e-4 || out.Z < -1e-4 {
			t.Errorf("%v: extreme input → out-of-[0,1]: (%v,%v,%v)", op, out.X, out.Y, out.Z)
		}
	}
}

func TestTonemapper_Monotonic(t *testing.T) {
	t.Parallel()
	// Brighter input → brighter output for each operator.
	for _, op := range []Tonemapper{TonemapReinhard, TonemapACES, TonemapAgX} {
		prev := op.Apply(v3(0.5, 0.5, 0.5), 0.5)
		for exp := float32(1.0); exp <= 3.0; exp += 0.5 {
			cur := op.Apply(v3(0.5, 0.5, 0.5), exp)
			if cur.X < prev.X-1e-6 {
				t.Errorf("%v: output decreased at exp=%v (%v→%v)", op, exp, prev.X, cur.X)
			}
			prev = cur
		}
	}
}

func TestTonemapper_ReinhardLuminance_PreservesHue(t *testing.T) {
	t.Parallel()
	in := v3(2, 1, 0.5) // HDR colour
	out := TonemapReinhardLuminance.Apply(in, 1.0)
	// R:G ratio (2:1 in) should be preserved.
	if out.Y > 1e-6 {
		ratio := out.X / out.Y
		if math.Abs(float64(ratio-2.0)) > 0.01 {
			t.Errorf("ReinhardLuminance: hue not preserved, R/G = %v, want ~2.0", ratio)
		}
	}
}

func TestTonemapper_BlackStaysBlack(t *testing.T) {
	t.Parallel()
	for _, op := range []Tonemapper{TonemapReinhard, TonemapReinhardLuminance, TonemapACES, TonemapAgX} {
		out := op.Apply(v3(0, 0, 0), 1.0)
		if out.X != 0 || out.Y != 0 || out.Z != 0 {
			t.Errorf("%v: black input → (%v,%v,%v), want (0,0,0)", op, out.X, out.Y, out.Z)
		}
	}
}
