package math

import (
	"math"
	"testing"
)

// ─── sRGB ↔ linear round-trip ──────────────────────────────────────────────

// TestSrgbLinearRoundtrip checks that sRGB→linear→sRGB is ≤ 1 ULP error.
func TestSrgbLinearRoundtrip(t *testing.T) {
	samples := []float32{0, 0.0031308, 0.04045, 0.5, 1}
	for _, s := range samples {
		got := LinearToSrgb(SrgbToLinear(s))
		diff := abs32(got - s)
		// Allow 2 ULP tolerance for float32.
		ulp := float32(math.Nextafter(float64(s), float64(s)+1) - float64(s))
		if ulp < 1e-7 {
			ulp = 1e-7
		}
		if diff > 2*ulp {
			t.Errorf("sRGB roundtrip %v: got %v, diff=%e (>2ULP=%e)", s, got, diff, 2*ulp)
		}
	}
}

func TestLinearSrgbRoundtrip(t *testing.T) {
	samples := []float32{0, 0.0031308, 0.04045, 0.5, 1}
	for _, s := range samples {
		got := SrgbToLinear(LinearToSrgb(s))
		diff := abs32(got - s)
		ulp := float32(math.Nextafter(float64(s), float64(s)+1) - float64(s))
		if ulp < 1e-7 {
			ulp = 1e-7
		}
		if diff > 2*ulp {
			t.Errorf("linear roundtrip %v: got %v, diff=%e (>2ULP=%e)", s, got, diff, 2*ulp)
		}
	}
}

// TestSrgbToLinearKnownValues verifies well-known sRGB→linear pairs.
func TestSrgbToLinearKnownValues(t *testing.T) {
	cases := []struct{ srgb, want float32 }{
		{0, 0},
		{1, 1},
		// 0.5 sRGB ≈ 0.2140 linear (IEC 61966-2-1)
		{0.5, 0.21404},
	}
	for _, tc := range cases {
		got := SrgbToLinear(tc.srgb)
		if abs32(got-tc.want) > 1e-4 {
			t.Errorf("SrgbToLinear(%v)=%v, want ≈%v", tc.srgb, got, tc.want)
		}
	}
}

// ─── LinearRgba <-> SrgbRgba ─────────────────────────────────────────────────

func TestLinearRgbaToSrgbRoundtrip(t *testing.T) {
	orig := LinearRgba{R: 0.2, G: 0.5, B: 0.8, A: 1}
	got := orig.ToSrgb().ToLinear()
	if abs32(got.R-orig.R) > 1e-5 || abs32(got.G-orig.G) > 1e-5 || abs32(got.B-orig.B) > 1e-5 {
		t.Errorf("LinearRgba roundtrip: got %+v, want %+v", got, orig)
	}
}

// ─── HSL round-trip ──────────────────────────────────────────────────────────

func TestHslRoundtrip(t *testing.T) {
	colors := []LinearRgba{
		{R: 1, G: 0, B: 0, A: 1},       // red
		{R: 0, G: 1, B: 0, A: 1},       // green
		{R: 0, G: 0, B: 1, A: 1},       // blue
		{R: 0.5, G: 0.5, B: 0.5, A: 1}, // grey
		{R: 1, G: 1, B: 1, A: 1},       // white
		{R: 0, G: 0, B: 0, A: 1},       // black
	}
	for _, c := range colors {
		got := c.ToHsl().ToLinear()
		if abs32(got.R-c.R) > 1e-5 || abs32(got.G-c.G) > 1e-5 || abs32(got.B-c.B) > 1e-5 {
			t.Errorf("HSL roundtrip %+v: got %+v", c, got)
		}
	}
}

// ─── HSV round-trip ──────────────────────────────────────────────────────────

func TestHsvRoundtrip(t *testing.T) {
	colors := []LinearRgba{
		{R: 1, G: 0, B: 0, A: 1},
		{R: 0, G: 0.5, B: 0.5, A: 1},
		{R: 0.3, G: 0.6, B: 0.9, A: 0.5},
	}
	for _, c := range colors {
		got := c.ToHsv().ToLinear()
		if abs32(got.R-c.R) > 1e-5 || abs32(got.G-c.G) > 1e-5 || abs32(got.B-c.B) > 1e-5 {
			t.Errorf("HSV roundtrip %+v: got %+v", c, got)
		}
	}
}

// ─── LerpColor ───────────────────────────────────────────────────────────────

func TestLerpColor(t *testing.T) {
	a := LinearRgba{R: 0, G: 0, B: 0, A: 0}
	b := LinearRgba{R: 1, G: 1, B: 1, A: 1}
	mid := LerpColor(a, b, 0.5)
	if abs32(mid.R-0.5) > 1e-6 || abs32(mid.G-0.5) > 1e-6 {
		t.Errorf("LerpColor 0.5 = %+v, want 0.5 across all channels", mid)
	}
	if got := LerpColor(a, b, 0); got != a {
		t.Errorf("LerpColor t=0 must equal a")
	}
	if got := LerpColor(a, b, 1); got != b {
		t.Errorf("LerpColor t=1 must equal b")
	}
}

// ─── PremultiplyAlpha ────────────────────────────────────────────────────────

func TestPremultiplyAlpha(t *testing.T) {
	c := LinearRgba{R: 1, G: 0.5, B: 0.25, A: 0.5}
	p := c.PremultiplyAlpha()
	if abs32(p.R-0.5) > 1e-6 || abs32(p.G-0.25) > 1e-6 || abs32(p.B-0.125) > 1e-6 {
		t.Errorf("PremultiplyAlpha: got %+v", p)
	}
	if p.A != 0.5 {
		t.Errorf("PremultiplyAlpha must not change A")
	}
}

// ─── Benchmarks ──────────────────────────────────────────────────────────────

func BenchmarkSrgbToLinear(b *testing.B) {
	for b.Loop() {
		_ = SrgbToLinear(0.5)
	}
}

func BenchmarkHslRoundtrip(b *testing.B) {
	c := LinearRgba{R: 0.3, G: 0.6, B: 0.9, A: 1}
	for b.Loop() {
		_ = c.ToHsl().ToLinear()
	}
}
