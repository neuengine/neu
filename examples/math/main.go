// Package main validates Neu math correctness against known mathematical properties.
//
// Each section prints PASS or FAIL with a description. All checks use closed-form
// reference values or algebraic identities that must hold exactly (or within
// float32 tolerance) regardless of implementation strategy.
//
// Run:  go run ./examples/math
// Test: go test -race ./examples/math
package main

import (
	"fmt"
	"math"

	neu "github.com/neuengine/neu/pkg/math"
)

const eps = float32(1e-5)

func near(a, b float32) bool { return a-b < eps && b-a < eps }

func checkVec3() bool {
	a := neu.Vec3{X: 1, Y: 2, Z: 3}
	b := neu.Vec3{X: 4, Y: 5, Z: 6}

	// Dot product: 1*4 + 2*5 + 3*6 = 32
	if !near(a.Dot(b), 32) {
		fmt.Println("FAIL Vec3.Dot")
		return false
	}
	// Cross product: (2*6-3*5, 3*4-1*6, 1*5-2*4) = (-3, 6, -3)
	cross := a.Cross(b)
	if !near(cross.X, -3) || !near(cross.Y, 6) || !near(cross.Z, -3) {
		fmt.Println("FAIL Vec3.Cross")
		return false
	}
	// Normalization: length must be 1
	n := a.Normalize()
	if !near(n.Dot(n), 1) {
		fmt.Println("FAIL Vec3.Normalize")
		return false
	}
	fmt.Println("PASS Vec3")
	return true
}

func checkMat4() bool {
	// Identity * any = any
	id := neu.Mat4Identity()
	v := neu.Vec4{X: 1, Y: 2, Z: 3, W: 1}
	result := id.MulVec4(v)
	if !near(result.X, 1) || !near(result.Y, 2) || !near(result.Z, 3) {
		fmt.Println("FAIL Mat4 identity MulVec4")
		return false
	}
	// Inverse: A * A⁻¹ = I  (use TransformPoint for a known point)
	scale := neu.Mat4FromScale(neu.Vec3{X: 2, Y: 3, Z: 4})
	inv, ok := scale.Inverse()
	if !ok {
		fmt.Println("FAIL Mat4.Inverse returned false")
		return false
	}
	// Scale(2,3,4) * (1,1,1,1) = (2,3,4,1); inv of that should be (1,1,1,1).
	pt := neu.Vec4{X: 1, Y: 1, Z: 1, W: 1}
	scaled := scale.MulVec4(pt)
	restored := inv.MulVec4(scaled)
	if !near(restored.X, 1) || !near(restored.Y, 1) || !near(restored.Z, 1) {
		fmt.Printf("FAIL Mat4 inverse roundtrip: got %+v\n", restored)
		return false
	}
	fmt.Println("PASS Mat4")
	return true
}

func checkQuat() bool {
	// Rotation 90° around Y: maps +X to +Z (right-handed, -Z forward)
	q := neu.QuatFromAxisAngle(neu.Vec3{X: 0, Y: 1, Z: 0}, float32(math.Pi/2))
	v := q.MulVec3(neu.Vec3{X: 1, Y: 0, Z: 0})
	if !near(v.X, 0) || !near(v.Y, 0) || !near(v.Z, -1) {
		fmt.Printf("FAIL Quat rotation: got %+v, want (0,0,-1)\n", v)
		return false
	}
	// Slerp t=0.5 between identity and 90° rotation must yield 45°.
	// cos(45°)=0.7071, so (1,0,0) rotated 45° around Y = (cos45, 0, -sin45).
	q90 := neu.QuatFromAxisAngle(neu.Vec3{X: 0, Y: 1, Z: 0}, float32(math.Pi/2))
	qMid := neu.QuatIdentity().Slerp(q90, 0.5)
	vMid := qMid.MulVec3(neu.Vec3{X: 1, Y: 0, Z: 0})
	want45 := float32(math.Sqrt2 / 2) // cos(45°) = sin(45°)
	if !near(vMid.X, want45) || !near(vMid.Z, -want45) {
		fmt.Printf("FAIL Quat slerp: got %+v, want (%v, 0, %v)\n", vMid, want45, -want45)
		return false
	}
	fmt.Println("PASS Quat")
	return true
}

func checkColor() bool {
	// sRGB → linear → sRGB must be identity within 1e-4.
	samples := []float32{0, 0.04045, 0.5, 1}
	for _, s := range samples {
		got := neu.LinearToSrgb(neu.SrgbToLinear(s))
		if !near(got, s) {
			fmt.Printf("FAIL Color sRGB roundtrip %v → %v\n", s, got)
			return false
		}
	}
	fmt.Println("PASS Color")
	return true
}

func checkCurves() bool {
	// Bezier endpoints
	p0, p3 := neu.Vec3{X: 0}, neu.Vec3{X: 10}
	p1, p2 := neu.Vec3{X: 2}, neu.Vec3{X: 8}
	if !near(neu.CubicBezierVec3(p0, p1, p2, p3, 0).X, 0) {
		fmt.Println("FAIL Bezier t=0")
		return false
	}
	if !near(neu.CubicBezierVec3(p0, p1, p2, p3, 1).X, 10) {
		fmt.Println("FAIL Bezier t=1")
		return false
	}
	// Hermite: tangent at t=0 matches m0 within finite-difference tolerance.
	m0 := float32(5)
	eps2 := float32(1e-3)
	v0 := neu.CubicHermite1D(0, m0, 1, 0, 0)
	v1 := neu.CubicHermite1D(0, m0, 1, 0, eps2)
	fdDeriv := (v1 - v0) / eps2
	// Finite-difference truncation error ≈ O(eps2²), allow 2% tolerance.
	if fdDeriv < m0*0.98 || fdDeriv > m0*1.02 {
		fmt.Printf("FAIL Hermite tangent: got %v, want ~%v\n", fdDeriv, m0)
		return false
	}
	fmt.Println("PASS Curves")
	return true
}

func checkTransformInterpolator() bool {
	from := neu.Affine3Identity()
	to := neu.Affine3{
		Translation: neu.Vec3{X: 10},
		Rotation:    neu.QuatIdentity(),
		Scale:       neu.Vec3{X: 1, Y: 1, Z: 1},
	}
	ti := neu.TransformInterpolator{From: from, To: to}
	mid := ti.Eval(0.5)
	if !near(mid.Translation.X, 5) {
		fmt.Printf("FAIL TransformInterpolator midpoint: got %v\n", mid.Translation.X)
		return false
	}
	fmt.Println("PASS TransformInterpolator")
	return true
}

func main() {
	pass := true
	pass = checkVec3() && pass
	pass = checkMat4() && pass
	pass = checkQuat() && pass
	pass = checkColor() && pass
	pass = checkCurves() && pass
	pass = checkTransformInterpolator() && pass

	if pass {
		fmt.Println("PASS: math correctness suite complete")
	} else {
		fmt.Println("FAIL: one or more math checks failed")
	}
}
