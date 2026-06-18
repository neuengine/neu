package predict

import (
	"github.com/neuengine/neu/internal/ecs/world"
	netcore "github.com/neuengine/neu/internal/net"
	"github.com/neuengine/neu/pkg/scene"
)

// ServerState is a world resource populated by game or replication code each
// time an authoritative state update arrives from the server. Valid is cleared
// to false by ReconciliationSystem after consumption, so stale state from a
// prior frame is never re-processed.
//
// Set Valid=true and fill Tick+Checksum when a server confirmation arrives.
// Query world.Resource[ServerState](w) and mutate the pointer directly.
type ServerState struct {
	Tick     uint64
	Checksum uint32
	Valid    bool
}

// ReconciliationSystem runs in PreUpdate. It reads the latest ServerState,
// compares the server's per-tick CRC32 against the local PredictionHistory,
// and initiates a rollback+resimulate when they disagree (INV-1/3/4):
//
//   - Match: prediction was correct — discard confirmed history through Tick.
//   - Mismatch: roll back to server Tick, resimulate forward to current tick,
//     and refresh the prediction-history checksums from the new snapshots.
//
// Requires NetworkPlugin resources: *RollbackCoordinator, *SnapshotManager.
// Requires a ServerState resource (registered by PredictionPlugin).
type ReconciliationSystem struct {
	history *PredictionHistory
}

// NewReconciliationSystem creates a ReconciliationSystem backed by history.
func NewReconciliationSystem(history *PredictionHistory) *ReconciliationSystem {
	return &ReconciliationSystem{history: history}
}

// Run is the ECS system entry point; called once per PreUpdate frame.
func (s *ReconciliationSystem) Run(w *world.World) {
	ssP, ok := world.Resource[ServerState](w)
	if !ok || !ssP.Valid {
		return
	}
	ssP.Valid = false // consume immediately; copy fields for local use
	tick, serverChecksum := ssP.Tick, ssP.Checksum

	rcP, ok := world.Resource[*netcore.RollbackCoordinator](w)
	if !ok {
		return
	}
	rc := *rcP

	entry, histOK := s.history.Get(tick)
	if !histOK {
		// Confirmed tick already evicted from ring (we were too far behind).
		s.history.DiscardBefore(tick)
		return
	}

	if entry.Checksum == serverChecksum {
		// Prediction correct: discard everything up to and including this tick.
		s.history.DiscardThrough(tick)
		return
	}

	// Misprediction: restore server tick, resimulate forward (INV-4).
	snapsP, ok := world.Resource[*netcore.SnapshotManager](w)
	if !ok {
		return
	}
	snaps := *snapsP

	reader := scene.NewWorldAdapter(w)
	if err := rc.Rollback(w, reader, tick, rc.Current()); err != nil {
		return // snapshot evicted; cannot recover
	}

	// Overwrite stale checksums in prediction history with the freshly-computed
	// post-rollback values so subsequent reconciliations compare correct state.
	for t := tick + 1; t <= rc.Current(); t++ {
		if h, hok := snaps.Get(t); hok {
			s.history.RecordTick(t, h.Checksum)
		}
	}
	// Discard the mispredicted entry at tick and everything older; the
	// resimulated entries (tick+1..current) have already been refreshed above.
	s.history.DiscardThrough(tick)
}
