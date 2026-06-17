package net

import (
	"fmt"

	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/scene"
)

// RollbackCoordinator is the reference rollback-resimulate implementation
// for client-prediction and lockstep netcode. When late remote input arrives
// for a past tick that was already simulated, it restores the snapshot at
// that tick, re-applies the corrected input, and re-runs the
// DeterministicSchedule forward to the current tick, snapshotting after each.
//
// Replace via world.SetResource[*RollbackCoordinator] to inject a custom
// netcode coordinator (the engine imposes no single netcode model).
type RollbackCoordinator struct {
	snaps   *SnapshotManager
	sched   *DeterministicSchedule
	current uint64 // last simulated tick
}

// NewRollbackCoordinator creates a coordinator backed by snaps and sched.
func NewRollbackCoordinator(snaps *SnapshotManager, sched *DeterministicSchedule) *RollbackCoordinator {
	return &RollbackCoordinator{snaps: snaps, sched: sched}
}

// SetTick records the coordinator's current simulation tick. Call this after
// each simulated tick so rollback knows how far to resimulate.
func (r *RollbackCoordinator) SetTick(tick uint64) { r.current = tick }

// Current returns the last recorded simulation tick.
func (r *RollbackCoordinator) Current() uint64 { return r.current }

// AdvanceTick runs one deterministic tick against w and records its snapshot.
// Call this each fixed-timestep tick instead of DeterministicSchedule.RunTick
// directly so the coordinator can maintain its snapshot ring.
func (r *RollbackCoordinator) AdvanceTick(w *world.World, reader scene.WorldReader, tick uint64) {
	r.sched.RunTick(w, tick)
	r.snaps.TakeSnapshot(reader, tick)
	r.current = tick
}

// Rollback restores the World to the snapshot at fromTick and re-runs the
// DeterministicSchedule for each tick in (fromTick, toTick], taking a snapshot
// after each. Returns an error if fromTick has been evicted from the snapshot ring.
func (r *RollbackCoordinator) Rollback(w *world.World, reader scene.WorldReader, fromTick, toTick uint64) error {
	h, ok := r.snaps.Get(fromTick)
	if !ok {
		return fmt.Errorf("rollback: snapshot for tick %d not in ring (ring capacity %d)", fromTick, r.snaps.cap)
	}
	if _, err := r.snaps.RestoreSnapshot(w, h); err != nil {
		return fmt.Errorf("rollback: restore tick %d: %w", fromTick, err)
	}
	for tick := fromTick + 1; tick <= toTick; tick++ {
		r.sched.RunTick(w, tick)
		r.snaps.TakeSnapshot(reader, tick)
	}
	return nil
}

// ReceiveRemoteInput records a remote player's input for the given tick and
// initiates a rollback if the tick is earlier than the last simulated tick.
// Returns whether a rollback was performed.
func (r *RollbackCoordinator) ReceiveRemoteInput(
	w *world.World,
	reader scene.WorldReader,
	buf *InputBuffer,
	tick uint64,
	player PlayerID,
	data []byte,
) bool {
	buf.RecordInput(tick, player, data)
	if tick >= r.current || r.current == 0 {
		return false // future or current tick; no rollback needed
	}
	_ = r.Rollback(w, reader, tick, r.current)
	return true
}
