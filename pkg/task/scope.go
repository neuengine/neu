package task

import (
	"runtime"
	"sync/atomic"
)

// Scope is a structured-concurrency group (INV-2). Every task spawned through
// it completes before the enclosing RunScope returns, so stack data borrowed
// by child tasks stays valid for their whole lifetime.
//
// Each Scope owns a work-stealing deque. Pool workers steal from it (parallel
// execution), while the RunScope goroutine drains only this deque while
// joining. Draining its own children — never unrelated pool work — is what
// makes nested RunScope calls livelock-free and keeps a single worker from
// deadlocking on its own scope.
type Scope struct {
	pool    *ComputePool
	dq      *deque
	pending atomic.Int64
}

// RunScope runs body, then blocks until every Scope.Spawn started inside it
// has finished. The join drains this scope's own deque inline and otherwise
// yields; it never recursively absorbs other scopes' blocking work.
func RunScope(p *ComputePool, body func(s *Scope)) {
	s := &Scope{pool: p, dq: newDeque(logInitialDeque)}
	p.registerScope(s.dq)
	defer p.unregisterScope(s.dq)

	body(s)

	for s.pending.Load() > 0 {
		if t, ok := s.dq.popBottom(); ok {
			runTask(t) // run our own child inline — always makes progress
			continue
		}
		// Remaining children are in flight on pool workers; wait without
		// stealing unrelated (possibly blocking) work.
		runtime.Gosched()
	}
}

// Spawn schedules fn as a child of the scope. It must be called from the
// scope's owning goroutine (inside body) — the deque is single-owner by the
// Chase-Lev contract; pool workers only ever steal from it.
func (s *Scope) Spawn(fn func()) {
	s.pending.Add(1)
	s.dq.pushBottom(func() {
		defer s.pending.Add(-1)
		fn()
	})
	s.pool.wake()
}

// ParChunkMap splits in into fixed-size chunks, maps each chunk element in
// parallel, and returns results in input order. Synchronous (built on
// RunScope).
func ParChunkMap[T, R any](p *ComputePool, in []T, chunk int, fn func(T) R) []R {
	out := make([]R, len(in))
	if len(in) == 0 {
		return out
	}
	if chunk < 1 {
		chunk = 1
	}
	RunScope(p, func(s *Scope) {
		for start := 0; start < len(in); start += chunk {
			start := start
			end := min(start+chunk, len(in))
			s.Spawn(func() {
				for i := start; i < end; i++ {
					out[i] = fn(in[i])
				}
			})
		}
	})
	return out
}

// ParSplatMap divides in into at most maxTasks roughly equal parts. Use it to
// bound task count rather than chunk size.
func ParSplatMap[T, R any](p *ComputePool, in []T, maxTasks int, fn func(T) R) []R {
	n := len(in)
	if n == 0 {
		return make([]R, 0)
	}
	if maxTasks < 1 {
		maxTasks = 1
	}
	parts := min(maxTasks, n)
	chunk := (n + parts - 1) / parts
	return ParChunkMap(p, in, chunk, fn)
}
