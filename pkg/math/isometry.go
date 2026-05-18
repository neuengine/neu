package math

// Dir2 is a unit 2D direction vector. The unexported field makes the normalization
// invariant unbreakable from outside the package (INV-3).
type Dir2 struct{ v Vec2 }

// NewDir2 normalizes v and returns (Dir2, true), or (zero, false) if v is near-zero.
func NewDir2(v Vec2) (Dir2, bool) {
	sq := v.X*v.X + v.Y*v.Y
	if sq < eps32*eps32 {
		return Dir2{}, false
	}
	inv := 1 / sqrt32(sq)
	return Dir2{Vec2{v.X * inv, v.Y * inv}}, true
}

// NewDir2Unchecked wraps v without normalization. Caller guarantees |v| == 1.
func NewDir2Unchecked(v Vec2) Dir2 { return Dir2{v} }

// Vec returns the underlying unit vector.
func (d Dir2) Vec() Vec2 { return d.v }

// Dir3 is a unit 3D direction vector. The unexported field enforces INV-3.
type Dir3 struct{ v Vec3 }

// NewDir3 normalizes v and returns (Dir3, true), or (zero, false) if v is near-zero.
func NewDir3(v Vec3) (Dir3, bool) {
	sq := v.X*v.X + v.Y*v.Y + v.Z*v.Z
	if sq < eps32*eps32 {
		return Dir3{}, false
	}
	inv := 1 / sqrt32(sq)
	return Dir3{Vec3{v.X * inv, v.Y * inv, v.Z * inv}}, true
}

// NewDir3Unchecked wraps v without normalization. Caller guarantees |v| == 1.
func NewDir3Unchecked(v Vec3) Dir3 { return Dir3{v} }

// Vec returns the underlying unit vector.
func (d Dir3) Vec() Vec3 { return d.v }

// Rot2 is a 2D rotation stored as an angle in radians.
type Rot2 struct{ angleRad float32 }

// Rot2FromDegrees converts degrees to a Rot2.
func Rot2FromDegrees(deg float32) Rot2 { return Rot2{deg * pi32 / 180} }

// Rot2FromRadians wraps an angle (radians) as a Rot2.
func Rot2FromRadians(rad float32) Rot2 { return Rot2{rad} }

// Rotate applies this 2D rotation to v.
func (r Rot2) Rotate(v Vec2) Vec2 {
	s, c := sin32(r.angleRad), cos32(r.angleRad)
	return Vec2{c*v.X - s*v.Y, s*v.X + c*v.Y}
}

// Isometry2D is a rigid body transform in 2D: rotation then translation.
// It preserves distances and angles (no scale, no shear).
type Isometry2D struct {
	Rotation    Rot2
	Translation Vec2
}

// TransformPoint applies rotation then translation to p.
func (i Isometry2D) TransformPoint(p Vec2) Vec2 {
	return i.Rotation.Rotate(p).Add(i.Translation)
}

// Inverse returns the inverse isometry.
func (i Isometry2D) Inverse() Isometry2D {
	invRot := Rot2{-i.Rotation.angleRad}
	return Isometry2D{
		Rotation:    invRot,
		Translation: invRot.Rotate(i.Translation.Neg()),
	}
}

// Mul composes i after o (apply o first, then i).
func (i Isometry2D) Mul(o Isometry2D) Isometry2D {
	return Isometry2D{
		Rotation:    Rot2{i.Rotation.angleRad + o.Rotation.angleRad},
		Translation: i.TransformPoint(o.Translation),
	}
}

// Isometry3D is a rigid body transform in 3D: rotation then translation.
// It preserves distances and angles (no scale, no shear).
type Isometry3D struct {
	Rotation    Quat
	Translation Vec3
}

// TransformPoint applies rotation then translation to p.
func (i Isometry3D) TransformPoint(p Vec3) Vec3 {
	return i.Rotation.MulVec3(p).Add(i.Translation)
}

// Inverse returns the inverse isometry.
func (i Isometry3D) Inverse() Isometry3D {
	invRot := i.Rotation.Inverse()
	return Isometry3D{
		Rotation:    invRot,
		Translation: invRot.MulVec3(i.Translation.Neg()),
	}
}

// Mul composes i after o (apply o first, then i).
func (i Isometry3D) Mul(o Isometry3D) Isometry3D {
	return Isometry3D{
		Rotation:    i.Rotation.Mul(o.Rotation),
		Translation: i.TransformPoint(o.Translation),
	}
}
