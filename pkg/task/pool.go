package task

import (
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"runtime"
	"sync"
	"sync/atomic"
)

// ErrPoolClosed is returned by submission paths after Shutdown. Work is never
// silently dropped.
var ErrPoolClosed = errors.New("task: pool is shut down")

const (
	// logInitialDeque sizes each per-worker / priority deque at 256 slots.
	// Deep enough that gameplay-frame fan-out never triggers a grow.
	logInitialDeque = 8
)

// Priority selects which pool-level deque a task drains from. Workers always
// drain Critical before Normal before Low (§4.10).
type Priority uint8

const (
	Critical Priority = iota
	Normal
	Low
	numPriorities
)

// TaskPoolConfig configures pool sizing. Zero value is valid: compute auto =
// NumCPU-1, IO 1..512.
type TaskPoolConfig struct {
	ComputeThreads *uint    // nil ⇒ auto (NumCPU-1)
	IOMinThreads   uint     // default 1
	IOMaxThreads   uint     // default 512
	PercentOfCores *float64 // overrides ComputeThreads when set
}

func (c TaskPoolConfig) resolveCompute() int {
	cpu := runtime.NumCPU()
	if c.PercentOfCores != nil {
		n := int(math.Floor(float64(cpu) * *c.PercentOfCores))
		return max(1, n)
	}
	if c.ComputeThreads != nil {
		return max(1, int(*c.ComputeThreads))
	}
	return max(1, cpu-1)
}

func (c TaskPoolConfig) resolveIO() (minT, maxT uint) {
	minT, maxT = c.IOMinThreads, c.IOMaxThreads
	if minT == 0 {
		minT = 1
	}
	if maxT == 0 {
		maxT = 512
	}
	if minT > maxT {
		minT = maxT
	}
	return
}

// worker owns one local deque and runs on a single long-lived goroutine
// (INV-1: no goroutine is spawned after NewTaskPools).
type worker struct {
	id    int
	pool  *ComputePool
	local *deque
}

// ComputePool is a bounded work-stealing pool. Exactly len(workers) goroutines
// exist for its whole lifetime.
type ComputePool struct {
	workers []*worker
	prio    [numPriorities]*deque // Critical, Normal, Low
	notify  chan struct{}         // wake-one signal for parked workers
	quit    chan struct{}
	closed  atomic.Bool
	wg      sync.WaitGroup

	// submitMu serializes external pushes onto the shared priority deques.
	// Chase-Lev pushBottom is single-owner; workers only ever steal() the
	// head of prio (never popBottom), so a serialized pusher + many thieves
	// is a valid configuration.
	submitMu sync.Mutex

	// Active RunScope deques. Workers steal from these so scope children run
	// in parallel; the owning RunScope goroutine drains only its own deque
	// (see scope.go) which keeps structured joins deadlock- and livelock-free.
	scopeMu  sync.Mutex
	scopeDqs []*deque
}

func (p *ComputePool) registerScope(d *deque) {
	p.scopeMu.Lock()
	p.scopeDqs = append(p.scopeDqs, d)
	p.scopeMu.Unlock()
}

func (p *ComputePool) unregisterScope(d *deque) {
	p.scopeMu.Lock()
	for i, x := range p.scopeDqs {
		if x == d {
			last := len(p.scopeDqs) - 1
			p.scopeDqs[i] = p.scopeDqs[last]
			p.scopeDqs[last] = nil
			p.scopeDqs = p.scopeDqs[:last]
			break
		}
	}
	p.scopeMu.Unlock()
}

// stealScope takes one task from any active scope deque. The deque steal
// itself is lock-free; the mutex only guards the registry slice and is
// released before the task runs.
func (p *ComputePool) stealScope() (task, bool) {
	p.scopeMu.Lock()
	for _, d := range p.scopeDqs {
		if t, ok := d.steal(); ok {
			p.scopeMu.Unlock()
			return t, true
		}
	}
	p.scopeMu.Unlock()
	return nil, false
}

// IOPool is an elastic, semaphore-bounded pool for blocking work (file/network
// IO). Goroutines are created on demand and bounded by IOMaxThreads; it is
// excluded from compute stealing so a slow read never blocks a CPU worker.
type IOPool struct {
	sem      chan struct{}
	minAlive uint
	closed   atomic.Bool
}

// NewTaskPools constructs both pools and starts the compute workers.
func NewTaskPools(cfg TaskPoolConfig) (*ComputePool, *IOPool) {
	n := cfg.resolveCompute()
	cp := &ComputePool{
		workers: make([]*worker, n),
		notify:  make(chan struct{}, n),
		quit:    make(chan struct{}),
	}
	for i := range cp.prio {
		cp.prio[i] = newDeque(logInitialDeque)
	}
	for i := range n {
		w := &worker{id: i, pool: cp, local: newDeque(logInitialDeque)}
		cp.workers[i] = w
	}
	cp.wg.Add(n)
	for _, w := range cp.workers {
		go w.run()
	}

	minT, maxT := cfg.resolveIO()
	io := &IOPool{sem: make(chan struct{}, maxT), minAlive: minT}
	return cp, io
}

// NumWorkers reports the fixed compute worker count (INV-1).
func (p *ComputePool) NumWorkers() int { return len(p.workers) }

// submitPrio enqueues t onto a pool-level priority deque and wakes one parked
// worker. Used by Spawn / ForBatched / Scope (later tasks). Returns
// ErrPoolClosed if the pool is shut down.
func (p *ComputePool) submitPrio(pr Priority, t task) error {
	if p.closed.Load() {
		return ErrPoolClosed
	}
	p.submitMu.Lock()
	p.prio[pr].pushBottom(t)
	p.submitMu.Unlock()
	p.wake()
	return nil
}

// submit enqueues at Normal priority.
func (p *ComputePool) submit(t task) error { return p.submitPrio(Normal, t) }

func (p *ComputePool) wake() {
	select {
	case p.notify <- struct{}{}:
	default: // a worker is already running or will observe the deque
	}
}

// nextTask drains in priority order: local LIFO → pool priority deques →
// steal a peer. ok == false means the pool is momentarily empty.
func (w *worker) nextTask() (task, bool) {
	if t, ok := w.local.popBottom(); ok {
		return t, true
	}
	for pr := range numPriorities {
		if t, ok := w.pool.prio[pr].steal(); ok {
			return t, true
		}
	}
	if t, ok := w.steal(); ok {
		return t, true
	}
	return w.pool.stealScope()
}

// steal probes peers starting at a random offset to spread contention.
func (w *worker) steal() (task, bool) {
	ws := w.pool.workers
	n := len(ws)
	if n <= 1 {
		return nil, false
	}
	start := rand.IntN(n)
	for i := range n {
		victim := ws[(start+i)%n]
		if victim.id == w.id {
			continue
		}
		if t, ok := victim.local.steal(); ok {
			return t, true
		}
	}
	return nil, false
}

// tryRunOne pops or steals a single task from anywhere in the pool and runs
// it, returning true if one was found. Used by cooperative waiters
// (RunScope, TaskHandle.BlockOn) so a blocked goroutine helps drain the pool
// instead of sleeping — and never deadlocks when called from a worker.
func (p *ComputePool) tryRunOne() bool {
	for pr := range numPriorities {
		if t, ok := p.prio[pr].steal(); ok {
			runTask(t)
			return true
		}
	}
	for _, w := range p.workers {
		if t, ok := w.local.steal(); ok {
			runTask(t)
			return true
		}
	}
	if t, ok := p.stealScope(); ok {
		runTask(t)
		return true
	}
	return false
}

func (w *worker) run() {
	defer w.pool.wg.Done()
	for {
		if t, ok := w.nextTask(); ok {
			runTask(t)
			continue
		}
		// Genuinely empty: park until woken or shut down. No busy-spin.
		select {
		case <-w.pool.quit:
			w.drain()
			return
		case <-w.pool.notify:
			// Re-check; a task was published somewhere.
		}
	}
}

// drain runs every task still queued at shutdown so no work is silently
// dropped (INV-4).
func (w *worker) drain() {
	for {
		if t, ok := w.nextTask(); ok {
			runTask(t)
			continue
		}
		return
	}
}

// runTask isolates a task panic to its worker: the goroutine recovers and
// survives (Error Handling table). Result/error propagation belongs to
// TaskHandle (T-3A02).
func runTask(t task) {
	defer func() { _ = recover() }()
	t()
}

// Shutdown closes submission, lets workers drain queued work, then joins them.
// ctx bounds the drain; on deadline it returns ctx.Err() without leaking
// goroutines (they still exit once drained).
func (p *ComputePool) Shutdown(ctx context.Context) error {
	if p.closed.Swap(true) {
		return ErrPoolClosed
	}
	close(p.quit)
	// Nudge every parked worker so it observes quit.
	for range p.workers {
		p.wake()
	}
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Go runs fn on the elastic IO pool, blocking only if IOMaxThreads in-flight
// reads are already saturated. Returns ErrPoolClosed after shutdown.
func (io *IOPool) Go(fn func()) error {
	if io.closed.Load() {
		return ErrPoolClosed
	}
	io.sem <- struct{}{}
	go func() {
		defer func() {
			_ = recover()
			<-io.sem
		}()
		fn()
	}()
	return nil
}

// Shutdown marks the IO pool closed. In-flight tasks finish; new submissions
// return ErrPoolClosed.
func (io *IOPool) Shutdown() { io.closed.Store(true) }
