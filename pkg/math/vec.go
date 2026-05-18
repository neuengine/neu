package math

// Vec2 is a 2-component float32 vector.
type Vec2 struct{ X, Y float32 }

func Vec2New(x, y float32) Vec2 { return Vec2{X: x, Y: y} }
func Vec2Zero() Vec2            { return Vec2{} }
func Vec2One() Vec2             { return Vec2{1, 1} }

func (v Vec2) Add(o Vec2) Vec2        { return Vec2{v.X + o.X, v.Y + o.Y} }
func (v Vec2) Sub(o Vec2) Vec2        { return Vec2{v.X - o.X, v.Y - o.Y} }
func (v Vec2) Mul(s float32) Vec2     { return Vec2{v.X * s, v.Y * s} }
func (v Vec2) MulComp(o Vec2) Vec2    { return Vec2{v.X * o.X, v.Y * o.Y} }
func (v Vec2) Div(s float32) Vec2     { return Vec2{v.X / s, v.Y / s} }
func (v Vec2) Neg() Vec2              { return Vec2{-v.X, -v.Y} }
func (v Vec2) Dot(o Vec2) float32     { return v.X*o.X + v.Y*o.Y }
func (v Vec2) LengthSquared() float32 { return v.X*v.X + v.Y*v.Y }
func (v Vec2) Length() float32        { return sqrt32(v.LengthSquared()) }
func (v Vec2) Normalize() Vec2 {
	sq := v.LengthSquared()
	if sq < eps32*eps32 {
		return Vec2{}
	}
	return v.Mul(1 / sqrt32(sq))
}
func (v Vec2) Lerp(o Vec2, t float32) Vec2 {
	return Vec2{v.X + (o.X-v.X)*t, v.Y + (o.Y-v.Y)*t}
}
func (v Vec2) DistanceTo(o Vec2) float32 { return o.Sub(v).Length() }
func (v Vec2) Min(o Vec2) Vec2           { return Vec2{min32(v.X, o.X), min32(v.Y, o.Y)} }
func (v Vec2) Max(o Vec2) Vec2           { return Vec2{max32(v.X, o.X), max32(v.Y, o.Y)} }
func (v Vec2) Clamp(lo, hi Vec2) Vec2 {
	return Vec2{clamp32(v.X, lo.X, hi.X), clamp32(v.Y, lo.Y, hi.Y)}
}

// Vec3 is a 3-component float32 vector.
type Vec3 struct{ X, Y, Z float32 }

func Vec3New(x, y, z float32) Vec3 { return Vec3{X: x, Y: y, Z: z} }
func Vec3Zero() Vec3               { return Vec3{} }
func Vec3One() Vec3                { return Vec3{1, 1, 1} }
func Vec3Up() Vec3                 { return Vec3{Y: 1} }
func Vec3Down() Vec3               { return Vec3{Y: -1} }
func Vec3Left() Vec3               { return Vec3{X: -1} }
func Vec3Right() Vec3              { return Vec3{X: 1} }
func Vec3Forward() Vec3            { return Vec3{Z: -1} } // right-handed: -Z forward
func Vec3Back() Vec3               { return Vec3{Z: 1} }

func (v Vec3) Add(o Vec3) Vec3     { return Vec3{v.X + o.X, v.Y + o.Y, v.Z + o.Z} }
func (v Vec3) Sub(o Vec3) Vec3     { return Vec3{v.X - o.X, v.Y - o.Y, v.Z - o.Z} }
func (v Vec3) Mul(s float32) Vec3  { return Vec3{v.X * s, v.Y * s, v.Z * s} }
func (v Vec3) MulComp(o Vec3) Vec3 { return Vec3{v.X * o.X, v.Y * o.Y, v.Z * o.Z} }
func (v Vec3) Div(s float32) Vec3  { return Vec3{v.X / s, v.Y / s, v.Z / s} }
func (v Vec3) Neg() Vec3           { return Vec3{-v.X, -v.Y, -v.Z} }
func (v Vec3) Dot(o Vec3) float32  { return v.X*o.X + v.Y*o.Y + v.Z*o.Z }
func (v Vec3) Cross(o Vec3) Vec3 {
	return Vec3{
		v.Y*o.Z - v.Z*o.Y,
		v.Z*o.X - v.X*o.Z,
		v.X*o.Y - v.Y*o.X,
	}
}
func (v Vec3) LengthSquared() float32 { return v.X*v.X + v.Y*v.Y + v.Z*v.Z }
func (v Vec3) Length() float32        { return sqrt32(v.LengthSquared()) }
func (v Vec3) Normalize() Vec3 {
	sq := v.LengthSquared()
	if sq < eps32*eps32 {
		return Vec3{}
	}
	return v.Mul(1 / sqrt32(sq))
}
func (v Vec3) Lerp(o Vec3, t float32) Vec3 {
	return Vec3{v.X + (o.X-v.X)*t, v.Y + (o.Y-v.Y)*t, v.Z + (o.Z-v.Z)*t}
}
func (v Vec3) DistanceTo(o Vec3) float32 { return o.Sub(v).Length() }
func (v Vec3) Min(o Vec3) Vec3 {
	return Vec3{min32(v.X, o.X), min32(v.Y, o.Y), min32(v.Z, o.Z)}
}
func (v Vec3) Max(o Vec3) Vec3 {
	return Vec3{max32(v.X, o.X), max32(v.Y, o.Y), max32(v.Z, o.Z)}
}
func (v Vec3) Clamp(lo, hi Vec3) Vec3 {
	return Vec3{clamp32(v.X, lo.X, hi.X), clamp32(v.Y, lo.Y, hi.Y), clamp32(v.Z, lo.Z, hi.Z)}
}

// Vec4 is a 4-component float32 vector.
type Vec4 struct{ X, Y, Z, W float32 }

func Vec4New(x, y, z, w float32) Vec4 { return Vec4{X: x, Y: y, Z: z, W: w} }
func Vec4Zero() Vec4                  { return Vec4{} }
func Vec4One() Vec4                   { return Vec4{1, 1, 1, 1} }

func (v Vec4) Add(o Vec4) Vec4     { return Vec4{v.X + o.X, v.Y + o.Y, v.Z + o.Z, v.W + o.W} }
func (v Vec4) Sub(o Vec4) Vec4     { return Vec4{v.X - o.X, v.Y - o.Y, v.Z - o.Z, v.W - o.W} }
func (v Vec4) Mul(s float32) Vec4  { return Vec4{v.X * s, v.Y * s, v.Z * s, v.W * s} }
func (v Vec4) MulComp(o Vec4) Vec4 { return Vec4{v.X * o.X, v.Y * o.Y, v.Z * o.Z, v.W * o.W} }
func (v Vec4) Div(s float32) Vec4  { return Vec4{v.X / s, v.Y / s, v.Z / s, v.W / s} }
func (v Vec4) Neg() Vec4           { return Vec4{-v.X, -v.Y, -v.Z, -v.W} }
func (v Vec4) Dot(o Vec4) float32  { return v.X*o.X + v.Y*o.Y + v.Z*o.Z + v.W*o.W }
func (v Vec4) LengthSquared() float32 {
	return v.X*v.X + v.Y*v.Y + v.Z*v.Z + v.W*v.W
}
func (v Vec4) Length() float32 { return sqrt32(v.LengthSquared()) }
func (v Vec4) Normalize() Vec4 {
	sq := v.LengthSquared()
	if sq < eps32*eps32 {
		return Vec4{}
	}
	return v.Mul(1 / sqrt32(sq))
}
func (v Vec4) Lerp(o Vec4, t float32) Vec4 {
	return Vec4{
		v.X + (o.X-v.X)*t, v.Y + (o.Y-v.Y)*t,
		v.Z + (o.Z-v.Z)*t, v.W + (o.W-v.W)*t,
	}
}
func (v Vec4) Min(o Vec4) Vec4 {
	return Vec4{min32(v.X, o.X), min32(v.Y, o.Y), min32(v.Z, o.Z), min32(v.W, o.W)}
}
func (v Vec4) Max(o Vec4) Vec4 {
	return Vec4{max32(v.X, o.X), max32(v.Y, o.Y), max32(v.Z, o.Z), max32(v.W, o.W)}
}
func (v Vec4) Clamp(lo, hi Vec4) Vec4 {
	return Vec4{
		clamp32(v.X, lo.X, hi.X), clamp32(v.Y, lo.Y, hi.Y),
		clamp32(v.Z, lo.Z, hi.Z), clamp32(v.W, lo.W, hi.W),
	}
}
func (v Vec4) XYZ() Vec3 { return Vec3{v.X, v.Y, v.Z} }
