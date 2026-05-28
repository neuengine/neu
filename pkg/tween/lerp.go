package tween

import "errors"

// ErrTweenTypeMismatch is returned when StartValue and EndValue have different
// types, or a type not supported by the lerp registry.
var ErrTweenTypeMismatch = errors.New("tween: Start/End value types differ or are unsupported")

// Lerpable constrains types that can be interpolated by the generic Lerp function.
// Supported: float32, [2]float32, [3]float32, [4]float32 (for color).
type Lerpable interface {
	float32 | [2]float32 | [3]float32 | [4]float32
}

// Lerp linearly interpolates between start and end by t ∈ [0,1].
// t is clamped to [0,1] before interpolation.
// For value types it is allocation-free (C-027).
func Lerp[T Lerpable](start, end T, t float32) T {
	t = float32(clamp01(float64(t)))
	switch s := any(start).(type) {
	case float32:
		e := any(end).(float32)
		r := s + (e-s)*t
		return any(r).(T)
	case [2]float32:
		e := any(end).([2]float32)
		r := [2]float32{s[0] + (e[0]-s[0])*t, s[1] + (e[1]-s[1])*t}
		return any(r).(T)
	case [3]float32:
		e := any(end).([3]float32)
		r := [3]float32{s[0] + (e[0]-s[0])*t, s[1] + (e[1]-s[1])*t, s[2] + (e[2]-s[2])*t}
		return any(r).(T)
	case [4]float32:
		e := any(end).([4]float32)
		r := [4]float32{
			s[0] + (e[0]-s[0])*t,
			s[1] + (e[1]-s[1])*t,
			s[2] + (e[2]-s[2])*t,
			s[3] + (e[3]-s[3])*t,
		}
		return any(r).(T)
	}
	// Unreachable: type constraint enforces the set above.
	var zero T
	return zero
}

// LerpAny interpolates two any values of the same Lerpable type.
// Returns the result and true on success; zero and false on type mismatch.
func LerpAny(start, end any, t float32) (any, bool) {
	switch s := start.(type) {
	case float32:
		e, ok := end.(float32)
		if !ok {
			return nil, false
		}
		return Lerp(s, e, t), true
	case [2]float32:
		e, ok := end.([2]float32)
		if !ok {
			return nil, false
		}
		return Lerp(s, e, t), true
	case [3]float32:
		e, ok := end.([3]float32)
		if !ok {
			return nil, false
		}
		return Lerp(s, e, t), true
	case [4]float32:
		e, ok := end.([4]float32)
		if !ok {
			return nil, false
		}
		return Lerp(s, e, t), true
	}
	return nil, false
}
