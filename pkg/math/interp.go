package math

// TransformInterpolator blends between two Affine3 transforms using separate
// lerp (translation/scale) and slerp (rotation) so the path through SE(3) is
// geometrically smooth regardless of angular distance.
type TransformInterpolator struct {
	From Affine3
	To   Affine3
}

// Eval returns the interpolated transform at t ∈ [0, 1].
// t=0 returns From; t=1 returns To.
func (ti TransformInterpolator) Eval(t float32) Affine3 {
	return Affine3{
		Translation: lerpVec3(ti.From.Translation, ti.To.Translation, t),
		Rotation:    ti.From.Rotation.Slerp(ti.To.Rotation, t),
		Scale:       lerpVec3(ti.From.Scale, ti.To.Scale, t),
	}
}

// EvalAffine3A returns the interpolated transform as an Affine3A (column-matrix
// form), suitable for direct upload to the hierarchy/global-transform pipeline.
func (ti TransformInterpolator) EvalAffine3A(t float32) Affine3A {
	a := ti.Eval(t)
	return FromTRS(a.Translation, a.Rotation, a.Scale)
}

// lerpVec3 linearly interpolates between two Vec3 values.
func lerpVec3(a, b Vec3, t float32) Vec3 {
	return Vec3{
		X: a.X + (b.X-a.X)*t,
		Y: a.Y + (b.Y-a.Y)*t,
		Z: a.Z + (b.Z-a.Z)*t,
	}
}
