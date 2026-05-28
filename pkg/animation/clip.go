// Package animation provides clip-based and graph-based animation for ECS entities.
// AnimationClip assets store sampled curves; AnimationPlayer components drive playback.
//
// Bootstrap: l2-animation-system-go Draft (C29 P5 gate open).
package animation

// Interpolation selects the keyframe blending mode.
type Interpolation uint8

const (
	// InterpolationStep holds the previous keyframe value until the next one.
	InterpolationStep Interpolation = iota
	// InterpolationLinear linearly blends between adjacent keyframes.
	// Uses slerp for quaternion values.
	InterpolationLinear
	// InterpolationCubicSpline uses in/out tangents per keyframe for smooth curves.
	InterpolationCubicSpline
)

// Keyframes stores the time-value data for one animated property curve.
// Values are stored flattened; stride = component width (1 for float32, 3 for Vec3, 4 for Quat).
// For CubicSpline, Tangents holds in-tangent and out-tangent pairs per keyframe.
type Keyframes struct {
	Times    []float32
	Values   []float32
	Tangents []float32 // CubicSpline only; len = 2 * len(Times) * stride
	Interp   Interpolation
}

// VariableCurve maps an AnimationTargetId to its sampled keyframe data.
type VariableCurve struct {
	Target    AnimationTargetId
	Keyframes Keyframes
}

// AnimationEvent fires at a specific time during playback.
// The Payload is dispatched through the event system when the playhead crosses Time.
type AnimationEvent struct {
	Time    float32
	Payload any
}

// AnimationClip is an immutable asset of sampled curves and timed events.
// Instances are loaded via the asset system; clips are never mutated at runtime.
type AnimationClip struct {
	Duration float32
	Curves   []VariableCurve
	Events   []AnimationEvent
}
