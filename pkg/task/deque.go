// Package task provides structured parallelism for the engine: a bounded
// work-stealing ComputePool, an elastic IOPool, scoped task groups, an
// atomic-claim batched dispatcher, and a main-thread executor.
//
// Implements specification l2-task-system-go.md (Layer: go, Phase 3
// [Bootstrap]). Zero external dependencies — stdlib only (C-003).
package task

import (
	"sync"
	"sync/atomic"
)

// task is the unit of work scheduled on the pools.
type task = func()

// tcell carries one task through a deque slot. Cells are recycled through
// cellPool so steady-state push/pop is allocation-free (C-027), and they give
// every enqueued task a unique identity — slot reuse on ring wrap publishes a
// *different* pointer, which is what makes the atomic slot race-free.
type tcell struct{ fn task }

var cellPool = sync.Pool{New: func() any { return new(tcell) }}

func acquireCell(fn task) *tcell {
	c := cellPool.Get().(*tcell)
	c.fn = fn
	return c
}

// consume extracts the task and returns the cell to the pool. Only ever called
// by the single goroutine that won ownership (popBottom owner / steal CAS
// winner), so it needs no synchronization.
func consume(c *tcell) task {
	fn := c.fn
	c.fn = nil
	cellPool.Put(c)
	return fn
}

// circArray is the growable ring backing a single deque. size is a power of
// two so index wrapping is a mask. Slots are atomic pointers: get/put are the
// only accesses to slot memory and they are race-free by construction.
type circArray struct {
	size int64
	mask int64
	buf  []atomic.Pointer[tcell]
}

func newCircArray(logSize uint) *circArray {
	size := int64(1) << logSize
	return &circArray{size: size, mask: size - 1, buf: make([]atomic.Pointer[tcell], size)}
}

func (a *circArray) get(i int64) *tcell    { return a.buf[i&a.mask].Load() }
func (a *circArray) put(i int64, c *tcell) { a.buf[i&a.mask].Store(c) }

// grow returns a doubled copy holding the live range [top, bottom).
func (a *circArray) grow(top, bottom int64) *circArray {
	n := &circArray{
		size: a.size << 1,
		mask: (a.size << 1) - 1,
		buf:  make([]atomic.Pointer[tcell], a.size<<1),
	}
	for i := top; i < bottom; i++ {
		n.put(i, a.get(i))
	}
	return n
}

// deque is a Chase-Lev lock-free work-stealing deque.
//
// The owning worker uses pushBottom / popBottom (LIFO tail — cache-hot).
// Thieves use steal (FIFO head). Correctness follows Lê, Pop, Cohen &
// Nardelli (2013); Go's sequentially consistent sync/atomic on top, bottom
// and the per-slot atomic pointers subsumes every required fence.
type deque struct {
	bottom atomic.Int64
	top    atomic.Int64
	array  atomic.Pointer[circArray]
}

func newDeque(logInitialSize uint) *deque {
	d := &deque{}
	d.array.Store(newCircArray(logInitialSize))
	return d
}

// pushBottom appends t at the owner end. Owner-only; never called concurrently
// with itself or popBottom.
func (d *deque) pushBottom(t task) {
	c := acquireCell(t)
	b := d.bottom.Load()
	tp := d.top.Load()
	a := d.array.Load()
	if b-tp > a.size-1 {
		a = a.grow(tp, b)
		d.array.Store(a)
	}
	a.put(b, c)
	d.bottom.Store(b + 1) // publishes the slot write
}

// popBottom removes and returns the most recently pushed task (LIFO).
// Owner-only. ok == false when the deque is empty or the last element was
// lost to a concurrent steal.
func (d *deque) popBottom() (t task, ok bool) {
	b := d.bottom.Load() - 1
	a := d.array.Load()
	d.bottom.Store(b)
	tp := d.top.Load()
	if tp > b {
		d.bottom.Store(b + 1) // empty: restore canonical state
		return nil, false
	}
	c := a.get(b)
	if tp != b {
		return consume(c), true // >1 element: no thief contention
	}
	// Exactly one element: a thief may be taking it concurrently.
	won := d.top.CompareAndSwap(tp, tp+1)
	d.bottom.Store(b + 1)
	if !won {
		return nil, false
	}
	return consume(c), true
}

// steal takes the oldest task (FIFO head) for another worker. Thief-safe.
func (d *deque) steal() (t task, ok bool) {
	tp := d.top.Load()
	b := d.bottom.Load()
	if tp >= b {
		return nil, false // empty
	}
	a := d.array.Load()
	c := a.get(tp)
	if !d.top.CompareAndSwap(tp, tp+1) {
		return nil, false // lost the race
	}
	return consume(c), true
}

// approxLen reports a non-authoritative element count for metrics/heuristics.
func (d *deque) approxLen() int64 {
	n := d.bottom.Load() - d.top.Load()
	if n < 0 {
		return 0
	}
	return n
}
