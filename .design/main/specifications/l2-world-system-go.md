# World System — Go Implementation

**Version:** 0.1.0
**Status:** Draft
**Layer:** go
**Implements:** [world-system.md](l1-world-system.md)

## Overview

This specification defines the Go implementation of the World — the central data store that owns all entities, components, resources, archetypes, and schedules.

- **Chunk-based Allocation**: Sparse Sets and Tables MUST allocate memory in fixed blocks (e.g., 16KB) to ensure data locality and minimize pointer chasing.
- **Reactive Hooks**: The World provides `OnAdd` and `OnRemove` signals for each component type, enabling observer-pattern behaviors for subsystems.
- **Entity Lifecycle**: Uses a 64-bit ID with a 32/32 index-to-generation split for safe ID recycling.
It leverages Go 1.23+ features such as the `unique` package for identity management and modern `iter` patterns for inspection.

## Related Specifications

- [world-system.md](l1-world-system.md) — L1 concept specification (parent)
- [task-system-go.md](l1-task-system.md) — Uses task system for parallel archetype iteration

## 1. Motivation

A concrete Go implementation of the World is needed to:

- Provide the actual data structures (archetypes, tables, sparse sets) for component storage.
- Implement the safety mechanisms described in the concept spec using Go's type system and memory model.
- Enable high-performance entity and component access via raw memory blocks and unsafe pointers.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: Relies on modern Go features like `unique`, `iter`, and `simd`.
- **Memory Locality**: Storage MUST remain contiguous to maximize cache hits.
- **Pinning**: Large component blocks are managed in a way that minimizes GC pressure (reuse via sync.Pool).

## 3. Core Invariants

> [!NOTE]
> See [world-system.md §3](l1-world-system.md) for technology-agnostic invariants.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: Entity belongs to one World | Enforced by internal ID mapping; IDs are unique per World instance. |
| **INV-2**: Archetype storage | The `ArchetypeStore` ensures entities with same component set share storage. |
| **INV-3**: Monotonic change tick | `Tick` is a `uint32` incremented only within `RunSchedule`. |
| **INV-4**: ClearTrackers timing | The `World.ClearTrackers()` method is the only entry point for reset. |
| **INV-5**: DeferredWorld | A dedicated `DeferredWorld` struct limits available methods. |

## Go Package

```plaintext
internal/ecs/
```

All types in this spec belong to package `ecs`.

## Type Definitions

### Tick

```go
// Tick is a monotonically increasing counter used for change detection.
// Incremented each time a system runs. Never wraps within a single run.
type Tick uint32
```

### World

```go
// World is the central data store of the Neu engine. It owns all entities,
// component storage, resources, and schedules. Not thread-safe — concurrent
// access must be coordinated by the schedule executor.
type World struct {
    entities        *EntityAllocator
    components      *ComponentRegistry
    archetypes      *ArchetypeStore
    tables          *TableStore
    sparseSets      *SparseSetStore
    resources       *ResourceMap
    schedules       map[string]*Schedule
    changeTick      Tick           // current global change tick
    lastChangeTick  Tick           // tick at last ClearTrackers call
}

// NewWorld creates a World with default initial capacity.
func NewWorld() *World

// NewWorldWithCapacity creates a World pre-allocated for the expected
// number of entities and component types.
func NewWorldWithCapacity(entityCapacity int, componentCapacity int) *World
```

### ArchetypeID and Archetype

```go
// ArchetypeID uniquely identifies an archetype within a World.
type ArchetypeID uint32

// Archetype represents a unique combination of component types.
// All entities with the same set of components share an archetype.
type Archetype struct {
    id            ArchetypeID
    componentIDs  []ComponentID    // sorted, defines the archetype identity
    tableID       TableID          // associated table for Table-stored components
    entities      []Entity         // entities in this archetype
    edges         map[ComponentID]ArchetypeEdge  // add/remove component -> target archetype
}

// ArchetypeEdge caches the target archetype when adding or removing a component.
type ArchetypeEdge struct {
    Add    ArchetypeID  // archetype after adding this component
    Remove ArchetypeID  // archetype after removing this component
}

// ArchetypeStore manages all archetypes and provides lookup by component set.
type ArchetypeStore struct {
    archetypes  []Archetype
    index       map[archetypeKey]ArchetypeID  // component set hash -> archetype ID
    generation  uint32  // incremented when a new archetype is created
}
```

### Table Storage

```go
// TableID uniquely identifies a table within a World.
type TableID uint32

// Table is column-oriented storage for Table-stored components.
// Each column is a contiguous byte slice holding component data for all
// entities in the associated archetype. Rows correspond to entities.
type Table struct {
    id       TableID
    columns  map[ComponentID]*Column  // one column per Table-stored component
    entities []Entity                 // row index -> entity mapping
    len      int                      // number of occupied rows
}

// Column is a type-erased contiguous array storing values of one component type.
type Column struct {
    componentID  ComponentID
    data         []byte        // raw storage: len = capacity * elemSize
    elemSize     uintptr       // size of one element
    changeTicks  []Tick        // per-row: tick of last mutation
    addedTicks   []Tick        // per-row: tick when component was added
    len          int
    cap          int
}

// TableStore manages all tables.
type TableStore struct {
    tables []Table
}
```

### Sparse Set Storage

```go
// SparseSet stores components indexed by entity for O(1) add/remove.
type SparseSet struct {
    componentID  ComponentID
    dense        []byte       // packed component data
    denseEntities []Entity    // parallel to dense: which entity owns each slot
    sparse       map[uint32]int  // entity index -> dense index
    elemSize     uintptr
    changeTicks  []Tick
    addedTicks   []Tick
    len          int
}

// SparseSetStore manages all sparse sets (one per SparseSet-stored component type).
type SparseSetStore struct {
    sets map[ComponentID]*SparseSet
}
```

### ResourceMap

```go
// ResourceMap stores global singleton resources keyed by Go type.
// Supports concurrent read access via RWMutex.
type ResourceMap struct {
    mu    sync.RWMutex
    store map[reflect.Type]any
}

func NewResourceMap() *ResourceMap
```

### Reactive Hooks (Observers)

To avoid frame-by-frame polling, the `World` maintains a registry of reactive hooks:

```go
type LifecycleHook func(Entity, any)

func (w *World) OnAdd[T any](handler LifecycleHook)
func (w *World) OnRemove[T any](handler LifecycleHook)
```

- **Triggering**: `OnAdd` fires immediately after component insertion; `OnRemove` fires immediately before removal.
- **Usage**: Typically used by the Render system to register new meshes or the Physics system to create rigid bodies.

### DeferredWorld

```go
// DeferredWorld provides limited World access for use inside component hooks
// and observers. It allows reading/writing existing components and resources,
// but forbids structural changes (spawn, despawn, add/remove components).
type DeferredWorld struct {
    world *World
}

// Get retrieves a read-only pointer to component T on the given entity.
func Get[T any](dw *DeferredWorld, entity Entity) (*T, bool)

// GetMut retrieves a mutable pointer to component T on the given entity.
// Marks the component as changed for change detection.
func GetMut[T any](dw *DeferredWorld, entity Entity) (*T, bool)

// Resource retrieves a read-only pointer to resource T.
func Resource[T any](dw *DeferredWorld) (*T, bool)

// SetResource sets a resource value of type T.
func SetResource[T any](dw *DeferredWorld, value T)
```

## Key Methods

### Entity Operations

```go
// Spawn creates a new entity with the given components.
// Registers any unregistered component types automatically.
// Resolves required components and inserts defaults for missing ones.
// Fires OnAdd and OnInsert hooks for each component.
//
// Pseudo-code:
//   entity = world.entities.Allocate()
//   resolve all required components, add defaults for missing
//   find or create archetype matching full component set
//   insert entity into archetype's table
//   fire hooks in dependency order
//   return entity
func (w *World) Spawn(components ...ComponentData) Entity

// SpawnEmpty creates an entity with no components.
func (w *World) SpawnEmpty() Entity

// Despawn removes an entity and all its components.
// Fires OnRemove hooks before removing data.
//
// Pseudo-code:
//   verify entity is alive
//   fire OnRemove hooks for each component
//   remove entity from archetype table (swap-remove)
//   free entity ID in allocator
func (w *World) Despawn(entity Entity)

// Contains reports whether the entity is alive in this World.
func (w *World) Contains(entity Entity) bool

// Entity returns a read-only EntityRef for the given entity.
func (w *World) Entity(entity Entity) (EntityRef, error)

// EntityMut returns a mutable EntityMut for the given entity.
func (w *World) EntityMut(entity Entity) (EntityMut, error)
```

### Component Operations

```go
// Insert adds or overwrites a component on an existing entity.
// May cause an archetype move if the entity gains a new component type.
//
// Pseudo-code:
//   if entity already has this component:
//     overwrite value in place
//     fire OnReplace, OnInsert hooks
//   else:
//     resolve required components
//     compute new archetype (old components + new ones)
//     move entity from old archetype table to new
//     fire OnAdd, OnInsert hooks for new components
func (w *World) Insert(entity Entity, data ...ComponentData) error

// Remove removes component T from an entity. May cause an archetype move.
// Fires OnRemove hook before removal.
func Remove[T any](w *World, entity Entity) error

// Get retrieves a read-only pointer to component T on the given entity.
// Returns nil, false if the entity does not have the component.
func Get[T any](w *World, entity Entity) (*T, bool)

// GetMut retrieves a mutable pointer to component T on the given entity.
// Marks the component as changed (updates change tick).
func GetMut[T any](w *World, entity Entity) (*T, bool)
```

### Resource Operations

```go
// SetResource inserts or overwrites a global resource of type T.
func SetResource[T any](w *World, value T)

// Resource retrieves a read-only pointer to resource T.
// Returns nil, false if the resource has not been set.
func Resource[T any](w *World) (*T, bool)

// RemoveResource removes the resource of type T.
func RemoveResource[T any](w *World) bool

// ContainsResource reports whether a resource of type T exists.
func ContainsResource[T any](w *World) bool
```

### Schedule Execution

```go
// AddSchedule registers a named schedule with the World.
func (w *World) AddSchedule(name string, schedule *Schedule)

// RunSchedule executes all systems in the named schedule.
// Increments the change tick before each system run.
//
// Pseudo-code:
//   schedule = world.schedules[name]
//   for each system in schedule's execution order:
//     world.changeTick++
//     system.Run(world)
//     apply deferred commands
func (w *World) RunSchedule(name string) error

// All returns an iterator over all entities in the World.
func (w *World) All() iter.Seq[Entity]

// RunSystemOnce runs a single system function immediately with World access.
func (w *World) RunSystemOnce(system SystemFunc) error
```

### Change Tick Management

```go
// ChangeTick returns the current change tick value.
func (w *World) ChangeTick() Tick

// LastChangeTick returns the tick at the last ClearTrackers call.
func (w *World) LastChangeTick() Tick

// IncrementChangeTick atomically increments and returns the new tick.
func (w *World) IncrementChangeTick() Tick

// ClearTrackers advances lastChangeTick to current changeTick.
// Called once per update cycle, typically at the end of the frame.
func (w *World) ClearTrackers()
```

### Archetype Management

```go
// FindOrCreateArchetype returns the archetype matching the given sorted
// component set. Creates a new archetype (and table) if none exists.
// Increments ArchetypeStore.generation on creation (invalidates query caches).
//
// Pseudo-code:
//   key = hash(sorted componentIDs)
//   if exists in index: return existing
//   create new Archetype with new ArchetypeID
//   create corresponding Table with columns for each Table-stored component
//   update archetype graph edges for neighboring archetypes
//   increment generation
//   return new archetype
func (w *World) FindOrCreateArchetype(componentIDs []ComponentID) *Archetype
```

## Performance Strategy

- **Archetype graph edges**: Adding/removing a component uses cached edges to find the target archetype in O(1) — no hash lookup needed after first traversal.
- **Table swap-remove**: Removing an entity from a table swaps it with the last row, O(1) removal without shifting data.
- **Column storage as `[]byte`**: Avoids interface boxing. Component data is accessed via `unsafe.Pointer` arithmetic on the hot path.
- **Pre-allocated capacity**: `NewWorldWithCapacity` avoids runtime allocations during gameplay.
- **Change tick per column row**: Stored in parallel `[]Tick` slices, no extra indirection.
- **Resource RWMutex**: Multiple systems can read the same resource concurrently; writes are exclusive.
- **Archetype generation counter**: Query caches check generation to avoid full re-scan of archetypes.

## Error Handling

- `Spawn` with unregistered component: auto-registers via `RegisterComponent`. No error.
- `Despawn` on dead entity: returns `ErrEntityNotAlive` sentinel error (does not panic).
- `Insert` on dead entity: returns `ErrEntityNotAlive`.
- `Get`/`GetMut` on dead entity: returns `nil, false`.
- `RunSchedule` with unknown name: returns `ErrScheduleNotFound`.
- Resource not found: returns `nil, false` (not an error — resources are optional).

```go
var (
    ErrEntityNotAlive    = errors.New("ecs: entity is not alive")
    ErrScheduleNotFound  = errors.New("ecs: schedule not found")
    ErrComponentNotFound = errors.New("ecs: component not found on entity")
)
```

## Testing Strategy

- **World lifecycle**: Create world, spawn entities, verify Contains, despawn, verify not alive.
- **Component storage**: Spawn with Table components, verify column data. Spawn with SparseSet components, verify sparse set data.
- **Archetype transitions**: Insert new component on entity, verify archetype change. Remove component, verify move back.
- **Resource CRUD**: Set, get, overwrite, remove resources. Verify type isolation.
- **Change ticks**: Verify tick increment on system run, verify ClearTrackers resets correctly.
- **DeferredWorld**: Verify allowed operations succeed, verify structural operations are prevented.
- **Archetype graph edges**: Verify cached edges are correct after multiple add/remove patterns.
- **Benchmarks**: `BenchmarkSpawn` (single and batch), `BenchmarkDespawn`, `BenchmarkGet`, `BenchmarkInsert`, `BenchmarkArchetypeTransition`.
- **Stress test**: Spawn 100K entities with varying component sets, verify archetype correctness.

## 7. Drawbacks & Alternatives

- **Drawback**: Archetype transitions (add/remove component) are expensive due to data copying between tables.
- **Alternative**: An all-SparseSet approach would avoid copies but kill cache locality for iteration.
- **Decision**: Hybrid approach (Table default, SparseSet optional) provides the best balance for game workloads.

## Canonical References

<!-- MANDATORY for Stable status. List authoritative source files that downstream agents
     MUST read before implementing this spec. Use relative paths from project root.
     Stub state — fill with concrete files when implementation begins (Phase 1+). -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

<!-- Empty table = no canonical sources yet. Populate one row per authoritative file
     when implementation lands (Phase 1+). Stable promotion requires ≥1 row. -->

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-03-26 | Initial L2 draft |
| 0.2.0 | 2026-05-14 | Phase 1 implementation complete; validated by `examples/ecs/poc` |
