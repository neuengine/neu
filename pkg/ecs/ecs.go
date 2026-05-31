// Package ecs is the public API surface of the Neu engine.
// It re-exports key types from the internal sub-packages behind a stable,
// minimal façade. Internal packages remain the canonical implementation;
// this package is the intended import path for application code and examples.
package ecs

import (
	"github.com/neuengine/neu/internal/ecs/command"
	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/event"
	"github.com/neuengine/neu/internal/ecs/query"
	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
)

// ── Core ──────────────────────────────────────────────────────────────────────

// World is the central ECS data store.
type World = world.World

// NewWorld creates a World with default initial capacities.
func NewWorld() *World { return world.NewWorld() }

// NewWorldWithCapacity creates a World pre-allocated for the expected number
// of entities and component types.
func NewWorldWithCapacity(entityCap, componentCap int) *World {
	return world.NewWorldWithCapacity(entityCap, componentCap)
}

// Entity is the lightweight entity identifier (32-bit index + 32-bit generation).
type Entity = entity.Entity

// ── Component ─────────────────────────────────────────────────────────────────

// Data is a type-erased component value paired with its registry ID.
type Data = component.Data

// Get returns a typed pointer to component T on entity e.
// Returns (nil, false) when the entity is dead or does not carry T.
func Get[T any](w *World, e Entity) (*T, bool) { return world.Get[T](w, e) }

// Insert adds or overwrites components on an existing entity.
func Insert(w *World, e Entity, data ...Data) error { return w.Insert(e, data...) }

// ── Resources ─────────────────────────────────────────────────────────────────

// SetResource stores a singleton resource of type T in the world.
func SetResource[T any](w *World, value T) { world.SetResource(w, value) }

// Resource retrieves a singleton resource of type T.
// Returns (nil, false) if the resource has not been set.
func Resource[T any](w *World) (*T, bool) { return world.Resource[T](w) }

// ── Scheduler ─────────────────────────────────────────────────────────────────

// System is the unit of work the scheduler dispatches each tick.
type System = scheduler.System

// Schedule is the named, DAG-ordered collection of systems.
type Schedule = scheduler.Schedule

// NewSchedule creates an empty named schedule.
func NewSchedule(name string) *Schedule { return scheduler.NewSchedule(name) }

// FuncSystem wraps a name and closure as a System.
type FuncSystem = scheduler.FuncSystem

// NewFuncSystem creates a closure-backed system.
func NewFuncSystem(name string, run func(*World)) *FuncSystem {
	return scheduler.NewFuncSystem(name, run)
}

// ── Commands ──────────────────────────────────────────────────────────────────

// CommandBuffer is the per-system deferred-mutation queue.
type CommandBuffer = command.CommandBuffer

// Commands is the builder façade over a CommandBuffer.
type Commands = command.Commands

// NewCommands wraps buf in a Commands facade.
func NewCommands(buf *CommandBuffer) *Commands { return command.NewCommands(buf) }

// AcquireBuffer rents a CommandBuffer from the pool, wired to w's entity allocator.
func AcquireBuffer(w *World) *CommandBuffer { return command.AcquireBuffer(w.Entities()) }

// ReleaseBuffer returns buf to the pool.
func ReleaseBuffer(buf *CommandBuffer) { command.ReleaseBuffer(buf) }

// ── Events ────────────────────────────────────────────────────────────────────

// EventBus is the double-buffered storage for broadcast events of type T.
type EventBus[T any] = event.EventBus[T]

// RegisterEvent installs an EventBus[T] on w (or returns the existing one).
func RegisterEvent[T any](w *World) *EventBus[T] { return event.RegisterEvent[T](w) }

// SwapAll rotates the double buffer for every registered event bus on w.
func SwapAll(w *World) { event.SwapAll(w) }

// ── Query ─────────────────────────────────────────────────────────────────────

// QueryFilter narrows which archetypes a query matches.
type QueryFilter = query.QueryFilter

// With[T] requires T to be present on matched archetypes without binding it
// as a fetched component.
type With[T any] = query.With[T]

// Without[T] excludes archetypes that contain T.
type Without[T any] = query.Without[T]

// Query1 is a single-component typed query.
type Query1[T any] = query.Query1[T]

// NewQuery1 builds a single-component typed query.
func NewQuery1[T any](w *World, filters ...QueryFilter) (*Query1[T], error) {
	return query.NewQuery1[T](w, filters...)
}

// Query2 is a two-component typed query.
type Query2[A, B any] = query.Query2[A, B]

// NewQuery2 builds a two-component typed query.
func NewQuery2[A, B any](w *World, filters ...QueryFilter) (*Query2[A, B], error) {
	return query.NewQuery2[A, B](w, filters...)
}

// Query3 is a three-component typed query.
type Query3[A, B, C any] = query.Query3[A, B, C]

// NewQuery3 builds a three-component typed query.
func NewQuery3[A, B, C any](w *World, filters ...QueryFilter) (*Query3[A, B, C], error) {
	return query.NewQuery3[A, B, C](w, filters...)
}

// The higher-arity ladder (Query4–Query6) is generated by cmd/codegen/query;
// see internal/ecs/query/query_gen.go.

// Query4 is a four-component typed query.
type Query4[A, B, C, D any] = query.Query4[A, B, C, D]

// NewQuery4 builds a four-component typed query.
func NewQuery4[A, B, C, D any](w *World, filters ...QueryFilter) (*Query4[A, B, C, D], error) {
	return query.NewQuery4[A, B, C, D](w, filters...)
}

// Query5 is a five-component typed query.
type Query5[A, B, C, D, E any] = query.Query5[A, B, C, D, E]

// NewQuery5 builds a five-component typed query.
func NewQuery5[A, B, C, D, E any](w *World, filters ...QueryFilter) (*Query5[A, B, C, D, E], error) {
	return query.NewQuery5[A, B, C, D, E](w, filters...)
}

// Query6 is a six-component typed query.
type Query6[A, B, C, D, E, F any] = query.Query6[A, B, C, D, E, F]

// NewQuery6 builds a six-component typed query.
func NewQuery6[A, B, C, D, E, F any](w *World, filters ...QueryFilter) (*Query6[A, B, C, D, E, F], error) {
	return query.NewQuery6[A, B, C, D, E, F](w, filters...)
}

// ── Event writer/reader ───────────────────────────────────────────────────────

// EventWriter is a send-only handle for an EventBus[T].
type EventWriter[T any] = event.EventWriter[T]

// NewEventWriter resolves the EventBus[T] on w and returns a writer.
// Panics if RegisterEvent has not been called for T.
func NewEventWriter[T any](w *World) *EventWriter[T] { return event.NewEventWriter[T](w) }

// EventReader is a per-cursor read handle for an EventBus[T].
type EventReader[T any] = event.EventReader[T]

// NewEventReaderAt returns a reader positioned at the current send frontier.
// The reader only sees events sent strictly after construction.
func NewEventReaderAt[T any](w *World) *EventReader[T] { return event.NewEventReaderAt[T](w) }
