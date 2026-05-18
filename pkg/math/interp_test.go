package math

import "testing"

func TestTransformInterpolatorEndpoints(t *testing.T) {
	from := Affine3{
		Translation: Vec3{X: 0, Y: 0, Z: 0},
		Rotation:    QuatIdentity(),
		Scale:       Vec3{X: 1, Y: 1, Z: 1},
	}
	to := Affine3{
		Translation: Vec3{X: 10, Y: 0, Z: 0},
		Rotation:    QuatFromAxisAngle(Vec3{X: 0, Y: 1, Z: 0}, pi32/2),
		Scale:       Vec3{X: 2, Y: 2, Z: 2},
	}
	ti := TransformInterpolator{From: from, To: to}

	r0 := ti.Eval(0)
	if abs32(r0.Translation.X) > 1e-5 || abs32(r0.Scale.X-1) > 1e-5 {
		t.Errorf("Eval(0) must equal From: got %+v", r0)
	}
	r1 := ti.Eval(1)
	if abs32(r1.Translation.X-10) > 1e-5 || abs32(r1.Scale.X-2) > 1e-5 {
		t.Errorf("Eval(1) must equal To: got %+v", r1)
	}
}

func TestTransformInterpolatorMidpoint(t *testing.T) {
	from := Affine3Identity()
	to := Affine3{
		Translation: Vec3{X: 4, Y: 0, Z: 0},
		Rotation:    QuatIdentity(),
		Scale:       Vec3{X: 3, Y: 3, Z: 3},
	}
	ti := TransformInterpolator{From: from, To: to}
	mid := ti.Eval(0.5)
	if abs32(mid.Translation.X-2) > 1e-5 {
		t.Errorf("midpoint translation: got %v, want 2", mid.Translation.X)
	}
	if abs32(mid.Scale.X-2) > 1e-5 {
		t.Errorf("midpoint scale: got %v, want 2", mid.Scale.X)
	}
}

func TestTransformInterpolatorAffine3A(t *testing.T) {
	ti := TransformInterpolator{From: Affine3Identity(), To: Affine3Identity()}
	a := ti.EvalAffine3A(0.5)
	// Identity → identity: the output must be identity.
	id := Affine3AIdentity()
	if abs32(a.Translation.X-id.Translation.X) > 1e-5 ||
		abs32(a.XAxis.X-id.XAxis.X) > 1e-5 {
		t.Errorf("EvalAffine3A identity: got %+v", a)
	}
}

func BenchmarkTransformInterpolator(b *testing.B) {
	ti := TransformInterpolator{
		From: Affine3Identity(),
		To: Affine3{
			Translation: Vec3{X: 1, Y: 2, Z: 3},
			Rotation:    QuatFromAxisAngle(Vec3{X: 0, Y: 1, Z: 0}, pi32/4),
			Scale:       Vec3{X: 2, Y: 2, Z: 2},
		},
	}
	for b.Loop() {
		_ = ti.Eval(0.5)
	}
}
