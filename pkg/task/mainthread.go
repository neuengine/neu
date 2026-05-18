package task

import (
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
)

// mtQueueSize is the buffered capacity of the main-thread work queue.
// 64 slots is sufficient for typical per-frame workloads (GPU sync callbacks,
// windowing events, audio buffer swaps) without blocking producers.
const mtQueueSize = 64

// MainThreadExecutor enqueues work for the OS thread that owns the frame loop
// (INV-3). Execute is safe to call from any goroutine; PollMainThread must
// only be called from the goroutine that called Bind.
type MainThreadExecutor struct {
	queue  chan func()
	mainID atomic.Int64 // 0 = unbound; set by Bind
}

// NewMainThreadExecutor returns an unbound MainThreadExecutor.
// Call Bind from the frame-loop goroutine (after runtime.LockOSThread) before
// the first PollMainThread call.
func NewMainThreadExecutor() *MainThreadExecutor {
	return &MainThreadExecutor{queue: make(chan func(), mtQueueSize)}
}

// Bind records the calling goroutine as the sole goroutine permitted to call
// PollMainThread. The caller is responsible for having already called
// runtime.LockOSThread() to pin the frame loop to an OS thread (INV-3).
func (m *MainThreadExecutor) Bind() {
	m.mainID.Store(currentGID())
}

// Execute enqueues fn for execution on the main thread. It blocks only if
// mtQueueSize functions are already queued. Safe to call from any goroutine.
func (m *MainThreadExecutor) Execute(fn func()) {
	m.queue <- fn
}

// PollMainThread drains all queued functions, running them in submission order.
// Must be called from the goroutine that called Bind (INV-3); panics otherwise.
func (m *MainThreadExecutor) PollMainThread() {
	if id := m.mainID.Load(); id != 0 && currentGID() != id {
		panic("task: PollMainThread must be called from the Bind goroutine (INV-3)")
	}
	for {
		select {
		case fn := <-m.queue:
			fn()
		default:
			return
		}
	}
}

// currentGID returns the current goroutine ID by parsing the first line of a
// minimal runtime stack trace. Used only by the INV-3 assertion in Bind and
// PollMainThread — never on the normal execution path.
func currentGID() int64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	s := strings.TrimPrefix(string(buf[:n]), "goroutine ")
	if i := strings.IndexByte(s, ' '); i > 0 {
		id, _ := strconv.ParseInt(s[:i], 10, 64)
		return id
	}
	return -1
}
