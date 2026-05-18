package math

import (
	"testing"
)

func approxMat4(a, b Mat4) bool {
	for c := range 4 {
		if !approxEq(a[c].X, b[c].X) || !approxEq(a[c].Y, b[c].Y) ||
			!approxEq(a[c].Z, b[c].Z) || !approxEq(a[c].W, b[c].W) {
			return false
		}
	}
	return true
}

func TestMat4IdentityMul(t *testing.T) {
	m := Mat4{
		{1, 2, 3, 4}, {5, 6, 7, 8}, {9, 10, 11, 12}, {13, 14, 15, 16},
	}
	if !approxMat4(m.MulMat(Mat4Identity()), m) {
		t.Fatal("M * I must equal M")
	}
	if !approxMat4(Mat4Identity().MulMat(m), m) {
		t.Fatal("I * M must equal M")
	}
}

func TestMat4Transpose(t *testing.T) {
	m := Mat4{
		{1, 2, 3, 4}, {5, 6, 7, 8}, {9, 10, 11, 12}, {13, 14, 15, 16},
	}
	tt := m.Transpose().Transpose()
	if !approxMat4(tt, m) {
		t.Fatal("double-transpose must return original matrix")
	}
}

func TestMat4TransposeSwapsOffDiagonal(t *testing.T) {
	m := Mat4Identity()
	m[1].X = 7 // row 0, col 1
	tr := m.Transpose()
	if !approxEq(tr[0].Y, 7) { // row 1, col 0
		t.Fatalf("transpose swaps [0][1] to [1][0]: got %v", tr[0].Y)
	}
}

func TestMat4Inverse(t *testing.T) {
	// Use a simple rotation+scale matrix that has a known inverse.
	m := Mat4FromScale(Vec3{2, 3, 4})
	inv, ok := m.Inverse()
	if !ok {
		t.Fatal("Inverse of non-singular matrix must succeed")
	}
	product := m.MulMat(inv)
	if !approxMat4(product, Mat4Identity()) {
		t.Fatalf("M * inv(M) must be identity; got %v", product)
	}
}

func TestMat4InverseSingular(t *testing.T) {
	var m Mat4
	_, ok := m.Inverse()
	if ok {
		t.Fatal("zero matrix must be singular")
	}
}

func TestMat4Determinant(t *testing.T) {
	if !approxEq(Mat4Identity().Determinant(), 1) {
		t.Fatal("det(I) must be 1")
	}
	m := Mat4FromScale(Vec3{2, 3, 4})
	if !approxEq(m.Determinant(), 24) { // 2*3*4
		t.Fatalf("det(scale(2,3,4)): got %v, want 24", m.Determinant())
	}
}

func TestMat4TransformPoint(t *testing.T) {
	m := Mat4FromTranslation(Vec3{1, 2, 3})
	p := Vec3{0, 0, 0}
	got := m.TransformPoint(p)
	want := Vec3{1, 2, 3}
	if !approxVec3(got, want) {
		t.Fatalf("TransformPoint: got %v, want %v", got, want)
	}
}

func TestMat4TransformVector(t *testing.T) {
	// TransformVector ignores translation.
	m := Mat4FromTranslation(Vec3{10, 10, 10})
	v := Vec3{1, 0, 0}
	got := m.TransformVector(v)
	if !approxVec3(got, v) {
		t.Fatalf("TransformVector must ignore translation: got %v", got)
	}
}

func TestMat4MulVec4(t *testing.T) {
	m := Mat4Identity()
	v := Vec4{1, 2, 3, 4}
	got := m.MulVec4(v)
	if got != v {
		t.Fatalf("I * v must equal v: got %v", got)
	}
}

func TestMat4ColMajorArray(t *testing.T) {
	m := Mat4Identity()
	a := m.ColMajorArray()
	// Identity: diagonal elements are 1, rest 0.
	expected := [16]float32{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
	if a != expected {
		t.Fatalf("ColMajorArray(I): got %v, want %v", a, expected)
	}
}

func TestMat3Inverse(t *testing.T) {
	m := Mat3Identity()
	m[0].X = 2
	inv, ok := m.Inverse()
	if !ok {
		t.Fatal("Inverse of non-singular Mat3 must succeed")
	}
	product := m.MulMat(inv)
	want := Mat3Identity()
	for c := range 3 {
		if !approxEq(product[c].X, want[c].X) ||
			!approxEq(product[c].Y, want[c].Y) ||
			!approxEq(product[c].Z, want[c].Z) {
			t.Fatalf("Mat3 M*inv(M) must be I at col %d: got %v", c, product[c])
		}
	}
}

// ─── Benchmarks ──────────────────────────────────────────────────────────────

func BenchmarkMat4MulMat(b *testing.B) {
	m := Mat4Identity()
	o := Mat4FromScale(Vec3{2, 3, 4})
	b.ReportAllocs()
	for b.Loop() {
		m = m.MulMat(o)
	}
	_ = m
}

func BenchmarkMat4Inverse(b *testing.B) {
	m := Mat4FromScale(Vec3{2, 3, 4})
	b.ReportAllocs()
	for b.Loop() {
		r, _ := m.Inverse()
		_ = r
	}
}
