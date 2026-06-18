package predict

import "github.com/neuengine/neu/internal/ecs/entity"

// DefaultHistoryCapacity is the default depth of the prediction ring (~1 s at 60 Hz).
const DefaultHistoryCapacity = 64

// PredictedSnapshot holds the serialized component bytes for one entity at one
// predicted tick, together with a crc32 checksum for fast mismatch detection
// (INV-3). Components are keyed by their full type name so restoration can
// match them to current registry IDs without assuming stable ordering.
type PredictedSnapshot struct {
	// Components maps component type name to its JSON-encoded value at this tick.
	Components map[string][]byte
	// Checksum is the crc32 of the concatenated Components bytes in sorted key order.
	Checksum uint32
}

// PredictionEntry is the prediction record for one tick: per-entity snapshots
// and a global checksum covering all predicted entities. The global Checksum is
// used for fast per-tick reconciliation; per-entity Entities are used by the
// reconciliation system to locate exactly which entities diverged.
type PredictionEntry struct {
	// Entities maps each predicted entity to its snapshot at this tick.
	// Populated by RecordFull; nil when only the checksum is stored.
	Entities map[entity.EntityID]PredictedSnapshot
	// Tick is the simulation tick this entry covers.
	Tick uint64
	// Checksum is the CRC32 of the full world snapshot at this tick, taken via
	// SnapshotManager. Compared against the server's confirmed checksum to detect
	// a misprediction without deserializing every entity.
	Checksum uint32
}

// PredictionHistory is a bounded ring of PredictionEntry values for recent
// predicted ticks. Entries older than the ring capacity are evicted automatically.
// Not safe for concurrent use; driven from the simulation thread.
type PredictionHistory struct {
	ring     []PredictionEntry
	capacity int
}

// NewPredictionHistory returns an empty history with the given capacity.
// A capacity ≤ 0 uses DefaultHistoryCapacity.
func NewPredictionHistory(capacity int) *PredictionHistory {
	if capacity <= 0 {
		capacity = DefaultHistoryCapacity
	}
	return &PredictionHistory{capacity: capacity}
}

// RecordTick stores or updates the prediction for tick with the given world
// checksum. Call once after each simulated tick via SnapshotManager.Get(tick).
func (h *PredictionHistory) RecordTick(tick uint64, checksum uint32) {
	for i := range h.ring {
		if h.ring[i].Tick == tick {
			h.ring[i].Checksum = checksum
			return
		}
	}
	h.insert(PredictionEntry{Tick: tick, Checksum: checksum})
}

// RecordFull stores a full per-entity snapshot for tick (used by the
// reconciliation system when it needs individual entity data for restoration).
func (h *PredictionHistory) RecordFull(tick uint64, checksum uint32, entities map[entity.EntityID]PredictedSnapshot) {
	for i := range h.ring {
		if h.ring[i].Tick == tick {
			h.ring[i].Checksum = checksum
			h.ring[i].Entities = entities
			return
		}
	}
	h.insert(PredictionEntry{Tick: tick, Checksum: checksum, Entities: entities})
}

// insert appends e to the ring, evicting the oldest entry when full.
func (h *PredictionHistory) insert(e PredictionEntry) {
	if len(h.ring) >= h.capacity {
		copy(h.ring, h.ring[1:])
		h.ring[len(h.ring)-1] = e
		return
	}
	h.ring = append(h.ring, e)
}

// Get returns the prediction entry for tick and whether it is still in the ring.
func (h *PredictionHistory) Get(tick uint64) (PredictionEntry, bool) {
	for i := range h.ring {
		if h.ring[i].Tick == tick {
			return h.ring[i], true
		}
	}
	return PredictionEntry{}, false
}

// DiscardThrough removes all entries with Tick ≤ tick (confirmed by the server).
func (h *PredictionHistory) DiscardThrough(tick uint64) {
	out := h.ring[:0]
	for _, e := range h.ring {
		if e.Tick > tick {
			out = append(out, e)
		}
	}
	h.ring = out
}

// DiscardBefore removes all entries with Tick < tick.
func (h *PredictionHistory) DiscardBefore(tick uint64) {
	out := h.ring[:0]
	for _, e := range h.ring {
		if e.Tick >= tick {
			out = append(out, e)
		}
	}
	h.ring = out
}

// Len returns the number of entries currently in the history.
func (h *PredictionHistory) Len() int { return len(h.ring) }
