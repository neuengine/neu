package query

import (
	"runtime"
	"sync"

	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/world"
)

// MinChunkSize is the smallest row count [Query1.ParIter] dispatches to a
// dedicated goroutine. Archetypes (or final partitions) below this size run
// inline to avoid goroutine-creation overhead dominating the work.
const MinChunkSize = 256

// ParIter dispatches the query across multiple goroutines, splitting each
// matched archetype into row-range chunks. Each chunk is processed by an
// independent goroutine; the call returns once every chunk has finished.
//
// Concurrency contract:
//
//   - fn MUST NOT perform structural mutations on the world (Spawn, Insert,
//     Remove, Despawn). Buffer such operations through a [command.CommandBuffer]
//     instead (T-1F).
//   - fn MAY mutate the component pointed to by *T; concurrent goroutines
//     touch disjoint rows, so per-row mutation is race-free.
//   - The query's [Access] declaration must not conflict with any other
//     concurrently running query — the schedule executor enforces this.
//
// Phase 1 implementation is a straightforward parallel-fan-out using
// [sync.WaitGroup]. Work-stealing across archetypes is deferred to Phase 3
// when the scheduler can supply live load data.
func (q *Query1[T]) ParIter(w *world.World, fn func(entity.Entity, *T)) {
	q.refresh(w)
	if len(q.matched) == 0 {
		return
	}

	chunkSize := chunkSizeFor(totalRows(w, q.matched))

	var wg sync.WaitGroup
	for _, archID := range q.matched {
		arch := w.Archetypes().At(archID)
		n := arch.Len()
		if n == 0 {
			continue
		}
		for start := 0; start < n; start += chunkSize {
			end := min(start+chunkSize, n)
			if end-start < MinChunkSize && start == 0 && n < MinChunkSize {
				// Tiny archetype: run inline to skip goroutine overhead.
				q.runChunk(w, arch, start, end, fn)
				continue
			}
			wg.Add(1)
			go func(a *world.Archetype, lo, hi int) {
				defer wg.Done()
				q.runChunk(w, a, lo, hi, fn)
			}(arch, start, end)
		}
	}
	wg.Wait()
}

// runChunk processes the [lo, hi) row range of arch, applying fn for every
// row that passes per-row filters. Caller is responsible for goroutine
// dispatch; this method intentionally has no synchronization of its own.
func (q *Query1[T]) runChunk(w *world.World, arch *world.Archetype, lo, hi int, fn func(entity.Entity, *T)) {
	entities := arch.Entities()
	for row := lo; row < hi; row++ {
		e := entities[row]
		if !passesPerRow(w, arch, row, q.perRow) {
			continue
		}
		ptr := fetchComponent(w, arch, e, row, q.id)
		fn(e, (*T)(ptr))
	}
}

// chunkSizeFor picks a row-count chunk size. Targets one chunk per CPU
// while never going below [MinChunkSize].
func chunkSizeFor(totalRows int) int {
	cpus := runtime.NumCPU()
	if cpus <= 1 || totalRows <= MinChunkSize {
		return MinChunkSize
	}
	per := totalRows / cpus
	if per < MinChunkSize {
		return MinChunkSize
	}
	return per
}

// totalRows sums the entity counts of every matched archetype.
func totalRows(w *world.World, archIDs []world.ArchetypeID) int {
	n := 0
	for _, id := range archIDs {
		n += w.Archetypes().At(id).Len()
	}
	return n
}
