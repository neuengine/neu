# Command System — Go Implementation

**Version:** 0.1.0
**Status:** Draft
**Layer:** go
**Implements:** [command-system.md](l1-command-system.md)

## Overview

Go-level design for the command system. Commands are deferred mutations that buffer structural World changes (spawn, despawn, insert, remove) during system execution and apply them atomically at synchronization points. This spec covers the `Command` interface, `CommandBuffer`, built-in command types, the `Commands` system parameter, entity reservation, the builder pattern, and flush semantics.

## Related Specifications

- [command-system.md](l1-command-system.md) — L1 concept specification (parent)

## 1. Motivation

The Go implementation of the Command system provides a safe, deferred mechanism for modifying the World. It ensures:

- Structural changes (spawn, despawn) do not invalidate pointers during system execution.
- Systems can express mutations without direct, mutable access to the entire World.
- Atomicity of changes at well-defined synchronization points.
- High-performance entity reservation for immediate ID linkage.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: Uses `atomic` for thread-safe entity reservation and `unique` for command tagging if needed.
- **FIFO Ordering**: Commands are applied strictly in the order they were pushed to the buffer.
- **Single-writer sync**: Command application (flush) is a serialized process at synchronization points.

## 3. Core Invariants

> [!NOTE]
> See [command-system.md §3](l1-command-system.md) for technology-agnostic invariants.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: Deferred execution | `CommandBuffer.Push` only stores the command; `Apply` is called at sync points. |
| **INV-2**: Entity reservation | `Commands.Spawn` returns a valid `Entity` ID immediately, backed by an atomic allocator counter. |
| **INV-3**: Builder chaining | `EntityCommands` methods return `*EntityCommands` for fluent API usage. |
| **INV-4**: Valid targets | `Apply` checks entity generation before performing mutations to avoid stale writes. |
| **INV-5**: System context | Each system receives its own `CommandBuffer` via the `Commands` parameter. |

## Go Package

```
internal/ecs
```

No external dependencies. All types use standard library only.

## Type Definitions

### Command Interface

```go
// Command represents a single deferred mutation to the World.
type Command interface {
    Apply(world *World)
}
```

### Command Buffer

```go
// CommandBuffer is a per-system, append-only collection of commands.
// Commands are applied in FIFO order during flush.
type CommandBuffer struct {
    commands []Command
    entities *EntityAllocator // shared reference for entity reservation
}
```

### Built-in Commands

```go
// SpawnCommand creates a new entity with the given components.
type SpawnCommand struct {
    entity     Entity          // reserved ID
    components []ComponentData
}

// SpawnEmptyCommand creates an entity with no components.
type SpawnEmptyCommand struct {
    entity Entity
}

// DespawnCommand destroys an entity and all its components.
type DespawnCommand struct {
    entity Entity
}

// InsertCommand adds or overwrites a component on an entity.
type InsertCommand struct {
    entity    Entity
    component ComponentData
}

// RemoveComponentCommand removes a component by type from an entity.
type RemoveComponentCommand struct {
    entity Entity
    typeID TypeID
}

// InsertResourceCommand inserts or overwrites a global resource.
type InsertResourceCommand struct {
    resource ResourceData
}

// RemoveResourceCommand removes a global resource by type.
type RemoveResourceCommand struct {
    typeID TypeID
}

// SendEventCommand enqueues an event into the event bus.
type SendEventCommand struct {
    event any
}

// TriggerObserverCommand triggers observer callbacks immediately during apply.
type TriggerObserverCommand struct {
    entity Entity
    event  any
}

// CustomCommand wraps a user-provided function as a command.
type CustomCommand struct {
    fn func(world *World)
}
```

### Component Data Wrapper

```go
// ComponentData is a type-erased component value paired with its type metadata.
type ComponentData struct {
    TypeID TypeID
    Value  any
}

// ResourceData is a type-erased resource value paired with its type metadata.
type ResourceData struct {
    TypeID TypeID
    Value  any
}
```

### Commands System Parameter

```go
// Commands is a system parameter providing a builder API for deferred mutations.
// Each system receives its own Commands instance backed by a dedicated CommandBuffer.
type Commands struct {
    buffer *CommandBuffer
}
```

### Entity Commands (Builder)

```go
// EntityCommands provides a chained builder API for a single entity.
type EntityCommands struct {
    entity Entity
    buffer *CommandBuffer
}
```

## Key Methods

### CommandBuffer Lifecycle

```
// NewCommandBuffer creates a buffer with pre-allocated capacity.
func NewCommandBuffer(allocator *EntityAllocator, initialCap int) *CommandBuffer

// Push appends a command to the buffer.
func (cb *CommandBuffer) Push(cmd Command)

// Apply executes all buffered commands in FIFO order against the World.
// After application, the buffer is not cleared — call Reset() explicitly.
func (cb *CommandBuffer) Apply(world *World)

// Reset clears the buffer for reuse, retaining allocated memory.
func (cb *CommandBuffer) Reset()

// Len returns the number of pending commands.
func (cb *CommandBuffer) Len() int
```

### Commands API (System Parameter)

```
// Spawn reserves an entity ID immediately and enqueues a SpawnCommand.
// The returned Entity is valid for use in subsequent commands within
// the same system, but does not exist in the World until flush.
func (c *Commands) Spawn(components ...ComponentData) Entity

// SpawnEmpty reserves an entity ID with no components.
func (c *Commands) SpawnEmpty() Entity

// Entity returns an EntityCommands builder for the given entity.
func (c *Commands) Entity(e Entity) *EntityCommands

// Despawn enqueues a DespawnCommand for the given entity.
func (c *Commands) Despawn(e Entity)

// InsertResource enqueues an InsertResourceCommand.
func (c *Commands) InsertResource(resource ResourceData)

// RemoveResource enqueues a RemoveResourceCommand.
func (c *Commands) RemoveResource(typeID TypeID)

// SendEvent enqueues a SendEventCommand.
func (c *Commands) SendEvent(event any)

// Add enqueues an arbitrary Command.
func (c *Commands) Add(cmd Command)
```

### EntityCommands Builder

```
// Insert adds or overwrites a component on the target entity.
// Returns self for chaining.
func (ec *EntityCommands) Insert(component ComponentData) *EntityCommands

// Remove removes a component by type from the target entity.
// Returns self for chaining.
func (ec *EntityCommands) Remove(typeID TypeID) *EntityCommands

// Despawn enqueues destruction of the target entity.
func (ec *EntityCommands) Despawn()

// WithChildren spawns child entities attached to the target entity via ChildOf.
// The callback receives a ChildSpawner scoped to the parent.
func (ec *EntityCommands) WithChildren(fn func(spawner *ChildSpawner)) *EntityCommands
```

### ChildSpawner

```
// ChildSpawner creates entities that are automatically parented to the owner.
type ChildSpawner struct {
    parent Entity
    buffer *CommandBuffer
}

// Spawn reserves a child entity, enqueues SpawnCommand + Insert(ChildOf{Parent}).
func (cs *ChildSpawner) Spawn(components ...ComponentData) Entity
```

### Entity Reservation

```
// ReserveEntity atomically reserves a new Entity ID from the allocator.
// The entity does not exist in archetypes until the SpawnCommand is applied.
// Thread-safe: uses atomic increment on the allocator's counter.
func (cb *CommandBuffer) ReserveEntity() Entity
```

### Flush Ordering

```
// ApplyDeferredCommands collects command buffers from all systems
// (in system execution order) and applies them sequentially.
// Called by the schedule executor at sync points.
func ApplyDeferredCommands(world *World, buffers []*CommandBuffer)

// Pseudo-code:
//   for each buffer in execution-order:
//       buffer.Apply(world)
//       buffer.Reset()
```

### SystemParam Implementation

```
// Commands implements SystemParam.
func (c *Commands) Fetch(world *World)
func (c *Commands) Access() []AccessDescriptor
// Access returns: write access to CommandBuffer (non-conflicting — each system gets its own)
```

## Performance Strategy

- **Pre-allocated slices**: `CommandBuffer` starts with configurable initial capacity (default 64 commands). Grows via `append` if exceeded, but typical frame usage stays within initial allocation.
- **Reuse across frames**: `Reset()` sets `len=0` without freeing backing array. Buffers persist for the lifetime of the system.
- **No interface boxing on hot path**: Built-in commands are concrete struct types. Only custom commands use the `Command` interface indirectly.
- **Entity reservation is lock-free**: `EntityAllocator` uses `atomic.AddUint64` for ID generation. No mutex contention between parallel systems.
- **FIFO application**: Simple slice iteration, no sorting or priority queue overhead.
- **ComponentData pooling**: For high-frequency spawn patterns, a `sync.Pool` of `[]ComponentData` slices reduces allocation pressure.

## Error Handling

| Condition | Behavior |
| :--- | :--- |
| Despawn on non-existent entity | No-op (INV-4 from L1). Logged at debug level via `slog.Debug`. |
| Insert on despawned entity | No-op. The entity generation check prevents stale writes. |
| Remove component not present | No-op. |
| Nil command pushed | Panic with descriptive message (programming error). |
| Resource type not registered | Panic at `Apply` time (programming error, should be caught in tests). |

```go
// No sentinel errors — command failures are either no-ops or panics
// (programming errors that should not occur in correct code).
```

## Testing Strategy

- **Unit tests**: `CommandBuffer` push, apply, reset; verify FIFO order.
- **Entity reservation**: Concurrent reservation from multiple goroutines, verify unique IDs.
- **Builder pattern**: Chain `Insert`, `Remove`, `WithChildren`, verify correct command sequence.
- **Integration tests**: Full schedule with systems producing commands, verify World state after `ApplyDeferred`.
- **Stale entity tests**: Despawn then insert on same entity, verify no-op.
- **Benchmarks**: `BenchmarkSpawn1000Entities`, `BenchmarkCommandBufferApply`, `BenchmarkEntityReservation` (parallel).

## 7. Drawbacks & Alternatives

- **Drawback**: Deferred commands can hide logic errors until the end of the frame (flush time).
- **Alternative**: Immediate World modification with widespread locking.
- **Decision**: Deferred commands are much faster and avoid the complexity of fine-grained locking.

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
| 0.2.0 | 2026-05-14 | Phase 1 implementation complete; validated by `examples/ecs/poc` |
