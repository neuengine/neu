package net

import (
	"testing"
	"time"

	"github.com/neuengine/neu/internal/ecs/typereg"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/scene"
)

func TestRollbackCoordinatorAdvanceTick(t *testing.T) {
	t.Parallel()
	w, reg, _ := netWorld(t, 2)
	adapter := scene.NewWorldAdapter(w)
	snaps := NewSnapshotManager(reg, 16)
	sched := NewDeterministicSchedule(0, time.Millisecond)
	rc := NewRollbackCoordinator(snaps, sched)

	if rc.Current() != 0 {
		t.Errorf("Current() before any tick = %d, want 0", rc.Current())
	}
	rc.AdvanceTick(w, adapter, 1)
	if rc.Current() != 1 {
		t.Errorf("Current() after tick 1 = %d, want 1", rc.Current())
	}
	if snaps.Len() != 1 {
		t.Errorf("snaps.Len() = %d, want 1 after one tick", snaps.Len())
	}
	if _, ok := snaps.Get(1); !ok {
		t.Error("snapshot for tick 1 not stored")
	}
}

func TestRollbackCoordinatorRollbackResimulates(t *testing.T) {
	t.Parallel()
	w, reg, _ := netWorld(t, 2)
	adapter := scene.NewWorldAdapter(w)
	snaps := NewSnapshotManager(reg, 16)
	sched := NewDeterministicSchedule(0, time.Millisecond)
	rc := NewRollbackCoordinator(snaps, sched)

	for tick := uint64(1); tick <= 4; tick++ {
		rc.AdvanceTick(w, adapter, tick)
	}
	if err := rc.Rollback(w, adapter, 2, 4); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	// Snapshots for ticks 1-4 must still exist (resimulate re-snapshots them).
	for _, tick := range []uint64{1, 2, 3, 4} {
		if _, ok := snaps.Get(tick); !ok {
			t.Errorf("snapshot for tick %d missing after rollback", tick)
		}
	}
}

func TestRollbackCoordinatorRollbackMissingSnapshot(t *testing.T) {
	t.Parallel()
	w, reg, _ := netWorld(t, 1)
	adapter := scene.NewWorldAdapter(w)
	snaps := NewSnapshotManager(reg, 16)
	sched := NewDeterministicSchedule(0, time.Millisecond)
	rc := NewRollbackCoordinator(snaps, sched)

	rc.AdvanceTick(w, adapter, 5)
	if err := rc.Rollback(w, adapter, 3, 5); err == nil {
		t.Error("Rollback for evicted tick should return error")
	}
}

func TestRollbackCoordinatorSetTick(t *testing.T) {
	t.Parallel()
	snaps := NewSnapshotManager(typereg.NewTypeRegistry(), 4)
	sched := NewDeterministicSchedule(0, time.Millisecond)
	rc := NewRollbackCoordinator(snaps, sched)
	rc.SetTick(42)
	if rc.Current() != 42 {
		t.Errorf("Current() = %d, want 42", rc.Current())
	}
}

func TestRollbackCoordinatorReceiveRemoteInput(t *testing.T) {
	t.Parallel()
	w, reg, _ := netWorld(t, 1)
	adapter := scene.NewWorldAdapter(w)
	snaps := NewSnapshotManager(reg, 16)
	sched := NewDeterministicSchedule(0, time.Millisecond)
	rc := NewRollbackCoordinator(snaps, sched)
	buf := NewInputBuffer()

	for tick := uint64(1); tick <= 5; tick++ {
		rc.AdvanceTick(w, adapter, tick)
	}

	// Past tick with a snapshot → rollback triggered.
	if !rc.ReceiveRemoteInput(w, adapter, buf, 3, 1, []byte("left")) {
		t.Error("ReceiveRemoteInput for past tick should trigger rollback")
	}
	in, ok := buf.GetInput(3, 1)
	if !ok || string(in.Data) != "left" {
		t.Errorf("input not recorded: %+v ok=%v", in, ok)
	}

	// Current tick → no rollback.
	if rc.ReceiveRemoteInput(w, adapter, buf, 5, 2, []byte("right")) {
		t.Error("ReceiveRemoteInput for current tick should not rollback")
	}

	// Future tick → no rollback.
	if rc.ReceiveRemoteInput(w, adapter, buf, 8, 2, []byte("up")) {
		t.Error("ReceiveRemoteInput for future tick should not rollback")
	}
}

func TestRollbackCoordinatorZeroCurrentNoRollback(t *testing.T) {
	t.Parallel()
	w := world.NewWorld()
	snaps := NewSnapshotManager(typereg.NewTypeRegistry(), 4)
	sched := NewDeterministicSchedule(0, time.Millisecond)
	rc := NewRollbackCoordinator(snaps, sched)
	buf := NewInputBuffer()

	// current == 0 → no rollback even for tick 0.
	if rc.ReceiveRemoteInput(w, nil, buf, 0, 1, nil) {
		t.Error("ReceiveRemoteInput with current==0 should not rollback")
	}
}
