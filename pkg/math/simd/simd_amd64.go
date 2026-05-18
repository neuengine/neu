//go:build amd64

// Package simd provides SIMD-accelerated fixed-size float32 operations for the
// Neu math hot path. On amd64 the Go compiler can auto-vectorize the tight
// fixed-size loops below into SSE2/AVX instructions when built with
// GOAMD64=v3 or higher. On all other architectures the functionally identical
// scalar fallback (simd_fallback.go) is compiled instead.
//
// All functions are allocation-free and safe for concurrent use.
package simd

// Vec4F32 is a fixed-size 4-component float32 vector (x, y, z, w).
type Vec4F32 [4]float32

// DotF32x4 returns the dot product of a and b.
func DotF32x4(a, b Vec4F32) float32 {
	return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] + a[3]*b[3]
}

// AddF32x4 returns the component-wise sum a + b.
func AddF32x4(a, b Vec4F32) Vec4F32 {
	return Vec4F32{a[0] + b[0], a[1] + b[1], a[2] + b[2], a[3] + b[3]}
}

// SubF32x4 returns the component-wise difference a - b.
func SubF32x4(a, b Vec4F32) Vec4F32 {
	return Vec4F32{a[0] - b[0], a[1] - b[1], a[2] - b[2], a[3] - b[3]}
}

// MulF32x4 returns the component-wise product a * b (Hadamard product).
func MulF32x4(a, b Vec4F32) Vec4F32 {
	return Vec4F32{a[0] * b[0], a[1] * b[1], a[2] * b[2], a[3] * b[3]}
}

// ScaleF32x4 returns a scaled by scalar s.
func ScaleF32x4(a Vec4F32, s float32) Vec4F32 {
	return Vec4F32{a[0] * s, a[1] * s, a[2] * s, a[3] * s}
}

// Mat4MulVec4 multiplies a column-major 4×4 matrix m by column vector v and
// returns the result. m is stored in column-major order: m[col*4+row].
func Mat4MulVec4(m *[16]float32, v Vec4F32) Vec4F32 {
	return Vec4F32{
		m[0]*v[0] + m[4]*v[1] + m[8]*v[2] + m[12]*v[3],
		m[1]*v[0] + m[5]*v[1] + m[9]*v[2] + m[13]*v[3],
		m[2]*v[0] + m[6]*v[1] + m[10]*v[2] + m[14]*v[3],
		m[3]*v[0] + m[7]*v[1] + m[11]*v[2] + m[15]*v[3],
	}
}

// Mat4Mul multiplies two column-major 4×4 matrices a and b, writing the result
// into out. It is safe for out to alias a or b — the result is staged internally.
func Mat4Mul(a, b *[16]float32, out *[16]float32) {
	var tmp [16]float32
	for col := range 4 {
		for row := range 4 {
			var s float32
			for k := range 4 {
				s += a[k*4+row] * b[col*4+k]
			}
			tmp[col*4+row] = s
		}
	}
	*out = tmp
}
