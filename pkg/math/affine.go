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

// Affine3 is an alias kept for spec compatibility (SIMD alignment deferred to Phase 3).
type Affine3 = Affine3A
