# Command System

**Version:** 0.1.0
**Status:** Draft
**Layer:** concept

## Overview

Commands are deferred mutations to the World. Systems cannot directly perform structural changes (spawn, despawn, add/remove components) because these would invalidate active query iterators. Instead, systems enqueue commands into a buffer, which is applied at defined synchronization points.

## Related Specifications

- [world-system.md](l1-world-system.md) — Commands are applied to the World
- [entity-system.md](l1-entity-system.md) — Spawn/despawn commands create/destroy entities
- [component-system.md](l1-component-system.md) — Insert/remove commands modify entity components
- [system-scheduling.md](l1-system-scheduling.md) — ApplyDeferred sync points in schedule

## 1. Motivation

- Buffering all structural changes.
- Applying them atomically at safe synchronization points.
- Allowing systems to "fire and forget" mutations without worrying about ordering.

## 2. Constraints & Assumptions

- Commands are per-system buffers, merged and applied sequentially.
- Command application order within a buffer is FIFO (first-in, first-out).
- Commands from different systems are applied in system execution order.
- Command application can trigger component hooks and observers.

## 3. Core Invariants

- **INV-1**: Commands are never applied during system execution — only at ApplyDeferred points.
- **INV-2**: Command buffers are flushed completely at each sync point — no partial application.
- **INV-3**: Entity IDs reserved by Commands are valid immediately (can be used in subsequent commands in the same buffer).
- **INV-4**: A command that references a despawned entity is a no-op (not an error).

## 4. Detailed Design

### 4.1 Built-in Commands

| Command | Description |
| :--- | :--- |
| `Spawn(components...)` | Create new entity with components |
| `SpawnEmpty()` | Create entity with no components |
| `Despawn(entity)` | Destroy entity and all its components |
| `Insert(entity, component)` | Add/overwrite component on entity |
| `Remove[T](entity)` | Remove component T from entity |
| `InsertResource(resource)` | Insert or overwrite a global resource |
| `RemoveResource[T]()` | Remove a resource |
| `SendEvent(event)` | Enqueue an event |
| `Trigger(event)` | Trigger an observer event |
| `RunSystem(system)` | Schedule a one-shot system to run |

### 4.2 Entity Commands (Builder Pattern)

```go
commands.Spawn(PlayerBundle{...})
    .Insert(Health{100})
    .Insert(Name{"Player 1"})
    .WithChildren(func(parent) {
        parent.Spawn(WeaponBundle{...})
    })
```

`Spawn` returns a handle to the not-yet-created entity. Subsequent `.Insert()` calls append to the same command buffer entry. The entity ID is reserved immediately and can be stored.

### 4.3 Custom Commands

Users can define custom commands implementing a `Command` interface:

```go
interface Command {
    Apply(world *World)
}
```

Custom commands have full `&mut World` access when applied. Use for complex operations that touch multiple entities or resources atomically.

### 4.4 Command Queue Lifecycle

1. System runs → writes commands to its per-system `CommandBuffer`.
2. System completes → buffer is sealed.
3. `ApplyDeferred` sync point reached → all pending buffers applied in system order.
4. Each command's `Apply(world)` runs sequentially.
5. Buffers cleared after application.

### 4.6 Command Components (Declarative Pattern)

In addition to deferred buffers, systems can use "Command Components" to trigger logic in other systems:

1. **Definition**: A plain component type representing a one-time request (e.g., `SpawnExplosion { pos Vec3 }`).
2. **Usage**: A system adds this component to an entity (often a temporary one).
3. **Processing**: A target system queries for this component, performs the action, and then removes the component (or despawns the entity) before the end of the frame.
4. **Benefit**: Decouples systems using the data-oriented pipeline instead of imperative buffers.

## 5. Patterns

### 5.1 One-Shot Actions

Use `Command Components` for actions that should be processed by specific systems in the standard update loop.

### 5.2 Structural Changes

Use `CommandBuffer` for spawn, despawn, and component insert/remove to ensure iterator safety.

## 6. Open Questions

- Should commands support rollback/undo for editor integration?
- Priority commands: should some commands apply before others regardless of system order?

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
