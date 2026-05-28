package postprocess

import (
	"github.com/neuengine/neu/pkg/asset"
	pkgmath "github.com/neuengine/neu/pkg/math"
	renderimage "github.com/neuengine/neu/pkg/render/image"
)

// Tonemapper selects the HDR→LDR tone curve operator.
type Tonemapper uint8

const (
	TonemapReinhard          Tonemapper = iota // per-channel: x/(1+x)
	TonemapReinhardLuminance                   // luminance-based Reinhard
	TonemapACES                                // Narkowicz 2015 ACES approximation
	TonemapAgX                                 // AgX default contrast (simplified)
	TonemapTonyMcMapface                       // 3-D LUT; falls back to Reinhard when LUT unloaded
)

// TonemappingConfig configures the tonemapping pass (SlotTonemapping, INV-1).
type TonemappingConfig struct {
	LUT      asset.Handle[renderimage.Image]
	Exposure float32
	Operator Tonemapper
}

// Apply maps a linear-HDR RGB triplet to LDR [0,1] using this operator.
// The input is multiplied by exposure before curve application.
//
// Reference values (unit tests ≤1e-4):
//   - Reinhard(1,1,1, exp=1) → (0.5, 0.5, 0.5)
//   - ACES(1,1,1, exp=1)     → ≈(0.6733, 0.6733, 0.6733)
func (t Tonemapper) Apply(hdr pkgmath.Vec3, exposure float32) pkgmath.Vec3 {
	x := pkgmath.Vec3{X: hdr.X * exposure, Y: hdr.Y * exposure, Z: hdr.Z * exposure}
	switch t {
	case TonemapReinhardLuminance:
		return reinhardLuminance(x)
	case TonemapACES:
		return acesNarkowicz(x)
	case TonemapAgX:
		return agxSimplified(x)
	default: // TonemapReinhard, TonemapTonyMcMapface (no LUT at this level)
		return reinhardPerChannel(x)
	}
}

// reinhardPerChannel applies Reinhard per colour channel: c/(1+c).
func reinhardPerChannel(x pkgmath.Vec3) pkgmath.Vec3 {
	return pkgmath.Vec3{
		X: x.X / (1 + x.X),
		Y: x.Y / (1 + x.Y),
		Z: x.Z / (1 + x.Z),
	}
}

// reinhardLuminance applies Reinhard on luminance (ITU-R BT.709) to
// preserve hue while tonemapping, then rescales RGB.
func reinhardLuminance(x pkgmath.Vec3) pkgmath.Vec3 {
	lum := 0.2126*x.X + 0.7152*x.Y + 0.0722*x.Z
	if lum <= 0 {
		return pkgmath.Vec3{}
	}
	lumOut := lum / (1 + lum)
	scale := lumOut / lum
	return pkgmath.Vec3{X: x.X * scale, Y: x.Y * scale, Z: x.Z * scale}
}

// acesNarkowicz implements the Narkowicz 2015 ACES film tone-map approximation.
//
//	f(x) = (x*(2.51x+0.03)) / (x*(2.43x+0.59)+0.14), x = input*0.6
func acesNarkowicz(x pkgmath.Vec3) pkgmath.Vec3 {
	apply := func(v float32) float32 {
		v *= 0.6
		const (
			a float32 = 2.51
			b float32 = 0.03
			c float32 = 2.43
			d float32 = 0.59
			e float32 = 0.14
		)
		r := (v * (a*v + b)) / (v*(c*v+d) + e)
		return max(float32(0), min(float32(1), r))
	}
	return pkgmath.Vec3{X: apply(x.X), Y: apply(x.Y), Z: apply(x.Z)}
}

// agxSimplified applies a simplified AgX-style s-curve (smooth-step) after
// clamping input to [0,1]. Full AgX requires a 3D LUT; this serves as the
// Bootstrap scalar approximation.
func agxSimplified(x pkgmath.Vec3) pkgmath.Vec3 {
	curve := func(v float32) float32 {
		v = max(float32(0), min(float32(1), v))
		return v * v * (3 - 2*v) // smoothstep
	}
	return pkgmath.Vec3{X: curve(x.X), Y: curve(x.Y), Z: curve(x.Z)}
}
