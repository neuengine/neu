package math

import stdmath "math"

// internal.go — minimal scalar helpers. Phase 3 replaces with SIMD intrinsics.

func sqrt32(x float32) float32 {
	if x <= 0 {
		return 0
	}
	return float32(stdmath.Sqrt(float64(x)))
}

func sin32(x float32) float32 { return float32(stdmath.Sin(float64(x))) }
func cos32(x float32) float32 { return float32(stdmath.Cos(float64(x))) }
