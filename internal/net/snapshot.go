package net

import (
	"encoding/json"
	"hash/crc32"
	"sort"

	"github.com/neuengine/neu/internal/ecs/typereg"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/internal/worldsnap"
	"github.com/neuengine/neu/pkg/scene"
)

// SnapshotHandle is a captured World state for a single tick. Data is a
// self-contained, deterministically-ordered serialization (l1-networking-system
// INV-3); Checksum is its CRC32, exchanged between peers for desync detection
// (l1-lockstep INV-4).
type SnapshotHandle struct {
	Data        []byte
	Tick        uint64
	Checksum    uint32
	EntityCount uint32
}

// SnapshotManager captures and restores World snapshots for rollback and
// synchronization, keeping a bounded ring of recent snapshots. It wraps the
// shared worldsnap capture/restore core (the same path hot-reload uses, with
// entity-ID-stable restore) and adds a deterministic, cross-machine-stable
// encoding so two peers running identical simulations produce identical
// checksums. Not safe for concurrent use; driven from the simulation thread.
type SnapshotManager struct {
	reg  *typereg.TypeRegistry
	ring []SnapshotHandle
	cap  int
}

// NewSnapshotManager returns a manager with a ring of the given capacity
// (default 16 if capacity <= 0), resolving component types through reg.
func NewSnapshotManager(reg *typereg.TypeRegistry, capacity int) *SnapshotManager {
	if capacity <= 0 {
		capacity = 16
	}
	return &SnapshotManager{reg: reg, cap: capacity}
}

// TakeSnapshot captures the World (via reader) at tick into a SnapshotHandle and
// stores it in the ring (evicting the oldest when full). The serialization is
// deterministic — entities sorted by ID, components by type name — so the
// CRC32 checksum is identical across machines for identical state.
func (m *SnapshotManager) TakeSnapshot(reader scene.WorldReader, tick uint64) SnapshotHandle {
	ents, _, _ := worldsnap.Capture(reader, m.reg)
	data := encodeDeterministic(ents)
	h := SnapshotHandle{
		Tick:        tick,
		Data:        data,
		Checksum:    crc32.ChecksumIEEE(data),
		EntityCount: uint32(len(ents)),
	}
	m.store(h)
	return h
}

// TakeChecksum returns just the CRC32 of the World at tick without storing a
// snapshot — the cheap path lockstep uses for periodic desync checks (INV-4).
func (m *SnapshotManager) TakeChecksum(reader scene.WorldReader) uint32 {
	ents, _, _ := worldsnap.Capture(reader, m.reg)
	return crc32.ChecksumIEEE(encodeDeterministic(ents))
}

// RestoreSnapshot rebuilds the World w from a handle, pinning entity IDs exactly
// (via worldsnap, the hot-reload mechanism). w must be a fresh World with the
// component types registered. A corrupt Data blob is reported via the returned
// error, before any mutation.
func (m *SnapshotManager) RestoreSnapshot(w *world.World, h SnapshotHandle) (worldsnap.RestoreResult, error) {
	var ents []worldsnap.EntitySnapshot
	if err := json.Unmarshal(h.Data, &ents); err != nil {
		return worldsnap.RestoreResult{}, err
	}
	return worldsnap.Restore(w, ents, m.reg), nil
}

// Get returns the stored snapshot for a tick and whether it is still in the ring.
func (m *SnapshotManager) Get(tick uint64) (SnapshotHandle, bool) {
	for i := range m.ring {
		if m.ring[i].Tick == tick {
			return m.ring[i], true
		}
	}
	return SnapshotHandle{}, false
}

// Len reports how many snapshots are currently buffered.
func (m *SnapshotManager) Len() int { return len(m.ring) }

// store appends a snapshot, evicting the oldest when the ring is full.
func (m *SnapshotManager) store(h SnapshotHandle) {
	if len(m.ring) >= m.cap {
		copy(m.ring, m.ring[1:])
		m.ring[len(m.ring)-1] = h
		return
	}
	m.ring = append(m.ring, h)
}

// encodeDeterministic serializes entity snapshots in a canonical order
// (entities by ID, components by type name) so identical World state always
// produces identical bytes — required for cross-machine checksum comparison.
func encodeDeterministic(ents []worldsnap.EntitySnapshot) []byte {
	sorted := make([]worldsnap.EntitySnapshot, len(ents))
	copy(sorted, ents)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	for i := range sorted {
		comps := append([]worldsnap.ComponentSnapshot(nil), sorted[i].Components...)
		sort.Slice(comps, func(a, b int) bool { return comps[a].TypeName < comps[b].TypeName })
		sorted[i].Components = comps
	}
	data, _ := json.Marshal(sorted)
	return data
}
