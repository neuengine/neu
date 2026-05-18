package main

import "testing"

func TestMathParity(t *testing.T) {
	if !checkVec3() {
		t.Error("Vec3 parity failed")
	}
	if !checkMat4() {
		t.Error("Mat4 parity failed")
	}
	if !checkQuat() {
		t.Error("Quat parity failed")
	}
	if !checkColor() {
		t.Error("Color parity failed")
	}
	if !checkCurves() {
		t.Error("Curves parity failed")
	}
	if !checkTransformInterpolator() {
		t.Error("TransformInterpolator parity failed")
	}
}
