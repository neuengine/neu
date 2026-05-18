package asset

import (
	"sync"
	"testing"
)

// TestHandleDropFreesSlot verifies that dropping all strong handles removes
// the slot (freed handle → !ok on re-resolve).
func TestHandleDropFreesSlot(t *testing.T) {
	a := NewAssets[string]()
	h := a.Add("hello")
	id := h.ID()

	if _, ok := a.Get(id); !ok {
		t.Fatal("slot should exist before Drop")
	}

	h.Drop()

	if _, ok := a.Get(id); ok {
		t.Fatal("slot should be gone after Drop")
	}
}

func TestHandleCloneSharesSlot(t *testing.T) {
	a := NewAssets[int]()
	h1 := a.Add(42)
	h2 := h1.Clone()
	id := h1.ID()

	h1.Drop() // rc → 1
	if _, ok := a.Get(id); !ok {
		t.Fatal("slot must exist after first Drop (rc still 1)")
	}

	h2.Drop() // rc → 0
	if _, ok := a.Get(id); ok {
		t.Fatal("slot must be freed after second Drop")
	}
}

func TestHandleDowngradeUpgrade(t *testing.T) {
	a := NewAssets[float64]()
	h := a.Add(3.14)
	id := h.ID()

	weak := h.Downgrade()
	if !weak.IsWeak() {
		t.Fatal("Downgrade must produce a weak handle")
	}

	strong, ok := a.Upgrade(weak)
	if !ok {
		t.Fatal("Upgrade should succeed while slot exists")
	}

	h.Drop()      // rc still 1 (strong holds it)
	strong.Drop() // rc → 0 → slot freed

	_, ok = a.Upgrade(weak)
	if ok {
		t.Fatal("Upgrade must fail after slot is freed")
	}
	_ = id
}

func TestHandleGenerationalInvalidation(t *testing.T) {
	a := NewAssets[string]()
	h1 := a.Add("first")
	old := h1.ID()
	h1.Drop() // frees slot; bumps gen for idx 0

	h2 := a.Add("second") // reuses idx 0 with gen+1
	if h2.ID() == old {
		t.Fatal("reused idx must have new generation; IDs should differ")
	}
	if _, ok := a.Get(old); ok {
		t.Fatal("old generational ID must not resolve to new slot")
	}
	h2.Drop()
}

func TestAssetsRemove(t *testing.T) {
	a := NewAssets[int]()
	h := a.Add(7)
	id := h.ID()

	val, ok := a.Remove(id)
	if !ok || val != 7 {
		t.Fatalf("Remove: got (%v, %v), want (7, true)", val, ok)
	}
	if _, ok := a.Get(id); ok {
		t.Fatal("slot should be gone after Remove")
	}
}

func TestAssetsIter(t *testing.T) {
	a := NewAssets[int]()
	var handles []Handle[int]
	for i := range 5 {
		handles = append(handles, a.Add(i))
	}

	seen := make(map[int]bool)
	for _, v := range a.Iter() {
		seen[*v] = true
	}
	if len(seen) != 5 {
		t.Fatalf("Iter: saw %d items, want 5", len(seen))
	}
	for i := range 5 {
		if !seen[i] {
			t.Errorf("Iter: missing value %d", i)
		}
	}
	for i := range handles {
		handles[i].Drop()
	}
}

// TestHandleRaceDrop verifies concurrent Clone + Drop is race-free.
func TestHandleRaceDrop(t *testing.T) {
	a := NewAssets[int]()
	h := a.Add(1)
	id := h.ID()

	var wg sync.WaitGroup
	const goroutines = 64

	// Each goroutine clones the handle and immediately drops it.
	// The original h is dropped last.
	clones := make([]Handle[int], goroutines)
	for i := range clones {
		clones[i] = h.Clone()
	}
	for i := range clones {
		wg.Go(func() {
			clones[i].Drop()
		})
	}
	wg.Wait()
	h.Drop() // rc → 0

	if _, ok := a.Get(id); ok {
		t.Fatal("slot must be freed after all handles dropped")
	}
}

func TestHandleClonePanicsOnWeak(t *testing.T) {
	a := NewAssets[int]()
	h := a.Add(1)
	weak := h.Downgrade()
	h.Drop()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Clone on weak handle must panic")
		}
	}()
	_ = weak.Clone()
}

func TestHandleDropWeakNoOp(t *testing.T) {
	a := NewAssets[int]()
	h := a.Add(1)
	weak := h.Downgrade()
	weak.Drop() // must be a no-op; should not panic or decrement rc
	h.Drop()    // rc → 0: slot freed normally
}

func TestAssetsAddLoading(t *testing.T) {
	a := NewAssets[string]()
	h, s := a.addLoading()
	id := h.ID()

	if a.GetLoadState(id) != Loading {
		t.Fatalf("addLoading: state = %d, want Loading", a.GetLoadState(id))
	}
	// simulate loader completing
	a.mu.Lock()
	s.val = "ready"
	s.state = Loaded
	a.mu.Unlock()

	v, ok := a.Get(id)
	if !ok || *v != "ready" {
		t.Fatalf("Get after load: got (%v, %v), want (ready, true)", v, ok)
	}
	h.Drop()
}

func TestAssetsLen(t *testing.T) {
	a := NewAssets[int]()
	if a.Len() != 0 {
		t.Fatalf("Len: got %d, want 0", a.Len())
	}
	h := a.Add(1)
	if a.Len() != 1 {
		t.Fatalf("Len: got %d, want 1", a.Len())
	}
	h.Drop()
	if a.Len() != 0 {
		t.Fatalf("Len after Drop: got %d, want 0", a.Len())
	}
}

func TestAssetIDIsValid(t *testing.T) {
	var zero AssetID
	if zero.IsValid() {
		t.Fatal("zero AssetID must be invalid")
	}
	a := NewAssets[int]()
	h := a.Add(1)
	if !h.ID().IsValid() {
		t.Fatal("allocated AssetID must be valid")
	}
	h.Drop()
}

func TestFreeIDUUIDKindNoOp(t *testing.T) {
	a := NewAssets[int]()
	// freeID with kind=1 (UUID) must not touch free list or gens
	uid := AssetID{kind: 1, uuid: [16]byte{1}}
	a.mu.Lock()
	a.freeID(uid) // must silently return
	a.mu.Unlock()
	if len(a.free) != 0 {
		t.Fatal("freeID on UUID-form AssetID must not modify free list")
	}
}

func TestAssetsUpgradeFreedSlot(t *testing.T) {
	a := NewAssets[int]()
	h := a.Add(99)
	weak := h.Downgrade()
	h.Drop() // slot freed

	_, ok := a.Upgrade(weak)
	if ok {
		t.Fatal("Upgrade must fail after slot is freed")
	}
}

func TestAssetsUpgradeMissingSlot(t *testing.T) {
	a := NewAssets[int]()
	unknown := Handle[int]{id: AssetID{kind: 0, idx: 99, gen: 0}}
	_, ok := a.Upgrade(unknown)
	if ok {
		t.Fatal("Upgrade on unknown AssetID must return false")
	}
}

func TestAssetsRemoveMissing(t *testing.T) {
	a := NewAssets[int]()
	_, ok := a.Remove(AssetID{kind: 0, idx: 99, gen: 0})
	if ok {
		t.Fatal("Remove on missing slot must return false")
	}
}

func TestAssetsIterEarlyExit(t *testing.T) {
	a := NewAssets[int]()
	var handles [5]Handle[int]
	for i := range handles {
		handles[i] = a.Add(i)
	}
	count := 0
	for range a.Iter() {
		count++
		break // early exit
	}
	if count != 1 {
		t.Fatalf("early-exit Iter: visited %d, want 1", count)
	}
	for i := range handles {
		handles[i].Drop()
	}
}

func TestAssetsGetLoadState(t *testing.T) {
	a := NewAssets[string]()
	h := a.Add("x")
	id := h.ID()

	if s := a.GetLoadState(id); s != Loaded {
		t.Fatalf("state = %d, want Loaded", s)
	}
	h.Drop()
	if s := a.GetLoadState(id); s != NotLoaded {
		t.Fatalf("state after drop = %d, want NotLoaded", s)
	}
}
