package math

// Mat3 is a column-major 3×3 float32 matrix. m[c] is column c (a Vec3);
// element at row r, col c = m[c][r] via .X/.Y/.Z (rows 0/1/2).
type Mat3 [3]Vec3

// Mat4 is a column-major 4×4 float32 matrix. m[c] is column c (a Vec4);
// element at row r, col c = m[c][r] via .X/.Y/.Z/.W (rows 0/1/2/3).
type Mat4 [4]Vec4

// ─── Mat3 ───────────────────────────────────────────────────────────────────

func Mat3Identity() Mat3 {
	return Mat3{{X: 1}, {Y: 1}, {Z: 1}}
}

func (m Mat3) MulMat(o Mat3) Mat3 {
	return Mat3{
		{m[0].X*o[0].X + m[1].X*o[0].Y + m[2].X*o[0].Z,
			m[0].Y*o[0].X + m[1].Y*o[0].Y + m[2].Y*o[0].Z,
			m[0].Z*o[0].X + m[1].Z*o[0].Y + m[2].Z*o[0].Z},
		{m[0].X*o[1].X + m[1].X*o[1].Y + m[2].X*o[1].Z,
			m[0].Y*o[1].X + m[1].Y*o[1].Y + m[2].Y*o[1].Z,
			m[0].Z*o[1].X + m[1].Z*o[1].Y + m[2].Z*o[1].Z},
		{m[0].X*o[2].X + m[1].X*o[2].Y + m[2].X*o[2].Z,
			m[0].Y*o[2].X + m[1].Y*o[2].Y + m[2].Y*o[2].Z,
			m[0].Z*o[2].X + m[1].Z*o[2].Y + m[2].Z*o[2].Z},
	}
}

func (m Mat3) MulVec3(v Vec3) Vec3 {
	return Vec3{
		m[0].X*v.X + m[1].X*v.Y + m[2].X*v.Z,
		m[0].Y*v.X + m[1].Y*v.Y + m[2].Y*v.Z,
		m[0].Z*v.X + m[1].Z*v.Y + m[2].Z*v.Z,
	}
}

func (m Mat3) Transpose() Mat3 {
	return Mat3{
		{m[0].X, m[1].X, m[2].X},
		{m[0].Y, m[1].Y, m[2].Y},
		{m[0].Z, m[1].Z, m[2].Z},
	}
}

func (m Mat3) Determinant() float32 {
	// Expand along first column.
	return m[0].X*(m[1].Y*m[2].Z-m[2].Y*m[1].Z) -
		m[1].X*(m[0].Y*m[2].Z-m[2].Y*m[0].Z) +
		m[2].X*(m[0].Y*m[1].Z-m[1].Y*m[0].Z)
}

func (m Mat3) Inverse() (Mat3, bool) {
	det := m.Determinant()
	if abs32(det) < eps32 {
		return Mat3{}, false
	}
	invDet := 1 / det
	return Mat3{
		{invDet * (m[1].Y*m[2].Z - m[2].Y*m[1].Z),
			invDet * -(m[0].Y*m[2].Z - m[2].Y*m[0].Z),
			invDet * (m[0].Y*m[1].Z - m[1].Y*m[0].Z)},
		{invDet * -(m[1].X*m[2].Z - m[2].X*m[1].Z),
			invDet * (m[0].X*m[2].Z - m[2].X*m[0].Z),
			invDet * -(m[0].X*m[1].Z - m[1].X*m[0].Z)},
		{invDet * (m[1].X*m[2].Y - m[2].X*m[1].Y),
			invDet * -(m[0].X*m[2].Y - m[2].X*m[0].Y),
			invDet * (m[0].X*m[1].Y - m[1].X*m[0].Y)},
	}, true
}

// ─── Mat4 ───────────────────────────────────────────────────────────────────

func Mat4Identity() Mat4 {
	return Mat4{{X: 1}, {Y: 1}, {Z: 1}, {W: 1}}
}

func Mat4FromScale(s Vec3) Mat4 {
	return Mat4{
		{X: s.X},
		{Y: s.Y},
		{Z: s.Z},
		{W: 1},
	}
}

func Mat4FromTranslation(t Vec3) Mat4 {
	m := Mat4Identity()
	m[3] = Vec4{t.X, t.Y, t.Z, 1}
	return m
}

// Mat4Perspective builds a right-handed perspective projection matrix (NDC: -1..1 depth).
func Mat4Perspective(fovYRad, aspect, near, far float32) Mat4 {
	t := cos32(fovYRad*0.5) / sin32(fovYRad*0.5) // cot(fovY/2)
	return Mat4{
		{X: t / aspect},
		{Y: t},
		{Z: -(far + near) / (far - near), W: -1},
		{Z: -(2 * far * near) / (far - near)},
	}
}

// Mat4Orthographic builds a right-handed orthographic projection matrix.
func Mat4Orthographic(l, r, b, t, near, far float32) Mat4 {
	return Mat4{
		{X: 2 / (r - l)},
		{Y: 2 / (t - b)},
		{Z: -2 / (far - near)},
		{X: -(r + l) / (r - l), Y: -(t + b) / (t - b), Z: -(far + near) / (far - near), W: 1},
	}
}

func (m Mat4) MulMat(o Mat4) Mat4 {
	// result[c] = m * o[c]
	mul := func(c Vec4) Vec4 {
		return Vec4{
			m[0].X*c.X + m[1].X*c.Y + m[2].X*c.Z + m[3].X*c.W,
			m[0].Y*c.X + m[1].Y*c.Y + m[2].Y*c.Z + m[3].Y*c.W,
			m[0].Z*c.X + m[1].Z*c.Y + m[2].Z*c.Z + m[3].Z*c.W,
			m[0].W*c.X + m[1].W*c.Y + m[2].W*c.Z + m[3].W*c.W,
		}
	}
	return Mat4{mul(o[0]), mul(o[1]), mul(o[2]), mul(o[3])}
}

func (m Mat4) MulVec4(v Vec4) Vec4 {
	return Vec4{
		m[0].X*v.X + m[1].X*v.Y + m[2].X*v.Z + m[3].X*v.W,
		m[0].Y*v.X + m[1].Y*v.Y + m[2].Y*v.Z + m[3].Y*v.W,
		m[0].Z*v.X + m[1].Z*v.Y + m[2].Z*v.Z + m[3].Z*v.W,
		m[0].W*v.X + m[1].W*v.Y + m[2].W*v.Z + m[3].W*v.W,
	}
}

// TransformPoint multiplies by the matrix with implicit w=1 (applies translation).
func (m Mat4) TransformPoint(v Vec3) Vec3 {
	return Vec3{
		m[0].X*v.X + m[1].X*v.Y + m[2].X*v.Z + m[3].X,
		m[0].Y*v.X + m[1].Y*v.Y + m[2].Y*v.Z + m[3].Y,
		m[0].Z*v.X + m[1].Z*v.Y + m[2].Z*v.Z + m[3].Z,
	}
}

// TransformVector multiplies by the matrix with implicit w=0 (ignores translation).
func (m Mat4) TransformVector(v Vec3) Vec3 {
	return Vec3{
		m[0].X*v.X + m[1].X*v.Y + m[2].X*v.Z,
		m[0].Y*v.X + m[1].Y*v.Y + m[2].Y*v.Z,
		m[0].Z*v.X + m[1].Z*v.Y + m[2].Z*v.Z,
	}
}

func (m Mat4) Transpose() Mat4 {
	return Mat4{
		{m[0].X, m[1].X, m[2].X, m[3].X},
		{m[0].Y, m[1].Y, m[2].Y, m[3].Y},
		{m[0].Z, m[1].Z, m[2].Z, m[3].Z},
		{m[0].W, m[1].W, m[2].W, m[3].W},
	}
}

func (m Mat4) Determinant() float32 {
	// Cofactor expansion along column 0 using 2×2 sub-factors.
	sf00 := m[2].Z*m[3].W - m[3].Z*m[2].W
	sf01 := m[2].Y*m[3].W - m[3].Y*m[2].W
	sf02 := m[2].Y*m[3].Z - m[3].Y*m[2].Z
	sf03 := m[2].X*m[3].W - m[3].X*m[2].W
	sf04 := m[2].X*m[3].Z - m[3].X*m[2].Z
	sf05 := m[2].X*m[3].Y - m[3].X*m[2].Y
	return m[0].X*(m[1].Y*sf00-m[1].Z*sf01+m[1].W*sf02) -
		m[0].Y*(m[1].X*sf00-m[1].Z*sf03+m[1].W*sf04) +
		m[0].Z*(m[1].X*sf01-m[1].Y*sf03+m[1].W*sf05) -
		m[0].W*(m[1].X*sf02-m[1].Y*sf04+m[1].Z*sf05)
}

// Inverse returns the inverse matrix using GLM-style cofactor expansion.
// Returns (zero, false) when |det| < 1e-6.
func (m Mat4) Inverse() (Mat4, bool) {
	// 18 unique 2×2 sub-factors from adjacent column pairs.
	sf00 := m[2].Z*m[3].W - m[3].Z*m[2].W
	sf01 := m[2].Y*m[3].W - m[3].Y*m[2].W
	sf02 := m[2].Y*m[3].Z - m[3].Y*m[2].Z
	sf03 := m[2].X*m[3].W - m[3].X*m[2].W
	sf04 := m[2].X*m[3].Z - m[3].X*m[2].Z
	sf05 := m[2].X*m[3].Y - m[3].X*m[2].Y
	sf06 := m[1].Z*m[3].W - m[3].Z*m[1].W
	sf07 := m[1].Y*m[3].W - m[3].Y*m[1].W
	sf08 := m[1].Y*m[3].Z - m[3].Y*m[1].Z
	sf09 := m[1].X*m[3].W - m[3].X*m[1].W
	sf10 := m[1].X*m[3].Z - m[3].X*m[1].Z
	sf11 := m[1].X*m[3].Y - m[3].X*m[1].Y
	sf12 := m[1].Z*m[2].W - m[2].Z*m[1].W
	sf13 := m[1].Y*m[2].W - m[2].Y*m[1].W
	sf14 := m[1].Y*m[2].Z - m[2].Y*m[1].Z
	sf15 := m[1].X*m[2].W - m[2].X*m[1].W
	sf16 := m[1].X*m[2].Z - m[2].X*m[1].Z
	sf17 := m[1].X*m[2].Y - m[2].X*m[1].Y

	// Build the adjugate divided by det in one pass.
	inv := Mat4{
		{+(m[1].Y*sf00 - m[1].Z*sf01 + m[1].W*sf02),
			-(m[1].X*sf00 - m[1].Z*sf03 + m[1].W*sf04),
			+(m[1].X*sf01 - m[1].Y*sf03 + m[1].W*sf05),
			-(m[1].X*sf02 - m[1].Y*sf04 + m[1].Z*sf05)},
		{-(m[0].Y*sf00 - m[0].Z*sf01 + m[0].W*sf02),
			+(m[0].X*sf00 - m[0].Z*sf03 + m[0].W*sf04),
			-(m[0].X*sf01 - m[0].Y*sf03 + m[0].W*sf05),
			+(m[0].X*sf02 - m[0].Y*sf04 + m[0].Z*sf05)},
		{+(m[0].Y*sf06 - m[0].Z*sf07 + m[0].W*sf08),
			-(m[0].X*sf06 - m[0].Z*sf09 + m[0].W*sf10),
			+(m[0].X*sf07 - m[0].Y*sf09 + m[0].W*sf11),
			-(m[0].X*sf08 - m[0].Y*sf10 + m[0].Z*sf11)},
		{-(m[0].Y*sf12 - m[0].Z*sf13 + m[0].W*sf14),
			+(m[0].X*sf12 - m[0].Z*sf15 + m[0].W*sf16),
			-(m[0].X*sf13 - m[0].Y*sf15 + m[0].W*sf17),
			+(m[0].X*sf14 - m[0].Y*sf16 + m[0].Z*sf17)},
	}

	det := m[0].X*inv[0].X + m[0].Y*inv[1].X + m[0].Z*inv[2].X + m[0].W*inv[3].X
	if abs32(det) < eps32 {
		return Mat4{}, false
	}
	invDet := 1 / det
	for c := range inv {
		inv[c].X *= invDet
		inv[c].Y *= invDet
		inv[c].Z *= invDet
		inv[c].W *= invDet
	}
	return inv, true
}

// ColMajorArray returns the backing [16]float32 in column-major order for GPU upload.
func (m Mat4) ColMajorArray() [16]float32 {
	return [16]float32{
		m[0].X, m[0].Y, m[0].Z, m[0].W,
		m[1].X, m[1].Y, m[1].Z, m[1].W,
		m[2].X, m[2].Y, m[2].Z, m[2].W,
		m[3].X, m[3].Y, m[3].Z, m[3].W,
	}
}
