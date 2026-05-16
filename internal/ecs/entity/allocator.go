package entity

import "sync"

// EntityAllocator manages entity ID allocation and recycling using a
// generational freelist arena. Thread-safe via an internal sync.RWMutex —
// concurrent reservation from multiple systems is supported (T-1F02).
//
// Reads (IsAlive, Len, Cap) take the read lock; mutations (Allocate,
// AllocateMany, Free, Reserve) take the write lock.
type EntityAllocator struct {
	freeList    []uint32
	generations []uint32
	mu          sync.RWMutex
	alive       uint32
}

// NewEntityAllocator creates an allocator with pre-allocated capacity for
// generations and freelist slices. Capacity is a hint; slots are not yet
// allocated.
func NewEntityAllocator(capacity int) *EntityAllocator {
	if capacity < 0 {
		capacity = 0
	}
	return &EntityAllocator{
		generations: make([]uint32, 0, capacity),
		freeList:    make([]uint32, 0, capacity),
	}
}

// Allocate reserves a new Entity. Pops from the freelist or extends the arena.
// The returned Entity carries the current generation for its slot. The first
// allocation for any new slot uses generation 1, so the null sentinel
// (Entity{}, index 0, generation 0) is never produced by the allocator.
//
// Safe for concurrent use; callers from multiple goroutines (e.g. parallel
// command-buffer reservation) serialise on the allocator's write lock.
func (a *EntityAllocator) Allocate() Entity {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.allocateLocked()
}

func (a *EntityAllocator) allocateLocked() Entity {
	var index uint32
	if n := len(a.freeList); n > 0 {
		index = a.freeList[n-1]
		a.freeList = a.freeList[:n-1]
	} else {
		index = uint32(len(a.generations))
		a.generations = append(a.generations, 1)
	}
	a.alive++
	return NewEntity(index, a.generations[index])
}

// AllocateMany allocates n entities in a single batch. More efficient than
// repeated Allocate calls because capacity is grown once. Returns nil for
// n <= 0. Safe for concurrent use.
func (a *EntityAllocator) AllocateMany(n int) []Entity {
	if n <= 0 {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	out := make([]Entity, 0, n)

	reuse := min(len(a.freeList), n)
	for range reuse {
		idx := a.freeList[len(a.freeList)-1]
		a.freeList = a.freeList[:len(a.freeList)-1]
		out = append(out, NewEntity(idx, a.generations[idx]))
	}

	remaining := n - reuse
	if remaining > 0 {
		base := uint32(len(a.generations))
		needed := len(a.generations) + remaining
		if cap(a.generations) < needed {
			grown := make([]uint32, len(a.generations), needed)
			copy(grown, a.generations)
			a.generations = grown
		}
		for i := range remaining {
			a.generations = append(a.generations, 1)
			out = append(out, NewEntity(base+uint32(i), 1))
		}
	}

	a.alive += uint32(n)
	return out
}

// Free releases an Entity. Increments the slot's generation and pushes the
// index onto the freelist. A no-op for the null entity, out-of-range indices,
// or already-dead entities (stale generation). Safe for concurrent use.
func (a *EntityAllocator) Free(entity Entity) {
	if !entity.IsValid() {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	idx := entity.Index()
	if int(idx) >= len(a.generations) {
		return
	}
	if a.generations[idx] != entity.Generation() {
		return
	}
	a.generations[idx]++
	a.freeList = append(a.freeList, idx)
	a.alive--
}

// IsAlive reports whether the given Entity matches the current generation for
// its slot. Returns false for the null entity and for out-of-range indices.
// Safe for concurrent use; takes a read lock.
func (a *EntityAllocator) IsAlive(entity Entity) bool {
	if !entity.IsValid() {
		return false
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	idx := entity.Index()
	if int(idx) >= len(a.generations) {
		return false
	}
	return a.generations[idx] == entity.Generation()
}

// Len returns the number of currently alive entities. Safe for concurrent use.
func (a *EntityAllocator) Len() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return int(a.alive)
}

// Cap returns the number of slots currently tracked by the allocator
// (alive + freed). Useful for diagnostics and capacity tuning.
// Safe for concurrent use.
func (a *EntityAllocator) Cap() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.generations)
}

// Reserve grows internal capacity to hold at least n additional slots beyond
// the current arena size. It does not allocate entities; it only avoids
// runtime growth on subsequent Allocate calls.
func (a *EntityAllocator) Reserve(n int) {
	if n <= 0 {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	need := len(a.generations) + n
	if cap(a.generations) < need {
		grown := make([]uint32, len(a.generations), need)
		copy(grown, a.generations)
		a.generations = grown
	}
	if cap(a.freeList) < n {
		grown := make([]uint32, len(a.freeList), n)
		copy(grown, a.freeList)
		a.freeList = grown
	}
}
