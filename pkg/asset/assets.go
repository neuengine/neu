package asset

import (
	"iter"
	"sync"
)

// assetSlot holds one entry in the typed store.
type assetSlot[A any] struct {
	val   A
	state LoadState
	err   error
	r     *rc // shared refcount; nil for programmatically-added entries
}

// Assets[A] is a typed ECS resource that stores and retrieves assets of type A
// by AssetID. It tracks slot lifetimes via the refcount carried in Handle[A].
// Not safe for concurrent read+write — the schedule executor serialises access.
type Assets[A any] struct {
	mu    sync.RWMutex
	slots map[AssetID]*assetSlot[A]
	gens  map[uint32]uint32 // idx → current generation for generational invalidation
	free  []uint32          // reusable slot indices
	next  uint32            // monotonically allocated when free list is empty
}

// NewAssets returns an empty Assets store.
// idx=0 is reserved as the invalid sentinel so the zero AssetID is never
// returned by Add or addLoading.
func NewAssets[A any]() *Assets[A] {
	return &Assets[A]{
		slots: make(map[AssetID]*assetSlot[A]),
		gens:  make(map[uint32]uint32),
		next:  1, // reserve 0
	}
}

// allocID pops a free index (or increments next) and returns a fresh AssetID
// with the current generation for that index.
func (a *Assets[A]) allocID() AssetID {
	var idx uint32
	if len(a.free) > 0 {
		idx = a.free[len(a.free)-1]
		a.free = a.free[:len(a.free)-1]
	} else {
		idx = a.next
		a.next++
	}
	gen := a.gens[idx]
	return AssetID{kind: 0, idx: idx, gen: gen}
}

// freeID returns idx to the free list and bumps its generation so stale
// handles with the old generation no longer resolve.
func (a *Assets[A]) freeID(id AssetID) {
	if id.kind != 0 {
		return
	}
	a.gens[id.idx]++
	a.free = append(a.free, id.idx)
}

// addSlot inserts slot under id and registers the slot's onZero hook so Drop
// cleans up without the caller passing *Assets back. Called with mu held.
func (a *Assets[A]) addSlot(id AssetID, s *assetSlot[A]) {
	if s.r != nil {
		s.r.onZero = func() {
			a.mu.Lock()
			delete(a.slots, id)
			a.freeID(id)
			a.mu.Unlock()
		}
	}
	a.slots[id] = s
}

// Add inserts asset as a strong slot and returns a Handle with rc=1.
// Programmatic addition — the slot lifetime is managed by the returned Handle.
func (a *Assets[A]) Add(asset A) Handle[A] {
	a.mu.Lock()
	id := a.allocID()
	r := newRC(nil) // onZero wired by addSlot
	s := &assetSlot[A]{val: asset, state: Loaded, r: r}
	a.addSlot(id, s)
	a.mu.Unlock()
	return Handle[A]{id: id, r: r}
}

// addLoading inserts a slot in Loading state and returns a Handle. Used by
// AssetServer.Load to register a slot before decoding starts.
func (a *Assets[A]) addLoading() (Handle[A], *assetSlot[A]) {
	a.mu.Lock()
	id := a.allocID()
	r := newRC(nil)
	s := &assetSlot[A]{state: Loading, r: r}
	a.addSlot(id, s)
	a.mu.Unlock()
	return Handle[A]{id: id, r: r}, s
}

// Get returns a mutable pointer to the asset value and true if the slot is
// Loaded and exists. Returns (nil, false) for missing, Loading, or Failed slots.
// Both state and the pointer are read under the shared lock to prevent data
// races with concurrent IOPool writes.
func (a *Assets[A]) Get(id AssetID) (*A, bool) {
	a.mu.RLock()
	s, ok := a.slots[id]
	if !ok || s.state != Loaded {
		a.mu.RUnlock()
		return nil, false
	}
	v := &s.val
	a.mu.RUnlock()
	return v, true
}

// GetLoadState returns the LoadState for id, or NotLoaded if absent.
// State is read under the shared lock to prevent races with IOPool writers.
func (a *Assets[A]) GetLoadState(id AssetID) LoadState {
	a.mu.RLock()
	s, ok := a.slots[id]
	if !ok {
		a.mu.RUnlock()
		return NotLoaded
	}
	state := s.state
	a.mu.RUnlock()
	return state
}

// Remove forcibly removes the slot and returns the value. Returns (zero, false)
// if the slot does not exist. Does NOT decrement the handle refcount — callers
// must also Drop their Handle to avoid dangling onZero references.
func (a *Assets[A]) Remove(id AssetID) (A, bool) {
	a.mu.Lock()
	s, ok := a.slots[id]
	if ok {
		delete(a.slots, id)
		a.freeID(id)
	}
	a.mu.Unlock()
	if !ok {
		var zero A
		return zero, false
	}
	return s.val, true
}

// Upgrade attempts to promote a weak handle to a strong one. Returns the
// strong handle and true if the slot still exists; false if already freed.
func (a *Assets[A]) Upgrade(h Handle[A]) (Handle[A], bool) {
	a.mu.RLock()
	s, ok := a.slots[h.id]
	a.mu.RUnlock()
	if !ok || s.r == nil {
		return Handle[A]{}, false
	}
	if !upgradeRC(s.r) {
		return Handle[A]{}, false
	}
	return Handle[A]{id: h.id, r: s.r}, true
}

// Iter returns an iterator over all Loaded slots in an unspecified order.
func (a *Assets[A]) Iter() iter.Seq2[AssetID, *A] {
	return func(yield func(AssetID, *A) bool) {
		a.mu.RLock()
		defer a.mu.RUnlock()
		for id, s := range a.slots {
			if s.state == Loaded {
				if !yield(id, &s.val) {
					return
				}
			}
		}
	}
}

// Len returns the total number of slots regardless of LoadState.
func (a *Assets[A]) Len() int {
	a.mu.RLock()
	n := len(a.slots)
	a.mu.RUnlock()
	return n
}
