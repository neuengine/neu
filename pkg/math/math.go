// Package math provides spatial math primitives for the Neu engine.
// This is a Bootstrap stub — full SIMD-optimized implementation lands in Phase 3 (T-3D).
// All types use float32 scalars.
package math

// Vec2 is a 2-component float32 vector.
type Vec2 struct {
	X, Y float32
}

// Vec2Zero returns the zero vector.
func Vec2Zero() Vec2 { return Vec2{} }

// Vec2New constructs a Vec2.
func Vec2New(x, y float32) Vec2 { return Vec2{X: x, Y: y} }

// Add returns v + b.
func (v Vec2) Add(b Vec2) Vec2 { return Vec2{v.X + b.X, v.Y + b.Y} }

// Sub returns v - b.
func (v Vec2) Sub(b Vec2) Vec2 { return Vec2{v.X - b.X, v.Y - b.Y} }

// Scale returns v * s.
func (v Vec2) Scale(s float32) Vec2 { return Vec2{v.X * s, v.Y * s} }

// Vec3 is a 3-component float32 vector.
type Vec3 struct {
	X, Y, Z float32
}

// Vec3Zero returns the zero vector.
func Vec3Zero() Vec3 { return Vec3{} }

// Vec3New constructs a Vec3.
func Vec3New(x, y, z float32) Vec3 { return Vec3{X: x, Y: y, Z: z} }

// Add returns v + b.
func (v Vec3) Add(b Vec3) Vec3 { return Vec3{v.X + b.X, v.Y + b.Y, v.Z + b.Z} }

// Sub returns v - b.
func (v Vec3) Sub(b Vec3) Vec3 { return Vec3{v.X - b.X, v.Y - b.Y, v.Z - b.Z} }

// Scale returns v * s.
func (v Vec3) Scale(s float32) Vec3 { return Vec3{v.X * s, v.Y * s, v.Z * s} }

// Neg returns -v.
func (v Vec3) Neg() Vec3 { return Vec3{-v.X, -v.Y, -v.Z} }

// Dot returns the dot product of v and b.
func (v Vec3) Dot(b Vec3) float32 { return v.X*b.X + v.Y*b.Y + v.Z*b.Z }

// Cross returns the cross product v × b.
func (v Vec3) Cross(b Vec3) Vec3 {
	return Vec3{
		X: v.Y*b.Z - v.Z*b.Y,
		Y: v.Z*b.X - v.X*b.Z,
		Z: v.X*b.Y - v.Y*b.X,
	}
}

// LenSq returns the squared length of v.
func (v Vec3) LenSq() float32 { return v.X*v.X + v.Y*v.Y + v.Z*v.Z }

// Normalize returns v scaled to unit length. Returns Vec3Zero if v is the zero vector.
func (v Vec3) Normalize() Vec3 {
	sq := v.LenSq()
	if sq == 0 {
		return Vec3{}
	}
	inv := float32(1) / sqrt32(sq)
	return Vec3{v.X * inv, v.Y * inv, v.Z * inv}
}

// Vec4 is a 4-component float32 vector.
type Vec4 struct {
	X, Y, Z, W float32
}

// LinearRgba is a linear-space RGBA color with float32 components [0, 1].
type LinearRgba struct {
	R, G, B, A float32
}

// Ray3D is an infinite ray in 3D space defined by an origin and a unit direction.
type Ray3D struct {
	Origin    Vec3
	Direction Vec3
}

// Quat is a unit quaternion in (x, y, z, w) order.
// The zero value is not a valid rotation — use QuatIdentity().
type Quat struct {
	X, Y, Z, W float32
}

// QuatIdentity returns the identity rotation (no rotation).
func QuatIdentity() Quat { return Quat{W: 1} }

// FromAxisAngle returns a quaternion for rotation of angle radians around axis.
// axis must be normalized.
func FromAxisAngle(axis Vec3, angle float32) Quat {
	s := sin32(angle * 0.5)
	c := cos32(angle * 0.5)
	return Quat{X: axis.X * s, Y: axis.Y * s, Z: axis.Z * s, W: c}
}

// Mul returns the product of two quaternions (composition of rotations).
func (q Quat) Mul(r Quat) Quat {
	return Quat{
		X: q.W*r.X + q.X*r.W + q.Y*r.Z - q.Z*r.Y,
		Y: q.W*r.Y - q.X*r.Z + q.Y*r.W + q.Z*r.X,
		Z: q.W*r.Z + q.X*r.Y - q.Y*r.X + q.Z*r.W,
		W: q.W*r.W - q.X*r.X - q.Y*r.Y - q.Z*r.Z,
	}
}

// Normalize returns q scaled to unit length.
func (q Quat) Normalize() Quat {
	sq := q.X*q.X + q.Y*q.Y + q.Z*q.Z + q.W*q.W
	if sq == 0 {
		return QuatIdentity()
	}
	inv := float32(1) / sqrt32(sq)
	return Quat{q.X * inv, q.Y * inv, q.Z * inv, q.W * inv}
}

// QuatFromRotMat3 builds a Quat from a column-major 3×3 rotation matrix.
// col0, col1, col2 are the three column vectors (XAxis, YAxis, ZAxis of Affine3A).
func QuatFromRotMat3(col0, col1, col2 Vec3) Quat {
	return fromRotMat3(col0, col1, col2)
}

// fromRotMat3 builds a Quat from a column-major 3x3 rotation matrix.
// col0, col1, col2 are the three column vectors.
func fromRotMat3(col0, col1, col2 Vec3) Quat {
	// Shepperd method
	m00, m10, m20 := col0.X, col0.Y, col0.Z
	m01, m11, m21 := col1.X, col1.Y, col1.Z
	m02, m12, m22 := col2.X, col2.Y, col2.Z
	trace := m00 + m11 + m22
	var q Quat
	switch {
	case trace > 0:
		s := sqrt32(trace+1) * 2 // s = 4w
		q.W = 0.25 * s
		q.X = (m21 - m12) / s
		q.Y = (m02 - m20) / s
		q.Z = (m10 - m01) / s
	case m00 > m11 && m00 > m22:
		s := sqrt32(1+m00-m11-m22) * 2 // s = 4x
		q.W = (m21 - m12) / s
		q.X = 0.25 * s
		q.Y = (m01 + m10) / s
		q.Z = (m02 + m20) / s
	case m11 > m22:
		s := sqrt32(1+m11-m00-m22) * 2 // s = 4y
		q.W = (m02 - m20) / s
		q.X = (m01 + m10) / s
		q.Y = 0.25 * s
		q.Z = (m12 + m21) / s
	default:
		s := sqrt32(1+m22-m00-m11) * 2 // s = 4z
		q.W = (m10 - m01) / s
		q.X = (m02 + m20) / s
		q.Y = (m12 + m21) / s
		q.Z = 0.25 * s
	}
	return q
}
