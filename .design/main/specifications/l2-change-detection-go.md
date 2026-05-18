# Change Detection — Go Implementation

**Version:** 0.1.0
**Status:** Stable
**Layer:** go
**Implements:** [change-detection.md](l1-change-detection.md)

## Overview

This specification defines the Go implementation of the change detection system described in the L1 concept spec. Change detection uses a tick-based mechanism — a monotonically increasing `Tick` counter — to track when components and resources were added or mutated. Smart wrappers `Ref[T]` and `Mut[T]` expose change metadata to systems. Query filters `Changed[T]` and `Added[T]` allow systems to iterate only over entities whose data changed. All types live in the `internal/ecs` package alongside the core ECS types.

## Related Specifications

- [change-detection.md](l1-change-detection.md) — L1 concept specification (parent)

## 1. Motivation

The Go implementation of change detection provides the high-performance tracking needed for reactive systems. It ensures:

- Tick-based change metadata is stored inline with component data for cache locality.
- Entity-level and archetype-level (column) change detection for efficient query evaluation.
- Type-safe wrappers (`Ref[T]`, `Mut[T]`) for automatic mutation marking.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: Relies on modern Go features like `iter` for removals and `unique` for component identification.
- **Tick wrapping**: A `uint32` is assumed to never wrap within a single game session (~828 days at 60Hz).
- **Single-writer rule**: Only one system may have mutable access to a component type at a time (enforced by the scheduler).

## 3. Core Invariants

> [!NOTE]
> See [change-detection.md §3](l1-change-detection.md) for technology-agnostic invariants.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: Monotonic Ticks | `World.ChangeTick` is a `uint32` incremented by the World before each system execution. |
| **INV-2**: Added vs Changed | `ComponentTicks` stores separate fields for `Added` and `Changed` ticks. |
| **INV-3**: Automatic mutation | `Mut[T].Value()` automatically sets the `Changed` tick to the current `World.ChangeTick`. |
| **INV-4**: ClearTrackers timing | `ClearTrackers` is called once per frame in the `Last` schedule. |
| **INV-5**: Removal persistence | `RemovedComponents[T]` entries are retained for two update cycles. |

## Go Package

```
internal/ecs/
```

All types in this spec belong to package `ecs`. Change detection is integral to the core ECS and is not a separate package.

## Type Definitions

### Tick

```go
// Tick is a monotonically increasing counter representing a logical time point.
// The World increments it before each system runs. Used for change comparison.
// A uint32 at 60 Hz wraps after ~828 days — sufficient for a single session.
type Tick uint32
```

### ComponentTicks

```go
// ComponentTicks tracks the insertion and last-mutation ticks for a single
// component instance on a single entity. Stored inline in table columns
// alongside component data.
type ComponentTicks struct {
    Added   Tick // tick when this component was first inserted
    Changed Tick // tick when this component was last mutated
}

// NewComponentTicks creates ticks with both Added and Changed set to the
// given change tick (used at insertion time).
func NewComponentTicks(changeTick Tick) ComponentTicks

// IsAdded reports whether this component was added after the given system tick.
func (ct ComponentTicks) IsAdded(lastSystemTick Tick) bool

// IsChanged reports whether this component was changed after the given system tick.
func (ct ComponentTicks) IsChanged(lastSystemTick Tick) bool

// SetChanged updates the Changed tick to the current world change tick.
func (ct *ComponentTicks) SetChanged(changeTick Tick)
```

### Ref[T]

```go
// Ref is a read-only wrapper that provides access to component data along
// with change detection metadata. Obtaining a Ref never marks the component
// as changed.
type Ref[T any] struct {
    value         *T
    ticks         ComponentTicks
    lastSystemTick Tick
}

// Value returns a pointer to the component data (read-only by convention).
func (r Ref[T]) Value() *T

// IsChanged reports whether the component was mutated since the querying
// system last ran: ComponentTicks.Changed > lastSystemTick.
func (r Ref[T]) IsChanged() bool

// IsAdded reports whether the component was inserted since the querying
// system last ran: ComponentTicks.Added > lastSystemTick.
func (r Ref[T]) IsAdded() bool

// LastChanged returns the raw Changed tick value.
func (r Ref[T]) LastChanged() Tick
```

### Mut[T]

```go
// Mut is a writable wrapper that automatically marks the component as changed
// when mutable access is obtained. Embeds change detection metadata.
type Mut[T any] struct {
    value          *T
    ticks          *ComponentTicks // mutable reference to ticks
    lastSystemTick Tick
    changeTick     Tick            // current world change tick
}

// Value returns a mutable pointer to the component data. Automatically
// sets ComponentTicks.Changed = changeTick on first access.
func (m Mut[T]) Value() *T

// IsChanged reports whether the component was mutated since the querying
// system last ran (before this system's own mutation).
func (m Mut[T]) IsChanged() bool

// IsAdded reports whether the component was inserted since the querying
// system last ran.
func (m Mut[T]) IsAdded() bool

// SetChanged explicitly marks the component as changed at the current tick.
func (m Mut[T]) SetChanged()

// BypassChangeDetection returns a mutable pointer WITHOUT marking the
// component as changed. Use for cache warming or non-semantic writes.
func (m Mut[T]) BypassChangeDetection() *T
```

### Query Filters

```go
// Changed is a query filter type that matches entities where component T
// has been mutated since the querying system last ran.
// Changed includes both modifications and additions.
//
// Usage in query definition:
//   Query[Position, Velocity].Filter(Changed[Velocity]{})
type Changed[T any] struct{}

// Added is a query filter type that matches entities where component T
// was inserted since the querying system last ran. Only newly added
// components match — modifications do not.
//
// Usage:
//   Query[Position].Filter(Added[Position]{})
type Added[T any] struct{}
```

### RemovedComponents[T]

```go
// RemovedComponents tracks entities that had component T removed in recent
// frames. Entries persist for two update cycles (matching ClearTrackers
// window) so that systems at different schedule points can observe removals.
type RemovedComponents[T any] struct {
    removals []removedEntry
}

type removedEntry struct {
    entity     Entity
    removalTick Tick
}

// Iter returns an iterator over entities that had component T removed
// since the given system tick.
func (rc *RemovedComponents[T]) Iter(lastSystemTick Tick) iter.Seq[Entity]

// Len returns the total number of pending removal entries.
func (rc *RemovedComponents[T]) Len() int
```

### Column-Level Tick

```go
// ColumnTicks holds an aggregate tick for an entire archetype column,
// enabling O(1) skip of unchanged columns during query iteration.
type ColumnTicks struct {
    ColumnChangedTick Tick // max of all ComponentTicks.Changed in this column
    ColumnAddedTick   Tick // max of all ComponentTicks.Added in this column
}
```

### World Tick State

```go
// Fields on World related to change detection (additions to World struct):
//
//   ChangeTick     Tick  // incremented before each system runs
//   LastChangeTick Tick  // value of ChangeTick at the last ClearTrackers call
```

### System Tick State

```go
// Fields on system metadata related to change detection:
//
//   LastRunTick Tick  // the ChangeTick when this system last executed
```

## Key Methods

### Tick Comparison Logic

Change detection resolves to a simple integer comparison:

```
FUNCTION is_changed(component_tick, last_system_tick):
  RETURN component_tick > last_system_tick
```

This relies on the invariant that ticks do not wrap within a session. A `uint32` at 60 Hz supports ~828 days.

### Mut[T] Auto-Marking

When a system obtains `Mut[T]` via a query, the component is marked changed:

```
ON CONSTRUCT Mut[T] for entity E:
  ticks.Changed = world.ChangeTick
```

This means obtaining mutable access is treated as a mutation, even if no field is modified. This is a deliberate trade-off: no value comparison overhead on the hot path.

### ClearTrackers

Called once per frame in the `Last` schedule:

```
FUNCTION ClearTrackers(world):
  world.LastChangeTick = world.ChangeTick
  FOR EACH RemovedComponents[T] in world:
    REMOVE entries WHERE entry.removalTick <= world.LastChangeTick - 2
```

The two-frame persistence window ensures systems at different schedule points can observe changes made in the previous frame.

### Query Filter Evaluation (Archetype-Level Skip)

```
FUNCTION evaluate_changed_filter(column, last_system_tick):
  // Step 1: Column-level check (O(1) skip)
  IF column.ColumnTicks.ColumnChangedTick <= last_system_tick:
    SKIP entire column — no entity in this column changed

  // Step 2: Per-entity check (only if column-level check passes)
  FOR EACH entity IN column:
    IF entity.ComponentTicks.Changed > last_system_tick:
      YIELD entity
```

### Component Insertion / Removal

```
ON INSERT component T on entity E at world.ChangeTick:
  ticks = ComponentTicks{ Added: world.ChangeTick, Changed: world.ChangeTick }
  column.ColumnTicks.ColumnAddedTick = max(column.ColumnTicks.ColumnAddedTick, world.ChangeTick)
  column.ColumnTicks.ColumnChangedTick = max(column.ColumnTicks.ColumnChangedTick, world.ChangeTick)

ON REMOVE component T from entity E:
  APPEND { entity: E, removalTick: world.ChangeTick } to RemovedComponents[T]
```

## Performance Strategy

- **Integer comparisons only**: All change checks are `uint32` comparisons — no allocations, no hashing, no value diffing.
- **Ticks stored inline**: `ComponentTicks` is stored alongside component data in the archetype table column. This preserves cache locality during iteration.
- **Column-level skip**: The `ColumnTicks` aggregate allows queries with `Changed[T]` filters to skip entire archetype columns in O(1) when no entity in that column changed.
- **Zero allocation**: `Ref[T]` and `Mut[T]` are value types (no heap allocation). Constructed on the stack during query iteration.
- **No value comparison**: The engine does not compare old vs. new values. Obtaining `Mut[T]` is sufficient to mark as changed. This avoids reflection and deep equality overhead.
- **RemovedComponents ring buffer**: Entries are stored in a flat slice, appended on removal, pruned in bulk during `ClearTrackers`. No per-frame allocation if the slice capacity is sufficient.

## Error Handling

- **Tick overflow**: At 60 Hz, `uint32` wraps after ~828 days. The engine logs a warning via `log/slog` if `ChangeTick` approaches `math.MaxUint32`. In practice, sessions do not last this long.
- **Stale Ref/Mut**: If a system stores a `Ref[T]` or `Mut[T]` across frames, the tick comparison becomes invalid. This is a misuse — documented as a usage constraint, not enforced at runtime.
- **Double mutable access**: Prevented at the query/schedule level (access conflict detection), not by change detection itself.
- **Missing component**: Attempting to get `Ref[T]` for a component an entity does not have returns a zero value with `IsAdded() == false` and `IsChanged() == false`.

## Testing Strategy

- **Unit tests**: Verify `ComponentTicks.IsChanged` and `IsAdded` with various tick relationships.
- **Ref/Mut semantics**: Confirm that `Ref[T]` does not mark changed, `Mut[T]` does mark changed.
- **BypassChangeDetection**: Confirm that `BypassChangeDetection` does not update the Changed tick.
- **Query filter**: Insert component, run system, verify `Added[T]` matches. Mutate component, run system, verify `Changed[T]` matches. Verify neither matches on subsequent frames after `ClearTrackers`.
- **Column-level skip**: Create archetype with 1000 entities, change one, verify column-level check passes and only one entity is yielded.
- **RemovedComponents**: Remove component, verify `RemovedComponents[T].Iter` yields the entity. Call `ClearTrackers` twice, verify entry is pruned.
- **Two-frame window**: Verify that changes made in frame N are visible to systems in both frame N and frame N+1, but not frame N+2.
- **Benchmarks**: `BenchmarkChangedFilter1K` (1000 entities, 10 changed), `BenchmarkChangedFilter10K`. Target: column-level skip provides measurable speedup over naive per-entity scan.

## 7. Drawbacks & Alternatives

- **Drawback**: `Mut[T]` access marks as changed even if no actual data was modified.
- **Alternative**: Value-based comparison (deep equality).
- **Decision**: Tick-based tracking is significantly faster and sufficient for 99% of ECS use cases.

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
