package event

import (
	"iter"
	"reflect"

	"github.com/neuengine/neu/internal/ecs/world"
)

const defaultMessageChannelCap = 64

// MessageChannel is a directed system-to-system message stream backed by a
// fixed-capacity ring buffer. Multiple readers each maintain an independent
// monotonic cursor; messages persist until every active reader has advanced
// past them OR until the buffer wraps and overwrites them (lossy under
// backpressure — the slowest reader silently fast-forwards on the next
// Read to skip the lost range).
//
// Unlike [EventBus] there is no double buffering: every write mutates the
// shared backing array in place. Readers cannot rewind. Construct via
// [RegisterMessage] and obtain typed reader handles via [NewMessageReader].
type MessageChannel[T any] struct {
	readers  map[uint32]uint64 // readerID -> monotonic cursor
	buffer   []T
	capacity int
	head     uint64 // monotonic global write position
	nextID   uint32
}

func newMessageChannel[T any](capacity int) *MessageChannel[T] {
	if capacity <= 0 {
		capacity = defaultMessageChannelCap
	}
	return &MessageChannel[T]{
		buffer:   make([]T, capacity),
		capacity: capacity,
		readers:  make(map[uint32]uint64),
	}
}

// RegisterMessage installs a MessageChannel[T] on w (or returns the existing
// one). The channel is stored as a `*MessageChannel[T]` resource and tracked
// in the world's [Registry] so [CleanupAll] can iterate it.
//
// capacity is the ring buffer size in messages. Values ≤ 0 fall back to the
// package default. Subsequent calls with a different capacity are ignored —
// the existing channel is returned unchanged.
func RegisterMessage[T any](w *world.World, capacity int) *MessageChannel[T] {
	if existing := lookupChannel[T](w); existing != nil {
		return existing
	}
	ch := newMessageChannel[T](capacity)
	world.SetResource(w, ch)
	reg := EnsureRegistry(w)
	reg.channels[reflect.TypeFor[T]()] = ch
	return ch
}

// Channel returns the registered MessageChannel[T] on w, or nil if
// [RegisterMessage] has not been called for T.
func Channel[T any](w *world.World) *MessageChannel[T] {
	return lookupChannel[T](w)
}

func lookupChannel[T any](w *world.World) *MessageChannel[T] {
	pp, ok := world.Resource[*MessageChannel[T]](w)
	if !ok || pp == nil {
		return nil
	}
	return *pp
}

// Capacity returns the ring buffer capacity (max messages retained).
func (c *MessageChannel[T]) Capacity() int { return c.capacity }

// Head returns the monotonic write position; equal to the cursor of a reader
// that has consumed every message ever written.
func (c *MessageChannel[T]) Head() uint64 { return c.head }

// ReaderCount returns the number of registered readers on this channel.
func (c *MessageChannel[T]) ReaderCount() int { return len(c.readers) }

// Write appends msg to the channel. If the buffer is full, the oldest
// not-yet-read message is overwritten — slow readers will fast-forward past
// the lost range on their next Read.
func (c *MessageChannel[T]) Write(msg T) {
	idx := c.head % uint64(c.capacity)
	c.buffer[idx] = msg
	c.head++
}

// RegisterReader allocates a new reader cursor positioned at the current
// head, so the new reader sees only messages written from this point on.
// Returns the reader ID for use with [MessageChannel.Read] and
// [MessageChannel.UnregisterReader].
func (c *MessageChannel[T]) RegisterReader() uint32 {
	c.nextID++
	id := c.nextID
	c.readers[id] = c.head
	return id
}

// UnregisterReader drops the cursor for readerID. Subsequent reads under the
// same ID return an empty iterator.
func (c *MessageChannel[T]) UnregisterReader(readerID uint32) {
	delete(c.readers, readerID)
}

// read iterates over unread messages for readerID, advancing the cursor past
// every yielded message. Lost messages (older than head-capacity) are
// silently skipped.
func (c *MessageChannel[T]) read(readerID uint32) iter.Seq[T] {
	return func(yield func(T) bool) {
		cursor, ok := c.readers[readerID]
		if !ok {
			return
		}
		// Fast-forward past the lost range if the writer has lapped us.
		if c.head > uint64(c.capacity) {
			oldest := c.head - uint64(c.capacity)
			if cursor < oldest {
				cursor = oldest
			}
		}
		for cursor < c.head {
			idx := cursor % uint64(c.capacity)
			msg := c.buffer[idx]
			cursor++
			if !yield(msg) {
				c.readers[readerID] = cursor
				return
			}
		}
		c.readers[readerID] = cursor
	}
}

// cleanup satisfies the type-erased cleaner interface. Ring-buffer storage
// has no per-tick reclamation; the hook exists so [CleanupAll] can iterate
// every registered channel uniformly.
func (c *MessageChannel[T]) cleanup() {}

// MessageWriter is a thin handle for sending into a [MessageChannel].
type MessageWriter[T any] struct {
	channel *MessageChannel[T]
}

// NewMessageWriter resolves the channel for T from w and returns a writer.
// Panics if [RegisterMessage] has not been called for T.
func NewMessageWriter[T any](w *world.World) *MessageWriter[T] {
	ch := lookupChannel[T](w)
	if ch == nil {
		panic("ecs/event: NewMessageWriter requires RegisterMessage first")
	}
	return &MessageWriter[T]{channel: ch}
}

// Write forwards to [MessageChannel.Write].
func (w *MessageWriter[T]) Write(msg T) { w.channel.Write(msg) }

// MessageReader observes messages on a [MessageChannel] with an independent
// monotonic cursor. Each reader sees every message written after its
// registration, subject to ring-buffer wrap.
type MessageReader[T any] struct {
	channel  *MessageChannel[T]
	readerID uint32
}

// NewMessageReader registers a fresh cursor on T's channel and returns a
// handle. Each reader is independent. Panics if [RegisterMessage] has not
// been called for T.
func NewMessageReader[T any](w *world.World) *MessageReader[T] {
	ch := lookupChannel[T](w)
	if ch == nil {
		panic("ecs/event: NewMessageReader requires RegisterMessage first")
	}
	return &MessageReader[T]{channel: ch, readerID: ch.RegisterReader()}
}

// All returns a Go 1.23 range-over-func iterator over unread messages.
func (r *MessageReader[T]) All() iter.Seq[T] { return r.channel.read(r.readerID) }

// Len reports unread messages for this reader, accounting for lost range.
func (r *MessageReader[T]) Len() int {
	cursor, ok := r.channel.readers[r.readerID]
	if !ok {
		return 0
	}
	if r.channel.head > uint64(r.channel.capacity) {
		oldest := r.channel.head - uint64(r.channel.capacity)
		if cursor < oldest {
			cursor = oldest
		}
	}
	return int(r.channel.head - cursor)
}

// IsEmpty reports whether the reader's [MessageReader.Len] is zero.
func (r *MessageReader[T]) IsEmpty() bool { return r.Len() == 0 }

// Close drops this reader's cursor on the channel. Calling [MessageReader.All]
// after Close yields an empty iterator.
func (r *MessageReader[T]) Close() {
	r.channel.UnregisterReader(r.readerID)
}
