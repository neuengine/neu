package world

import (
	"reflect"

	"github.com/neuengine/neu/internal/ecs/changedetect"
	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/entity"
)

// RegisterRemovedComponents creates (or returns) the [changedetect.RemovedComponents][T]
// resource for the given World and wires it into the change-detection drain
// pipeline so that [World.ClearTrackers] prunes stale entries automatically.
//
// Call once per component type T at world-setup time (before any system runs).
// Calling it multiple times for the same T is idempotent — the same tracker
// instance is returned every time.
func RegisterRemovedComponents[T any](w *World) *changedetect.RemovedComponents[T] {
	if pp, ok := Resource[*changedetect.RemovedComponents[T]](w); ok && pp != nil {
		return *pp
	}

	rc := new(changedetect.RemovedComponents[T])
	SetResource(w, rc)

	// Wire the drain callback into the shared RemovedRegistry.
	reg := ensureRemovedRegistry(w)
	reg.Register(rc.DrainBefore)

	// Register the per-entity notification callback so Remove[T] / RemoveByID
	// can append to the tracker without knowing T at the call site.
	id := w.components.RegisterByType(reflect.TypeFor[T]())
	if w.removedCallbacks == nil {
		w.removedCallbacks = make(map[component.ID]func(entity.Entity, changedetect.Tick))
	}
	w.removedCallbacks[id] = func(e entity.Entity, tick changedetect.Tick) {
		rc.Append(e, tick)
	}
	return rc
}

// ensureRemovedRegistry returns the [changedetect.RemovedRegistry] stored as a
// resource, creating and storing one if absent.
func ensureRemovedRegistry(w *World) *changedetect.RemovedRegistry {
	if pp, ok := Resource[*changedetect.RemovedRegistry](w); ok && pp != nil {
		return *pp
	}
	reg := new(changedetect.RemovedRegistry)
	SetResource(w, reg)
	return reg
}
