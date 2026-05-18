package math

// Affine3A is a 3D affine transform stored as column-major (XAxis, YAxis, ZAxis, Translation).
// The implicit fourth row is [0, 0, 0, 1].
//
// Column semantics:
//   - XAxis: where the local +X unit vector maps in world space (scaled by X-scale)
//   - YAxis: where the local +Y unit vector maps in world space (scaled by Y-scale)
//   - ZAxis: where the local +Z unit vector maps in world space (scaled by Z-scale)
//   - Translation: world-space origin of the transform
type Affine3A struct {
	XAxis       Vec3
	YAxis       Vec3
	ZAxis       Vec3
	Translation Vec3
}

// Affine3AIdentity returns the identity transform.
func Affine3AIdentity() Affine3A {
	return Affine3A{
		XAxis:       Vec3{X: 1},
		YAxis:       Vec3{Y: 1},
		ZAxis:       Vec3{Z: 1},
		Translation: Vec3{},
	}
}

// FromTRS builds an Affine3A from translation t, unit quaternion r, and scale s.
func FromTRS(t Vec3, r Quat, s Vec3) Affine3A {
	x, y, z, w := r.X, r.Y, r.Z, r.W
	xx, yy, zz := x*x, y*y, z*z
	xy, xz, yz := x*y, x*z, y*z
	wx, wy, wz := w*x, w*y, w*z

	return Affine3A{
		XAxis: Vec3{
			X: (1 - 2*(yy+zz)) * s.X,
			Y: 2 * (xy + wz) * s.X,
			Z: 2 * (xz - wy) * s.X,
		},
		YAxis: Vec3{
			X: 2 * (xy - wz) * s.Y,
			Y: (1 - 2*(xx+zz)) * s.Y,
			Z: 2 * (yz + wx) * s.Y,
		},
		ZAxis: Vec3{
			X: 2 * (xz + wy) * s.Z,
			Y: 2 * (yz - wx) * s.Z,
			Z: (1 - 2*(xx+yy)) * s.Z,
		},
		Translation: t,
	}
}

// TransformVec applies the linear (rotation+scale) part of the transform to v.
// Does NOT add translation — use TransformPoint for position vectors.
func (a Affine3A) TransformVec(v Vec3) Vec3 {
	return Vec3{
		X: a.XAxis.X*v.X + a.YAxis.X*v.Y + a.ZAxis.X*v.Z,
		Y: a.XAxis.Y*v.X + a.YAxis.Y*v.Y + a.ZAxis.Y*v.Z,
		Z: a.XAxis.Z*v.X + a.YAxis.Z*v.Y + a.ZAxis.Z*v.Z,
	}
}

// TransformPoint transforms a position by the full affine transform (rotation+scale+translation).
func (a Affine3A) TransformPoint(p Vec3) Vec3 {
	return Vec3{
		X: a.XAxis.X*p.X + a.YAxis.X*p.Y + a.ZAxis.X*p.Z + a.Translation.X,
		Y: a.XAxis.Y*p.X + a.YAxis.Y*p.Y + a.ZAxis.Y*p.Z + a.Translation.Y,
		Z: a.XAxis.Z*p.X + a.YAxis.Z*p.Y + a.ZAxis.Z*p.Z + a.Translation.Z,
	}
}

// Mul returns the composition a * b (apply b first, then a).
func (a Affine3A) Mul(b Affine3A) Affine3A {
	return Affine3A{
		XAxis:       a.TransformVec(b.XAxis),
		YAxis:       a.TransformVec(b.YAxis),
		ZAxis:       a.TransformVec(b.ZAxis),
		Translation: a.TransformPoint(b.Translation),
	}
}

// ExtractTranslation returns the translation component of the transform.
func (a Affine3A) ExtractTranslation() Vec3 { return a.Translation }

// ExtractQuat extracts the rotation quaternion (ignores scale).
// Assumes the axes encode rotation*scale — normalizes each column first.
func (a Affine3A) ExtractQuat() Quat {
	col0 := a.XAxis.Normalize()
	col1 := a.YAxis.Normalize()
	col2 := a.ZAxis.Normalize()
	return fromRotMat3(col0, col1, col2)
}

// LookAt builds an Affine3A for a camera at eye looking toward target, with given up hint.
func LookAt(eye, target, up Vec3) Affine3A {
	forward := target.Sub(eye).Normalize()
	right := forward.Cross(up).Normalize()
	upOrtho := right.Cross(forward)
	// right-handed, -Z forward
	return Affine3A{
		XAxis:       right,
		YAxis:       upOrtho,
		ZAxis:       forward.Neg(),
		Translation: eye,
	}
}

// Affine3 is a 3D affine transform stored as Translation-Rotation-Scale components.
// All methods use value receivers and return new values (L1 INV-1).
//
// Note: Mul and Inverse are exact only when Scale is uniform or axes are independent.
// For non-uniform scale combined with rotation, use AffineInverse and ToMat4/Affine3FromMat4.
type Affine3 struct {
	Translation Vec3
	Rotation    Quat
	Scale       Vec3
}

// Affine3Identity returns the identity transform (no translation, no rotation, scale=1).
func Affine3Identity() Affine3 {
	return Affine3{Rotation: QuatIdentity(), Scale: Vec3{X: 1, Y: 1, Z: 1}}
}

// TransformPoint applies scale → rotate → translate to p.
func (a Affine3) TransformPoint(p Vec3) Vec3 {
	return a.Rotation.MulVec3(a.Scale.MulComp(p)).Add(a.Translation)
}

// TransformVector applies scale → rotate (no translation) to v.
func (a Affine3) TransformVector(v Vec3) Vec3 {
	return a.Rotation.MulVec3(a.Scale.MulComp(v))
}

// Mul composes a after o (apply o first, then a).
// Exact for uniform scale; approximate for non-uniform scale mixed with rotation.
func (a Affine3) Mul(o Affine3) Affine3 {
	return Affine3{
		Translation: a.TransformPoint(o.Translation),
		Rotation:    a.Rotation.Mul(o.Rotation),
		Scale:       a.Scale.MulComp(o.Scale),
	}
}

// ToMat4 builds the equivalent 4×4 column-major matrix.
func (a Affine3) ToMat4() Mat4 {
	m := a.Rotation.rotMat3()
	sx, sy, sz := a.Scale.X, a.Scale.Y, a.Scale.Z
	return Mat4{
		{m[0] * sx, m[3] * sx, m[6] * sx, 0},
		{m[1] * sy, m[4] * sy, m[7] * sy, 0},
		{m[2] * sz, m[5] * sz, m[8] * sz, 0},
		{a.Translation.X, a.Translation.Y, a.Translation.Z, 1},
	}
}

// Affine3FromMat4 decomposes a 4×4 matrix into TRS components.
// Scale is extracted from column lengths; rotation from normalized columns.
func Affine3FromMat4(m Mat4) Affine3 {
	col0 := Vec3{m[0].X, m[0].Y, m[0].Z}
	col1 := Vec3{m[1].X, m[1].Y, m[1].Z}
	col2 := Vec3{m[2].X, m[2].Y, m[2].Z}
	sx := col0.Length()
	sy := col1.Length()
	sz := col2.Length()
	invSx, invSy, invSz := float32(1), float32(1), float32(1)
	if sx > eps32 {
		invSx = 1 / sx
	}
	if sy > eps32 {
		invSy = 1 / sy
	}
	if sz > eps32 {
		invSz = 1 / sz
	}
	return Affine3{
		Translation: Vec3{m[3].X, m[3].Y, m[3].Z},
		Rotation:    QuatFromRotMat3(col0.Mul(invSx), col1.Mul(invSy), col2.Mul(invSz)),
		Scale:       Vec3{sx, sy, sz},
	}
}

// Inverse returns the fast inverse, exact for orthonormal rotation and any scale.
func (a Affine3) Inverse() Affine3 {
	invRot := a.Rotation.Inverse()
	invScale := Vec3{X: 1, Y: 1, Z: 1}
	if abs32(a.Scale.X) > eps32 {
		invScale.X = 1 / a.Scale.X
	}
	if abs32(a.Scale.Y) > eps32 {
		invScale.Y = 1 / a.Scale.Y
	}
	if abs32(a.Scale.Z) > eps32 {
		invScale.Z = 1 / a.Scale.Z
	}
	invTrans := invRot.MulVec3(invScale.MulComp(a.Translation.Neg()))
	return Affine3{Translation: invTrans, Rotation: invRot, Scale: invScale}
}

// AffineInverse returns the general matrix inverse, valid for non-uniform scale and shear.
// Returns the identity transform if the matrix is singular.
func (a Affine3) AffineInverse() Affine3 {
	m, ok := a.ToMat4().Inverse()
	if !ok {
		return Affine3Identity()
	}
	return Affine3FromMat4(m)
}
