# Physics Queries

**Version:** 0.1.0
**Status:** Draft
**Layer:** concept

## Overview

Physics queries allow game systems to interrogate the simulation world without waiting for a physics step. A ray cast finds the first (or all) bodies intersecting a ray. A shape cast sweeps a collision shape through space and reports the first contact. An overlap test returns all bodies currently touching a given shape. All queries run synchronously on the calling thread during `Update` or `FixedUpdate` — they are read-only operations against the backend's spatial acceleration structure and never mutate simulation state.

## Related Specifications

- [physics-system.md](l1-physics-system.md) — PhysicsServer service exposes the query API
- [collider.md](l1-collider.md) — CollisionGroups used as query filter
- [rigid-body.md](l1-rigid-body.md) — BodyType affects query results
- [math-system.md](l1-math-system.md) — Ray3D, Vec3, Quat primitives
- [event-system.md](l1-event-system.md) — Queries are pull-based; contrast with push-based collision events

## 1. Motivation

Collision events (push-based) cover the case where the simulation tells you something happened. Queries (pull-based) cover the case where you want to ask the simulation a question right now:

- "Is there ground beneath the player's feet?" — character controller floor detection.
- "What does this bullet hit along its path?" — hitscan weapons.
- "Which enemies are within 10 metres of the player?" — AI aggro radius.
- "Can I place a building here without overlapping anything?" — RTS construction.
- "Where would this grenade land if thrown?" — trajectory prediction via shape cast.

Without a query API, game code would have to maintain its own spatial data structures in parallel with the physics backend — duplicated work that drifts out of sync.

## 2. Constraints & Assumptions

- Queries are **read-only**. They never modify body positions, velocities, or contact state.
- Queries execute against the state of the physics world at the **end of the last completed step**. They do not see mid-step intermediate state.
- Queries run on the calling goroutine. They block until results are ready. For expensive batch queries, parallel dispatch is the caller's responsibility via the task system.
- Queries respect `CollisionGroups` filtering — a collider excluded by its filter mask is invisible to queries using that mask.
- `TriMesh` and `HeightField` colliders are queryable. `sensor: true` colliders are optionally included via `QueryFilter.include_sensors`.
- Sleeping bodies are included in query results — sleep affects integration, not spatial presence.

## 3. Core Invariants

- **INV-1**: Queries never mutate simulation state. Calling any query method during a physics step is safe and returns consistent results from the last completed step.
- **INV-2**: A query that hits a sensor collider only returns a result if `QueryFilter.include_sensors = true`. Default is false.
- **INV-3**: All query results return `Entity` identifiers, not `PhysicsBodyHandle`. The mapping from handle to entity is the backend's responsibility via `user_data`.
- **INV-4**: `RayCastAll` and `OverlapAll` results are unordered unless the caller requests sorting. The engine does not guarantee hit order without explicit sort.
- **INV-5**: A query with a `QueryFilter` that excludes all groups returns an empty result immediately — no traversal performed.

## 4. Detailed Design

### 4.1 QueryFilter

Every query accepts a `QueryFilter` that controls which colliders are considered:

```plaintext
QueryFilter
  collision_groups:  CollisionGroups    // membership + filter mask (collider.md §5.5)
                                        // default: all groups
  exclude_entities:  []Entity           // ignore these specific entities
  exclude_rigid_body_types: BodyTypeMask // bitmask: Skip Static | Kinematic | Dynamic
  include_sensors:   bool               // include sensor colliders, default false
  predicate:         Option[func(Entity) bool]
                                        // custom per-entity rejection function
                                        // called only after group filter passes
```

`predicate` is an escape hatch for cases the bitfield cannot express — for example "ignore all entities that have a `TeamComponent` matching my own team". It is called at most once per candidate hit.

### 4.2 Ray Cast

Projects a ray from an origin in a direction:

```plaintext
RayCastHit
  entity:   Entity        // the hit entity
  point:    Vec3          // world-space hit point on the collider surface
  normal:   Vec3          // world-space surface normal at hit point, pointing outward
  distance: float32       // distance from ray origin to hit point
  collider: ColliderIndex // which shape on a compound body was hit (0 for simple bodies)
```

API on `PhysicsServer`:

- `RayCast(ray, max_dist, filter) -> Option[RayCastHit]` (Closest hit only)
- `RayCastAll(ray, max_dist, filter) -> []RayCastHit` (All hits, unsorted)

### 4.3 Shape Cast (Sweep)

Sweeps a collision shape through space along a direction vector:

```plaintext
ShapeCastHit
  entity:       Entity
  point:        Vec3          // deepest contact point on the target collider
  normal:       Vec3          // surface normal at contact, pointing away from target
  distance:     float32       // distance from shape origin to first contact
  time_of_impact: float32     // in [0, 1]: fraction of direction vector at first contact
```

API:

`ShapeCast(shape, origin, rotation, direction, filter) -> Option[ShapeCastHit]`

### 4.4 Overlap Test

Returns all colliders whose shapes currently intersect a given test shape:

- `OverlapSphere(center, radius, filter) -> []OverlapResult`
- `OverlapBox(center, extents, rotation, filter) -> []OverlapResult`
- `OverlapShape(shape, position, rotation, filter) -> []OverlapResult` (General form)
- `IntersectsShape(shape, pos, rot, filter) -> bool` (Fast boolean version — stops at first hit)

### 4.5 Point Test

Tests whether a world-space point is inside any collider:

- `PointQuery(point, filter) -> Option[PointResult]`
- `ProjectPoint(point, filter) -> PointResult` (Projects onto nearest surface)

### 4.6 Contact Pair Query

Reads the current contact manifold between one or two specific entities:

- `ContactsBetween(entity_a, entity_b) -> Option[ContactPair]`
- `ContactsWithEntity(entity) -> []ContactPair`

### 4.7 Batch Queries

For high-density queries (AI, particles), a batch API avoids per-query overhead and allows backend parallelisation:

```plaintext
ExecuteQueryBatch(batch: QueryBatch) -> QueryBatchResults
```

The backend is thread-safe for concurrent reads, so callers can also issue individual queries from parallel system iteration.

### 4.8 Query Performance Guidelines

| Query type | Approximate cost | Notes |
| :--- | :--- | :--- |
| `RayCast` | ~0.5–2 µs | BVH traversal + shape test |
| `ShapeCast` | ~2–10 µs | More expensive narrow-phase |
| `OverlapSphere` | ~1–5 µs per result | Scales with hit count |
| `OverlapShape` | ~3–15 µs per result | Shape-dependent |
| `IntersectsShape` | ~1–3 µs | Boolean result, exits at first hit |
| `ContactsBetween` | O(1) | Direct hash lookup in contact graph |
| `ContactsWithEntity` | O(contacts) | Linear in contact count |

*Practical limits (60 Hz, 1ms budget):* ~500 individual `RayCast` calls. For >1000 queries, use batch/parallel API.

### 4.9 Debug Visualisation

Integrated with `PhysicsDebugSettings`:

- `draw_queries: bool` visualises rays (hit red, miss grey) and swept shapes.

### 4.10 Spatial Selection API

Instead of iterating all entities or even all colliders, the engine provides a **Spatial Selection API** to retrieve entities based on their world-space location:

- **Query Types**: `SelectedInRadius(pos, radius)`, `SelectedInFrustum(frustum)`, `SelectedInVolume(aabb)`.
- **Filtering**: These selections natively support `QueryFilter` (layers and masks).
- **Use Cases**: AI perception, area-of-effect damage, sound attenuation, and editor selection.

### 4.11 Predictive Look-ahead (Swept Queries)

To support robust movement, systems can perform **Predictive Look-ahead** queries to determine if a future movement will cause a collision:

- **Movement Sweep**: A shape is cast along the intended velocity vector.
- **Result Interpretation**: If a hit occurs, the distance to the hit point is used to truncate movement, preventing interpenetration.
- **Architecture**: This is a non-reactive pull query used primarily by the `CharacterControllerSystem`.

## 5. Open Questions

- Should `RayCastAll` accept a result capacity limit?
- Closure-based `predicate` safety for parallel batch execution.
- Should queries be available during `FixedUpdate` against *in-progress* state?

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
| 0.1.0 | 2026-03-27 | Initial draft — ray cast, shape cast, overlap, point test, contact query, batch API | [examples/physics](file:///d:/Projects/src/github.com/neuengine/neu/examples/physics) |
