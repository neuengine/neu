# Query System — Go Implementation

**Version:** 0.1.0
**Status:** Draft
**Layer:** go
**Implements:** [query-system.md](l1-query-system.md)

## Overview

This specification defines the Go implementation of the query system. Queries are the primary way systems access entity data. Due to Go's lack of variadic generics, multi-component queries require explicit arity types (`Query1`, `Query2`, `Query3`, etc.). Each query tracks its read/write access for schedule-time conflict detection and caches matched archetypes for efficient iteration. It leverages Go 1.23+ features such as modern `iter` patterns for inspection.

## Related Specifications

- [query-system.md](l1-query-system.md) — L1 concept specification (parent)

## 1. Motivation

The Go implementation of the Query system provides the primary mechanism for efficient entity data access. It ensures:

- Type-safe iteration over entities with specific component sets.
- High-performance caching of matched archetypes to avoid repeated searches.
- Parallel iteration capabilities for massive entity counts.
- Seamless integration with Go 1.23+ `iter` patterns for modern ergonomics.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: Relies on generics for type-safe queries and `iter` for range-over-func loops.
- **Arity limits**: Fixed-arity types (`Query1` to `Query8`) are used to work around Go's variadic generic limitations.
- **Cache validity**: Query caches are invalidated only when new archetypes are created or destroyed.

## 3. Core Invariants

> [!NOTE]
> See [query-system.md §3](l1-query-system.md) for technology-agnostic invariants.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: Filter matching | `QueryState.Matches` correctly identifies archetypes containing all required components. |
| **INV-2**: Cached iteration | `matchedArchetypes` stores direct column pointers for O(1) row access. |
| **INV-3**: Read/Write Isolation | `Access` metadata is used by the scheduler to group non-conflicting systems. |
| **INV-4**: Deferred Safety | Query results are immutable regarding structural changes during iteration. |
| **INV-5**: Parallel Scaling | `ParIter` utilizes `errgroup` to distribute work across CPU cores safely. |

## Go Package

```
internal/ecs/
```

All types in this spec belong to package `ecs`.

## Type Definitions

### Access

```go
// Access describes the component read/write requirements of a query or system.
// Used by the scheduler to detect conflicts and enable parallelism.
type Access struct {
    ReadSet  []ComponentID  // components read (immutable access)
    WriteSet []ComponentID  // components written (mutable access)
}

// IsDisjoint reports whether two Access sets have no conflicting overlap.
// Conflict = one writes a component that the other reads or writes.
func (a Access) IsDisjoint(other Access) bool

// Merge combines two Access values into one (union of sets).
func (a Access) Merge(other Access) Access
```

### QueryFilter

```go
// QueryFilter is an interface for filtering which archetypes/entities
// a query matches.
type QueryFilter interface {
    // Matches reports whether the given archetype passes this filter.
    Matches(archetype *Archetype) bool
}

// TickFilter extends QueryFilter with tick-based per-entity filtering.
// Used by Changed and Added filters that need per-row tick comparison.
type TickFilter interface {
    QueryFilter
    // MatchesEntity reports whether a specific entity row passes
    // the tick-based condition.
    MatchesEntity(column *Column, row int, lastRun Tick, thisTick Tick) bool
}
```

### Concrete Filters

```go
// With filters for entities that have component T.
// T is not fetched — only used for archetype matching.
type With[T any] struct{}

func (f With[T]) Matches(archetype *Archetype) bool
// Pseudo-code: return archetype.HasComponent(componentIDOf[T]())

// Without filters for entities that do NOT have component T.
type Without[T any] struct{}

func (f Without[T]) Matches(archetype *Archetype) bool
// Pseudo-code: return !archetype.HasComponent(componentIDOf[T]())

// Changed filters for entities where component T was mutated since the
// system's last run. Archetype-level: matches if archetype has T.
// Entity-level: compares the component's change tick against lastRun tick.
type Changed[T any] struct{}

func (f Changed[T]) Matches(archetype *Archetype) bool
func (f Changed[T]) MatchesEntity(column *Column, row int, lastRun Tick, thisTick Tick) bool
// Pseudo-code: return column.changeTicks[row] > lastRun

// Added filters for entities where component T was added since the
// system's last run.
type Added[T any] struct{}

func (f Added[T]) Matches(archetype *Archetype) bool
func (f Added[T]) MatchesEntity(column *Column, row int, lastRun Tick, thisTick Tick) bool
// Pseudo-code: return column.addedTicks[row] > lastRun
```

### QueryState (Single Component)

```go
// QueryState1 is the cached state for a single-component query.
// It tracks which archetypes match and caches column pointers.
type QueryState1[T any] struct {
    componentID      ComponentID
    access           Access
    filters          []QueryFilter
    matchedArchetypes []matchedArchetype
    archetypeGen     uint32  // archetype generation at last cache update
    lastRunTick      Tick    // tick when this query's system last ran
}

type matchedArchetype struct {
    archetypeID  ArchetypeID
    tableID      TableID
    columnIndex  int  // index of the target component column in the table
}

// NewQueryState1 creates a new single-component query state.
// Registers the component type if needed.
func NewQueryState1[T any](world *World, filters ...QueryFilter) *QueryState1[T]
```

### Multi-Component Query Types

Due to Go's lack of variadic generics, multi-component queries use separate types per arity:

```go
// QueryState2 is the cached state for a two-component query.
type QueryState2[A, B any] struct {
    componentIDs     [2]ComponentID
    access           Access
    filters          []QueryFilter
    matchedArchetypes []matchedArchetype2
    archetypeGen     uint32
    lastRunTick      Tick
}

// QueryState3 is the cached state for a three-component query.
type QueryState3[A, B, C any] struct {
    componentIDs     [3]ComponentID
    access           Access
    filters          []QueryFilter
    matchedArchetypes []matchedArchetype3
    archetypeGen     uint32
    lastRunTick      Tick
}

// QueryState4 is the cached state for a four-component query.
type QueryState4[A, B, C, D any] struct {
    componentIDs     [4]ComponentID
    access           Access
    filters          []QueryFilter
    matchedArchetypes []matchedArchetype4
    archetypeGen     uint32
    lastRunTick      Tick
}

// Additional arity types up to QueryState8 follow the same pattern.
// Beyond 8, users should restructure their data or use multiple queries.
```

### Query Mutable Access Markers

```go
// Mut is a marker type wrapping a component type to indicate mutable access.
// Used as a type parameter: QueryState2[Position, Mut[Velocity]]
type Mut[T any] struct{}
```

When `Mut[T]` is used as a type parameter, the query registers T in the WriteSet instead of ReadSet. The iteration callback receives `*T` (same pointer type) but the component's change tick is updated.

## Key Methods

### Iteration (Go 1.23+ Range-over-func)

Instead of callbacks, queries utilize Go 1.23 iterators for use with `range`.

```go
// All returns an iterator over all matching entities and their component.
//
// Usage:
//   ```go
// for entity, transform := range world.Query2[Entity, *Transform]() {
//     // ...
// }
// ```
func (q *QueryState1[T]) All(world *World) iter.Seq2[Entity, *T]

// All for two-component query. Note: Go 1.23 standard iterators support
// up to 2 values. For 3+ components, a struct-tuple is yielded.
func (q *QueryState2[A, B]) All(world *World) iter.Seq2[Entity, Tuple2[*A, *B]]

type Tuple2[A, B any] struct {
    A A
    B B
}

// All for three-component query.
func (q *QueryState3[A, B, C]) All(world *World) iter.Seq2[Entity, Tuple3[*A, *B, *C]]

type Tuple3[A, B, C any] struct {
    A A
    B B
    C C
}
```

### Single Entity Lookup

```go
// Get retrieves the component for a specific entity.
// Returns an error if the entity is dead or does not match the query.
func (q *QueryState1[T]) Get(world *World, entity Entity) (*T, error)

// Get for two-component query.
func (q *QueryState2[A, B]) Get(world *World, entity Entity) (*A, *B, error)
```

### Parallel Iteration

```go
// ParIter divides work across goroutines by splitting matched archetypes
// into chunks. Each goroutine processes a chunk independently.
//
// Pseudo-code:
//   updateCache(world)
//   collect all (archetype, table) pairs
//   divide rows into chunks of size >= minChunkSize
//   launch one goroutine per chunk via errgroup
//   each goroutine iterates its row range and calls fn
//
// Safety: fn must not perform structural World changes. The query's Access
// must not conflict with any concurrently running query.
func (q *QueryState1[T]) ParIter(world *World, fn func(Entity, *T))

// minChunkSize is the minimum number of entities per goroutine to avoid
// excessive goroutine overhead.
const minChunkSize = 256
```

### Cache Invalidation

```go
// updateCache checks the World's archetype generation counter.
// If new archetypes have been created since the last update,
// scans new archetypes for matches and appends to the cache.
//
// Pseudo-code:
//   if world.archetypes.generation == q.archetypeGen: return
//   for each new archetype since q.archetypeGen:
//     if archetype contains all required components:
//       if all filters match:
//         append to matchedArchetypes
//   q.archetypeGen = world.archetypes.generation
func (q *QueryState1[T]) updateCache(world *World)
```

### Access Conflict Detection

```go
// Conflicts reports whether two query accesses conflict.
// Used at schedule build time to determine system ordering.
//
// Conflict rules:
//   - Write-Write on same ComponentID: conflict
//   - Write-Read on same ComponentID: conflict
//   - Read-Read: no conflict
func (a Access) Conflicts(other Access) bool
```

Schedule-time validation (pseudo-code):

```go
func validateSchedule(systems []System) error {
    for each pair (sysA, sysB) in systems:
        if sysA.Access().Conflicts(sysB.Access()):
            if no explicit ordering between sysA and sysB:
                return error("ambiguous system order: {sysA} and {sysB} conflict on {component}")
    return nil
}
```

### Count and Single

```go
// Count returns the number of entities matching the query.
func (q *QueryState1[T]) Count(world *World) int

// Single asserts that exactly one entity matches. Returns it directly.
// Returns an error if zero or more than one entity matches.
func (q *QueryState1[T]) Single(world *World) (Entity, *T, error)
```

## Performance Strategy

- **Archetype cache**: `matchedArchetypes` stores pre-resolved archetype/table/column references. Iteration avoids any map lookups.
- **Incremental cache update**: Only new archetypes are scanned (since last `archetypeGen`), not all archetypes.
- **Column pointer arithmetic**: Component access via `unsafe.Pointer` on contiguous `[]byte`. No interface boxing during iteration.
- **Tick filter short-circuit**: Archetype-level `Matches()` filters entire archetypes before row-level tick checks. Empty archetypes skipped.
- **ParIter chunk sizing**: Minimum 256 entities per goroutine prevents overhead from dominating small workloads.
- **Go 1.23 iterators**: Yield-based iteration (`iter.Seq2`) is highly optimized by the compiler and avoids closure capture overhead where possible.
- **Access sets as sorted slices**: Conflict detection uses merge-join on sorted `[]ComponentID`, O(n+m) instead of O(n*m).

## Error Handling

- `Get` on dead entity: returns `ErrEntityNotAlive`.
- `Get` on entity without matching components: returns `ErrComponentNotFound`.
- `Single` with zero matches: returns `ErrQueryNoMatch`.
- `Single` with multiple matches: returns `ErrQueryMultipleMatches`.
- Access conflict detected at schedule build time: returns error with descriptive message naming the conflicting systems and components.
- `ParIter` panics are caught per-goroutine and returned as errors via `errgroup`.

```go
var (
    ErrQueryNoMatch        = errors.New("ecs: query matched zero entities")
    ErrQueryMultipleMatches = errors.New("ecs: query matched more than one entity")
)
```

## Testing Strategy

- **Basic iteration**: Spawn entities with known components, iterate via query, verify all and only matching entities are visited.
- **Filter correctness**: `With`, `Without`, `Changed`, `Added` — verify correct filtering at archetype and entity level.
- **Cache invalidation**: Spawn entities, iterate, spawn more entities with new component sets (creating new archetypes), iterate again — verify new entities included.
- **Get/Single**: Verify correct return for existing, missing, and dead entities.
- **Access conflict detection**: Build test schedules with overlapping/disjoint access sets, verify conflict detection.
- **ParIter correctness**: Run parallel iteration on 10K+ entities, verify every entity visited exactly once (use atomic counter).

### 5.1 Flush Point

Commands are strictly buffered and only applied at explicit synchronization points (Flush).

### 5.2 Thread-Safe Buffering

- **Multi-arity queries**: Test QueryState2, QueryState3, QueryState4 with various component combinations.
- **Mut marker**: Verify mutable access updates change ticks, verify access set reflects write.
- **Benchmarks**: `BenchmarkIter1Component`, `BenchmarkIter3Components`, `BenchmarkParIter`, `BenchmarkQueryGet`, `BenchmarkCacheUpdate`. Compare callback iteration vs. hypothetical iterator alternatives.
- **Fuzz tests**: Random spawn/despawn/query sequences to verify no panics or missed entities.

## 7. Drawbacks & Alternatives

- **Drawback**: Fixed arity limits (up to 8) can be restrictive for extremely complex entity patterns.
- **Alternative**: Reflection-based dynamic queries (type-erased).
- **Decision**: Typed arity queries are significantly faster and cover the vast majority of real-world use cases.

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
