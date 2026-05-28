// Package animation provides the internal animation evaluation engine:
// curve sampling, blend trees, skeletal transform write-back, and the ECS systems.
//
// Evaluation is deterministic (INV-1): same (clip, time, weights) → same output.
// No PRNG, no wall-clock, no map iteration order in any output path.
package animation

import (
	"sort"

	pkganimation "github.com/neuengine/neu/pkg/animation"
	pkgmath "github.com/neuengine/neu/pkg/math"
)

// sampleKeyframes evaluates a single Keyframes track at time t.
// Returns the interpolated value as a []float32.
// t is in [0, clip.Duration]; clamping is applied by the caller.
func sampleKeyframes(k pkganimation.Keyframes, t float32) []float32 {
	if len(k.Times) == 0 {
		return nil
	}
	// Find surrounding keyframe pair via binary search.
	n := len(k.Times)
	i := max(sort.Search(n, func(i int) bool { return k.Times[i] > t })-1, 0)

	switch k.Interp {
	case pkganimation.InterpolationStep:
		return strideSlice(k.Values, i, strideOf(k.Values, n))

	case pkganimation.InterpolationLinear:
		if i >= n-1 {
			return strideSlice(k.Values, n-1, strideOf(k.Values, n))
		}
		stride := strideOf(k.Values, n)
		t0, t1 := k.Times[i], k.Times[i+1]
		alpha := (t - t0) / (t1 - t0)
		a := strideSlice(k.Values, i, stride)
		b := strideSlice(k.Values, i+1, stride)
		return lerpSlice(a, b, alpha)

	case pkganimation.InterpolationCubicSpline:
		if i >= n-1 {
			stride := strideOf(k.Values, n)
			return strideSlice(k.Values, n-1, stride)
		}
		stride := strideOf(k.Values, n)
		t0, t1 := k.Times[i], k.Times[i+1]
		dt := t1 - t0
		if dt == 0 {
			return strideSlice(k.Values, i, stride)
		}
		tc := (t - t0) / dt
		// Hermite cubic: p(tc) = h00*v0 + h10*dt*m0 + h01*v1 + h11*dt*m1
		h00 := (1 + 2*tc) * (1 - tc) * (1 - tc)
		h10 := tc * (1 - tc) * (1 - tc)
		h01 := tc * tc * (3 - 2*tc)
		h11 := tc * tc * (tc - 1)
		tStride := stride * 2 // tangents: in + out per keyframe
		v0 := strideSlice(k.Values, i, stride)
		v1 := strideSlice(k.Values, i+1, stride)
		m0 := tangentSlice(k.Tangents, i, 1, tStride)   // out-tangent of i
		m1 := tangentSlice(k.Tangents, i+1, 0, tStride) // in-tangent of i+1
		result := make([]float32, stride)
		for x := range stride {
			result[x] = h00*v0[x] + h10*dt*m0[x] + h01*v1[x] + h11*dt*m1[x]
		}
		return result

	default:
		return strideSlice(k.Values, i, strideOf(k.Values, n))
	}
}

// strideOf returns the number of float32 values per keyframe.
func strideOf(values []float32, n int) int {
	if n == 0 {
		return 0
	}
	return len(values) / n
}

// strideSlice returns a copy of values[i*stride : (i+1)*stride].
func strideSlice(values []float32, i, stride int) []float32 {
	if stride == 0 || i*stride+stride > len(values) {
		return nil
	}
	out := make([]float32, stride)
	copy(out, values[i*stride:(i+1)*stride])
	return out
}

// tangentSlice extracts an in (which=0) or out (which=1) tangent.
// CubicSpline tangents layout: [in0, out0, in1, out1, ...], each 'stride/2' wide.
func tangentSlice(tangents []float32, keyIndex, which, tStride int) []float32 {
	half := tStride / 2
	offset := keyIndex*tStride + which*half
	if offset+half > len(tangents) {
		return make([]float32, half) // zero tangent fallback
	}
	out := make([]float32, half)
	copy(out, tangents[offset:offset+half])
	return out
}

// lerpSlice linearly interpolates two float32 slices of equal length.
func lerpSlice(a, b []float32, t float32) []float32 {
	if len(a) != len(b) {
		return a
	}
	out := make([]float32, len(a))
	for i := range a {
		out[i] = a[i] + (b[i]-a[i])*t
	}
	return out
}

// SampleCurve is the exported entry point for sampling a single Keyframes track
// at time t. Used by examples and by higher-level animation systems.
func SampleCurve(k pkganimation.Keyframes, t float32) []float32 {
	return sampleKeyframes(k, t)
}

// slerpQuat performs spherical linear interpolation between two quaternions
// stored as [x, y, z, w] float32 slices.
func slerpQuat(a, b []float32, t float32) []float32 {
	if len(a) != 4 || len(b) != 4 {
		return a
	}
	qa := pkgmath.Quat{X: a[0], Y: a[1], Z: a[2], W: a[3]}
	qb := pkgmath.Quat{X: b[0], Y: b[1], Z: b[2], W: b[3]}
	qr := qa.Slerp(qb, t)
	return []float32{qr.X, qr.Y, qr.Z, qr.W}
}
