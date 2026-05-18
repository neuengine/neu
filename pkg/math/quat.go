package math

// EulerOrder specifies the intrinsic rotation sequence for Euler angle decomposition.
// EulerXYZ means rotate around local X by a, then local Y by b, then local Z by c.
type EulerOrder uint8

const (
	EulerXYZ EulerOrder = iota
	EulerXZY
	EulerYXZ
	EulerYZX
	EulerZXY
	EulerZYX
)

// Quat is a unit quaternion (X, Y, Z, W) representing a 3D rotation.
// The zero value is NOT a valid rotation — use QuatIdentity().
// INV-2: all public constructors normalize before returning; Mul and Slerp re-normalize.
type Quat struct{ X, Y, Z, W float32 }

// QuatIdentity returns the identity rotation (no rotation).
func QuatIdentity() Quat { return Quat{W: 1} }

// QuatFromAxisAngle returns a quaternion rotating angleRad radians around axis.
// axis is normalized internally; a zero axis returns the identity.
func QuatFromAxisAngle(axis Vec3, angleRad float32) Quat {
	n := axis.Normalize()
	if n == (Vec3{}) {
		return QuatIdentity()
	}
	s := sin32(angleRad * 0.5)
	c := cos32(angleRad * 0.5)
	return Quat{X: n.X * s, Y: n.Y * s, Z: n.Z * s, W: c}
}

// QuatFromEuler builds a quaternion from intrinsic Euler angles (a, b, c) in the given order.
// Angle a applies to the first axis, b to the second, c to the third.
// Composition is right-to-left: R = R3(c) * R2(b) * R1(a).
func QuatFromEuler(order EulerOrder, a, b, c float32) Quat {
	switch order {
	case EulerXYZ:
		return quatAxisZ(c).mulRaw(quatAxisY(b).mulRaw(quatAxisX(a))).normalize()
	case EulerXZY:
		return quatAxisY(c).mulRaw(quatAxisZ(b).mulRaw(quatAxisX(a))).normalize()
	case EulerYXZ:
		return quatAxisZ(c).mulRaw(quatAxisX(b).mulRaw(quatAxisY(a))).normalize()
	case EulerYZX:
		return quatAxisX(c).mulRaw(quatAxisZ(b).mulRaw(quatAxisY(a))).normalize()
	case EulerZXY:
		return quatAxisY(c).mulRaw(quatAxisX(b).mulRaw(quatAxisZ(a))).normalize()
	case EulerZYX:
		return quatAxisX(c).mulRaw(quatAxisY(b).mulRaw(quatAxisZ(a))).normalize()
	}
	return QuatIdentity()
}

func quatAxisX(a float32) Quat { s, c := sin32(a*0.5), cos32(a*0.5); return Quat{X: s, W: c} }
func quatAxisY(a float32) Quat { s, c := sin32(a*0.5), cos32(a*0.5); return Quat{Y: s, W: c} }
func quatAxisZ(a float32) Quat { s, c := sin32(a*0.5), cos32(a*0.5); return Quat{Z: s, W: c} }

// QuatFromRotationArc returns a quaternion rotating from the direction `from` to `to`.
func QuatFromRotationArc(from, to Vec3) Quat {
	from = from.Normalize()
	to = to.Normalize()
	dot := clamp32(from.Dot(to), -1, 1)
	if dot > 1-eps32 {
		return QuatIdentity()
	}
	if dot < -1+eps32 {
		// Anti-parallel: pick any perpendicular axis.
		perp := Vec3{X: 1}
		if abs32(from.X) > 0.9 {
			perp = Vec3{Y: 1}
		}
		axis := from.Cross(perp).Normalize()
		return QuatFromAxisAngle(axis, pi32)
	}
	axis := from.Cross(to)
	// Half-vector shortcut: skip computing the angle explicitly.
	q := Quat{X: axis.X, Y: axis.Y, Z: axis.Z, W: 1 + dot}
	return q.normalize()
}

// Mul composes two quaternions and re-normalizes the result (INV-2).
func (q Quat) Mul(o Quat) Quat { return q.mulRaw(o).normalize() }

// mulRaw is the raw Hamilton product without re-normalizing (used in composition chains).
func (q Quat) mulRaw(o Quat) Quat {
	return Quat{
		X: q.W*o.X + q.X*o.W + q.Y*o.Z - q.Z*o.Y,
		Y: q.W*o.Y - q.X*o.Z + q.Y*o.W + q.Z*o.X,
		Z: q.W*o.Z + q.X*o.Y - q.Y*o.X + q.Z*o.W,
		W: q.W*o.W - q.X*o.X - q.Y*o.Y - q.Z*o.Z,
	}
}

// MulVec3 rotates v by the quaternion using the optimized double-cross formula.
func (q Quat) MulVec3(v Vec3) Vec3 {
	u := Vec3{q.X, q.Y, q.Z}
	t := u.Cross(v).Mul(2)
	return v.Add(t.Mul(q.W)).Add(u.Cross(t))
}

// Slerp spherically interpolates between q and o by t ∈ [0, 1] and re-normalizes.
func (q Quat) Slerp(o Quat, t float32) Quat {
	dot := q.X*o.X + q.Y*o.Y + q.Z*o.Z + q.W*o.W
	if dot < 0 {
		o = Quat{-o.X, -o.Y, -o.Z, -o.W}
		dot = -dot
	}
	dot = clamp32(dot, -1, 1)
	if dot > 1-eps32 {
		r := Quat{
			X: q.X + t*(o.X-q.X), Y: q.Y + t*(o.Y-q.Y),
			Z: q.Z + t*(o.Z-q.Z), W: q.W + t*(o.W-q.W),
		}
		return r.normalize()
	}
	theta := acos32(dot)
	sinTheta := sin32(theta)
	w1 := sin32((1-t)*theta) / sinTheta
	w2 := sin32(t*theta) / sinTheta
	return Quat{
		X: q.X*w1 + o.X*w2, Y: q.Y*w1 + o.Y*w2,
		Z: q.Z*w1 + o.Z*w2, W: q.W*w1 + o.W*w2,
	}
}

// Inverse returns the inverse of a unit quaternion (equals its conjugate).
func (q Quat) Inverse() Quat { return Quat{-q.X, -q.Y, -q.Z, q.W} }

// AngleBetween returns the angle in radians between two quaternion rotations.
func (q Quat) AngleBetween(o Quat) float32 {
	dot := clamp32(q.X*o.X+q.Y*o.Y+q.Z*o.Z+q.W*o.W, -1, 1)
	if dot < 0 {
		dot = -dot
	}
	return 2 * acos32(dot)
}

// Normalize returns q scaled to unit length.
func (q Quat) Normalize() Quat { return q.normalize() }

func (q Quat) normalize() Quat {
	sq := q.X*q.X + q.Y*q.Y + q.Z*q.Z + q.W*q.W
	if sq < eps32*eps32 {
		return QuatIdentity()
	}
	inv := 1 / sqrt32(sq)
	return Quat{q.X * inv, q.Y * inv, q.Z * inv, q.W * inv}
}

// ToEuler extracts intrinsic Euler angles (a, b, c) for the given order.
// Singularity (gimbal lock) when |sin(b)| ≈ 1: only a±c is determined; c is set to 0.
func (q Quat) ToEuler(order EulerOrder) (a, b, c float32) {
	m := q.rotMat3()
	// m[row*3+col]: m[0]=M[0][0], m[1]=M[0][1], m[2]=M[0][2],
	//               m[3]=M[1][0], m[4]=M[1][1], m[5]=M[1][2],
	//               m[6]=M[2][0], m[7]=M[2][1], m[8]=M[2][2]
	switch order {
	case EulerXYZ: // q = Rz(c)*Ry(b)*Rx(a); sin(b) = -M[2][0]
		sinB := clamp32(-m[6], -1, 1)
		b = asin32(sinB)
		if abs32(sinB) < 1-eps32 {
			a = atan232(m[7], m[8])
			c = atan232(m[3], m[0])
		} else {
			// gimbal lock: a±c determined; set c=0
			if sinB > 0 {
				a = atan232(m[1], m[2])
			} else {
				a = atan232(-m[1], -m[2])
			}
			c = 0
		}
	case EulerXZY: // q = Ry(c)*Rz(b)*Rx(a); sin(b) = M[1][0]
		sinB := clamp32(m[3], -1, 1)
		b = asin32(sinB)
		if abs32(sinB) < 1-eps32 {
			a = atan232(-m[5], m[4])
			c = atan232(-m[6], m[0])
		} else {
			if sinB > 0 {
				a = atan232(-m[7], m[8])
			} else {
				a = atan232(m[7], m[8])
			}
			c = 0
		}
	case EulerYXZ: // q = Rz(c)*Rx(b)*Ry(a); sin(b) = M[2][1]
		sinB := clamp32(m[7], -1, 1)
		b = asin32(sinB)
		if abs32(sinB) < 1-eps32 {
			a = atan232(-m[6], m[8])
			c = atan232(-m[1], m[4])
		} else {
			if sinB > 0 {
				a = atan232(m[3], m[0])
			} else {
				a = atan232(-m[3], m[0])
			}
			c = 0
		}
	case EulerYZX: // q = Rx(c)*Rz(b)*Ry(a); sin(b) = -M[0][1]
		sinB := clamp32(-m[1], -1, 1)
		b = asin32(sinB)
		if abs32(sinB) < 1-eps32 {
			a = atan232(m[2], m[0])
			c = atan232(m[7], m[4])
		} else {
			if sinB > 0 {
				a = atan232(m[5], m[8])
			} else {
				a = atan232(-m[5], m[8])
			}
			c = 0
		}
	case EulerZXY: // q = Ry(c)*Rx(b)*Rz(a); sin(b) = -M[1][2]
		sinB := clamp32(-m[5], -1, 1)
		b = asin32(sinB)
		if abs32(sinB) < 1-eps32 {
			a = atan232(m[3], m[4])
			c = atan232(m[2], m[8])
		} else {
			if sinB > 0 {
				a = atan232(m[6], m[0])
			} else {
				a = atan232(-m[6], m[0])
			}
			c = 0
		}
	case EulerZYX: // q = Rx(c)*Ry(b)*Rz(a); sin(b) = M[0][2]
		sinB := clamp32(m[2], -1, 1)
		b = asin32(sinB)
		if abs32(sinB) < 1-eps32 {
			a = atan232(-m[1], m[0])
			c = atan232(-m[5], m[8])
		} else {
			if sinB > 0 {
				a = atan232(m[7], m[4])
			} else {
				a = atan232(-m[7], m[4])
			}
			c = 0
		}
	}
	return
}

// rotMat3 returns the 9 rotation-matrix elements in row-major order (m[row*3+col]).
func (q Quat) rotMat3() [9]float32 {
	x, y, z, w := q.X, q.Y, q.Z, q.W
	return [9]float32{
		1 - 2*(y*y+z*z), 2 * (x*y - w*z), 2 * (x*z + w*y),
		2 * (x*y + w*z), 1 - 2*(x*x+z*z), 2 * (y*z - w*x),
		2 * (x*z - w*y), 2 * (y*z + w*x), 1 - 2*(x*x+y*y),
	}
}

// QuatFromRotMat3 builds a Quat from a column-major 3×3 rotation matrix.
func QuatFromRotMat3(col0, col1, col2 Vec3) Quat {
	return fromRotMat3(col0, col1, col2)
}

// fromRotMat3 uses Shepperd's method; called by affine.go.
func fromRotMat3(col0, col1, col2 Vec3) Quat {
	m00, m10, m20 := col0.X, col0.Y, col0.Z
	m01, m11, m21 := col1.X, col1.Y, col1.Z
	m02, m12, m22 := col2.X, col2.Y, col2.Z
	trace := m00 + m11 + m22
	var q Quat
	switch {
	case trace > 0:
		s := sqrt32(trace+1) * 2
		q.W = 0.25 * s
		q.X = (m21 - m12) / s
		q.Y = (m02 - m20) / s
		q.Z = (m10 - m01) / s
	case m00 > m11 && m00 > m22:
		s := sqrt32(1+m00-m11-m22) * 2
		q.W = (m21 - m12) / s
		q.X = 0.25 * s
		q.Y = (m01 + m10) / s
		q.Z = (m02 + m20) / s
	case m11 > m22:
		s := sqrt32(1+m11-m00-m22) * 2
		q.W = (m02 - m20) / s
		q.X = (m01 + m10) / s
		q.Y = 0.25 * s
		q.Z = (m12 + m21) / s
	default:
		s := sqrt32(1+m22-m00-m11) * 2
		q.W = (m10 - m01) / s
		q.X = (m02 + m20) / s
		q.Y = (m12 + m21) / s
		q.Z = 0.25 * s
	}
	return q
}
