# Event System

**Version:** 0.3.0
**Status:** Draft
**Layer:** concept

## Overview

The event system provides three communication mechanisms between systems: **Events** (broadcast, double-buffered), **Messages** (typed system-to-system, cursor-based), and **Observers** (reactive triggers that fire immediately). Each mechanism serves a different communication pattern.

## Related Specifications

- [world-system.md](l1-world-system.md) — Events/messages stored in World resources
- [system-scheduling.md](l1-system-scheduling.md) — Event readers/writers are system parameters
- [command-system.md](l1-command-system.md) — Observers can trigger commands
- [entity-system.md](l1-entity-system.md) — Entity events target specific entities

## 1. Motivation

Systems need to communicate without direct coupling. Three patterns emerge:

1. **Broadcast**: "Something happened" — any interested system can react (Events).
2. **Point-to-point**: "System A tells System B" — ordered, reliable delivery (Messages).
3. **Reactive**: "When X is added to entity Y, do Z immediately" — no frame delay (Observers).

## 2. Constraints & Assumptions

- Events persist for exactly 2 frames (double-buffered), then are dropped.
- Messages persist until all registered readers have consumed them (cursor-based).
- Observers fire synchronously during the trigger — they block the triggering operation.
- All three mechanisms are type-safe: each event/message/trigger has a concrete type.

## 3. Core Invariants

- **INV-1**: An EventReader sees each event exactly once, even if it runs multiple times per frame.
- **INV-2**: Events not consumed within 2 frames are silently dropped.
- **INV-3**: Messages are never dropped until all registered readers advance past them.
- **INV-4**: Observers execute immediately and can trigger other observers (chain), but must terminate (no infinite loops).
- **INV-5**: Entity events only fire observers attached to the target entity (or global observers).

## 4. Detailed Design

### 4.1 Events (Broadcast)

Double-buffered event bus. Writers push events; readers consume them.

```
// Writing
fn spawn_particles(mut writer: EventWriter[CollisionEvent]) {
    writer.Send(CollisionEvent{entity_a, entity_b, point})
}

// Reading
fn play_sound(mut reader: EventReader[CollisionEvent]) {
    for event in reader.Read() {
        audio.play("hit.wav")
    }
}
```

**Buffer rotation**: Each frame, the "current" buffer becomes the "previous" buffer, and a new empty buffer becomes "current". Events live for 2 frames, ensuring systems running in different schedules can see them.

### 4.2 Messages (System-to-System)

Cursor-based communication channel. Each reader maintains its own cursor position.

```
// Writing
fn ai_system(mut writer: MessageWriter[MoveCommand]) {
    writer.Write(MoveCommand{entity, direction})
}

// Reading
fn movement_system(mut reader: MessageReader[MoveCommand]) {
    for cmd in reader.Read() {
        // process movement command
    }
}
```

**Key differences from Events:**

- Messages are not frame-limited — they persist until all readers consume them.
- Each reader tracks its own cursor, so late readers don't miss messages.
- Messages are ordered (FIFO within a single writer).

### 4.3 Observers (Reactive Triggers)

Observers fire immediately when a trigger occurs. They do not wait for the next schedule tick.

```
// Global observer
world.AddObserver(func(trigger: On[OnAdd[Health]]) {
    log("Entity gained health component")
})

// Entity-targeted observer
world.Entity(player).Observe(func(trigger: On[Damage]) {
    health := trigger.Entity.Get[Health]()
    health.current -= trigger.amount
})
```

**Trigger types:**

- **Component lifecycle**: `OnAdd[T]`, `OnInsert[T]`, `OnReplace[T]`, `OnRemove[T]`
- **Custom events**: Any user-defined event type
- **Entity events**: Events with an `Entity` field — dispatched to entity's observers

**Execution model:**

1. Trigger fires (e.g., component added).
2. All matching observers execute immediately, synchronously.
3. Observers can enqueue Commands and trigger additional events.
4. Chained triggers execute depth-first.

### 4.4 Entity Events

Events that target specific entities. They propagate up the entity hierarchy (bubbling):

```
EntityEvent[Damage] triggered on child_entity:
  1. child_entity's observers fire
  2. parent_entity's observers fire (if any)
  3. grandparent_entity's observers fire (if any)
  ... up to root
```

Bubbling can be stopped by an observer calling `event.StopPropagation()`.

### 4.5 Event Registration

Events and messages must be registered with the App/World before use:

```
app.AddEvent[CollisionEvent]()
app.AddMessage[MoveCommand]()
```

Registration sets up the storage buffers and enables the scheduler to track access.

### 4.6 Conditional Event Handling

Run conditions based on events:

```
system.RunIf(on_event[CollisionEvent]())    // run only if events exist
system.RunIf(on_message[MoveCommand]())     // run only if messages exist
```

### 4.7 Deferred Call Deduplication

When multiple systems trigger the same deferred event or callback within a single frame, the engine deduplicates by hashing `(target, event_type)`:

```plaintext
DeferredCallKey = hash(target_entity, event_type_id)

deferred_calls: HashMap[DeferredCallKey, DeferredCall]
```

If a call with the same key is already queued, the new call replaces it (last-writer-wins) rather than duplicating. This prevents redundant processing — for example, if three systems all mark the same entity as "needs layout recalculation", only one recalculation runs.

Deduplication is opt-in per event type. Events registered with `AddEvent[T]()` use standard (non-deduplicated) delivery. Events registered with `AddDedupEvent[T]()` use deduplication.

### 4.8 Event Testing Utilities

The event system provides a testing utility for asserting event emissions without mock frameworks:

```plaintext
// Test code
watcher := EventWatcher.Watch(bus, "CollisionEvent")

// ... perform actions that should trigger events ...

watcher.Check("CollisionEvent", []any{entity_a, entity_b})   // assert event with args
watcher.CheckNone("DeathEvent")                               // assert no event
watcher.Clear()                                                // reset between test cases
```

`EventWatcher` connects to the event bus and records all emissions with their arguments into an internal map. Between test cases, `Clear()` resets state. This is composable and requires no mock infrastructure — it uses the real event bus.

### 4.9 Component Change Notification Chain

When a component is added, removed, or replaced on an entity, a structured notification chain propagates through the system registry:

```plaintext
Entity.OnComponentChanged(index, oldComponent, newComponent)
  │
  ├─ EntityManager.NotifyComponentChanged(entity, oldType, newType)
  │   │
  │   ├─ Phase 1 — Discover new processors:
  │   │   CollectProcessorsByComponentType(newType)
  │   │   → auto-instantiate processors for unknown types (see system-scheduling.md §4.10)
  │   │
  │   ├─ Phase 2 — Process removal (if oldComponent != nil):
  │   │   For each processor matching oldType:
  │   │     processor.ProcessEntityComponent(entity, oldComponent, forceRemove=true)
  │   │
  │   ├─ Phase 3 — Process addition (if newComponent != nil):
  │   │   For each processor matching newType:
  │   │     processor.ProcessEntityComponent(entity, newComponent, forceRemove=false)
  │   │
  │   └─ Phase 4 — Update dependents:
  │       For each component on entity that has dependent processors:
  │         dependentProcessor.Revalidate(entity, component)
```

**Why Phase 4 matters**: When entity E has components [A, B, C] and a processor requires both A and B, adding C doesn't directly affect that processor. But if another processor requires A and C, it now matches — and the A-processor may need to revalidate its cached data because the entity's component set changed. Phase 4 handles this cascade.

**Reentrancy safety**: The notification chain is protected by a reentrancy guard. If a processor's `ProcessEntityComponent` callback modifies components on other entities, those notifications are queued and processed after the current chain completes. This prevents infinite loops while ensuring all changes are eventually propagated.

## 5. Open Questions

- Should event buses support priority ordering?
- Maximum event buffer size — should there be backpressure or just unbounded?
- Should observers support async execution (fire-and-forget to a worker thread)?

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
| 0.2.0 | 2026-03-26 | Added deferred call deduplication, event testing utilities (EventWatcher) |
| 0.3.0 | 2026-03-26 | Added component change notification chain with reentrancy safety |
| 0.4.0 | 2026-05-14 | Phase 1 implementation complete; validated by `examples/ecs/poc` |
