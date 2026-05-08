# Entity System — Go Implementation

**Version:** 0.1.0
**Status:** Draft
**Layer:** go
**Implements:** [entity-system.md](l1-entity-system.md)

## Overview

This specification defines the Go implementation of the entity system described in the L1 concept spec. Entities are lightweight generational indices packed into a single `uint64`. The entity allocator uses a freelist-based arena for O(1) allocation and deallocation with generational safety against stale references.

## Related Specifications

- [entity-system.md](l1-entity-system.md) — L1 concept specification (parent)

## 1. Motivation

The Go implementation of the Entity system provides the core identity mechanism for the engine. It ensures:

- Extremely lightweight (8-byte) identifiers that fit in CPU registers.
- Generational safety to prevent "stale reference" bugs where a new entity is mistaken for a deleted one.
- O(1) allocation and deallocation performance.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: Uses `unique` for internal labels if needed, though pure `uint64` is preferred for IDs.
- **Memory Efficiency**: Entity IDs are packed into a single word to avoid pointer overhead in large slices.
- **Generation limit**: A `uint32` generation counter is assumed to never wrap for a single slot in a session (~4 billion reuses).

## 3. Core Invariants

> [!NOTE]
> See [entity-system.md §3](l1-entity-system.md) for technology-agnostic invariants.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: Unique ID | `EntityID` is a monotonically increasing index paired with a generation counter. |
| **INV-2**: Generational shift | `EntityAllocator.Free` increments the generation counter for the slot. |
| **INV-3**: Null entity | `EntityID(0)` is reserved as the null sentinel; index 0 starts at generation 1. |
| **INV-4**: Valid check | `IsAlive` compares the provided generation with the current slot generation. |

## Go Package

```
internal/ecs/
```

All types in this spec belong to package `ecs`.

## Type Definitions

### EntityID

```go
// EntityID is a packed 64-bit identifier: lower 32 bits = index, upper 32 bits = generation.
// The zero value (0) represents an invalid/null entity.
type EntityID uint64
```

Bit packing layout:

```
  63              32 31               0
  +-----------------+-----------------+
  |   generation    |     index       |
  +-----------------+-----------------+
```

Helper functions for packing/unpacking:

```go
// NewEntityID constructs an EntityID from an index and generation.
func NewEntityID(index uint32, generation uint32) EntityID

// Index returns the lower 32 bits (slot index).
func (id EntityID) Index() uint32

// Generation returns the upper 32 bits (generation counter).
func (id EntityID) Generation() uint32
```

### Entity

```go
// Entity wraps an EntityID and provides convenience methods.
// The zero value Entity{} is the invalid/null entity sentinel.
type Entity struct {
    id EntityID
}

// NewEntity creates an Entity from index and generation.
func NewEntity(index uint32, generation uint32) Entity

// ID returns the packed EntityID.
func (e Entity) ID() EntityID

// Index returns the entity's slot index.
func (e Entity) Index() uint32

// Generation returns the entity's generation counter.
func (e Entity) Generation() uint32

// IsValid reports whether the entity is not the zero/null sentinel.
func (e Entity) IsValid() bool
```

Zero-value semantics: `Entity{}` has `id == 0`, meaning index 0 and generation 0. Index 0 with generation 0 is reserved as the null sentinel and never allocated to a live entity. The allocator starts generation at 1 for slot 0.

### EntityAllocator

```go
// EntityAllocator manages entity ID allocation and recycling using a
// generational freelist arena. Not thread-safe — must be used under
// exclusive access (main thread or World lock).
type EntityAllocator struct {
    generations []uint32  // generation counter per slot index
    freeList    []uint32  // LIFO stack of available indices
    len         uint32    // number of currently alive entities
}

// NewEntityAllocator creates an allocator with pre-allocated capacity.
func NewEntityAllocator(capacity int) *EntityAllocator

// Allocate reserves a new Entity. Pops from freelist or extends the arena.
// Returns Entity with current generation for the assigned slot.
func (a *EntityAllocator) Allocate() Entity

// Free releases an Entity. Increments the slot's generation and pushes
// the index onto the freelist. Panics if entity is already dead (debug mode).
func (a *EntityAllocator) Free(entity Entity)

// IsAlive reports whether the given Entity matches the current generation
// for its slot. Returns false for the null entity.
func (a *EntityAllocator) IsAlive(entity Entity) bool

// Len returns the number of currently alive entities.
func (a *EntityAllocator) Len() int

// Reserve pre-allocates capacity for n entities without actually allocating them.
// Useful for batch spawn hints.
func (a *EntityAllocator) Reserve(n int)
```

### Entity Collections

```go
// EntitySet is an unordered set of entities with O(1) insert, remove, contains.
// Implemented as a dense array + sparse index map keyed by EntityID.
type EntitySet struct {
    dense  []Entity
    sparse map[EntityID]int  // EntityID -> index in dense
}

func NewEntitySet() *EntitySet
func (s *EntitySet) Insert(entity Entity) bool   // returns false if already present
func (s *EntitySet) Remove(entity Entity) bool   // returns false if not found
func (s *EntitySet) Contains(entity Entity) bool
func (s *EntitySet) Len() int
func (s *EntitySet) Iter(fn func(Entity))         // callback-based iteration
func (s *EntitySet) Clear()

// EntityMap is a generic entity-keyed map with O(1) operations.
type EntityMap[V any] struct {
    entries map[EntityID]V
}

func NewEntityMap[V any]() *EntityMap[V]
func (m *EntityMap[V]) Set(entity Entity, value V)
func (m *EntityMap[V]) Get(entity Entity) (V, bool)
func (m *EntityMap[V]) Remove(entity Entity) bool
func (m *EntityMap[V]) Contains(entity Entity) bool
func (m *EntityMap[V]) Len() int
func (m *EntityMap[V]) Iter(fn func(Entity, V))
func (m *EntityMap[V]) Clear()
```

### Entity References

```go
// EntityRef provides read-only access to an entity's components within a World.
type EntityRef struct {
    entity    Entity
    world     *World
    archetype *Archetype
    row       uint32
}

func (r EntityRef) Entity() Entity
func (r EntityRef) Contains(componentID ComponentID) bool

// EntityMut provides read-write access to an entity's components.
// Does not permit structural changes (add/remove components).
type EntityMut struct {
    entity    Entity
    world     *World
    archetype *Archetype
    row       uint32
}

func (m EntityMut) Entity() Entity
func (m EntityMut) Contains(componentID ComponentID) bool
```

## Key Methods

### Allocation Strategy

1. `Allocate()` pops from the LIFO freelist. If freelist is empty, extend `generations` slice by one slot and use the new index.
2. Slot 0, generation 0 is reserved as null. The allocator initializes slot 0 with generation 1 so it is never returned as `Entity{}`.
3. `Free()` increments `generations[index]` and pushes index onto freelist.
4. `IsAlive()` compares `entity.Generation()` with `generations[entity.Index()]`. Returns false if index is out of bounds.

### Batch Operations

```go
// AllocateMany allocates n entities in a single batch, returning them in a slice.
// More efficient than n individual Allocate calls due to single capacity check.
func (a *EntityAllocator) AllocateMany(n int) []Entity
```

## Performance Strategy

- **Freelist is a `[]uint32` stack**: LIFO ordering improves cache locality for recently freed slots.
- **Pre-allocation**: `NewEntityAllocator(capacity)` and `Reserve(n)` avoid runtime slice growth on the hot path.
- **No map lookups on hot path**: `IsAlive` is a bounds check + array index, O(1) with no hash computation.
- **EntityID as uint64**: Fits in a register, cheap to copy and compare. Used as map key without allocation.
- **EntitySet sparse map**: For large sets, consider replacing `map[EntityID]int` with a flat array indexed by `entity.Index()` to avoid hash overhead. This is an optimization left to implementation phase.

## Error Handling

- `Free()` on an already-dead entity: panic in debug builds (build tag `ecsdebug`), silent no-op in release.
- `IsAlive()` on out-of-bounds index: returns `false` (no panic).
- Generation overflow (`uint32` wraparound): practically impossible (4 billion reuses of a single slot). Log a warning if detected.
- All public methods validate the null entity (`Entity{}`) and return appropriate zero values or false.

## Testing Strategy

- **Unit tests**: Allocate/Free cycle, generation increment, IsAlive for stale entities, null entity behavior.
- **Freelist correctness**: Verify LIFO ordering after mixed alloc/free sequences.
- **Boundary tests**: Maximum index allocation, generation overflow detection.
- **Benchmarks**: `BenchmarkAllocate`, `BenchmarkFree`, `BenchmarkIsAlive` — target zero allocations.
- **EntitySet/EntityMap**: Insert/remove/contains correctness and performance with 10K+ entities.
- **Fuzz tests**: Random sequences of Allocate/Free to verify invariant INV-1 through INV-4.

## 7. Drawbacks & Alternatives

- **Drawback**: Packing ID into `uint64` limits the maximum number of entities to 4 billion.
- **Alternative**: 128-bit UUIDs for entities.
- **Decision**: 64-bit packed IDs are much faster and sufficient for even the largest game worlds.

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
