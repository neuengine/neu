// Package state provides a generic finite-state-machine layer for the ECS.
// States are comparable types (typically uint8 iota enums) stored as World
// resources. Transitions are requested via NextState[S] and applied once
// per frame by the system registered in the StateTransition schedule.
package state

import (
	"fmt"

	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
)

// State holds the currently active value of state type S.
// Read-only for user systems — request transitions via NextState[S].
type State[S comparable] struct {
	current S
}

// Current returns the active state value.
func (s *State[S]) Current() S { return s.current }

// NextState is a resource used to queue a state transition.
// The engine processes it during the StateTransition schedule.
type NextState[S comparable] struct {
	value *S
}

// Set queues a transition to next. Overwrites any pending transition.
func (n *NextState[S]) Set(next S) { n.value = &next }

// Get returns the pending transition value, or nil if none is pending.
func (n *NextState[S]) Get() *S { return n.value }

// Clear cancels the pending transition.
func (n *NextState[S]) Clear() { n.value = nil }

// IsPending reports whether a transition is queued.
func (n *NextState[S]) IsPending() bool { return n.value != nil }

// LastTransition records the most recent state change for detection by systems.
// Reset each frame (within the transition system) before the next transition.
type LastTransition[S comparable] struct {
	From    S
	To      S
	Changed bool
}

// TransitionTo is a convenience function that sets NextState[S] in the world.
// If NextState[S] is not registered, this is a no-op.
func TransitionTo[S comparable](w *world.World, next S) {
	if ns, ok := world.Resource[NextState[S]](w); ok {
		ns.Set(next)
	}
}

// InitState registers State[S]{initial}, NextState[S]{}, and LastTransition[S]{}
// as world resources, then registers the per-type transition system in the
// StateTransition schedule. Calling InitState twice for the same S is a no-op.
func InitState[S comparable](app appface.Builder, initial S) {
	w := app.World()
	if world.ContainsResource[State[S]](w) {
		return // already initialised
	}
	world.SetResource(w, State[S]{current: initial})
	world.SetResource(w, NextState[S]{})
	world.SetResource(w, LastTransition[S]{})

	typeName := fmt.Sprintf("%T", initial)
	app.AddSystem(appface.StateTransition, TransitionSystemFor[S]("state.Transition["+typeName+"]"))
}

// applyTransition[S] is the per-type transition system body.
func applyTransition[S comparable](w *world.World) {
	lt, ltOK := world.Resource[LastTransition[S]](w)
	if ltOK {
		lt.Changed = false
	}

	ns, nsOK := world.Resource[NextState[S]](w)
	if !nsOK || !ns.IsPending() {
		return
	}
	st, stOK := world.Resource[State[S]](w)
	if !stOK {
		return
	}

	next := *ns.Get()
	if next == st.current {
		ns.Clear()
		return
	}

	if ltOK {
		lt.From = st.current
		lt.To = next
		lt.Changed = true
	}
	st.current = next
	ns.Clear()
}

// TransitionSystemFor returns a named FuncSystem that applies transitions for
// state type S. Suitable for passing to app.AddSystem in StateTransition.
func TransitionSystemFor[S comparable](name string) scheduler.System {
	return scheduler.NewFuncSystem(name, applyTransition[S])
}

// ── Schedule labels ───────────────────────────────────────────────────────────

// OnEnter returns the schedule label for systems that run when state S enters value.
func OnEnter[S comparable](value S) string {
	var zero S
	return fmt.Sprintf("OnEnter:%T:%v", zero, value)
}

// OnExit returns the schedule label for systems that run when state S exits value.
func OnExit[S comparable](value S) string {
	var zero S
	return fmt.Sprintf("OnExit:%T:%v", zero, value)
}

// OnTransition returns the schedule label that runs on every transition of type S.
func OnTransition[S comparable]() string {
	var zero S
	return fmt.Sprintf("OnTransition:%T", zero)
}

// ── Run conditions ────────────────────────────────────────────────────────────

// InState returns a run condition that passes when State[S].Current() == target.
// Returns false when State[S] resource does not exist (inactive SubState).
func InState[S comparable](target S) scheduler.RunCondition {
	return func(w *world.World) bool {
		s, ok := world.Resource[State[S]](w)
		return ok && s.Current() == target
	}
}

// StateChanged returns a run condition that passes when State[S] transitioned
// this frame (i.e. LastTransition[S].Changed == true).
func StateChanged[S comparable]() scheduler.RunCondition {
	return func(w *world.World) bool {
		lt, ok := world.Resource[LastTransition[S]](w)
		return ok && lt.Changed
	}
}

// StateExists returns a run condition that passes when State[S] is registered.
func StateExists[S comparable]() scheduler.RunCondition {
	return func(w *world.World) bool {
		return world.ContainsResource[State[S]](w)
	}
}

// ── DespawnOnExit ─────────────────────────────────────────────────────────────

// DespawnOnExit is a marker component. Entities carrying it are automatically
// despawned when State[S] exits the value stored in the State field.
type DespawnOnExit[S comparable] struct {
	State S
}
