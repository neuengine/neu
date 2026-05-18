package simd_test

import (
	"math/rand"
	"testing"

	"github.com/neuengine/neu/pkg/math/simd"
)

// ─── scalar reference implementations ────────────────────────────────────────

func scalarDot(a, b simd.Vec4F32) float32 {
	return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] + a[3]*b[3]
}

func scalarAdd(a, b simd.Vec4F32) simd.Vec4F32 {
	return simd.Vec4F32{a[0] + b[0], a[1] + b[1], a[2] + b[2], a[3] + b[3]}
}

func scalarSub(a, b simd.Vec4F32) simd.Vec4F32 {
	return simd.Vec4F32{a[0] - b[0], a[1] - b[1], a[2] - b[2], a[3] - b[3]}
}

func scalarMul(a, b simd.Vec4F32) simd.Vec4F32 {
	return simd.Vec4F32{a[0] * b[0], a[1] * b[1], a[2] * b[2], a[3] * b[3]}
}

func scalarScale(a simd.Vec4F32, s float32) simd.Vec4F32 {
	return simd.Vec4F32{a[0] * s, a[1] * s, a[2] * s, a[3] * s}
}

func scalarMat4MulVec4(m *[16]float32, v simd.Vec4F32) simd.Vec4F32 {
	return simd.Vec4F32{
		m[0]*v[0] + m[4]*v[1] + m[8]*v[2] + m[12]*v[3],
		m[1]*v[0] + m[5]*v[1] + m[9]*v[2] + m[13]*v[3],
		m[2]*v[0] + m[6]*v[1] + m[10]*v[2] + m[14]*v[3],
		m[3]*v[0] + m[7]*v[1] + m[11]*v[2] + m[15]*v[3],
	}
}

func scalarMat4Mul(a, b *[16]float32) [16]float32 {
	var out [16]float32
	for col := range 4 {
		for row := range 4 {
			var s float32
			for k := range 4 {
				s += a[k*4+row] * b[col*4+k]
			}
			out[col*4+row] = s
		}
	}
	return out
}

// ─── fuzz-seed corpus ─────────────────────────────────────────────────────────

// seedCorpus generates N pairs of Vec4F32 from a deterministic source.
func seedCorpus(n int) [][2]simd.Vec4F32 {
	rng := rand.New(rand.NewSource(0xdead_beef_1234))
	corpus := make([][2]simd.Vec4F32, n)
	for i := range corpus {
		for j := range 4 {
			corpus[i][0][j] = float32(rng.Float64()*200 - 100)
			corpus[i][1][j] = float32(rng.Float64()*200 - 100)
		}
	}
	return corpus
}

func seedMats(n int) [][2][16]float32 {
	rng := rand.New(rand.NewSource(0xcafe_babe_5678))
	mats := make([][2][16]float32, n)
	for i := range mats {
		for j := range 16 {
			mats[i][0][j] = float32(rng.Float64()*10 - 5)
			mats[i][1][j] = float32(rng.Float64()*10 - 5)
		}
	}
	return mats
}

// ─── TestSIMDParity: bit-for-bit equality with scalar reference ───────────────

func TestSIMDParity(t *testing.T) {
	corpus := seedCorpus(256)
	for i, pair := range corpus {
		a, b := pair[0], pair[1]

		if got, want := simd.DotF32x4(a, b), scalarDot(a, b); got != want {
			t.Errorf("[%d] DotF32x4: got %v, want %v", i, got, want)
		}
		if got, want := simd.AddF32x4(a, b), scalarAdd(a, b); got != want {
			t.Errorf("[%d] AddF32x4: got %v, want %v", i, got, want)
		}
		if got, want := simd.SubF32x4(a, b), scalarSub(a, b); got != want {
			t.Errorf("[%d] SubF32x4: got %v, want %v", i, got, want)
		}
		if got, want := simd.MulF32x4(a, b), scalarMul(a, b); got != want {
			t.Errorf("[%d] MulF32x4: got %v, want %v", i, got, want)
		}
		s := a[0]
		if got, want := simd.ScaleF32x4(a, s), scalarScale(a, s); got != want {
			t.Errorf("[%d] ScaleF32x4: got %v, want %v", i, got, want)
		}
	}

	mats := seedMats(64)
	for i, pair := range mats {
		m := pair[0]
		v := simd.Vec4F32{pair[1][0], pair[1][1], pair[1][2], pair[1][3]}
		if got, want := simd.Mat4MulVec4(&m, v), scalarMat4MulVec4(&m, v); got != want {
			t.Errorf("[%d] Mat4MulVec4: got %v, want %v", i, got, want)
		}

		b := pair[1]
		var out [16]float32
		simd.Mat4Mul(&pair[0], &b, &out)
		want := scalarMat4Mul(&pair[0], &b)
		if out != want {
			t.Errorf("[%d] Mat4Mul: got %v, want %v", i, out, want)
		}
	}
}

// TestSIMDMat4MulAlias verifies that out may alias one of the inputs.
func TestSIMDMat4MulAlias(t *testing.T) {
	var a [16]float32
	for i := range 16 {
		a[i] = float32(i + 1)
	}
	b := a
	want := scalarMat4Mul(&a, &b)

	simd.Mat4Mul(&a, &b, &a) // out aliases a
	if a != want {
		t.Errorf("Mat4Mul aliased output mismatch")
	}
}

// ─── Benchmarks ──────────────────────────────────────────────────────────────

func BenchmarkDotF32x4(b *testing.B) {
	a := simd.Vec4F32{1, 2, 3, 4}
	c := simd.Vec4F32{5, 6, 7, 8}
	for b.Loop() {
		_ = simd.DotF32x4(a, c)
	}
}

func BenchmarkMat4MulVec4(b *testing.B) {
	m := [16]float32{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
	v := simd.Vec4F32{1, 2, 3, 1}
	for b.Loop() {
		_ = simd.Mat4MulVec4(&m, v)
	}
}

func BenchmarkMat4Mul(b *testing.B) {
	var a, c [16]float32
	for i := range a {
		a[i] = float32(i)
	}
	c = a
	var out [16]float32
	for b.Loop() {
		simd.Mat4Mul(&a, &c, &out)
	}
}
