package event

import (
	"iter"
	"reflect"

	"github.com/teratron/boltengine/internal/ecs/world"
)

const defaultEventBusCap = 64

// EventBus is the double-buffered storage for broadcast events of type T.
// Two slices alternate roles each frame:
//
//   - the "current" slice receives every [EventWriter.Send] this tick;
//   - the "previous" slice still carries the writes from the prior tick.
//
// A monotonic global counter (sentCount) and the boundary (baseCount) where
// the current buffer starts let [EventReader] track progress with a single
// integer cursor across rotations.
//
// EventBus is NOT safe for concurrent Send/Read; system-parameter access is
// scheduler-coordinated. Construct via [RegisterEvent].
type EventBus[T any] struct {
	buffers   [2][]T
	current   uint8 // 0 or 1 — index of the write buffer
	sentCount int   // monotonic count of every Send across the bus's lifetime
	baseCount int   // global index where buffers[current] starts
}

func newEventBus[T any]() *EventBus[T] {
	return &EventBus[T]{
		buffers: [2][]T{
			make([]T, 0, defaultEventBusCap),
			make([]T, 0, defaultEventBusCap),
		},
	}
}

// RegisterEvent installs an EventBus[T] on w (or returns the existing one).
// The bus is stored as a `*EventBus[T]` resource and tracked in the world's
// [Registry] so [SwapAll] can rotate it each frame.
func RegisterEvent[T any](w *world.World) *EventBus[T] {
	if existing := lookupBus[T](w); existing != nil {
		return existing
	}
	bus := newEventBus[T]()
	world.SetResource(w, bus)
	reg := EnsureRegistry(w)
	reg.buses[reflect.TypeFor[T]()] = bus
	return bus
}

// Bus returns the registered EventBus[T] on w, or nil if [RegisterEvent] has
// not yet been called for T.
func Bus[T any](w *world.World) *EventBus[T] {
	return lookupBus[T](w)
}

func lookupBus[T any](w *world.World) *EventBus[T] {
	pp, ok := world.Resource[*EventBus[T]](w)
	if !ok || pp == nil {
		return nil
	}
	return *pp
}

// Send appends e to the current write buffer. Pointer-receiver, no return
// value — symmetric with [EventWriter.Send].
func (b *EventBus[T]) Send(e T) {
	b.buffers[b.current] = append(b.buffers[b.current], e)
	b.sentCount++
}

// Swap rotates the double buffer at the frame boundary: the current buffer
// becomes "previous", the previous buffer is cleared and becomes the new
// "current". After Swap a fresh tick can write to a clean buffer while
// readers still observe the prior tick's events. baseCount tracks the global
// index where the new current starts.
func (b *EventBus[T]) Swap() {
	next := 1 - b.current
	b.buffers[next] = b.buffers[next][:0] // clear the slice that's about to receive new writes
	b.current = next
	b.baseCount = b.sentCount
}

// swap satisfies the type-erased swapper interface so [SwapAll] can iterate.
func (b *EventBus[T]) swap() { b.Swap() }

// Len reports how many events are stored across both buffers (i.e. visible
// to a freshly-constructed reader). Useful for diagnostics and tests.
func (b *EventBus[T]) Len() int {
	return len(b.buffers[0]) + len(b.buffers[1])
}

// SentCount returns the monotonic global event index. Equal to the cursor
// value of a reader that has consumed every event ever sent on this bus.
func (b *EventBus[T]) SentCount() int { return b.sentCount }

// readAt returns the event at the given global index, advancing the cursor
// past any lost (>2 frames old) entries. Reports false when *cursor is at or
// beyond [EventBus.SentCount].
func (b *EventBus[T]) readAt(cursor *int) (T, bool) {
	prevIdx := 1 - b.current
	prevLen := len(b.buffers[prevIdx])
	oldest := b.baseCount - prevLen
	if *cursor < oldest {
		// Lost events: fast-forward to the oldest still-available entry.
		*cursor = oldest
	}
	if *cursor < b.baseCount {
		e := b.buffers[prevIdx][*cursor-oldest]
		*cursor++
		return e, true
	}
	if *cursor < b.sentCount {
		e := b.buffers[b.current][*cursor-b.baseCount]
		*cursor++
		return e, true
	}
	var zero T
	return zero, false
}

// EventWriter is a thin handle over an [EventBus] specialised for sends. It
// carries no state beyond the bus pointer and is safe to copy.
type EventWriter[T any] struct {
	bus *EventBus[T]
}

// NewEventWriter resolves the EventBus[T] on w and returns a writer.
// Panics if [RegisterEvent] has not yet been called for T.
func NewEventWriter[T any](w *world.World) *EventWriter[T] {
	bus := lookupBus[T](w)
	if bus == nil {
		panic("ecs/event: NewEventWriter requires RegisterEvent first")
	}
	return &EventWriter[T]{bus: bus}
}

// Send forwards to the underlying [EventBus.Send].
func (w *EventWriter[T]) Send(e T) { w.bus.Send(e) }

// SendBatch appends events in order. Equivalent to a Send loop but
// collapses the bookkeeping into a single sentCount update.
func (w *EventWriter[T]) SendBatch(events []T) {
	if len(events) == 0 {
		return
	}
	w.bus.buffers[w.bus.current] = append(w.bus.buffers[w.bus.current], events...)
	w.bus.sentCount += len(events)
}

// EventReader observes events on an [EventBus] with an independent cursor.
// Each Read call advances the cursor past every yielded event so a subsequent
// Read sees only newer entries. Two readers on the same bus consume the same
// events independently.
type EventReader[T any] struct {
	bus    *EventBus[T]
	cursor int
}

// NewEventReader returns a reader positioned to see every currently-buffered
// event. Equivalent to setting the cursor at the oldest available entry.
// Panics if T's bus is not registered.
func NewEventReader[T any](w *world.World) *EventReader[T] {
	bus := lookupBus[T](w)
	if bus == nil {
		panic("ecs/event: NewEventReader requires RegisterEvent first")
	}
	return &EventReader[T]{bus: bus}
}

// NewEventReaderAt returns a reader positioned at the bus's current send
// frontier. The reader sees only events sent strictly after construction.
func NewEventReaderAt[T any](w *world.World) *EventReader[T] {
	bus := lookupBus[T](w)
	if bus == nil {
		panic("ecs/event: NewEventReaderAt requires RegisterEvent first")
	}
	return &EventReader[T]{bus: bus, cursor: bus.sentCount}
}

// Len returns the count of unread events for this reader, accounting for
// lost (>2 frame old) entries.
func (r *EventReader[T]) Len() int {
	prevIdx := 1 - r.bus.current
	oldest := r.bus.baseCount - len(r.bus.buffers[prevIdx])
	c := r.cursor
	if c < oldest {
		c = oldest
	}
	return r.bus.sentCount - c
}

// IsEmpty reports whether [EventReader.Len] is zero.
func (r *EventReader[T]) IsEmpty() bool { return r.Len() == 0 }

// Clear advances the reader's cursor past every currently-stored event.
// Subsequent Reads see only future Sends.
func (r *EventReader[T]) Clear() { r.cursor = r.bus.sentCount }

// All returns a Go 1.23 range-over-func iterator over unread events.
// The cursor is advanced past every yielded event; calling All again returns
// only events arrived since the previous traversal. Lost events (older than
// the retention window) are silently skipped via [EventBus.readAt].
func (r *EventReader[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for {
			e, ok := r.bus.readAt(&r.cursor)
			if !ok {
				return
			}
			if !yield(e) {
				return
			}
		}
	}
}
