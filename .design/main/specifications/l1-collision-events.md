# Collision Events

**Version:** 0.1.0
**Status:** Draft
**Layer:** concept

## Overview

Collision events are the push-based complement to physics queries. Where queries ask "what is touching X right now?", collision events tell you "something started touching X" or "something stopped touching X" without polling. Events are emitted by the Physics Server at the end of each fixed step, after the contact solver has run, and delivered through the standard event bus. Game systems subscribe to them and react accordingly.

## Related Specifications

- [physics-system.md](l1-physics-system.md) — Events phase that diffs contact sets and emits events
- [collider.md](l1-collider.md) — Collider sensor flag, CollisionGroups, contact_force_threshold
- [rigid-body.md](l1-rigid-body.md) — BodyType affects which event types are generated
- [event-system.md](l1-event-system.md) — EventWriter/EventReader infrastructure used for delivery
- [physics-query.md](l1-physics-query.md) — Pull-based alternative; ContactsBetween for synchronous reads

## 1. Motivation

Polling `ContactsWithEntity` every frame is wasteful. Most frames nothing changes. Collision events invert this: the engine pays the diffing cost once per step and notifies only the systems that registered interest. Game code becomes reactive rather than polling, and the physics budget is spent where state actually changes.

## 2. Constraints & Assumptions

- Events are emitted once per physics step. At 60 Hz fixed step and 120 Hz render, each event fires once every two render frames.
- `CollisionStarted` and `CollisionEnded` are guaranteed to be paired — every start has a corresponding end, even if the body is despawned mid-simulation.
- Two sensor colliders produce trigger events but never contact events.
- Collision events are not emitted between two `Static` bodies.
- `ContactManifold` data inside events is a snapshot valid for the frame it is read.

## 3. Core Invariants

- **INV-1**: `CollisionStarted` fires exactly once when a contact pair enters the contact graph.
- **INV-2**: `CollisionEnded` fires exactly once when a contact pair leaves the contact graph.
- **INV-3**: `CollisionPersisting` fires every step while a pair remains active (optional, off by default).
- **INV-4**: If a body is despawned, the entity ID in the event remains valid until ECS processes the despawn command in `ApplyDeferred`.
- **INV-5**: `ContactForceEvent` fires only when impulse exceeds `contact_force_threshold` on the receiver.

## 4. Detailed Design

### 4.1 Event Types Overview

- **Contact events (solid)**: `CollisionStarted`, `CollisionPersisting`, `CollisionEnded`.
- **Trigger events (sensor)**: `TriggerEntered`, `TriggerExited`.
- **Force events (threshold)**: `ContactForceEvent`.

### 4.2 ContactManifold

Provides geometry snapshot: `contact_points`, `normal` (B to A), `total_impulse`, `tangent_impulse`.
`ContactPoint` includes `position`, `depth`, and `impulse`.

### 4.3 Trigger Events

Sensor colliders generate `TriggerEntered` and `TriggerExited`. No `ContactManifold` data is provided as sensors have no contact response.

### 4.4 Event Filtering Patterns

1. **Tag Check**: Read all events, continue if entity lacks a specific component.
2. **Observer (Preferred)**: Use `world.Entity(e).Observe(fn(Trigger[CollisionStarted]))` for targeted reactions.
3. **Layer Pre-filtering**: Configure `CollisionGroups` so irrelevant pairs never enter the contact graph. "The cheapest event is the one that doesn't happen."

### 4.5 Contact Graph State Machine

The Physics Server diffs current and previous contact pairs:
`started = curr - prev`, `ended = prev - curr`, `persisting = curr ∩ prev`.

### 4.6 Event Volume (Opt-in Persisting)

`CollisionPersisting` can generate thousands of events. It is **disabled by default** via `ColliderEventFlags.emit_persisting`. Enable only for entities requiring continuous contact data (e.g., conveyor belts, ice, damage zones).

### 4.7 Deferred Despawn Safety

Despawning an entity in a collision event is safe. The entity remains alive for the rest of the frame. The physics backend emits `CollisionEnded` for this pair in the next step, as the entity is only removed from the physics world during the next Sync phase.

### 4.8 Collision Resolution Data (MTV)

To provide actionable data for systems that resolve collisions (like the `CharacterController`), the engine includes **Minimum Translation Vector (MTV)** data in every `ContactManifold`:

- **MTV Direction**: A normalized `Vec3` indicating the shortest path to separate the two shapes.
- **MTV Magnitude**: A `float32` representing the overlap depth.
- **Usage**: By moving an entity by `-MTV_Direction * MTV_Magnitude`, a system can resolve a static collision instantly, preventing interpenetration.

## 5. Open Questions

- Should `CollisionPersisting` be opt-in per collider or opt-out globally?
- Behavior of events during large delta "fast-forwards" (bursts of events).

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
| 0.1.0 | 2026-03-27 | Initial draft — contact/trigger/force events, state machine, deferred despawn | [examples/physics](file:///d:/Projects/src/github.com/neuengine/neu/examples/physics) |
