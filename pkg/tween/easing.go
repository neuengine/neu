package tween

import "math"

// EasingFn maps normalised progress t ∈ [0,1] to an eased value ∈ [0,1].
type EasingFn func(t float64) float64

// Standard easing functions (INV-2).
// Endpoints are exact: f(0)=0, f(1)=1 for all functions.
var (
	Linear EasingFn = func(t float64) float64 { return t }

	EaseInQuad    EasingFn = func(t float64) float64 { return t * t }
	EaseOutQuad   EasingFn = func(t float64) float64 { return t * (2 - t) }
	EaseInOutQuad EasingFn = func(t float64) float64 {
		if t < 0.5 {
			return 2 * t * t
		}
		return -1 + (4-2*t)*t
	}

	EaseInCubic    EasingFn = func(t float64) float64 { return t * t * t }
	EaseOutCubic   EasingFn = func(t float64) float64 { return (t - 1) * (t - 1) * (t - 1) + 1 }
	EaseInOutCubic EasingFn = func(t float64) float64 {
		if t < 0.5 {
			return 4 * t * t * t
		}
		return (t-1)*(2*t-2)*(2*t-2) + 1
	}

	EaseInSine    EasingFn = func(t float64) float64 { return 1 - math.Cos(t*math.Pi/2) }
	EaseOutSine   EasingFn = func(t float64) float64 { return math.Sin(t * math.Pi / 2) }
	EaseInOutSine EasingFn = func(t float64) float64 { return -(math.Cos(math.Pi*t) - 1) / 2 }

	EaseOutBounce EasingFn = func(t float64) float64 {
		const (
			n1 = 7.5625
			d1 = 2.75
		)
		switch {
		case t < 1/d1:
			return n1 * t * t
		case t < 2/d1:
			t -= 1.5 / d1
			return n1*t*t + 0.75
		case t < 2.5/d1:
			t -= 2.25 / d1
			return n1*t*t + 0.9375
		default:
			t -= 2.625 / d1
			return n1*t*t + 0.984375
		}
	}
	EaseInBounce EasingFn = func(t float64) float64 { return 1 - EaseOutBounce(1-t) }

	EaseInElastic EasingFn = func(t float64) float64 {
		if t == 0 || t == 1 {
			return t
		}
		return -math.Pow(2, 10*t-10) * math.Sin((t*10-10.75)*2*math.Pi/3)
	}
	EaseOutElastic EasingFn = func(t float64) float64 {
		if t == 0 || t == 1 {
			return t
		}
		return math.Pow(2, -10*t)*math.Sin((t*10-0.75)*2*math.Pi/3) + 1
	}
)

// clamp01 clamps t to [0, 1].
func clamp01(t float64) float64 {
	if t < 0 {
		return 0
	}
	if t > 1 {
		return 1
	}
	return t
}
