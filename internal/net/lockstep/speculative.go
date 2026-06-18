package lockstep

import (
	"bytes"

	"github.com/neuengine/neu/internal/ecs/world"
	netcore "github.com/neuengine/neu/internal/net"
	"github.com/neuengine/neu/pkg/scene"
)

// SpeculativeConfig controls opt-in speculative execution for LockstepScheduler.
// When enabled, missing peer inputs are predicted so the scheduler does not
// stall. On real input arrival, if the prediction was wrong, a rollback is
// triggered reusing the same RollbackCoordinator path as client-prediction.
type SpeculativeConfig struct {
	// Enabled activates speculative execution. Off by default.
	Enabled bool
	// MaxSpeculative caps how many ticks may run ahead on predicted input.
	// Zero defaults to defaultMaxSpeculative (4).
	MaxSpeculative uint8
}

const defaultMaxSpeculative uint8 = 4

// speculativeKey identifies a pending prediction: the future input-tick for
// which a peer's real input has not yet arrived.
type speculativeKey struct {
	inputTick uint64
	player    netcore.PlayerID
}

// speculativeEntry records what was predicted and which simulation tick
// consumed that prediction (the rollback target when a mismatch is found).
type speculativeEntry struct {
	simTick   uint64 // simulation tick executed with this predicted input
	predicted []byte // predicted bytes compared against real input on arrival
}

// SpeculativeExecutor runs in FixedPreUpdate (before LockstepScheduler.Step).
// When all-peers-ready would fail for the target tick, it predicts missing peer
// inputs via InputBuffer.PredictInput and records them, allowing the scheduler
// to advance. On subsequent frames it compares real inputs (written by
// RemoteInputSystem) against predictions; a mismatch triggers a rollback.
type SpeculativeExecutor struct {
	ls        *LockstepScheduler
	config    SpeculativeConfig
	pending   map[speculativeKey]speculativeEntry
	specCount uint8 // outstanding speculative ticks
}

// NewSpeculativeExecutor creates an executor backed by ls with the given config.
func NewSpeculativeExecutor(ls *LockstepScheduler, cfg SpeculativeConfig) *SpeculativeExecutor {
	if cfg.MaxSpeculative == 0 {
		cfg.MaxSpeculative = defaultMaxSpeculative
	}
	return &SpeculativeExecutor{
		ls:      ls,
		config:  cfg,
		pending: make(map[speculativeKey]speculativeEntry),
	}
}

// Run is the ECS system entry point; called once per FixedPreUpdate.
func (s *SpeculativeExecutor) Run(w *world.World) {
	if !s.config.Enabled || s.ls.Halted() {
		return
	}

	bufP, ok := world.Resource[*netcore.InputBuffer](w)
	if !ok {
		return
	}
	buf := *bufP

	rcP, ok := world.Resource[*netcore.RollbackCoordinator](w)
	if !ok {
		return
	}
	rc := *rcP

	// Phase 1: check for mispredicted inputs that arrived since last frame.
	if s.rollbackIfMispredicted(w, buf, rc) {
		return // after rollback, don't predict more this frame
	}

	// Phase 2: predict missing peer inputs for the upcoming target tick.
	if s.specCount >= s.config.MaxSpeculative {
		return
	}
	target := s.ls.currentTick + uint64(s.ls.cfg.InputDelay)
	for _, player := range s.ls.cfg.Peers {
		if _, has := buf.GetInput(target, player); has {
			continue
		}
		pred := buf.PredictInput(target, player)
		buf.RecordInput(target, player, pred.Data)
		key := speculativeKey{inputTick: target, player: player}
		if _, already := s.pending[key]; !already {
			s.pending[key] = speculativeEntry{
				simTick:   s.ls.currentTick + 1,
				predicted: append([]byte(nil), pred.Data...),
			}
			s.specCount++
		}
	}
}

// rollbackIfMispredicted scans pending entries for real inputs that differ
// from predictions. On the first mismatch it rolls back to the snapshot before
// the speculative tick, clears all pending state, and returns true.
func (s *SpeculativeExecutor) rollbackIfMispredicted(w *world.World, buf *netcore.InputBuffer, rc *netcore.RollbackCoordinator) bool {
	for key, entry := range s.pending {
		real, ok := buf.GetInput(key.inputTick, key.player)
		if !ok {
			continue // real input not yet arrived; wait
		}
		delete(s.pending, key)
		if s.specCount > 0 {
			s.specCount--
		}
		if bytes.Equal(real.Data, entry.predicted) {
			continue // correct prediction; no rollback needed
		}
		// Misprediction: restore snapshot before the speculative sim tick.
		reader := scene.NewWorldAdapter(w)
		if entry.simTick > 0 {
			_ = rc.Rollback(w, reader, entry.simTick-1, rc.Current())
		}
		// All remaining pending entries were computed from the wrong state.
		s.pending = make(map[speculativeKey]speculativeEntry)
		s.specCount = 0
		return true
	}
	return false
}
