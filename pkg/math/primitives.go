package math

// Ray2D is an infinite ray in 2D: origin + unit direction.
type Ray2D struct {
	Origin    Vec2
	Direction Dir2
}

// Ray3D is an infinite ray in 3D: origin + unit direction.
type Ray3D struct {
	Origin    Vec3
	Direction Dir3
}

// AABB is an axis-aligned bounding box defined by its minimum and maximum corners.
type AABB struct{ Min, Max Vec3 }

// AABBFromCenterSize builds an AABB from a center point and half-extents.
func AABBFromCenterSize(center, halfExtents Vec3) AABB {
	return AABB{Min: center.Sub(halfExtents), Max: center.Add(halfExtents)}
}

// Center returns the center of the AABB.
func (b AABB) Center() Vec3 {
	return Vec3{(b.Min.X + b.Max.X) * 0.5, (b.Min.Y + b.Max.Y) * 0.5, (b.Min.Z + b.Max.Z) * 0.5}
}

// Sphere is a 3D sphere defined by center and radius.
type Sphere struct {
	Center Vec3
	Radius float32
}

// Plane is an infinite plane: a point p lies on the plane when Normal·p + Distance == 0.
// The normal points toward the "positive" half-space.
type Plane struct {
	Normal   Dir3
	Distance float32
}

// PlaneFromNormalPoint builds a Plane from an outward normal and a point on the plane.
// Returns (Plane, false) if normal is zero-length.
func PlaneFromNormalPoint(normal Vec3, point Vec3) (Plane, bool) {
	d, ok := NewDir3(normal)
	if !ok {
		return Plane{}, false
	}
	dist := -d.Vec().Dot(point)
	return Plane{Normal: d, Distance: dist}, true
}

// SignedDistance returns the signed distance from p to the plane.
// Positive means p is in the half-space the normal points toward.
func (pl Plane) SignedDistance(p Vec3) float32 {
	return pl.Normal.Vec().Dot(p) + pl.Distance
}

// Frustum is a view frustum defined by 6 planes.
// Convention: plane normals point inward (toward the interior of the frustum).
type Frustum struct{ Planes [6]Plane }

// ─── Intersection tests ──────────────────────────────────────────────────────

// RayAABB tests a ray against an AABB using the slab method.
// Returns the hit distance (t) and true on intersection; t may be 0 if inside.
// The ray is treated as starting at Origin and extending in the positive direction.
func RayAABB(r Ray3D, b AABB) (float32, bool) {
	dir := r.Direction.Vec()
	orig := r.Origin
	tMin := float32(-3.4028235e+38)
	tMax := float32(3.4028235e+38)

	for axis := range 3 {
		var o, d, lo, hi float32
		switch axis {
		case 0:
			o, d, lo, hi = orig.X, dir.X, b.Min.X, b.Max.X
		case 1:
			o, d, lo, hi = orig.Y, dir.Y, b.Min.Y, b.Max.Y
		case 2:
			o, d, lo, hi = orig.Z, dir.Z, b.Min.Z, b.Max.Z
		}
		if abs32(d) < eps32 {
			if o < lo || o > hi {
				return 0, false
			}
			continue
		}
		t1 := (lo - o) / d
		t2 := (hi - o) / d
		if t1 > t2 {
			t1, t2 = t2, t1
		}
		if t1 > tMin {
			tMin = t1
		}
		if t2 < tMax {
			tMax = t2
		}
		if tMin > tMax {
			return 0, false
		}
	}
	if tMax < 0 {
		return 0, false
	}
	t := tMin
	if t < 0 {
		t = tMax
	}
	return t, true
}

// RaySphere tests a ray against a sphere.
// Returns the hit distance (nearest non-negative t) and true on intersection.
func RaySphere(r Ray3D, s Sphere) (float32, bool) {
	oc := r.Origin.Sub(s.Center)
	dir := r.Direction.Vec()
	b := oc.Dot(dir)
	c := oc.Dot(oc) - s.Radius*s.Radius
	disc := b*b - c
	if disc < 0 {
		return 0, false
	}
	sqrtD := sqrt32(disc)
	t := -b - sqrtD
	if t < 0 {
		t = -b + sqrtD
	}
	if t < 0 {
		return 0, false
	}
	return t, true
}

// AABBAABB reports whether two AABBs overlap.
func AABBAABB(a, b AABB) bool {
	return a.Min.X <= b.Max.X && a.Max.X >= b.Min.X &&
		a.Min.Y <= b.Max.Y && a.Max.Y >= b.Min.Y &&
		a.Min.Z <= b.Max.Z && a.Max.Z >= b.Min.Z
}

// SphereSphere reports whether two spheres overlap.
func SphereSphere(a, b Sphere) bool {
	r := a.Radius + b.Radius
	d := a.Center.Sub(b.Center)
	return d.LengthSquared() <= r*r
}

// FrustumAABB reports whether an AABB is (potentially) inside the frustum.
// Uses conservative positive-vertex test: may return true for AABBs that barely
// miss a corner (false positives), but never misses a visible AABB (no false negatives).
func FrustumAABB(f Frustum, b AABB) bool {
	for _, pl := range f.Planes {
		n := pl.Normal.Vec()
		// Choose the AABB corner most aligned with the inward normal (p-vertex).
		var px, py, pz float32
		if n.X >= 0 {
			px = b.Max.X
		} else {
			px = b.Min.X
		}
		if n.Y >= 0 {
			py = b.Max.Y
		} else {
			py = b.Min.Y
		}
		if n.Z >= 0 {
			pz = b.Max.Z
		} else {
			pz = b.Min.Z
		}
		if n.Dot(Vec3{px, py, pz})+pl.Distance < 0 {
			return false
		}
	}
	return true
}

// AABBContains reports whether point p lies inside or on the surface of b.
func AABBContains(b AABB, p Vec3) bool {
	return p.X >= b.Min.X && p.X <= b.Max.X &&
		p.Y >= b.Min.Y && p.Y <= b.Max.Y &&
		p.Z >= b.Min.Z && p.Z <= b.Max.Z
}
