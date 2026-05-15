package event

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/teratron/boltengine/internal/ecs/command"
	"github.com/teratron/boltengine/internal/ecs/entity"
	"github.com/teratron/boltengine/internal/ecs/world"
)

const maxObserverDepth = 64

// ErrObserverDepthExceeded is panicked when observer triggering nests deeper
// than maxObserverDepth levels (most likely a cycle in the ChildOf hierarchy
// or observers that mutually trigger each other).
var ErrObserverDepthExceeded = errors.New("ecs: observer trigger depth limit exceeded")

// ObserverID uniquely identifies a registered observer.
type ObserverID uint32

// TriggerKind enumerates the built-in event categories that observers can
// react to.
type TriggerKind uint8

const (
	TriggerOnAdd     TriggerKind = iota // component added to entity for the first time
	TriggerOnInsert                     // component value inserted (add or overwrite)
	TriggerOnReplace                    // component value replaced (was already present)
	TriggerOnRemove                     // component removed from entity
	TriggerOnEvent                      // custom event type
)

// TriggerType is a comparable key that identifies what kind of trigger an
// observer listens to. TypeID is the component.ID or event-type identifier
// cast to uint32; use 0 when the trigger kind does not refer to a specific type
// (e.g. a catch-all OnEvent listener).
type TriggerType struct {
	Kind   TriggerKind
	TypeID uint32
}

// ObserverCallback is the function signature for observer handlers.
// The callback receives an ObserverContext that provides read access to the
// World (via DeferredWorld) and helpers to stop bubbling or enqueue commands.
type ObserverCallback func(ctx *ObserverContext)

// ObserverContext provides an observer callback with event data, the entity
// that caused the trigger, restricted World access, and the ability to enqueue
// deferred mutations or stop event bubbling.
//
// An ObserverContext is valid only for the duration of the callback invocation.
// Do not retain a pointer to it after the callback returns.
type ObserverContext struct {
	dw              *world.DeferredWorld
	trigEvent       any
	target          entity.Entity
	propagationStop bool
	buf             *command.CommandBuffer
}

// ObserverContextEvent returns the trigger event payload type-asserted to T.
// Returns the zero value of T when the payload is not of type T.
func ObserverContextEvent[T any](ctx *ObserverContext) T {
	v, _ := ctx.trigEvent.(T)
	return v
}

// Entity returns the entity that caused the trigger.
func (ctx *ObserverContext) Entity() entity.Entity { return ctx.target }

// World returns the DeferredWorld for read operations and resource access
// during the callback.
func (ctx *ObserverContext) World() *world.DeferredWorld { return ctx.dw }

// StopPropagation prevents the event from bubbling to parent entities in the
// ChildOf hierarchy. Observers at the current entity level still finish
// dispatching; only the upward traversal is suppressed.
func (ctx *ObserverContext) StopPropagation() { ctx.propagationStop = true }

// Commands returns a Commands handle backed by the shared deferred
// command buffer for this trigger dispatch. Commands enqueued here are
// applied (via [command.CommandBuffer.Apply]) after all observers for the
// current TriggerObservers call have completed.
func (ctx *ObserverContext) Commands() *command.Commands {
	return command.NewCommands(ctx.buf)
}

// observer is the internal registration record for a single callback.
type observer struct {
	id       ObserverID
	callback ObserverCallback
}

// ObserverRegistry stores all registered observers indexed by trigger type.
// Global observers fire for every matching trigger; entity-bound observers fire
// only when the triggering entity matches the registered target.
type ObserverRegistry struct {
	global      map[TriggerType][]observer
	entityBound map[entity.Entity]map[TriggerType][]observer
	nextID      ObserverID
}

func newObserverRegistry() *ObserverRegistry {
	return &ObserverRegistry{
		global:      make(map[TriggerType][]observer),
		entityBound: make(map[entity.Entity]map[TriggerType][]observer),
	}
}

// EnsureObserverRegistry returns the ObserverRegistry stored on w, creating
// and installing it on first call. Callers that only read (without adding
// observers) should use [LookupObserverRegistry] to avoid unnecessary
// allocation.
func EnsureObserverRegistry(w *world.World) *ObserverRegistry {
	if pp, ok := world.Resource[*ObserverRegistry](w); ok && pp != nil {
		return *pp
	}
	r := newObserverRegistry()
	world.SetResource(w, r)
	return r
}

// LookupObserverRegistry returns the ObserverRegistry stored on w, or nil
// when no observers have been registered yet.
func LookupObserverRegistry(w *world.World) *ObserverRegistry {
	pp, ok := world.Resource[*ObserverRegistry](w)
	if !ok || pp == nil {
		return nil
	}
	return *pp
}

// AddObserver registers a global observer that fires whenever trigger matches
// on any entity. Returns the ObserverID for later removal.
func AddObserver(w *world.World, trigger TriggerType, callback ObserverCallback) ObserverID {
	reg := EnsureObserverRegistry(w)
	reg.nextID++
	id := reg.nextID
	reg.global[trigger] = append(reg.global[trigger], observer{id: id, callback: callback})
	return id
}

// Observe registers an entity-targeted observer that fires only when trigger
// matches on e specifically. Returns the ObserverID for later removal.
func Observe(w *world.World, e entity.Entity, trigger TriggerType, callback ObserverCallback) ObserverID {
	reg := EnsureObserverRegistry(w)
	reg.nextID++
	id := reg.nextID
	if reg.entityBound[e] == nil {
		reg.entityBound[e] = make(map[TriggerType][]observer)
	}
	reg.entityBound[e][trigger] = append(reg.entityBound[e][trigger], observer{id: id, callback: callback})
	return id
}

// RemoveObserver unregisters the observer with the given id. It searches both
// global and entity-bound registries. A no-op when the id is unknown.
func RemoveObserver(w *world.World, id ObserverID) {
	reg := LookupObserverRegistry(w)
	if reg == nil {
		return
	}
	for trigger, observers := range reg.global {
		for i, obs := range observers {
			if obs.id == id {
				reg.global[trigger] = append(observers[:i], observers[i+1:]...)
				return
			}
		}
	}
	for e, triggers := range reg.entityBound {
		for trigger, observers := range triggers {
			for i, obs := range observers {
				if obs.id == id {
					triggers[trigger] = append(observers[:i], observers[i+1:]...)
					if len(triggers[trigger]) == 0 {
						delete(triggers, trigger)
					}
					if len(triggers) == 0 {
						delete(reg.entityBound, e)
					}
					return
				}
			}
		}
	}
}

// TriggerObservers fires all matching observers for trigger on target, then
// bubbles up the ChildOf hierarchy until the root is reached or
// StopPropagation is called.
//
// Dispatch order per entity level:
//  1. Entity-bound observers registered for target.
//  2. Global observers for the trigger type.
//  3. Advance to target's ChildOf parent (if any) and repeat.
//
// Commands enqueued via [ObserverContext.Commands] during any callback are
// applied once all observers for this call have completed.
//
// Panics with ErrObserverDepthExceeded when the ChildOf hierarchy exceeds
// maxObserverDepth levels (guards against malformed cycles).
func TriggerObservers(dw *world.DeferredWorld, trigger TriggerType, target entity.Entity, event any) {
	reg := LookupObserverRegistry(dw.World())
	if reg == nil {
		return
	}
	buf := command.AcquireBuffer(dw.World().Entities())
	defer func() {
		buf.Apply(dw.World())
		command.ReleaseBuffer(buf)
	}()

	current := target
	depth := 0
	propagationStopped := false

	for current.IsValid() && !propagationStopped {
		if depth > maxObserverDepth {
			panic(fmt.Sprintf("%s (depth=%d)", ErrObserverDepthExceeded, depth))
		}

		ctx := &ObserverContext{
			dw:        dw,
			trigEvent: event,
			target:    current,
			buf:       buf,
		}

		// Entity-bound observers for current entity.
		if entityObservers, ok := reg.entityBound[current]; ok {
			for _, obs := range entityObservers[trigger] {
				dispatchObserver(obs, ctx)
			}
		}

		// Global observers.
		for _, obs := range reg.global[trigger] {
			dispatchObserver(obs, ctx)
		}

		propagationStopped = ctx.propagationStop

		// Traverse up the ChildOf hierarchy.
		childOf, ok := world.Get[entity.ChildOf](dw.World(), current)
		if !ok {
			break
		}
		current = childOf.Parent
		depth++
	}
}

// dispatchObserver calls obs.callback(ctx) and recovers any panic, logging it
// via slog. The observer is not removed on panic.
func dispatchObserver(obs observer, ctx *ObserverContext) {
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("ecs/event: observer callback panicked",
				slog.Any("id", obs.id),
				slog.Any("recover", r),
			)
		}
	}()
	obs.callback(ctx)
}
