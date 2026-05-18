package task

import "sync/atomic"

// ForBatched processes items in parallel, claiming work one batch at a time
// with a single atomic add per batch (§4.8) — contention is on the cursor
// only, not per item, so fine-grained component iteration stays cheap. It is
// synchronous: it returns once every batch has been processed.
//
// The per-batch / per-item hot path performs no allocation (subslicing is
// allocation-free); per-call setup is O(numWorkers) and amortizes to ~0
// allocs per item over a large item count.
func ForBatched[T any](p *ComputePool, items []T, batchSize int, fn func(batch []T)) {
	n := len(items)
	if n == 0 {
		return
	}
	if batchSize < 1 {
		batchSize = 1
	}
	var cursor atomic.Int64
	workers := p.NumWorkers()
	if workers < 1 {
		workers = 1
	}
	// Cap dispatched runners at the number of batches — never spawn idle work.
	batches := (n + batchSize - 1) / batchSize
	if workers > batches {
		workers = batches
	}
	RunScope(p, func(s *Scope) {
		for i := 0; i < workers; i++ {
			s.Spawn(func() {
				for {
					start := int(cursor.Add(int64(batchSize))) - batchSize
					if start >= n {
						return
					}
					end := min(start+batchSize, n)
					fn(items[start:end])
				}
			})
		}
	})
}
