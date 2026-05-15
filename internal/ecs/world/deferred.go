package world

import (
	"github.com/teratron/boltengine/internal/ecs/component"
	"github.com/teratron/boltengine/internal/ecs/entity"
)

// DeferredWorld is a restricted view of the World for use inside component
// hooks and observer callbacks. It allows reading and writing components and
// resources on existing entities but forbids structural mutations (spawn,
// despawn, add/remove components). Structural operations must go through the
// command buffer (Track F / T-1F01).
//
// *DeferredWorld satisfies component.HookContext so it can be passed to
// OnAdd/OnRemove hooks without introducing a circular import between the
// component and world packages.
type DeferredWorld struct {
	world *World
}

// compile-time assertion: *DeferredWorld satisfies component.HookContext.
var _ component.HookContext = (*DeferredWorld)(nil)

// NewDeferredWorld wraps w in a DeferredWorld view.
func NewDeferredWorld(w *World) *DeferredWorld {
	return &DeferredWorld{world: w}
}

// NewDeferred is a convenience method that creates a DeferredWorld from the
// receiver. Useful in tests and hook dispatchers.
func (w *World) NewDeferred() *DeferredWorld {
	return NewDeferredWorld(w)
}

// World returns the underlying World. Intended for trusted internal callers
// (command apply point, serialization, tests) — not for application code.
func (dw *DeferredWorld) World() *World { return dw.world }

// IsAlive reports whether the entity is alive in the underlying World.
func (dw *DeferredWorld) IsAlive(e entity.Entity) bool {
	return dw.world.Contains(e)
}

// ApplyDeferred flushes every registered deferred flusher on the underlying
// World. Equivalent to calling [World.ApplyDeferred] directly.
func (dw *DeferredWorld) ApplyDeferred() {
	dw.world.ApplyDeferred()
}

// DeferredResource returns a read-only pointer to the singleton resource of
// type T from the underlying World.
func DeferredResource[T any](dw *DeferredWorld) (*T, bool) {
	return Resource[T](dw.world)
}

// SetDeferredResource inserts or overwrites the singleton resource of type T
// in the underlying World.
func SetDeferredResource[T any](dw *DeferredWorld, value T) {
	SetResource(dw.world, value)
}

// RemoveDeferredResource removes the resource of type T and returns true if
// it was present.
func RemoveDeferredResource[T any](dw *DeferredWorld) bool {
	return RemoveResource[T](dw.world)
}

// ContainsDeferredResource reports whether a resource of type T exists.
func ContainsDeferredResource[T any](dw *DeferredWorld) bool {
	return ContainsResource[T](dw.world)
}
