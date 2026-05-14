# Event System — Go Implementation

**Version:** 0.1.0
**Status:** Draft
**Layer:** go
**Implements:** [event-system.md](l1-event-system.md)

## Overview

Go-level design for the three communication mechanisms: **Events** (broadcast, double-buffered), **Messages** (system-to-system, cursor-based ring buffer), and **Observers** (reactive triggers). Covers type definitions, registration, read/write system parameters, buffer management, observer dispatch, entity event bubbling, and concurrency constraints.

## Related Specifications

- [event-system.md](l1-event-system.md) — L1 concept specification (parent)

## 1. Motivation

The Go implementation of the Event system provides decoupled communication between systems and reactive responses to World changes. It ensures:

- Double-buffered "broadcast" events for frame-to-frame signaling.
- High-performance, cursor-based message channels for directed system-to-system communication.
- Low-latency observers for immediate reaction to entity structural changes.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: Relies on generics for type-safe event/message streams and `iter` for consumption.
- **Double-buffering**: Events are retained for exactly two frames to ensure all systems have a chance to read them.
- **Observer Depth**: A hard recursion limit (default 64) prevents infinite trigger loops.

## 3. Core Invariants

> [!NOTE]
> See [event-system.md §3](l1-event-system.md) for technology-agnostic invariants.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: Broadcast Delivery | `EventBus[T]` uses double-buffering; `EventReader` cursors track progress across buffers. |
| **INV-2**: Directed Messages | `MessageChannel[T]` uses a ring buffer with per-reader cursors to prevent global consumption. |
| **INV-3**: Immediate Response | `TriggerObservers` executes callbacks synchronously during the triggering action. |
| **INV-4**: Re-entrancy Guard | `ObserverContext` provides a `DeferredWorld` to restrict available mutations during callbacks. |
| **INV-5**: Bubbling Support | `bubbleEvent` traverses the `ChildOf` hierarchy until Root or `StopPropagation`. |

## Go Package

```
internal/ecs
```

No external dependencies. All buffer management uses standard slices and maps.

## Type Definitions

### Events (Broadcast)

```go
// EventBus[T] stores a double-buffered event stream for type T.
// Buffer A is "current" (writers append here), Buffer B is "previous"
// (readers consume from both). Buffers swap each frame.
type EventBus[T any] struct {
    buffers  [2][]T    // double buffer: [current, previous]
    current  uint8     // index of the current write buffer (0 or 1)
    eventID  EventID   // unique identifier for this event type
}

// EventID uniquely identifies a registered event type.
type EventID uint32

// EventWriter[T] is a system parameter that appends events to the current buffer.
type EventWriter[T any] struct {
    bus *EventBus[T]
}

// EventReader[T] is a system parameter that reads events from both buffers.
// Maintains a cursor to ensure each event is seen exactly once per reader.
type EventReader[T any] struct {
    bus         *EventBus[T]
    lastCount   int    // number of events already read from previous buffer
    currentRead int    // number of events already read from current buffer
}
```

### Messages (System-to-System)

```go
// MessageChannel[T] is a ring buffer with per-reader cursor tracking.
// Messages persist until all registered readers have advanced past them.
type MessageChannel[T any] struct {
    buffer    []T            // ring buffer storage
    head      uint64         // write position (monotonically increasing)
    capacity  int            // ring buffer capacity
    readers   []uint64       // per-reader cursor positions
    readerIDs map[uint32]int // readerID -> index in readers slice
    messageID MessageID      // unique identifier for this message type
}

// MessageID uniquely identifies a registered message type.
type MessageID uint32

// MessageWriter[T] is a system parameter that appends messages to the channel.
type MessageWriter[T any] struct {
    channel *MessageChannel[T]
}

// MessageReader[T] is a system parameter that reads messages from a channel.
// Each reader has an independent cursor — messages are not consumed globally.
type MessageReader[T any] struct {
    channel  *MessageChannel[T]
    readerID uint32
}
```

### Observers (Reactive)

```go
// Observer binds a trigger type to a callback function.
type Observer struct {
    id          ObserverID
    triggerType TriggerType
    callback    ObserverCallback
    entity      *Entity          // nil for global observers, set for entity-targeted
}

// ObserverID uniquely identifies a registered observer.
type ObserverID uint32

// ObserverCallback is the function signature for observer handlers.
// Receives a DeferredWorld (limited access to prevent re-entrancy)
// and the trigger event payload.
type ObserverCallback func(ctx *ObserverContext)

// ObserverContext provides the callback with event data and world access.
type ObserverContext struct {
    world           *DeferredWorld
    event           any
    entity          Entity        // the entity that triggered the event
    propagationStop bool          // set to true to stop bubbling
}

// TriggerType identifies what kind of event triggers an observer.
type TriggerType struct {
    Kind   TriggerKind
    TypeID TypeID      // the component or event type involved
}

// TriggerKind enumerates the built-in trigger categories.
type TriggerKind uint8

const (
    TriggerOnAdd     TriggerKind = iota // component added to entity for first time
    TriggerOnInsert                      // component value inserted (add or overwrite)
    TriggerOnReplace                     // component value replaced (was already present)
    TriggerOnRemove                      // component removed from entity
    TriggerOnEvent                       // custom event type
)
```

### Trigger Type Helpers

```go
// OnAdd[T] creates a TriggerType for component addition.
type OnAdd[T any] struct{}

// OnInsert[T] creates a TriggerType for component insertion.
type OnInsert[T any] struct{}

// OnReplace[T] creates a TriggerType for component replacement.
type OnReplace[T any] struct{}

// OnRemove[T] creates a TriggerType for component removal.
type OnRemove[T any] struct{}
```

### Observer Registry

```go
// ObserverRegistry stores all registered observers, indexed by trigger type.
type ObserverRegistry struct {
    global       map[TriggerType][]Observer          // global observers
    entityBound  map[Entity]map[TriggerType][]Observer // per-entity observers
    nextID       ObserverID
}
```

## Key Methods

### Event Registration and Lifecycle

```
// RegisterEvent[T] registers an event type with the World,
// creating its EventBus[T] and storing it as a resource.
func RegisterEvent[T any](world *World)

// SwapEventBuffers rotates the double buffer for all registered event buses.
// Called once per frame by the schedule runner before systems execute.
//   current becomes previous, previous is cleared and becomes current.
func SwapEventBuffers(world *World)
```

### EventWriter Methods

```
// Send appends an event to the current write buffer.
func (w *EventWriter[T]) Send(event T)

// SendBatch appends multiple events to the current write buffer.
func (w *EventWriter[T]) SendBatch(events []T)
```

### EventReader Methods

```
// Read returns an iterator over all unread events across both buffers.
// Events from the previous frame are yielded first, then current frame events.
// Each call advances the cursor — calling Read again yields only new events.
func (r *EventReader[T]) Read() EventIterator[T]

// Len returns the number of unread events.
func (r *EventReader[T]) Len() int

// IsEmpty returns true if there are no unread events.
func (r *EventReader[T]) IsEmpty() bool

// Clear advances the cursor to the end, discarding all unread events.
func (r *EventReader[T]) Clear()
```

### EventIterator

```
// EventIterator[T] iterates over events without allocation.
type EventIterator[T any] struct { ... }

// Next returns the next event and true, or zero value and false if exhausted.
func (it *EventIterator[T]) Next() (T, bool)
```

### Message Registration and Lifecycle

```
// RegisterMessage[T] registers a message type with the World,
// creating its MessageChannel[T] with the given ring buffer capacity.
func RegisterMessage[T any](world *World, capacity int)

// RegisterMessageReader[T] registers a new reader cursor on the channel.
// Returns a readerID used to construct MessageReader instances.
func RegisterMessageReader[T any](world *World) uint32

// CleanupMessages advances the minimum reader position and discards
// messages that all readers have consumed. Called periodically by the runtime.
func CleanupMessages[T any](world *World)
```

### MessageWriter Methods

```
// Write appends a message to the channel's ring buffer.
// If the buffer is full and the slowest reader has not consumed
// the oldest message, the write overwrites it (lossy under backpressure).
func (w *MessageWriter[T]) Write(msg T)
```

### MessageReader Methods

```
// Read returns an iterator over all unread messages for this reader.
// Advances the reader's cursor.
func (r *MessageReader[T]) Read() MessageIterator[T]

// Len returns the count of unread messages for this reader.
func (r *MessageReader[T]) Len() int

// IsEmpty returns true if no unread messages remain.
func (r *MessageReader[T]) IsEmpty() bool
```

### Observer Registration

```
// AddObserver registers a global observer that fires on the given trigger type.
func (w *World) AddObserver(trigger TriggerType, callback ObserverCallback) ObserverID

// Observe registers an entity-targeted observer.
func (w *World) Observe(entity Entity, trigger TriggerType, callback ObserverCallback) ObserverID

// RemoveObserver unregisters an observer by its ID.
func (w *World) RemoveObserver(id ObserverID)
```

### Observer Dispatch

```
// TriggerObservers fires all matching observers for the given trigger.
// Execution order:
//   1. Entity-bound observers on the target entity (if any).
//   2. Global observers for the trigger type.
//   3. If entity has a ChildOf parent and propagation is not stopped,
//      repeat from step 1 with the parent entity (bubbling).
func TriggerObservers(world *DeferredWorld, trigger TriggerType, entity Entity, event any)
```

### Observer Context Methods

```
// Event returns the trigger event payload, type-asserted to T.
func ObserverContextEvent[T any](ctx *ObserverContext) T

// Entity returns the entity that caused the trigger.
func (ctx *ObserverContext) Entity() Entity

// StopPropagation prevents the event from bubbling to parent entities.
func (ctx *ObserverContext) StopPropagation()

// Commands returns a Commands handle for enqueuing deferred mutations.
func (ctx *ObserverContext) Commands() *Commands
```

### Entity Event Bubbling

```
// bubbleEvent traverses the ChildOf hierarchy upward, dispatching
// observers at each level until the root is reached or propagation is stopped.
//
// Pseudo-code:
//   current = targetEntity
//   while current is valid AND NOT propagationStopped:
//       dispatch entity-bound observers for current
//       dispatch global observers
//       current = current.Get(ChildOf).Parent   // traverse up
func bubbleEvent(world *DeferredWorld, trigger TriggerType, entity Entity, event any)
```

### SystemParam Implementations

```
// EventWriter[T] implements SystemParam.
func (w *EventWriter[T]) Fetch(world *World)   // resolves EventBus[T] from world resources
func (w *EventWriter[T]) Access() []AccessDescriptor  // write access to EventBus[T]

// EventReader[T] implements SystemParam.
func (r *EventReader[T]) Fetch(world *World)   // resolves EventBus[T] from world resources
func (r *EventReader[T]) Access() []AccessDescriptor  // read access to EventBus[T]

// MessageWriter[T] implements SystemParam.
func (w *MessageWriter[T]) Fetch(world *World)
func (w *MessageWriter[T]) Access() []AccessDescriptor  // write access to MessageChannel[T]

// MessageReader[T] implements SystemParam.
func (r *MessageReader[T]) Fetch(world *World)
func (r *MessageReader[T]) Access() []AccessDescriptor  // read access to MessageChannel[T]
```

## Performance Strategy

- **Double-buffer swap** is O(1): swap an index, clear one slice (`len=0`, retain backing array).
- **Event slices grow once** and stabilize: typical games send a bounded number of events per frame. Backing arrays are retained across frames via the swap mechanism.
- **Ring buffer for messages** avoids unbounded growth. Capacity is set at registration time based on expected throughput.
- **Cursor-based reads** are zero-allocation: no copying, no filtering. The iterator walks the underlying slice directly.
- **Observer dispatch** uses pre-built maps keyed by `TriggerType`. No linear scan of all observers.
- **Entity-bound observers** stored in a map keyed by Entity — O(1) lookup per entity during dispatch.
- **No concurrency primitives on hot path**: EventWriter/EventReader are system params with schedule-enforced exclusivity. No mutexes, no atomics during read/write.

## Error Handling

| Condition | Behavior |
| :--- | :--- |
| Event type not registered | Panic at `Fetch` time (programming error) |
| Message channel full (ring buffer wrap) | Overwrite oldest unread message; log warning via `slog.Warn` |
| Observer callback panics | Recovered, wrapped as error, logged. Observer is not removed. |
| Observer infinite loop (A triggers B triggers A) | Depth limit (default 64). Exceeding limit panics with descriptive error. |
| Read on cleared reader | Returns empty iterator (no error) |

```go
var (
    ErrObserverDepthExceeded = errors.New("ecs: observer trigger depth limit exceeded")
)
```

## Testing Strategy

- **Event unit tests**: Write N events, read from two readers, verify each sees all events exactly once. Test buffer rotation across multiple frames.
- **Event frame boundary**: Verify events persist for exactly 2 frames, then disappear.
- **Message unit tests**: Multiple writers, multiple readers with independent cursors. Verify FIFO order and cursor independence.
- **Message backpressure**: Fill ring buffer, verify wrap behavior and warning.
- **Observer unit tests**: Register global and entity-bound observers, trigger, verify callback execution.
- **Observer bubbling**: Parent-child hierarchy, verify upward dispatch and `StopPropagation`.
- **Observer depth limit**: Create circular trigger chain, verify panic at depth limit.
- **Benchmarks**: `BenchmarkEventWrite1000`, `BenchmarkEventRead1000`, `BenchmarkObserverDispatch`, `BenchmarkMessageWrite`.

## 7. Drawbacks & Alternatives

- **Drawback**: Immediate observers can make performance unpredictable if heavily used for frequent events.
- **Alternative**: All events are deferred to the next frame.
- **Decision**: Immediate observers are required for essential logic (e.g. cleanup hooks) and are safe when used with depth limits.

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
