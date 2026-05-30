package diag

import "math"

// pi as float32 for gizmo geometry generation.
const pi = float32(math.Pi)

// cos32 / sin32 / abs32 are float32 wrappers over the stdlib math functions,
// used by the procedural gizmo shapes.
func cos32(x float32) float32 { return float32(math.Cos(float64(x))) }
func sin32(x float32) float32 { return float32(math.Sin(float64(x))) }
func abs32(x float32) float32 { return float32(math.Abs(float64(x))) }
