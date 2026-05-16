// Package event provides decoupled communication primitives for the ECS
// runtime: double-buffered broadcast events ([EventBus]) and per-reader-cursor
// ring-buffer messages ([MessageChannel]).
//
// Events are retained for exactly two frames so every system gets one tick to
// observe them; callers rotate the buffers via [SwapAll] (typically wired into
// the schedule's frame-start callback). Messages live in a fixed-capacity ring
// buffer with independent reader cursors — slow readers may lose the oldest
// messages on wrap, matching the lossy-under-backpressure semantics of the
// L2 spec.
//
// # Phase 1 scope (T-1G01)
//
// Per-system writer/reader handles are concrete generic structs that are
// scheduler-coordinated (no internal locking). [Registry] holds type-erased
// references so [SwapAll] / [CleanupAll] can iterate every registered bus or
// channel for a single World.
//
// Observers and entity-event bubbling along ChildOf chains are out of scope —
// they land in T-1G02 once the lifecycle-pattern track is in place.
package event

import (
	"reflect"

	"github.com/neuengine/neu/internal/ecs/world"
)

// swapper is implemented by [EventBus] for type-erased frame swap.
type swapper interface {
	swap()
}

// cleaner is implemented by [MessageChannel] for type-erased per-tick cleanup.
type cleaner interface {
	cleanup()
}

// Registry holds the type-erased event buses and message channels owned by a
// single [world.World]. It is created lazily on the first [RegisterEvent] /
// [RegisterMessage] call and stored as a `*Registry` resource. Registration
// and iteration are intended to happen at world setup time on a single
// goroutine; concurrent mutation is not supported.
type Registry struct {
	buses    map[reflect.Type]swapper
	channels map[reflect.Type]cleaner
}

func newRegistry() *Registry {
	return &Registry{
		buses:    make(map[reflect.Type]swapper),
		channels: make(map[reflect.Type]cleaner),
	}
}

// EnsureRegistry returns the Registry stored on w, creating it on first call.
// Exposed for tests and for the rare integrator that needs to introspect the
// bus list directly; production code typically uses [RegisterEvent] /
// [RegisterMessage] / [SwapAll] / [CleanupAll].
func EnsureRegistry(w *world.World) *Registry {
	if pp, ok := world.Resource[*Registry](w); ok && pp != nil {
		return *pp
	}
	r := newRegistry()
	world.SetResource(w, r)
	return r
}

// LookupRegistry returns the Registry stored on w, or nil if no event or
// message type has been registered yet.
func LookupRegistry(w *world.World) *Registry {
	pp, ok := world.Resource[*Registry](w)
	if !ok || pp == nil {
		return nil
	}
	return *pp
}

// SwapAll rotates the double buffer of every registered [EventBus] on w.
// Typically called at frame start so writes from the previous tick become the
// "previous" buffer that readers can still observe. Safe to call when no
// events are registered (no-op).
func SwapAll(w *world.World) {
	r := LookupRegistry(w)
	if r == nil {
		return
	}
	for _, b := range r.buses {
		b.swap()
	}
}

// CleanupAll runs the per-tick cleanup hook on every registered
// [MessageChannel] on w. For ring-buffer storage this is a no-op; the hook
// exists for parity with future Phase 2+ expandable-queue strategies.
func CleanupAll(w *world.World) {
	r := LookupRegistry(w)
	if r == nil {
		return
	}
	for _, c := range r.channels {
		c.cleanup()
	}
}
