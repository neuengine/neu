# Task System — Go Implementation

**Version:** 0.1.0
**Status:** Stable
**Layer:** go
**Implements:** [l1-task-system.md](l1-task-system.md)

## Overview

Go-level design for structured parallelism: a bounded **ComputePool** of pinned
worker goroutines with per-worker work-stealing deques, an elastic **IOPool**,
scoped task groups with stack-borrow safety via `sync.WaitGroup`, an
atomic-claim batched dispatcher, cooperative work-stealing on block, and a
main-thread executor polled once per frame. Zero external dependencies — only
`sync`, `sync/atomic`, `runtime`, and `context` from the standard library.

## Related Specifications

- [l1-task-system.md](l1-task-system.md) — L1 concept specification (parent)
- [l2-system-scheduling-go.md](l2-system-scheduling-go.md) — systems are dispatched as tasks onto the ComputePool
- [l1-asset-system.md](l1-asset-system.md) — async asset loading uses the IOPool

## 1. Motivation

The engine must exploit multi-core hardware without uncontrolled goroutine
spawning (which causes scheduler oversubscription and GC pressure). This package
provides bounded CPU parallelism, elastic IO concurrency, and a main-thread
escape hatch — all behind a small typed API so gameplay code never touches
`go` directly.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: generics for `TaskHandle[T]`, `iter` for parallel iteration.
- **No raw `go` for gameplay**: all parallelism flows through the pools.
- **C-003**: stdlib only (`sync`, `sync/atomic`, `runtime`, `context`).
- **C-027**: per-task control blocks are pooled via `sync.Pool` to avoid
  per-spawn heap allocation on the hot path.
- **C-005**: every concurrent path is `-race` clean; this is the primary CI gate.
- **No `Option[T]`**: L1 `Option[uint]` config fields map to `*uint` (nil =
  auto-detect); `TaskHandle.poll()` maps to `(T, bool)`.
- **Goroutine pinning caveat**: Go's runtime preempts goroutines; "pinned
  worker" means one long-lived goroutine per logical worker slot, *not* OS
  thread affinity. True OS-thread pinning (`runtime.LockOSThread`) is used
  only by the main-thread executor.

## 3. Core Invariants

> [!NOTE]
> See [l1-task-system.md §3](l1-task-system.md) for technology-agnostic
> invariants. Go-specific compliance is tabulated in §4.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: No oversubscription | `ComputePool` spawns exactly `n` worker goroutines at construction (`n = max(1, runtime.NumCPU()-1)` or config). No worker goroutine is ever created after `NewComputePool`. |
| **INV-2**: Scope safety | `Scope.Spawn` increments an internal `sync.WaitGroup`; `Scope` is created and consumed inside `RunScope(func(*Scope))` which calls `wg.Wait()` before returning — borrowed slices outlive all children by construction. |
| **INV-3**: Main-thread affinity | `MainThreadExecutor` holds a buffered `chan func()`. Its consumer runs only inside `PollMainThread()`, which the frame loop calls from the goroutine that did `runtime.LockOSThread()` at startup. |
| **INV-4**: Graceful shutdown | `Pool.Shutdown(ctx)` closes the submission channel, lets workers drain remaining deque batches, then `wg.Wait()`s the workers; `ctx` deadline bounds the drain. |

## Go Package

```
pkg/task/
  pool.go        // ComputePool, IOPool, TaskPoolConfig
  scope.go       // Scope, RunScope, parallel slice ops
  handle.go      // TaskHandle[T]
  dispatch.go    // ForBatched atomic-claim dispatcher
  mainthread.go  // MainThreadExecutor
```

Public, reusable, no ECS dependency (mirrors `pkg/math` boundary).

## Type Definitions

```go
type TaskPoolConfig struct {
    ComputeThreads *uint   // nil ⇒ auto (NumCPU-1)
    IOMinThreads   uint    // default 1
    IOMaxThreads   uint    // default 512
    PercentOfCores *float64 // alternative sizing; overrides ComputeThreads if set
}

type ComputePool struct {
    workers []*worker     // fixed-size; len == thread count (INV-1)
    global  chan batch    // overflow / external submission
    prio    [3]deque      // Critical, Normal, Low (§4.10)
    wg      sync.WaitGroup
    quit    chan struct{}
}

type IOPool struct {
    sem      chan struct{} // bounded by IOMaxThreads
    minAlive uint
}

// TaskHandle[T] carries the result of an async task.
type TaskHandle[T any] struct {
    done chan struct{}
    res  T
    err  error
    st   atomic.Uint32 // pending|running|finished|detached
}

type Scope struct {
    pool *ComputePool
    wg   sync.WaitGroup
}

type MainThreadExecutor struct {
    queue chan func()
}
```

## Key Methods

```
func NewTaskPools(cfg TaskPoolConfig) (*ComputePool, *IOPool)

// Scoped tasks (INV-2): blocks until every Spawn completes.
func RunScope(p *ComputePool, body func(s *Scope))
func (s *Scope) Spawn(fn func())

// Parallel slice ops (built on RunScope; synchronous).
func ParChunkMap[T, R any](in []T, chunk int, fn func(T) R) []R
func ParSplatMap[T, R any](in []T, maxTasks int, fn func(T) R) []R

// Batched dispatcher (§4.8): single atomic increment claims a batch.
func ForBatched[T any](p *ComputePool, items []T, batchSize int, fn func(batch []T))

// Async + handle (§4.6).
func Spawn[T any](p *ComputePool, fn func() T) *TaskHandle[T]
func (h *TaskHandle[T]) Poll() (T, bool)   // non-blocking
func (h *TaskHandle[T]) BlockOn() T         // cooperative steal while waiting (§4.9)
func (h *TaskHandle[T]) Detach()
func (h *TaskHandle[T]) IsFinished() bool

// Main thread (INV-3).
func (m *MainThreadExecutor) Execute(fn func())
func (m *MainThreadExecutor) PollMainThread() // called once per frame

// Shutdown (INV-4).
func (p *ComputePool) Shutdown(ctx context.Context) error
```

`BlockOn` implements §4.9 TryCooperate: while `!IsFinished()`, attempt
`pool.tryStealBatch()` and execute it; only `runtime.Gosched()` when the pool
is genuinely empty — never `time.Sleep`, never busy-spin.

## Performance Strategy

- **Work-stealing deques**: owner pushes/pops the tail (LIFO, cache-hot);
  thieves steal the head (FIFO) — classic Chase-Lev, lock-free via `atomic`.
- **Atomic batch claim** (§4.8): one `atomic.Uint64.Add(1)` per batch, not per
  item — minimal contention for fine-grained component iteration.
- **`sync.Pool` task blocks** (C-027): `TaskHandle`/`batch` structs are reused
  across spawns; steady-state spawn is allocation-free.
- **Priority deques** (§4.10): workers drain Critical → Normal → Low; IOPool is
  excluded from stealing so a slow file read never blocks a compute worker.

## Error Handling

| Condition | Behavior |
| :--- | :--- |
| Task panics | Recovered in the worker; wrapped into `TaskHandle.err`; worker survives and continues |
| `BlockOn` on a panicked task | Re-panics on the caller goroutine with the wrapped error |
| Submit after `Shutdown` | Returns `ErrPoolClosed`; never silently drops work |
| `PollMainThread` from non-main goroutine | Panics (programming error — violates INV-3) |

```go
var ErrPoolClosed = errors.New("task: pool is shut down")
```

## Testing Strategy

- **Race gate (C-005)**: every test runs under `-race`; a stress test spawns
  10k tasks across scopes and asserts no race, no leak (`runtime.NumGoroutine`
  returns to baseline after `Shutdown`).
- **Scope safety**: a scope borrowing a stack slice mutated by children; assert
  all writes visible after `RunScope` returns.
- **Oversubscription**: assert worker goroutine count is constant under load.
- **Cooperative steal**: `BlockOn` on the main goroutine measurably reduces
  tail latency vs. a sleep-wait baseline.
- **Benchmarks**: `BenchmarkForBatched`, `BenchmarkSpawnPooled` (alloc/op must
  be 0 in steady state).

## 7. Drawbacks & Alternatives

- **Drawback**: fixed worker count can underutilize cores if many tasks block.
  **Alternative**: unbounded goroutines (Go-native).
  **Decision**: bounded is required by INV-1; blocking work belongs on the
  elastic IOPool, not ComputePool.
- **Drawback**: Go preempts goroutines, so "pinned worker" is logical, not
  physical — NUMA locality is not guaranteed.
  **Alternative**: `runtime.LockOSThread` per worker.
  **Decision**: locking all workers harms the Go scheduler; only the
  main-thread executor locks. Physical pinning is L1 Open Question Q3 —
  <!-- TBD (L1 §5 Q3): revisit OS-thread pinning if profiling shows NUMA
       stalls dominate. Not in 0.1.0 scope. -->
- **Drawback**: no GPU-compute pool.
  **Decision**: L1 Open Question Q2 — out of scope until the render backend
  lands (Phase 4, C-002 Hold).

## Canonical References

<!-- MANDATORY for Stable status. Stub — populate when implementation lands
     (P3 Assets & Math). Stable promotion blocked: (1) L1 parent Draft;
     (2) C29 — no validating examples/ implementation yet. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-17 | Initial L2 draft — Go translation of l1-task-system v0.2.0. Chase-Lev work-stealing, sync.WaitGroup scopes, atomic-claim ForBatched, sync.Pool task blocks (C-027), main-thread LockOSThread executor. L1 Q2/Q3 carried as TBD. |
