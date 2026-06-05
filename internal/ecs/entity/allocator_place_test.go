package entity

import "testing"

func TestPlaceAtPinsExactID(t *testing.T) {
	t.Parallel()
	a := NewEntityAllocator(0)
	a.PlaceAt(5, 3)
	if got := a.Cap(); got != 6 {
		t.Errorf("Cap after PlaceAt(5,..) = %d, want 6 (arena grown to cover index)", got)
	}
	if !a.IsAlive(NewEntity(5, 3)) {
		t.Error("placed entity (5,3) should be alive")
	}
	if a.IsAlive(NewEntity(5, 2)) {
		t.Error("(5,2) must not be alive — wrong generation")
	}
	if a.Len() != 1 {
		t.Errorf("Len = %d, want 1", a.Len())
	}
}

func TestPlaceAtGenerationZeroRejected(t *testing.T) {
	t.Parallel()
	a := NewEntityAllocator(0)
	a.PlaceAt(3, 0) // null generation rejected
	if a.Cap() != 0 || a.Len() != 0 {
		t.Errorf("PlaceAt(_,0) should be a no-op: cap=%d len=%d", a.Cap(), a.Len())
	}
}

func TestPlaceAtIdempotentReplace(t *testing.T) {
	t.Parallel()
	a := NewEntityAllocator(0)
	a.PlaceAt(2, 7)
	a.PlaceAt(2, 9) // re-place same slot, new generation
	if a.Len() != 1 {
		t.Errorf("re-placing the same index double-counted alive: Len=%d, want 1", a.Len())
	}
	if !a.IsAlive(NewEntity(2, 9)) || a.IsAlive(NewEntity(2, 7)) {
		t.Error("re-place should make (2,9) alive and (2,7) stale")
	}
}

func TestRebuildFreeListReclaimsGaps(t *testing.T) {
	t.Parallel()
	a := NewEntityAllocator(0)
	// Place entities at indices 5 and 2, leaving 0,1,3,4 as gaps.
	a.PlaceAt(5, 3)
	a.PlaceAt(2, 7)
	a.RebuildFreeList()

	// The four gaps are reclaimed and handed out lowest-first as valid,
	// generation-1 entities (never the null sentinel).
	seen := map[uint32]bool{}
	for range 4 {
		e := a.Allocate()
		if !e.IsValid() {
			t.Fatalf("Allocate returned the null sentinel from a reclaimed gap: %+v", e)
		}
		if e.Generation() != 1 {
			t.Errorf("reclaimed gap entity generation = %d, want 1", e.Generation())
		}
		seen[e.Index()] = true
	}
	for _, idx := range []uint32{0, 1, 3, 4} {
		if !seen[idx] {
			t.Errorf("gap index %d was not reclaimed", idx)
		}
	}
	// The placed entities are untouched and still alive.
	if !a.IsAlive(NewEntity(5, 3)) || !a.IsAlive(NewEntity(2, 7)) {
		t.Error("RebuildFreeList disturbed a placed entity")
	}
	// Next allocation extends the arena past the highest placed index.
	next := a.Allocate()
	if next.Index() != 6 {
		t.Errorf("post-gap Allocate index = %d, want 6 (arena extends past max)", next.Index())
	}
}
