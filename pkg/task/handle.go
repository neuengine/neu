package task

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"sync/atomic"
)

// Task lifecycle states.
const (
	stPending uint32 = iota
	stRunning
	stFinished
	stDetached
)

// PanicError wraps a value recovered from a panicking task. BlockOn re-panics
// with this on the caller goroutine; Poll surfaces it via the ok==false path
// after IsFinished reports true (inspect with Err()).
type PanicError struct {
	Value any
	Stack []byte
}

func (e *PanicError) Error() string {
	return fmt.Sprintf("task: panicked: %v", e.Value)
}

// TaskHandle carries the result of an async task spawned with Spawn.
type TaskHandle[T any] struct {
	done chan struct{}
	res  T
	err  error
	st   atomic.Uint32
}

// Spawn schedules fn on the compute pool and returns a handle to its result.
// If the pool is shut down, fn runs inline so the handle still resolves.
func Spawn[T any](p *ComputePool, fn func() T) *TaskHandle[T] {
	h := &TaskHandle[T]{done: make(chan struct{})}
	run := func() {
		h.st.Store(stRunning)
		defer func() {
			if r := recover(); r != nil {
				h.err = &PanicError{Value: r, Stack: debug.Stack()}
			}
			h.st.Store(stFinished)
			close(h.done)
		}()
		h.res = fn()
	}
	if err := p.submit(run); err != nil {
		run() // pool closed: resolve inline, never drop the work
	}
	return h
}

// Poll returns the result without blocking. ok == false while the task is
// still running or if it panicked (check Err()).
func (h *TaskHandle[T]) Poll() (T, bool) {
	if h.st.Load() == stFinished {
		if h.err != nil {
			var zero T
			return zero, false
		}
		return h.res, true
	}
	var zero T
	return zero, false
}

// BlockOn waits for completion, cooperatively draining the pool instead of
// sleeping (§4.9). Re-panics on the caller if the task panicked.
func (h *TaskHandle[T]) BlockOn(p *ComputePool) T {
	for h.st.Load() != stFinished {
		if !p.tryRunOne() {
			runtime.Gosched()
		}
	}
	if h.err != nil {
		panic(h.err)
	}
	return h.res
}

// Detach abandons the result; the task still runs to completion. Subsequent
// Poll/BlockOn are not expected.
func (h *TaskHandle[T]) Detach() {
	h.st.CompareAndSwap(stPending, stDetached)
	h.st.CompareAndSwap(stRunning, stDetached)
}

// IsFinished reports whether the task has completed (successfully or by
// panic).
func (h *TaskHandle[T]) IsFinished() bool {
	return h.st.Load() == stFinished
}

// Err returns the panic error if the task failed, else nil. Valid once
// IsFinished is true.
func (h *TaskHandle[T]) Err() error { return h.err }
