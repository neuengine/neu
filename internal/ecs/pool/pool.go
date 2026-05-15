// Package pool provides type-safe [sync.Pool] wrappers for engine-internal
// short-lived allocations: command payloads, event payloads, transient view
// structs, and other hot-path objects whose lifetime is bounded by a single
// tick.
//
// The wrappers enforce the C27 constraint that hot-path allocations flow
// through sync.Pool. They produce zero-allocation Get/Put cycles in steady
// state (after the pool warms up) while keeping the API type-safe via Go
// generics — callers never type-assert or box concrete values.
//
// # Use cases
//
//   - [Pool] for single values: command payload structs, observer contexts.
//   - [SlicePool] for reusable backing arrays: the `[]component.Data` carried
//     by SpawnCommand, event-batch buffers, query iteration scratch.
//
// # Concurrency
//
// Both wrappers are concurrent-safe (the underlying sync.Pool is). The Reset
// hook on Put runs on the calling goroutine so callers must ensure their
// reset function is itself concurrent-safe when the pool is shared across
// systems running in parallel — typically trivial since Reset operates only
// on the value being returned to the pool.
package pool

import "sync"

// Pool is a type-safe wrapper around [sync.Pool] for values of type T.
// Get returns *T (so callers can mutate fields directly); Put returns the
// pointer for reuse. nil pointers passed to Put are silently dropped.
//
// The optional reset hook (set via [NewWithReset]) runs on Put before the
// value re-enters the pool, useful for clearing fields that should not hold
// GC-rooted references across recycle cycles.
type Pool[T any] struct {
	reset func(*T)
	inner sync.Pool
}

// New returns a Pool whose New function returns a fresh zero-valued *T.
// Equivalent to NewWithFactory(new[T]) without the cgo-style boilerplate.
func New[T any]() *Pool[T] {
	return &Pool[T]{
		inner: sync.Pool{
			New: func() any { return new(T) },
		},
	}
}

// NewWithReset returns a Pool that calls reset(*T) on every [Pool.Put]
// before the value re-enters the pool. nil reset is treated like [New].
func NewWithReset[T any](reset func(*T)) *Pool[T] {
	return &Pool[T]{
		inner: sync.Pool{
			New: func() any { return new(T) },
		},
		reset: reset,
	}
}

// NewWithFactory returns a Pool that constructs new values via fn instead of
// returning the zero value. Useful when T must be initialised on creation
// (e.g. holds a pre-allocated slice with a nominal capacity).
func NewWithFactory[T any](fn func() *T) *Pool[T] {
	if fn == nil {
		return New[T]()
	}
	return &Pool[T]{
		inner: sync.Pool{
			New: func() any { return fn() },
		},
	}
}

// Get returns a *T from the pool. The returned pointer either references a
// recycled value or a freshly-constructed one — callers must not assume any
// particular initial state and should overwrite fields they care about.
func (p *Pool[T]) Get() *T {
	return p.inner.Get().(*T)
}

// Put returns v to the pool. nil values are silently dropped. When the pool
// was constructed with a reset hook, it runs before v re-enters.
func (p *Pool[T]) Put(v *T) {
	if v == nil {
		return
	}
	if p.reset != nil {
		p.reset(v)
	}
	p.inner.Put(v)
}

// SliceBuf wraps a reusable []T backing array. The wrapper exists so that
// [SlicePool] can store a single stable pointer per slice instead of boxing
// a slice header into an `any` interface on every Put — boxing itself
// heap-allocates and defeats the purpose of pooling.
//
// Callers Get a *SliceBuf, mutate Data freely (typically via append), and
// Put it back when done. The Data field's backing array survives across
// Get/Put cycles; only the wrapper struct is recycled.
type SliceBuf[T any] struct {
	Data []T
}

// SlicePool is a type-safe pool for reusable []T backing arrays. Get returns
// a *[SliceBuf] with Data.len=0 and Data.cap≥minCap (after warm-up); Put
// clears element references, truncates to len=0, and stores the wrapper for
// reuse. After warm-up Get/Put round-trips perform zero heap allocations.
type SlicePool[T any] struct {
	inner  sync.Pool
	minCap int
}

// NewSlicePool returns a slice pool that pre-allocates each new buffer's
// Data with capacity ≥ minCap. minCap < 0 is clamped to 0 (no preallocation).
func NewSlicePool[T any](minCap int) *SlicePool[T] {
	if minCap < 0 {
		minCap = 0
	}
	return &SlicePool[T]{
		minCap: minCap,
		inner: sync.Pool{
			New: func() any {
				return &SliceBuf[T]{Data: make([]T, 0, minCap)}
			},
		},
	}
}

// MinCap returns the pre-allocation hint passed to [NewSlicePool].
func (p *SlicePool[T]) MinCap() int { return p.minCap }

// Get returns a *[SliceBuf] from the pool with Data.len=0 and Data.cap≥MinCap
// (after at least one warm-up Get/Put cycle). The capacity may be larger if
// a previous caller grew Data via append before returning it.
func (p *SlicePool[T]) Get() *SliceBuf[T] {
	buf := p.inner.Get().(*SliceBuf[T])
	buf.Data = buf.Data[:0]
	return buf
}

// Put returns buf to the pool with its backing array intact and len reset
// to zero. Element references are cleared first so the backing array does
// not pin GC roots. nil buffers are silently dropped. After Put callers
// MUST NOT retain references to buf — a concurrent Get may hand it out.
func (p *SlicePool[T]) Put(buf *SliceBuf[T]) {
	if buf == nil {
		return
	}
	clear(buf.Data)
	buf.Data = buf.Data[:0]
	p.inner.Put(buf)
}
