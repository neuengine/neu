# Query System

**Version:** 0.1.0
**Status:** Draft
**Layer:** concept

## Overview

Queries are the primary mechanism for systems to access entity data. A query declaratively specifies which components to fetch and which filters to apply. The scheduler uses query access information to determine which systems can run in parallel.

## Related Specifications

- [world-system.md](l1-world-system.md) — Queries read from World data
- [component-system.md](l1-component-system.md) — Queries fetch components
- [system-scheduling.md](l1-system-scheduling.md) — Query access drives parallel scheduling
- [change-detection.md](l1-change-detection.md) — Changed/Added filters use change ticks

## 1. Motivation

Systems need a safe, declarative way to access entity data. The query system provides:

- Type-safe component access (read or write).
- Automatic archetype matching (only iterate relevant entities).
- Access tracking for safe parallelism (two read-only queries can overlap; a write query is exclusive).
- Filtering by component presence, absence, or change state.

## 2. Constraints & Assumptions

- Queries are stateless declarations — they do not own data.
- QueryState caches matched archetypes for efficient repeated iteration.
- A query with `&mut T` access is exclusive — no other query can read `T` simultaneously.
- Empty archetypes are skipped during iteration (zero-cost for absent entities).

## 3. Core Invariants

- **INV-1**: A query that requests `&mut T` prevents any other query from accessing `T` in the same system.
- **INV-2**: Query iteration visits each matching entity exactly once per iteration.
- **INV-3**: QueryState is invalidated and rebuilt when new archetypes are created.
- **INV-4**: Filters are applied during iteration, not during archetype matching (Changed/Added are tick-based).

## 4. Detailed Design

### 4.1 Query Declaration

A query specifies:

- **Fetch items**: Component references to retrieve (`&Position`, `&mut Velocity`, `Entity`).
- **Filters**: Constraints on which entities match (`With[T]`, `Without[T]`, `Changed[T]`, `Added[T]`).

```
Query[(&Position, &mut Velocity), With[Player], Without[Dead]]
```

This fetches Position (read) and Velocity (write) for entities that have Player but not Dead.

### 4.2 Fetch Types

| Fetch | Access | Description |
| :--- | :--- | :--- |
| `&T` | Read | Immutable reference to component T |
| `&mut T` | Write | Mutable reference to component T |
| `Entity` | Read | The entity ID itself |
| `Option[&T]` | Read | Component T if present, nil otherwise |
| `Option[&mut T]` | Write | Mutable access if present |
| `Has[T]` | Read | Boolean: does the entity have T? |
| `Ref[T]` | Read | Read access with change detection metadata |

### 4.3 Filter Types

| Filter | Description |
| :--- | :--- |
| `With[T]` | Entity must have component T (T is not fetched) |
| `Without[T]` | Entity must NOT have component T |
| `Changed[T]` | Component T was mutated since last system run |
| `Added[T]` | Component T was added since last system run |
| `Or[A, B]` | Entity matches filter A OR filter B |

### 4.4 Iteration Modes

- **Iter()** — Sequential iteration over all matching entities. Returns tuples of fetched components.
- **ParIter()** — Parallel iteration. Work is divided across worker threads by archetype table chunks.
- **Get(entity)** — Single-entity lookup. Returns the fetched components for one specific entity, or error if not matched.
- **GetMany(entities)** — Multi-entity lookup. Ensures no aliased mutable references.
- **Single()** — Asserts exactly one entity matches. Returns it directly or errors.

### 4.5 QueryState (Caching)

QueryState caches which archetypes match the query's component requirements:

- Built lazily on first iteration.
- Incrementally updated when new archetypes are created (archetype generation counter).
- Stored per-system for reuse across frames.
- Avoids O(archetypes) scan on every iteration.

### 4.6 Access Tracking

Each query declares its access set:

- **Read set**: Component types read.
- **Write set**: Component types written.

The scheduler compares access sets between systems:

- Two systems with disjoint access sets can run in parallel.
- Two systems where one writes a component the other reads must be ordered.
- Resource access (`Res[T]` / `ResMut[T]`) is tracked the same way.

### 4.7 Query Conflicts

- Within a single system, two queries that both mutably access the same component type are a conflict.
- Resolved by merging into a single query or using `QuerySet` (disjoint query guarantees).
- Detected at system initialization, not at iteration time.

### 4.8 World Queries

Special query types with broader access:

- **WorldQuery**: Trait that defines how a query item is fetched from the World.
- Custom WorldQuery implementations enable user-defined fetch patterns.

## 5. Open Questions

- Should queries support `AnyOf[A, B, C]` — fetch whichever of these components exist?
- Performance: should very large query results support cursor-based pagination?

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
| 0.2.0 | 2026-05-14 | Phase 1 implementation complete; validated by `examples/ecs/poc` |
