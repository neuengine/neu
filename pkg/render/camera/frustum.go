package camera

import (
	pkgmath "github.com/neuengine/neu/pkg/math"
)

// FrustumFromViewProj extracts six inward-pointing frustum planes from a
// column-major view-projection matrix using the Gribb–Hartmann method.
//
// pkgmath.Mat4 is [4]Vec4 (column-major): element at row r, col c = m[c].<r>.
// The six planes cover left, right, bottom, top, near, far in that order.
// Plane normals point inward; math.FrustumAABB uses the same convention.
//
// If any plane normal has zero length (degenerate VP matrix), that plane is
// stored as a zero Plane — degenerate frustums never produce false negatives
// because math.FrustumAABB treats the zero-distance plane as passing everything.
func FrustumFromViewProj(vp pkgmath.Mat4) pkgmath.Frustum {
	// Reconstruct the four matrix rows from the column-major storage.
	// vp[c].X/Y/Z/W = element (row 0/1/2/3, col c).
	type row struct{ x, y, z, w float32 }
	r0 := row{vp[0].X, vp[1].X, vp[2].X, vp[3].X}
	r1 := row{vp[0].Y, vp[1].Y, vp[2].Y, vp[3].Y}
	r2 := row{vp[0].Z, vp[1].Z, vp[2].Z, vp[3].Z}
	r3 := row{vp[0].W, vp[1].W, vp[2].W, vp[3].W}

	mk := func(a, b, c, d float32) pkgmath.Plane {
		n := pkgmath.Vec3{X: a, Y: b, Z: c}
		l := n.Length()
		if l == 0 {
			return pkgmath.Plane{}
		}
		dir, ok := pkgmath.NewDir3(n)
		if !ok {
			return pkgmath.Plane{}
		}
		return pkgmath.Plane{Normal: dir, Distance: d / l}
	}

	return pkgmath.Frustum{Planes: [6]pkgmath.Plane{
		mk(r3.x+r0.x, r3.y+r0.y, r3.z+r0.z, r3.w+r0.w), // left
		mk(r3.x-r0.x, r3.y-r0.y, r3.z-r0.z, r3.w-r0.w), // right
		mk(r3.x+r1.x, r3.y+r1.y, r3.z+r1.z, r3.w+r1.w), // bottom
		mk(r3.x-r1.x, r3.y-r1.y, r3.z-r1.z, r3.w-r1.w), // top
		mk(r3.x+r2.x, r3.y+r2.y, r3.z+r2.z, r3.w+r2.w), // near
		mk(r3.x-r2.x, r3.y-r2.y, r3.z-r2.z, r3.w-r2.w), // far
	}}
}

// IntersectsAabb is a convenience wrapper that delegates to math.FrustumAABB.
// Returns true if the AABB may be visible (conservative — no false negatives).
func IntersectsAabb(f pkgmath.Frustum, b pkgmath.AABB) bool {
	return pkgmath.FrustumAABB(f, b)
}
