package math

import "math"

// LinearRgba is a linear-light RGBA color (each component in [0, 1] for SDR).
// It is the primary in-engine color representation — GPU shaders expect linear.
type LinearRgba struct{ R, G, B, A float32 }

// SrgbRgba is a gamma-encoded sRGB RGBA color.
// Byte images from disk are typically sRGB; convert to LinearRgba before use.
type SrgbRgba struct{ R, G, B, A float32 }

// HslaColor is a cylindrical HSL color (Hue in [0,360), S/L/A in [0,1]).
type HslaColor struct{ H, S, L, A float32 }

// HsvaColor is a cylindrical HSV color (Hue in [0,360), S/V/A in [0,1]).
type HsvaColor struct{ H, S, V, A float32 }

// ─── sRGB ↔ Linear conversions ───────────────────────────────────────────────

// SrgbToLinear converts a single sRGB channel to linear light (IEC 61966-2-1).
func SrgbToLinear(c float32) float32 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return float32(math.Pow(float64((c+0.055)/1.055), 2.4))
}

// LinearToSrgb converts a single linear-light channel to sRGB.
func LinearToSrgb(c float32) float32 {
	if c <= 0.0031308 {
		return 12.92 * c
	}
	return 1.055*float32(math.Pow(float64(c), 1.0/2.4)) - 0.055
}

// ToLinear converts an sRGB color to linear-light RGBA.
func (s SrgbRgba) ToLinear() LinearRgba {
	return LinearRgba{
		R: SrgbToLinear(s.R),
		G: SrgbToLinear(s.G),
		B: SrgbToLinear(s.B),
		A: s.A,
	}
}

// ToSrgb converts a linear-light RGBA color to sRGB.
func (l LinearRgba) ToSrgb() SrgbRgba {
	return SrgbRgba{
		R: LinearToSrgb(l.R),
		G: LinearToSrgb(l.G),
		B: LinearToSrgb(l.B),
		A: l.A,
	}
}

// ─── HSL conversions ─────────────────────────────────────────────────────────

// ToHsl converts a linear-light RGBA color to HSL.
func (l LinearRgba) ToHsl() HslaColor {
	r, g, b := float64(l.R), float64(l.G), float64(l.B)
	cmax := max64(r, max64(g, b))
	cmin := min64(r, min64(g, b))
	delta := cmax - cmin
	ll := (cmax + cmin) * 0.5

	var h, s float64
	if delta != 0 {
		if ll < 0.5 {
			s = delta / (cmax + cmin)
		} else {
			s = delta / (2.0 - cmax - cmin)
		}
		switch cmax {
		case r:
			h = math.Mod((g-b)/delta, 6.0)
		case g:
			h = (b-r)/delta + 2.0
		default:
			h = (r-g)/delta + 4.0
		}
		h *= 60.0
		if h < 0 {
			h += 360.0
		}
	}
	return HslaColor{H: float32(h), S: float32(s), L: float32(ll), A: l.A}
}

// ToLinear converts an HSL color to linear-light RGBA.
func (h HslaColor) ToLinear() LinearRgba {
	hh, ss, ll := float64(h.H), float64(h.S), float64(h.L)
	c := (1.0 - math.Abs(2.0*ll-1.0)) * ss
	x := c * (1.0 - math.Abs(math.Mod(hh/60.0, 2.0)-1.0))
	m := ll - c*0.5

	var r, g, b float64
	switch {
	case hh < 60:
		r, g, b = c, x, 0
	case hh < 120:
		r, g, b = x, c, 0
	case hh < 180:
		r, g, b = 0, c, x
	case hh < 240:
		r, g, b = 0, x, c
	case hh < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return LinearRgba{R: float32(r + m), G: float32(g + m), B: float32(b + m), A: h.A}
}

// ─── HSV conversions ─────────────────────────────────────────────────────────

// ToHsv converts a linear-light RGBA color to HSV.
func (l LinearRgba) ToHsv() HsvaColor {
	r, g, b := float64(l.R), float64(l.G), float64(l.B)
	cmax := max64(r, max64(g, b))
	cmin := min64(r, min64(g, b))
	delta := cmax - cmin

	var h, s float64
	if cmax != 0 {
		s = delta / cmax
	}
	if delta != 0 {
		switch cmax {
		case r:
			h = math.Mod((g-b)/delta, 6.0)
		case g:
			h = (b-r)/delta + 2.0
		default:
			h = (r-g)/delta + 4.0
		}
		h *= 60.0
		if h < 0 {
			h += 360.0
		}
	}
	return HsvaColor{H: float32(h), S: float32(s), V: float32(cmax), A: l.A}
}

// ToLinear converts an HSV color to linear-light RGBA.
func (h HsvaColor) ToLinear() LinearRgba {
	hh, ss, vv := float64(h.H), float64(h.S), float64(h.V)
	c := vv * ss
	x := c * (1.0 - math.Abs(math.Mod(hh/60.0, 2.0)-1.0))
	m := vv - c

	var r, g, b float64
	switch {
	case hh < 60:
		r, g, b = c, x, 0
	case hh < 120:
		r, g, b = x, c, 0
	case hh < 180:
		r, g, b = 0, c, x
	case hh < 240:
		r, g, b = 0, x, c
	case hh < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return LinearRgba{R: float32(r + m), G: float32(g + m), B: float32(b + m), A: h.A}
}

// ─── Color utilities ─────────────────────────────────────────────────────────

// LerpColor linearly interpolates between two linear-light RGBA colors.
func LerpColor(a, b LinearRgba, t float32) LinearRgba {
	return LinearRgba{
		R: a.R + (b.R-a.R)*t,
		G: a.G + (b.G-a.G)*t,
		B: a.B + (b.B-a.B)*t,
		A: a.A + (b.A-a.A)*t,
	}
}

// PremultiplyAlpha returns the color with RGB pre-multiplied by alpha.
func (l LinearRgba) PremultiplyAlpha() LinearRgba {
	return LinearRgba{R: l.R * l.A, G: l.G * l.A, B: l.B * l.A, A: l.A}
}

// ─── internal float64 helpers ─────────────────────────────────────────────────

func max64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func min64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
