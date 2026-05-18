package math

import stdmath "math"

const (
	pi32  = float32(stdmath.Pi)
	eps32 = float32(1e-6)
)

func sqrt32(x float32) float32 {
	if x <= 0 {
		return 0
	}
	return float32(stdmath.Sqrt(float64(x)))
}

func sin32(x float32) float32      { return float32(stdmath.Sin(float64(x))) }
func cos32(x float32) float32      { return float32(stdmath.Cos(float64(x))) }
func asin32(x float32) float32     { return float32(stdmath.Asin(float64(x))) }
func acos32(x float32) float32     { return float32(stdmath.Acos(float64(x))) }
func atan232(y, x float32) float32 { return float32(stdmath.Atan2(float64(y), float64(x))) }
func abs32(x float32) float32      { return float32(stdmath.Abs(float64(x))) }

func clamp32(x, lo, hi float32) float32 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
