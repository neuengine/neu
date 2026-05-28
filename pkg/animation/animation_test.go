package animation

import "testing"

func TestAnimationClip_ZeroValue(t *testing.T) {
	var c AnimationClip
	if c.Duration != 0 {
		t.Errorf("Duration = %v, want 0", c.Duration)
	}
	if len(c.Curves) != 0 {
		t.Error("Curves should be empty by default")
	}
}

func TestKeyframes_Interpolation(t *testing.T) {
	k := Keyframes{
		Times:  []float32{0, 1},
		Values: []float32{0, 1},
		Interp: InterpolationLinear,
	}
	if k.Interp != InterpolationLinear {
		t.Errorf("Interp = %v, want Linear", k.Interp)
	}
}

func TestRepeatMode_Constants(t *testing.T) {
	if RepeatOnce != 0 {
		t.Error("RepeatOnce should be iota 0")
	}
	if RepeatLoop == RepeatOnce {
		t.Error("RepeatLoop should differ from RepeatOnce")
	}
	if RepeatPingPong == RepeatLoop {
		t.Error("RepeatPingPong should differ from RepeatLoop")
	}
}

func TestAnimationTargetId_Empty(t *testing.T) {
	// Empty EntityPath means the player entity itself.
	id := AnimationTargetId{Property: "Transform.Translation"}
	if id.EntityPath != "" {
		t.Errorf("EntityPath = %q, want empty", id.EntityPath)
	}
}

func TestAnimationEvent_ZeroTime(t *testing.T) {
	ev := AnimationEvent{Time: 0, Payload: "footstep"}
	if ev.Time != 0 {
		t.Errorf("Time = %v, want 0", ev.Time)
	}
}

func TestVariableCurve_TargetRoundtrip(t *testing.T) {
	curve := VariableCurve{
		Target: AnimationTargetId{
			EntityPath: "Armature/Hips",
			Property:   "Transform.Rotation",
		},
		Keyframes: Keyframes{Interp: InterpolationCubicSpline},
	}
	if curve.Target.Property != "Transform.Rotation" {
		t.Error("property path not preserved")
	}
}
