// Package world provides the central ECS data store: the World, which owns
// all entities, component storage, resources, archetypes, and schedules.
package world

import (
	"errors"

	"github.com/neuengine/neu/internal/ecs/changedetect"
	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/entity"
)

// Tick is a monotonically increasing counter used for change detection.
// Incremented each time a system runs; never wraps within a single game run.
type Tick uint32

// IsNewerThan returns true when t was set after last (accounting for the
// standard change-detection contract: a component is "changed" when its
// tick is strictly newer than the last-cleared tick).
func (t Tick) IsNewerThan(last Tick) bool { return t > last }

var (
	ErrEntityNotAlive    = errors.New("ecs: entity is not alive")
	ErrScheduleNotFound  = errors.New("ecs: schedule not found")
	ErrComponentNotFound = errors.New("ecs: component not found on entity")
)

const (
	defaultEntityCapacity    = 256
	defaultComponentCapacity = 64
)

// World is the central data store of the Neu engine. It owns all entities,
// component registrations, resources, archetypes, sparse-set storage, and
// the global change tick. Not thread-safe — concurrent access must be
// coordinated by the schedule executor.
type World struct {
	entities         *entity.EntityAllocator
	components       *component.Registry
	resources        *ResourceMap
	archetypes       *ArchetypeStore
	sparseSets       map[component.ID]*component.SparseSet
	records          map[entity.EntityID]entityRecord
	removedCallbacks map[component.ID]func(entity.Entity, changedetect.Tick)
	deferredFlushers []func(*World)
	changeTick       Tick
	lastChangeTick   Tick
}

// NewWorld creates a World with default initial capacities.
func NewWorld() *World {
	return NewWorldWithCapacity(defaultEntityCapacity, defaultComponentCapacity)
}

// NewWorldWithCapacity creates a World pre-allocated for the expected number
// of entities and component types. Both values are hints; the World grows
// automatically beyond them.
func NewWorldWithCapacity(entityCapacity, _ int) *World {
	return &World{
		entities:   entity.NewEntityAllocator(entityCapacity),
		components: component.NewRegistry(),
		resources:  NewResourceMap(),
		archetypes: newArchetypeStore(),
		sparseSets: make(map[component.ID]*component.SparseSet),
		records:    make(map[entity.EntityID]entityRecord, entityCapacity),
	}
}

// Archetypes exposes the archetype store for query and observer subsystems.
func (w *World) Archetypes() *ArchetypeStore { return w.archetypes }

// Entities exposes the underlying EntityAllocator for packages that need
// direct allocator access (archetype graph, commands).
func (w *World) Entities() *entity.EntityAllocator { return w.entities }

// Components exposes the underlying component Registry for packages that
// register or look up component metadata.
func (w *World) Components() *component.Registry { return w.components }

// Resources exposes the ResourceMap for packages that need bulk resource
// iteration or direct map access (e.g., serialization).
func (w *World) Resources() *ResourceMap { return w.resources }

// SparseSet returns the global SparseSet for the given component ID, if one
// has been created. Components stored in tables (the default) have no sparse
// set and yield (nil, false). Used by query and observer subsystems to read
// values for [component.StorageSparseSet] components without owning the map.
func (w *World) SparseSet(id component.ID) (*component.SparseSet, bool) {
	ss, ok := w.sparseSets[id]
	return ss, ok
}

// SpawnEmpty allocates a new entity and parks it in the empty archetype.
// The entity has no components and lives in archetype 0 until [World.Insert]
// or a related operation moves it.
func (w *World) SpawnEmpty() entity.Entity {
	e := w.entities.Allocate()
	empty := w.archetypes.get(0)
	row := len(empty.entities)
	empty.entities = append(empty.entities, e)
	w.records[e.ID()] = entityRecord{archetypeID: 0, row: row}
	return e
}

// Contains reports whether the entity is currently alive in this World.
func (w *World) Contains(e entity.Entity) bool {
	return w.entities.IsAlive(e)
}

// ArchetypeOf returns the archetype identifier that currently holds e. The
// second return is false when e has no record in the world — either because
// it is dead, or because it was reserved via [command.CommandBuffer.ReserveEntity]
// but its SpawnCommand has not yet applied.
func (w *World) ArchetypeOf(e entity.Entity) (ArchetypeID, bool) {
	rec, ok := w.records[e.ID()]
	if !ok {
		return 0, false
	}
	return rec.archetypeID, true
}

// Despawn removes the entity from the World, evicting it from its archetype
// (table row + sparse-set slots) and freeing its ID. Returns ErrEntityNotAlive
// if the entity is already dead.
func (w *World) Despawn(e entity.Entity) error {
	if !w.entities.IsAlive(e) {
		return ErrEntityNotAlive
	}
	if rec, ok := w.records[e.ID()]; ok {
		arch := w.archetypes.get(rec.archetypeID)
		w.removeEntityFromArchetype(arch, e, rec.row, true)
		delete(w.records, e.ID())
	}
	w.entities.Free(e)
	return nil
}

// ChangeTick returns the current global change tick.
func (w *World) ChangeTick() Tick { return w.changeTick }

// LastChangeTick returns the tick value at the last ClearTrackers call.
func (w *World) LastChangeTick() Tick { return w.lastChangeTick }

// IncrementChangeTick advances the global tick by one and returns the new
// value. Called by the schedule executor before each system run.
func (w *World) IncrementChangeTick() Tick {
	w.changeTick++
	return w.changeTick
}

// ClearTrackers advances lastChangeTick to the current changeTick and drains
// any registered [changedetect.RemovedComponents] trackers, pruning entries
// outside the two-frame retention window. Called once per frame, typically at
// the end of the update schedule (wired by ChangeDetectionPlugin in T-2F03).
func (w *World) ClearTrackers() {
	w.lastChangeTick = w.changeTick
	if pp, ok := Resource[*changedetect.RemovedRegistry](w); ok && pp != nil {
		(*pp).DrainAll(changedetect.Tick(w.lastChangeTick))
	}
}

// RegisterDeferredFlusher installs fn as a deferred flusher. Every call to
// [World.ApplyDeferred] invokes the registered flushers in registration order.
// Used by long-lived [command.CommandBuffer] instances to participate in the
// schedule's apply-point flush. Registration is intended to happen at world
// setup time (single-threaded); concurrent registration during a flush is a
// programming error.
func (w *World) RegisterDeferredFlusher(fn func(*World)) {
	if fn == nil {
		return
	}
	w.deferredFlushers = append(w.deferredFlushers, fn)
}

// ApplyDeferred invokes every registered deferred flusher in registration
// order. Called by the schedule executor at sync points after all systems in
// a tick have run. Idempotent when no flushers are registered.
func (w *World) ApplyDeferred() {
	for _, fn := range w.deferredFlushers {
		fn(w)
	}
}

// ResetDeferredFlushers drops every registered flusher. Intended for tests
// and teardown only; production code registers flushers once at setup.
func (w *World) ResetDeferredFlushers() {
	w.deferredFlushers = w.deferredFlushers[:0]
}
