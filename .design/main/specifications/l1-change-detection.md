# Change Detection

**Version:** 0.1.0
**Status:** Stable
**Layer:** concept

## Overview

Change detection enables systems to efficiently react only to data that has actually changed, avoiding unnecessary computation. The engine uses a tick-based tracking mechanism: a global change tick is incremented on every system run, and every component and resource mutation records the tick at which it was last modified. Query filters (`Changed[T]`, `Added[T]`) and smart reference wrappers (`Ref[T]`, `Mut[T]`) expose this information to systems declaratively.

## Related Specifications

- [world-system.md](l1-world-system.md) — The World maintains the global change tick and calls ClearTrackers
- [query-system.md](l1-query-system.md) — Changed and Added are query filter types
- [component-system.md](l1-component-system.md) — Components carry change tick metadata

## 1. Motivation

In a typical game, most data is stable frame-to-frame. A transform system that recomputes global transforms for all 10,000 entities every frame wastes cycles when only 50 moved. Change detection makes it possible to:

- Skip systems entirely when no relevant data changed.
- Iterate only over entities whose components were modified.
- Detect newly added components for initialization logic.
- Track removed components for cleanup logic.

Without engine-level change detection, developers build ad-hoc dirty flags that are error-prone and inconsistent.

## 2. Constraints & Assumptions

- Change detection is automatic — any write through `Mut[T]` or `ResMut[T]` marks the data as changed. No manual dirty flags.
- "Changed" means the data was potentially mutated (the mutable reference was obtained), not that the value actually differs. The engine does not perform value comparison.
- Change detection is per-component-column, not per-field. If any field of a component is written, the entire component is marked changed.
- Tick values are unsigned integers. They must not wrap during a single engine run (see INV-3 in [world-system.md](l1-world-system.md)).
- Zero external dependencies (C24).

## 3. Core Invariants

- **INV-1**: A component is marked as "changed" if and only if a mutable reference to it was obtained since the querying system last ran.
- **INV-2**: A component is marked as "added" if and only if it was inserted onto an entity since the querying system last ran.
- **INV-3**: `ClearTrackers()` is called exactly once per update cycle. Change information persists for at least two frames to allow systems running at different points in the schedule to observe changes.
- **INV-4**: Obtaining a `Ref[T]` (read-only) never marks data as changed. Only `Mut[T]` marks changes.
- **INV-5**: RemovedComponents entries persist for exactly two frames (same as change flags).

## 4. Detailed Design

### 4.1 Tick Model

The World maintains a global tick counter:

```
World
  - ChangeTick    uint32   // incremented before each system runs
```

Each system records when it last ran:

```
SystemState
  - LastRunTick   uint32   // the ChangeTick when this system last executed
```

Each component and resource stores two ticks:

```
ComponentTicks
  - AddedTick     uint32   // tick when this component was first inserted
  - ChangedTick   uint32   // tick when this component was last mutated
```

A component is considered "changed" from a system's perspective when:

```
component.ChangedTick > system.LastRunTick
```

A component is considered "added" when:

```
component.AddedTick > system.LastRunTick
```

### 4.2 Ref[T] and Mut[T] Wrappers

These wrappers provide access to component data along with change metadata:

```
Ref[T] (read-only access)
  - Value         *T       // pointer to component data
  - Ticks         ComponentTicks

Methods:
  IsChanged() bool         // ChangedTick > last_system_tick
  IsAdded() bool           // AddedTick > last_system_tick
  LastChanged() uint32     // raw ChangedTick value

Mut[T] (read-write access, extends Ref[T])
  - Value         *T
  - Ticks         *ComponentTicks  // mutable reference to ticks

On creation:
  Ticks.ChangedTick = World.ChangeTick  // mark as changed when mutable access obtained

Methods:
  SetChanged()             // manually mark as changed (automatic on construction)
  BypassChangeDetection() *T  // get mutable pointer WITHOUT marking as changed
```

The `BypassChangeDetection` escape hatch exists for performance-critical paths where the system knows a write is not semantically meaningful (e.g., cache warming).

### 4.3 Query Filters

Change detection integrates with the query system through filter types:

```
Changed[T]
  Matches entities where component T has ChangedTick > system.LastRunTick
  Includes both modifications and additions

Added[T]
  Matches entities where component T has AddedTick > system.LastRunTick
  Only matches newly inserted components

Usage in query:
  Query[Position, Velocity].Filter(Changed[Velocity])
  // iterates only entities whose Velocity was modified since this system last ran
```

These filters are evaluated during query iteration and skip non-matching entities efficiently using archetype-level and table-level tick checks before falling through to per-entity checks.

### 4.4 Resource Change Detection

Resources use the same tick mechanism:

```
Res[T] (read-only resource access)
  IsChanged() bool
  IsAdded() bool

ResMut[T] (read-write resource access)
  IsChanged() bool
  IsAdded() bool
  // Obtaining ResMut[T] automatically marks the resource as changed
```

### 4.5 ClearTrackers

At the end of each update cycle, `ClearTrackers()` advances the baseline:

```
ClearTrackers():
  World.LastChangeTick = World.ChangeTick
  Clear RemovedComponents buffers older than 2 frames
```

This is called in the Last schedule (see [app-framework.md](l1-app-framework.md)). The two-frame persistence window ensures that systems running at different points in the schedule all have a chance to observe changes. A system that runs early in frame N can still detect changes made late in frame N-1.

### 4.6 RemovedComponents[T]

When a component is removed from an entity (or the entity is despawned), the removal is recorded:

```
RemovedComponents[T]
  Iteration:
    Iter() -> []Entity   // entities that had component T removed since this system last ran

Storage:
  Ring buffer of (Entity, RemovalTick) pairs
  Entries older than 2 update cycles are discarded by ClearTrackers
```

This allows cleanup systems to react to component removal without polling every entity.

### 4.7 Optimization: Archetype-Level Tick Check

To avoid per-entity tick comparisons when no changes occurred, each archetype column maintains a column-level tick:

```
ArchetypeColumn
  - ColumnChangedTick  uint32  // max of all component ChangedTicks in this column
```

During query iteration:

```
1. Check column-level tick: if ColumnChangedTick <= system.LastRunTick, skip entire column
2. Otherwise, fall through to per-entity tick checks
```

This optimization means that queries with `Changed[T]` filters over large archetype tables can skip unchanged tables in O(1) rather than scanning every entity.

## 5. Open Questions

- Should the engine support "deep change detection" (value comparison) as an opt-in feature for specific component types?
- Should change ticks use `uint32` or `uint64`? A `uint32` at 60 Hz wraps after ~828 days, which may be sufficient.
- Should there be a `Mutated[T]` filter that excludes `Added[T]` (only captures modifications to existing components)?
- How should change detection interact with entity cloning — is a cloned entity's component marked as "added"?

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
| 0.1.0 | 2026-03-25 | Initial draft |
| — | — | Planned examples: `examples/world/` |
