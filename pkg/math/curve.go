package math

// CubicBezier1D evaluates a 1D cubic Bezier curve at parameter t ∈ [0,1].
// p0, p1 are endpoints; p1ctrl, p2ctrl are the two interior control points.
func CubicBezier1D(p0, p1ctrl, p2ctrl, p3 float32, t float32) float32 {
	u := 1 - t
	return u*u*u*p0 + 3*u*u*t*p1ctrl + 3*u*t*t*p2ctrl + t*t*t*p3
}

// CubicBezierVec3 evaluates a 3D cubic Bezier curve at t ∈ [0,1].
func CubicBezierVec3(p0, p1, p2, p3 Vec3, t float32) Vec3 {
	u := 1 - t
	a := u * u * u
	b := 3 * u * u * t
	c := 3 * u * t * t
	d := t * t * t
	return Vec3{
		X: a*p0.X + b*p1.X + c*p2.X + d*p3.X,
		Y: a*p0.Y + b*p1.Y + c*p2.Y + d*p3.Y,
		Z: a*p0.Z + b*p1.Z + c*p2.Z + d*p3.Z,
	}
}

// CubicBezierVec3Derivative returns the first derivative (tangent) of a 3D cubic
// Bezier at t — useful for computing arc length or orientation along a path.
func CubicBezierVec3Derivative(p0, p1, p2, p3 Vec3, t float32) Vec3 {
	u := 1 - t
	a := 3 * u * u
	b := 6 * u * t
	c := 3 * t * t
	d01 := Vec3{X: p1.X - p0.X, Y: p1.Y - p0.Y, Z: p1.Z - p0.Z}
	d12 := Vec3{X: p2.X - p1.X, Y: p2.Y - p1.Y, Z: p2.Z - p1.Z}
	d23 := Vec3{X: p3.X - p2.X, Y: p3.Y - p2.Y, Z: p3.Z - p2.Z}
	return Vec3{
		X: a*d01.X + b*d12.X + c*d23.X,
		Y: a*d01.Y + b*d12.Y + c*d23.Y,
		Z: a*d01.Z + b*d12.Z + c*d23.Z,
	}
}

// ─── Cubic Hermite ────────────────────────────────────────────────────────────

// CubicHermite1D evaluates a 1D cubic Hermite segment at t ∈ [0,1].
// p0, p1 are endpoints; m0, m1 are the tangent vectors at each endpoint.
func CubicHermite1D(p0, m0, p1, m1, t float32) float32 {
	t2 := t * t
	t3 := t2 * t
	h00 := 2*t3 - 3*t2 + 1
	h10 := t3 - 2*t2 + t
	h01 := -2*t3 + 3*t2
	h11 := t3 - t2
	return h00*p0 + h10*m0 + h01*p1 + h11*m1
}

// CubicHermiteVec3 evaluates a 3D cubic Hermite segment at t ∈ [0,1].
func CubicHermiteVec3(p0, m0, p1, m1 Vec3, t float32) Vec3 {
	t2 := t * t
	t3 := t2 * t
	h00 := 2*t3 - 3*t2 + 1
	h10 := t3 - 2*t2 + t
	h01 := -2*t3 + 3*t2
	h11 := t3 - t2
	return Vec3{
		X: h00*p0.X + h10*m0.X + h01*p1.X + h11*m1.X,
		Y: h00*p0.Y + h10*m0.Y + h01*p1.Y + h11*m1.Y,
		Z: h00*p0.Z + h10*m0.Z + h01*p1.Z + h11*m1.Z,
	}
}

// CubicHermiteVec3Derivative returns the first derivative of the 3D Hermite segment at t.
func CubicHermiteVec3Derivative(p0, m0, p1, m1 Vec3, t float32) Vec3 {
	t2 := t * t
	dh00 := 6*t2 - 6*t
	dh10 := 3*t2 - 4*t + 1
	dh01 := -6*t2 + 6*t
	dh11 := 3*t2 - 2*t
	return Vec3{
		X: dh00*p0.X + dh10*m0.X + dh01*p1.X + dh11*m1.X,
		Y: dh00*p0.Y + dh10*m0.Y + dh01*p1.Y + dh11*m1.Y,
		Z: dh00*p0.Z + dh10*m0.Z + dh01*p1.Z + dh11*m1.Z,
	}
}

// ─── HermiteSpline ────────────────────────────────────────────────────────────

// HermiteSpline is a piecewise cubic Hermite spline over N≥2 keyframes.
// Each keyframe has a position and a tangent. The spline is C1-continuous at
// all interior knots when tangents are set appropriately (e.g. Catmull-Rom).
type HermiteSpline struct {
	Points   []Vec3
	Tangents []Vec3
}

// Eval evaluates the spline at normalized parameter u ∈ [0, N-1].
// Out-of-range u is clamped to the spline endpoints.
func (s *HermiteSpline) Eval(u float32) Vec3 {
	n := len(s.Points)
	if n < 2 {
		if n == 1 {
			return s.Points[0]
		}
		return Vec3{}
	}
	maxU := float32(n - 1)
	if u <= 0 {
		return s.Points[0]
	}
	if u >= maxU {
		return s.Points[n-1]
	}
	seg := int(u)
	if seg >= n-1 {
		seg = n - 2
	}
	t := u - float32(seg)
	return CubicHermiteVec3(s.Points[seg], s.Tangents[seg], s.Points[seg+1], s.Tangents[seg+1], t)
}

// NewCatmullRomSpline builds a HermiteSpline with Catmull-Rom tangents.
// Interior tangent at i = (points[i+1] - points[i-1]) / 2.
// Endpoint tangents mirror the first/last interior tangent.
func NewCatmullRomSpline(points []Vec3) HermiteSpline {
	n := len(points)
	tangents := make([]Vec3, n)
	for i := 1; i < n-1; i++ {
		tangents[i] = Vec3{
			X: (points[i+1].X - points[i-1].X) * 0.5,
			Y: (points[i+1].Y - points[i-1].Y) * 0.5,
			Z: (points[i+1].Z - points[i-1].Z) * 0.5,
		}
	}
	if n >= 2 {
		tangents[0] = Vec3{
			X: points[1].X - points[0].X,
			Y: points[1].Y - points[0].Y,
			Z: points[1].Z - points[0].Z,
		}
		last := n - 1
		tangents[last] = Vec3{
			X: points[last].X - points[last-1].X,
			Y: points[last].Y - points[last-1].Y,
			Z: points[last].Z - points[last-1].Z,
		}
	}
	return HermiteSpline{Points: points, Tangents: tangents}
}
