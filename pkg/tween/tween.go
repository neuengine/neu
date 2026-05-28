// Package tween provides a data-driven tweening system: a Tween component
// interpolates a target component field over time using an easing function.
// The system self-cleans on completion or entity despawn (INV-1).
//
// Bootstrap: l2-tweening-system-go Draft (C29 P5 gate open).
package tween

// LoopMode controls what happens when a tween reaches its end.
type LoopMode uint8

const (
	// LoopOnce plays once then removes itself (or despawns the entity).
	LoopOnce LoopMode = iota
	// Loop restarts from the beginning indefinitely.
	Loop
	// PingPong reverses direction at each end.
	PingPong
)

// TimeDimension selects which time source drives the tween's delta.
type TimeDimension uint8

const (
	// Virtual is affected by time scale; pauses when VirtualTime pauses (INV-3).
	Virtual TimeDimension = iota
	// Real runs at wall-clock rate regardless of VirtualTime state.
	Real
)

// Tween is an ECS component that smoothly interpolates a target component field
// from StartValue to EndValue over Duration seconds.
//
// TargetField is a dot-separated reflection path (e.g. "Transform.Translation.X").
// The field is resolved once on component insertion and cached; per-frame apply
// is allocation-free (C-027).
//
// StartValue / EndValue must be the same type: float32, [2]float32, [3]float32, or Color.
// A type mismatch surfaces as ErrTweenTypeMismatch at insertion time — the tween is not activated.
type Tween struct {
	StartValue    any
	EndValue      any
	Easing        EasingFn
	TargetField   string
	Duration      float64
	Elapsed       float64
	LoopMode      LoopMode
	TimeDimension TimeDimension
	DespawnOnDone bool
}
