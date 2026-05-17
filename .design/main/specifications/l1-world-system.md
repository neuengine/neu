# World System

**Version:** 0.2.1
**Status:** Stable
**Layer:** concept

## Overview

The World is the central data store of the Neu engine. It owns all entities, components, resources, and schedules. Every game operation reads from or writes to a World instance. The World orchestrates entity allocation, archetype management, resource storage, and change tracking.

## Related Specifications

- [entity-system.md](l1-entity-system.md) — Entity allocation lives inside the World
- [component-system.md](l1-component-system.md) — Component storage managed by the World
- [query-system.md](l1-query-system.md) — Queries operate on World data
- [command-system.md](l1-command-system.md) — Commands are deferred mutations applied to the World
- [change-detection.md](l1-change-detection.md) — World maintains the global change tick

## 1. Motivation

Every ECS needs a single owner for all game data. Without a centralized World:

- Entity and component lifetimes become ambiguous.
- Parallel system execution cannot be safely coordinated.
- Change detection has no global reference point.

The World is the answer: one struct that owns everything, enabling safe parallel access through controlled borrowing.

## 2. Constraints & Assumptions

- A single World instance is the norm. Multiple Worlds are permitted for isolation (e.g., render world, test world).
- The World is NOT thread-safe by default. Concurrent access is mediated by the scheduler.
- All entity and component operations go through the World or through Commands (which are applied to the World).
- Resources are global singletons keyed by type — one instance per type per World.

## 3. Core Invariants

- **INV-1**: Every Entity belongs to exactly one World. Entities cannot migrate between Worlds.
- **INV-2**: Component data for an Entity is always stored in the archetype corresponding to that Entity's component set.
- **INV-3**: The World's change tick is monotonically increasing and never wraps within a single run.
- **INV-4**: Clearing trackers (change detection reset) happens exactly once per update cycle, at a well-defined point.
- **INV-5**: A DeferredWorld provides limited, re-entrant-safe access for hooks and observers.

## 4. Detailed Design

### 4.1 World Structure

The World is composed of:

- **Entity Allocator** — Manages entity IDs and generations (see entity-system).
- **Archetypes** — Collection of all archetype definitions. An archetype = unique set of component types.
- **Tables** — Column-oriented storage for Table-stored components. One table per archetype.
- **Sparse Sets** — Entity-indexed storage for SparseSet-stored components.
- **Resources** — Type-keyed singleton storage.
- **Schedules** — Named schedule definitions registered with the World.
- **Change Tick** — Global monotonic counter incremented on every system run.
- **Component Registry** — Metadata for all registered component types.

### 4.2 Entity Operations

```plaintext
World.Spawn(components...) -> Entity
World.SpawnEmpty() -> Entity
World.Despawn(entity)
World.Entity(entity) -> EntityRef        // read-only access
World.EntityMut(entity) -> EntityMut     // read-write access
World.Contains(entity) -> bool
```

### 4.3 Resource Operations

```plaintext
World.InsertResource(resource)
World.InitResource[T]()                  // insert default
World.GetResource[T]() -> *T
World.RemoveResource[T]()
World.ContainsResource[T]() -> bool
```

### 4.4 Schedule Execution

```plaintext
World.AddSchedule(label, schedule)
World.RunSchedule(label)                 // runs all systems in the named schedule
World.RunSystemOnce(system)              // run a single system immediately
```

### 4.5 DeferredWorld

A restricted view of the World available inside component hooks and observers. Prevents re-entrant archetype mutations that would invalidate internal pointers.

Allowed: read/write components on existing entities, send events, access resources.
Forbidden: spawn, despawn, add/remove components (these must go through Commands).

### 4.6 Multiple Worlds

Sub-applications (e.g., render pipeline) run their own World. Data transfer between Worlds happens via an explicit "extract" phase — a function that copies selected data from the main World to the sub-World.

### 4.7 Change Tick Management

- Each World has a `last_change_tick` and `current_change_tick`.
- When a system runs, it receives the range `[last_run_tick, current_tick]`.
- `ClearTrackers()` advances the tick and resets per-frame change flags.
- Called once per update cycle, typically at the end of the frame.

### 4.8 Batch Entity Operations

For mass-spawning scenarios (loading a scene, spawning a particle burst), the World provides batch APIs that amortize archetype lookup and table allocation costs:

```plaintext
World
  SpawnBatch(count: int, template: Bundle) -> []Entity
    // 1. Pre-allocate 'count' entity IDs in one pass
    // 2. Ensure archetype table has capacity for 'count' new rows
    // 3. Bulk-copy template data into contiguous table rows
    // 4. Fire OnAdd hooks in batch (not per-entity)
    // Return all new entity IDs

  DespawnBatch(entities: []Entity)
    // 1. Sort by archetype for table-contiguous removal
    // 2. Bulk-remove in reverse order (no swap-remove cascades)
    // 3. Fire OnRemove hooks in batch
    // 4. Return IDs to allocator free list
```

**Why batch matters**: Spawning 10,000 entities one-by-one causes 10,000 archetype lookups, 10,000 table capacity checks, and 10,000 individual hook invocations. `SpawnBatch` does one lookup, one capacity check (with pre-allocation), and one batched hook invocation — typically 10–50x faster for large spawns.

**Integration with dual-phase registration**: `SpawnBatch` increments the `add_level` counter once for the entire batch (see [entity-system.md §4.8](l1-entity-system.md)), so any system discovery triggered by the new components is deferred until the batch completes. This prevents partially-initialized batches from being processed.

### 4.9 Processor Registry

The World maintains a mapping from component types to their processing systems, enabling automatic dispatch when entities change:

```plaintext
ProcessorRegistry (World internal)
  type_to_processors:  map[TypeID][]SystemID     // direct processors
  type_to_dependents:  map[TypeID][]SystemID     // systems that need revalidation
  pending_processors:  []SystemDescriptor         // waiting for flush

  NotifyComponentChanged(entity, oldType, newType):
    // Phase 1: discover new processors for newType
    // Phase 2: remove entity from oldType processors
    // Phase 3: add entity to newType processors
    // Phase 4: revalidate dependent processors
```

This centralized registry enables the automatic system discovery pattern (see [system-scheduling.md §4.10](l1-system-scheduling.md)) — when a new component type appears, the registry checks its descriptor for a `DefaultProcessor` and auto-instantiates it. The registry also drives the component change notification chain (see [event-system.md §4.9](l1-event-system.md)).

## 5. Open Questions

- Should the World expose direct archetype iteration for advanced use cases, or keep it behind Query?
- Memory budget: should the World support pre-allocation hints for expected entity counts?

## Canonical References

<!-- MANDATORY for Stable status. List authoritative source files that downstream agents
     MUST read before implementing this spec. Use relative paths from project root.
     Stub state — fill with concrete files when implementation begins (Phase 1+). -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

<!-- Empty table = no canonical sources yet. Populate one row per authoritative file
     when implementation lands (Phase 1+). Stable promotion requires ≥1 row. -->

## Document History

| Version | Date | Description | Examples |
| :--- | :--- | :--- | :--- |
| 0.1.0 | 2026-03-25 | Initial draft | [examples/world](file:///d:/Projects/src/github.com/neuengine/neu/examples/world) |
| 0.2.0 | 2026-03-26 | Added batch entity operations, processor registry with auto-dispatch | [examples/world](file:///d:/Projects/src/github.com/neuengine/neu/examples/world) |
| 0.2.1 | 2026-04-19 | Spec hygiene: promoted orphan `### 1. Motivation` to `## 1. Motivation` (heading level consistency) | [examples/world](file:///d:/Projects/src/github.com/neuengine/neu/examples/world) |
| 0.3.0 | 2026-05-14 | Phase 1 implementation complete; validated by `examples/ecs/poc` | [examples/ecs/poc](../../../examples/ecs/poc) |
