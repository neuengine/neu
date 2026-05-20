// Package asset provides asynchronous asset management: ref-counted handles,
// typed stores, a virtual file system, and a dev-mode hot-reload watcher.
package asset

import "sync/atomic"

// LoadState is the lifecycle stage of an asset slot.
type LoadState uint8

const (
	NotLoaded LoadState = iota
	Loading
	Loaded
	Failed
)

// AssetID uniquely identifies an asset slot. Comparable — safe as a map key.
// kind=0: generational (idx, gen) index; kind=1: content UUID.
type AssetID struct {
	kind uint8
	idx  uint32
	gen  uint32
	uuid [16]byte
}

// IsValid reports whether id is non-zero (zero value is always invalid).
func (id AssetID) IsValid() bool { return id != (AssetID{}) }

// rc is the reference-count cell shared by all strong Handles for one slot.
// onZero fires exactly once when the count reaches zero.
type rc struct {
	onZero func()
	n      atomic.Int64
}

// Handle[A] is a typed reference-counted pointer to an asset slot.
// A nil r field marks a weak (non-owning) reference.
//
// Handles are value types — clone with Clone(), not assignment.
type Handle[A any] struct {
	r  *rc
	id AssetID
}

// ID returns the slot identifier.
func (h Handle[A]) ID() AssetID { return h.id }

// IsWeak reports whether this is a non-owning (weak) handle.
func (h Handle[A]) IsWeak() bool { return h.r == nil }

// Clone returns a new strong Handle that shares the same slot. Panics on a
// weak handle — use Assets.Upgrade instead.
func (h Handle[A]) Clone() Handle[A] {
	if h.r == nil {
		panic("asset: Clone on a weak handle — use Assets.Upgrade")
	}
	h.r.n.Add(1)
	return Handle[A]{id: h.id, r: h.r}
}

// Downgrade returns a weak handle with no ownership. The underlying slot may
// be freed while a weak handle exists.
func (h Handle[A]) Downgrade() Handle[A] {
	return Handle[A]{id: h.id}
}

// Drop decrements the reference count. When it reaches zero the slot is
// removed from the owning store via the onZero hook registered at creation.
// Calling Drop on a weak handle or more than once is a no-op.
func (h *Handle[A]) Drop() {
	if h.r == nil {
		return
	}
	if h.r.n.Add(-1) == 0 && h.r.onZero != nil {
		h.r.onZero()
	}
	h.r = nil
}

// newRC creates a reference-count cell with count=1 and an optional cleanup
// hook that fires when the count reaches zero.
func newRC(onZero func()) *rc {
	r := &rc{onZero: onZero}
	r.n.Store(1)
	return r
}

// upgradeRC atomically increments n from a positive value. Returns false if n
// is already 0 (slot concurrently freed). Used by Assets.Upgrade.
func upgradeRC(r *rc) bool {
	for {
		n := r.n.Load()
		if n <= 0 {
			return false
		}
		if r.n.CompareAndSwap(n, n+1) {
			return true
		}
	}
}
