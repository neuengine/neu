package interp

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/neuengine/neu/internal/ecs/entity"
)

// EntityState holds component values for one entity at one snapshot tick.
// Keys are component type names; values are JSON-encoded component data.
type EntityState map[string]json.RawMessage

// SnapshotEntry is one server-tick worth of authoritative entity state.
type SnapshotEntry struct {
	Tick       uint64
	Timestamp  time.Duration // server-side monotonic time at this tick
	ReceivedAt time.Duration // local render-clock time when received
	Entities   map[entity.EntityID]EntityState
}

// DefaultBufferCapacity is the default number of entries in a SnapshotBuffer.
const DefaultBufferCapacity = 32

// SnapshotBuffer is a bounded, tick-sorted ring of server snapshots.
// Out-of-order and duplicate entries (Tick <= latestTick) are discarded (INV-3).
// Not safe for concurrent use; driven from the ECS receive thread.
type SnapshotBuffer struct {
	ring       []SnapshotEntry
	capacity   int
	latestTick uint64
}

// NewSnapshotBuffer returns a SnapshotBuffer with the given capacity.
// capacity <= 0 defaults to DefaultBufferCapacity.
func NewSnapshotBuffer(capacity int) *SnapshotBuffer {
	if capacity <= 0 {
		capacity = DefaultBufferCapacity
	}
	return &SnapshotBuffer{
		ring:     make([]SnapshotEntry, 0, capacity),
		capacity: capacity,
	}
}

// Insert adds e to the buffer if e.Tick > latestBufferedTick (INV-3).
// Entries are stored in ascending Tick order; the oldest is evicted when full.
func (b *SnapshotBuffer) Insert(e SnapshotEntry) {
	if e.Tick <= b.latestTick {
		return // out-of-order or duplicate — discard (INV-3)
	}
	b.latestTick = e.Tick

	// Find insertion point to maintain sorted order.
	pos := sort.Search(len(b.ring), func(i int) bool {
		return b.ring[i].Tick >= e.Tick
	})

	if len(b.ring) < b.capacity {
		b.ring = append(b.ring, SnapshotEntry{})
		copy(b.ring[pos+1:], b.ring[pos:])
		b.ring[pos] = e
		return
	}
	// Ring full: shift left to evict oldest, insert at sorted position.
	if pos == 0 {
		// New entry would go at front but we'd evict it immediately — skip.
		return
	}
	copy(b.ring, b.ring[1:])
	pos-- // adjust after eviction shift
	b.ring[len(b.ring)-1] = SnapshotEntry{}
	// Insert at pos within [0, len-1].
	copy(b.ring[pos+1:], b.ring[pos:len(b.ring)-1])
	b.ring[pos] = e
}

// Bracket finds the two consecutive entries prev, next such that
// prev.Timestamp <= renderTime < next.Timestamp.
// Returns !ok when no such pair exists.
func (buf *SnapshotBuffer) Bracket(renderTime time.Duration) (prev, next SnapshotEntry, ok bool) {
	for i := 0; i+1 < len(buf.ring); i++ {
		if buf.ring[i].Timestamp <= renderTime && renderTime < buf.ring[i+1].Timestamp {
			return buf.ring[i], buf.ring[i+1], true
		}
	}
	return SnapshotEntry{}, SnapshotEntry{}, false
}

// Len returns the number of buffered entries.
func (b *SnapshotBuffer) Len() int { return len(b.ring) }

// Latest returns the most recently buffered entry and whether one exists.
func (b *SnapshotBuffer) Latest() (SnapshotEntry, bool) {
	if len(b.ring) == 0 {
		return SnapshotEntry{}, false
	}
	return b.ring[len(b.ring)-1], true
}
