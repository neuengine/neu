package changedetect

import (
	"slices"
	"testing"

	"github.com/neuengine/neu/internal/ecs/entity"
)

// makeEntity is a test helper that builds a synthetic entity value.
func makeEntity(index uint32) entity.Entity {
	return entity.NewEntity(index, 1)
}

func TestRemovedComponentsAppendAndIter(t *testing.T) {
	t.Parallel()

	var rc RemovedComponents[int]

	e1 := makeEntity(1)
	e2 := makeEntity(2)
	rc.Append(e1, 5)
	rc.Append(e2, 8)

	if rc.Len() != 2 {
		t.Fatalf("Len = %d, want 2", rc.Len())
	}

	var got []entity.Entity
	for e := range rc.Iter(4) { // lastSystemTick = 4; both 5 and 8 are newer
		got = append(got, e)
	}
	if len(got) != 2 {
		t.Fatalf("Iter(4) yielded %d entries, want 2", len(got))
	}
}

func TestRemovedComponentsIterFiltersOldEntries(t *testing.T) {
	t.Parallel()

	var rc RemovedComponents[int]
	e1 := makeEntity(1)
	e2 := makeEntity(2)
	rc.Append(e1, 3) // older than lastSystemTick 5
	rc.Append(e2, 7) // newer than 5

	var got []entity.Entity
	for e := range rc.Iter(5) {
		got = append(got, e)
	}
	if len(got) != 1 || got[0] != e2 {
		t.Fatalf("Iter(5) = %v, want [e2]", got)
	}
}

func TestRemovedComponentsDrainBeforeTwoFrameWindow(t *testing.T) {
	t.Parallel()

	// Entry at removalTick=5 is visible at lastTick=5 and 6 (cutoff=3,4),
	// and pruned at lastTick=7 (cutoff=5; 5 > 5 is false).
	var rc RemovedComponents[int]
	e := makeEntity(1)
	rc.Append(e, 5)

	// lastTick=5: cutoff = 3; 5 > 3 → keep (frame of removal)
	rc.DrainBefore(5)
	if rc.Len() != 1 {
		t.Fatalf("after DrainBefore(5): Len = %d, want 1 (frame of removal)", rc.Len())
	}

	// lastTick=6: cutoff = 4; 5 > 4 → keep (one frame after removal)
	rc.DrainBefore(6)
	if rc.Len() != 1 {
		t.Fatalf("after DrainBefore(6): Len = %d, want 1 (still in window)", rc.Len())
	}

	// lastTick=7: cutoff = 5; 5 > 5 is false → prune (two frames after removal)
	rc.DrainBefore(7)
	if rc.Len() != 0 {
		t.Fatalf("after DrainBefore(7): Len = %d, want 0 (expired)", rc.Len())
	}
}

func TestRemovedComponentsDrainBeforeUnderflowGuard(t *testing.T) {
	t.Parallel()

	// With lastTick < 2, DrainBefore must not prune any entry (underflow guard).
	var rc RemovedComponents[int]
	rc.Append(makeEntity(1), 0)
	rc.Append(makeEntity(2), 1)

	rc.DrainBefore(0)
	if rc.Len() != 2 {
		t.Fatalf("DrainBefore(0): Len = %d, want 2 (underflow guard)", rc.Len())
	}
	rc.DrainBefore(1)
	if rc.Len() != 2 {
		t.Fatalf("DrainBefore(1): Len = %d, want 2 (underflow guard)", rc.Len())
	}
}

func TestRemovedComponentsDrainPreservesOrder(t *testing.T) {
	t.Parallel()

	// Entries newer than cutoff must remain in their original insertion order.
	var rc RemovedComponents[int]
	e1, e2, e3 := makeEntity(1), makeEntity(2), makeEntity(3)
	rc.Append(e1, 2) // pruned at lastTick=5 (cutoff=3)
	rc.Append(e2, 4) // kept
	rc.Append(e3, 6) // kept

	rc.DrainBefore(5) // cutoff = 3; prune 2, keep 4 and 6

	var got []entity.Entity
	for e := range rc.Iter(0) {
		got = append(got, e)
	}
	want := []entity.Entity{e2, e3}
	if !slices.Equal(got, want) {
		t.Fatalf("after drain: Iter = %v, want %v", got, want)
	}
}

func TestRemovedComponentsIterEarlyStop(t *testing.T) {
	t.Parallel()

	var rc RemovedComponents[int]
	for i := range 5 {
		rc.Append(makeEntity(uint32(i)), Tick(i+1))
	}

	count := 0
	for range rc.Iter(0) {
		count++
		if count == 2 {
			break
		}
	}
	if count != 2 {
		t.Fatalf("early stop yielded %d, want 2", count)
	}
}

func TestRemovedRegistryDrainAll(t *testing.T) {
	t.Parallel()

	var reg RemovedRegistry
	var rc1, rc2 RemovedComponents[int]

	rc1.Append(makeEntity(1), 3)
	rc2.Append(makeEntity(2), 3)

	// Register both drain callbacks.
	reg.Register(rc1.DrainBefore)
	reg.Register(rc2.DrainBefore)

	// lastTick=6: cutoff=4; entry at 3 ≤ 4 is pruned.
	reg.DrainAll(6)

	if rc1.Len() != 0 {
		t.Fatalf("rc1 after DrainAll(6): Len = %d, want 0", rc1.Len())
	}
	if rc2.Len() != 0 {
		t.Fatalf("rc2 after DrainAll(6): Len = %d, want 0", rc2.Len())
	}
}

func TestRemovedComponentsMultipleDrainCycles(t *testing.T) {
	t.Parallel()

	// Simulate three ClearTrackers calls to verify two-frame window semantics:
	//   frame 1: removal at tick 10, ClearTrackers sets lastTick=10 → entry kept
	//   frame 2: ClearTrackers sets lastTick=11 → cutoff=9; 10 > 9 → kept
	//   frame 3: ClearTrackers sets lastTick=12 → cutoff=10; 10 > 10 is false → pruned

	var rc RemovedComponents[int]
	e := makeEntity(42)
	rc.Append(e, 10)

	rc.DrainBefore(10) // frame 1
	if rc.Len() != 1 {
		t.Fatalf("frame1 after DrainBefore(10): Len = %d, want 1", rc.Len())
	}

	rc.DrainBefore(11) // frame 2
	if rc.Len() != 1 {
		t.Fatalf("frame2 after DrainBefore(11): Len = %d, want 1", rc.Len())
	}

	rc.DrainBefore(12) // frame 3
	if rc.Len() != 0 {
		t.Fatalf("frame3 after DrainBefore(12): Len = %d, want 0 (expired)", rc.Len())
	}
}
