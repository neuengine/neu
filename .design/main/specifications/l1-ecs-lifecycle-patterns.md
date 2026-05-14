# ECS Lifecycle and Optimization Patterns

**Version:** 0.2.0
**Status:** Draft
**Layer:** concept

## Overview

This specification defines high-performance patterns for managing the lifecycle of entities and components, and optimizing data access through bitmask tagging and cached entity views.

## Related Specifications

- [world-system.md](l1-world-system.md) - Central data store and entity management.
- [query-system.md](l1-query-system.md) - Foundation for data access and filtering.
- [component-system.md](l1-component-system.md) - Component registration and storage.

## 1. Motivation

As the number of entities and component types grows, the engine requires highly efficient mechanisms to:

1. Filter entities based on component combinations without full archetypal scans in every turn.
2. Ensure safe resource cleanup (e.g., GPU memory, file handles) when components are removed or entities are disposed.
3. Speed up repetitive queries that target stable sets of entities.

## 2. Constraints & Assumptions

- **Bitmask Limit**: Initial implementation assumes a maximum of 64 distinct component types for bitmask optimizations.
- **Thread Safety**: Access to component data and entity tags must be protected by appropriate synchronization primitives (e.g., RWMutex) to support parallel system execution.
- **Single World Scope**: Patterns described here apply to a single `World` instance.

## 3. Core Invariants

- **Invariant 1: Atomic Tag Updates**: Entity tags (bitmasks) must be updated atomically when a component is added or removed.
- **Invariant 2: Deterministic Cleanup**: Component destructors must be called immediately or at defined synchronization points before the entity's data is fully purged.
- **Invariant 3: View Consistency**: Cached entity views must remain synchronized with the current world state at all times (Reactive updates).

## 4. Invariant Compliance (Layer 2 only)

> This section is for Layer 2 implementation specifications.

## 5. Detailed Design

### 5.1 Bitmask Tagging System

Each entity carries a `Tag` (bitmask) representing the presence of registered components.

- **Component ID**: Every registered component type is assigned a unique incremental ID (0-63).
- **Tag Structure**: A `uint64` flags field where each bit represents a component ID.
- **Matching Logic**: A query matches an entity if: `(entity.Tag & query.RequiredTag) == query.RequiredTag`.

**Pseudo-code Flow:**

```plaintext
Function AddComponent(entity, component):
    Set bit at component.ID in entity.Tag
    Notify active Views of the tag change

Function Matches(tag, requirement):
    Return (tag AND requirement) == requirement
```

### 5.2 Component Destructors

Components may register a destructor function that is triggered during lifecycle events.

- **OnRemove**: Triggered when a specific component is removed from an entity.
- **OnDispose**: Triggered when the entity itself is destroyed.
- **Context**: The destructor receives the entity identifier and a reference to the component data.

**Flow Diagram:**

```mermaid
graph TD;
    A[DisposeEntity] --> B{Has Components?};
    B -- Yes --> C[Iterate Components];
    C --> D{Has Destructor?};
    D -- Yes --> E[Execute Destructor];
    E --> F[Remove Component Data];
    F --> C;
    B -- No --> G[Free Entity ID/Generation];
```

### 5.3 Entity Views (Caching)

A `View` is a cached collection of entities that match a specific `Tag` requirement.

- **Reactive Updates**: When an entity's tag changes (component added/removed), the world checks all active Views.
- **Addition**: If the new tag matches the view's requirement and it didn't match before, the entity is added to the cache.
- **Removal**: If the old tag matched but the new one doesn't, the entity is removed.
- **Performance**: Provides O(1) or O(N_matches) access for systems, bypassing the need to iterate all entities.

### 5.4 Spatial Registry Update Lifecycle

For entities with spatial presence (e.g. `Transform`, `Collider`), the engine maintains their state in a **Spatial Acceleration Structure** (Grid/Quadtree/BVH). The lifecycle of this registry is as follows:

- **Entity Movement**: When an entity's `Transform` changes, it is marked for spatial update.
- **Batch Processing**: Instead of updating the spatial registry per-entity, the `PhysicsSystem` performs a batch update at the beginning of each physics step.
- **Dormancy**: Entities that reach a "sleeping" state in the physics engine are skipped in subsequent spatial registry updates until awakened, reducing broad-phase overhead.
- **Clean Registration**: On entity disposal, its entry must be purged from the spatial registry immediately to prevent "ghost" query results.

### 5.5 Frame Delay Mitigation

In a system-based architecture, data written by System A may not be visible to System B until the next frame if B executes before A. This "one-frame lag" causes visual stuttering, delayed reactions, and hard-to-debug timing issues.

**Strategies:**

1. **System Reordering**: The primary solution. If System B depends on System A's output, declare `B.After(A)` so both run in the same frame. The scheduler (see [system-scheduling.md §4.4](l1-system-scheduling.md)) enforces this ordering.

2. **On-Entity-Created Callbacks**: When an entity must be fully initialized the same frame it is spawned (e.g., visual representation ready before first render), use lifecycle hooks ([component-system.md §4.8](l1-component-system.md)) to run initialization logic immediately during the spawn, rather than waiting for a system to pick it up next frame.
   - *Example*: A bullet entity needs a `Transform` computed from the weapon's barrel position. Without a callback, the bullet renders at origin for one frame before the positioning system moves it.

3. **LateUpdate Schedule Stage**: Place visual-finalization systems (camera follow, UI sync, animation blending) in `PostUpdate` or a dedicated `LateUpdate` stage to ensure all gameplay mutations are complete before rendering reads begin.

4. **Immediate State Transitions**: For state changes that must be visible in the same frame (e.g., `enterState("dead")` triggering a ragdoll), apply the transition directly in the component rather than deferring to a system. Combine with a command component (e.g., `StateChangedTag`) so interested systems can react in subsequent stages of the same frame.

**When to Accept Frame Delays:**

- Non-critical visual feedback (particle spawn positions, secondary animations).
- AI decision-making (one frame of "thinking time" is imperceptible).
- Physics-driven effects (physics already runs on a fixed timestep with interpolation).

### 5.6 Entity Object Pooling

Frequently created and destroyed entities (projectiles, particles, damage numbers, sound effects) cause allocation pressure and archetype table churn. Object pooling amortizes this cost.

**Design:**

```plaintext
EntityPool {
    available:  []Entity       // pre-spawned, disabled entities
    capacity:   int            // maximum pool size
    archetype:  []ComponentType // component template for new entities
}

Function Acquire(pool) -> Entity:
    If pool.available is not empty:
        entity = pool.available.Pop()
        Remove DisabledTag from entity
        Reset mutable component data to defaults
        Return entity
    Else if total spawned < pool.capacity:
        entity = Spawn(pool.archetype)
        Return entity
    Else:
        Return Entity.PLACEHOLDER  // pool exhausted

Function Release(pool, entity):
    Reset mutable component data to defaults
    Add DisabledTag to entity
    pool.available.Push(entity)
```

**Integration with Entity Disabling** (see [entity-system.md §4.4](l1-entity-system.md)): Pooled entities use the `DisabledTag` mechanism — they remain allocated with all components intact but invisible to default queries. This avoids the cost of archetype migration on spawn/despawn.

**Go Implementation Consideration**: Leverage `sync.Pool` semantics for goroutine-safe pool access when systems run in parallel. However, unlike `sync.Pool`, entity pools must NOT garbage-collect idle entries — the whole point is keeping them allocated.

## 6. Implementation Notes

1. Bitmask Extension: For projects requiring >64 components, the system should transition to bit-arrays or hierarchical tagging.
2. View Synchronization: Views should be updated immediately during `AddComponent`/`RemoveComponent` to ensure consistency for systems running in the same frame.

## 7. Drawbacks & Alternatives

- **Memory Overhead**: Maintaining multiple cached Views increases memory usage.
- **Alternative (Archetypes)**: While Archetypes (SoA) are the primary storage, bitmask tagging provides a faster "filter layer" for heterogeneous queries.

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
| 0.1.0 | 2026-03-27 | Initial Draft of lifecycle and optimization patterns |
| 0.2.0 | 2026-03-28 | Added frame delay mitigation strategies, entity object pooling pattern |
| 0.3.0 | 2026-05-14 | Phase 1 implementation complete; validated by `examples/ecs/poc` |
| [example](examples/ecs-lifecycle) | | TBD: Placeholder for correlation |
