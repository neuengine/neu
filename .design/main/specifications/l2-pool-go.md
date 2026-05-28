# Object Pool — Go Implementation

**Version:** 0.1.0
**Status:** Stable
**Layer:** go
**Implements:** [l1-ecs-lifecycle-patterns.md](l1-ecs-lifecycle-patterns.md)

## Overview

Go implementation of the C27 hot-path allocation constraint. Provides two generic, concurrency-safe `sync.Pool` wrappers: `Pool[T]` for single-value recycling and `SlicePool[T]` for reusable backing arrays. Both wrappers produce zero heap allocations per `Get`/`Put` in steady state and keep the API type-safe via generics — callers never type-assert or box concrete values.

## Related Specifications

- [l1-ecs-lifecycle-patterns.md](l1-ecs-lifecycle-patterns.md) — L1 parent; §5.6 Entity Object Pooling and C27 constraint.
- [l2-command-system-go.md](l2-command-system-go.md) — primary consumer of `Pool[T]` for command payload structs.
- [l2-event-system-go.md](l2-event-system-go.md) — primary consumer of `SlicePool[T]` for event-batch buffers.
- [l2-view-go.md](l2-view-go.md) — canonical use case: transient view structs are a target allocation class for `Pool[T]`.

## 1. Motivation

Go's scheduler may collect idle `sync.Pool` objects between GC cycles, but during a running frame the pool warms up and subsequent `Get`/`Put` pairs are allocation-free. The engine's per-frame hot path — command dispatch, event delivery, query scratch buffers — allocates O(entity count) short-lived structs. Without pooling this generates GC pressure proportional to frame rate × entity density.

Raw `sync.Pool` requires a type-assertion on every `Get`. The assertion is cheap, but boxing the value into `any` on `Put` heap-allocates an interface header unless the compiler can devirtualise — which it cannot for open `any`. The generic wrapper eliminates the box by storing the pool's `New` factory once; all subsequent `Get`/`Put` paths are allocation-free in steady state, verified by the package's `bench_test.go`.

## 2. Constraints & Assumptions

- **C27**: Every hot-path, tick-bounded allocation MUST flow through `Pool[T]` or `SlicePool[T]`. Allocating a pooled type directly via `new(T)` in the hot path is a policy violation.
- **Pool lifetime**: Pool instances MUST be created once at system/World initialization — never per-frame.
- **Reset safety**: The `reset func(*T)` hook passed to `NewWithReset` runs on the Put-site goroutine. Callers sharing a pool across parallel systems must ensure the reset function itself is goroutine-safe.
- **Entity pools excluded**: Entity-level pools (§5.6) must NOT use `Pool[T]` — entity pools require persistent allocation across GC cycles. `Pool[T]` defers ownership to the GC between frames.
- **Nil guard**: `Put(nil)` is silently dropped; callers need not nil-check before returning.
- **Post-Put invalidation**: After `Put`, the caller MUST NOT retain any reference to the returned pointer. A concurrent `Get` may hand it out immediately.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1 (Atomic Tag Updates)** | Not in scope — `pool` does not manage entity tags or component bitmasks. |
| **INV-2 (Deterministic Cleanup)** | `NewWithReset` accepts a `reset func(*T)` hook that runs synchronously on `Put` before the value re-enters the pool. Callers use this to zero GC-rooted fields, equivalent to a deterministic destructor call at a defined synchronization point (§5.2 Component Destructors analogue for pooled values). |
| **INV-3 (View Consistency)** | Not in scope — `pool` does not manage archetype views. |

## 5. Detailed Design

### 5.1 `Pool[T]` — Single-Value Pool

`Pool[T]` wraps `sync.Pool` for a concrete pointer type `*T`.

**Project Structure:**

```plaintext
internal/ecs/pool/
├── pool.go          # Pool[T], SlicePool[T], SliceBuf[T]
└── bench_test.go    # zero-alloc steady-state benchmarks
```

**Constructors:**

| Constructor | New factory | Reset hook | Use case |
| :--- | :--- | :--- | :--- |
| `New[T]()` | `new(T)` — zero value | — | Command / event payload structs |
| `NewWithReset[T](reset)` | `new(T)` | `reset(*T)` on Put | Fields holding GC-rooted pointers |
| `NewWithFactory[T](fn)` | `fn()` — custom | — | `T` needs non-zero init (e.g. pre-allocated inner slice) |

**Get/Put contract (pseudo-code):**

```plaintext
Get() *T:
  ← recycled *T, or fresh new(T) if pool is cold
  → caller must overwrite all fields it reads (state is arbitrary after recycle)

Put(*T):
  if v == nil: no-op
  if reset != nil: reset(v)   // runs synchronously on calling goroutine
  → v returned to pool; caller MUST NOT use v after this point
```

### 5.2 `SlicePool[T]` — Slice Backing Array Pool

`SlicePool[T]` pools `*SliceBuf[T]` wrappers that carry a reusable `[]T` backing array. The wrapper exists so the pool stores one stable pointer instead of boxing a three-word slice header into `any` on every `Put` — such boxing would itself heap-allocate and defeat the purpose.

**API (pseudo-code):**

```plaintext
SliceBuf[T] { Data []T }   // stable wrapper; Data's backing array survives Get/Put

NewSlicePool[T](minCap int) *SlicePool[T]
  New: &SliceBuf{Data: make([]T, 0, minCap)}

Get() *SliceBuf[T]:
  buf.Data = buf.Data[:0]  // len reset; cap and backing array preserved
  ← buf ready for append

Put(*SliceBuf[T]):
  clear(buf.Data)          // zero element refs — prevents GC root pinning
  buf.Data = buf.Data[:0]
  → backing array retained for the next Get
```

`clear(buf.Data)` zeroes element pointers before truncation. Without it the backing array would pin GC roots (pointer-typed `T`) across pool rounds, defeating the pool's memory-pressure benefit.

### 5.3 Concurrency Model

Both wrappers are concurrency-safe — the underlying `sync.Pool` uses per-P local stores with a steal fallback, requiring no external lock. The reset hook (§5.1) runs on the Put goroutine; callers are responsible for reset-function safety when the pool is shared across parallel systems.

## 6. Implementation Notes

1. The pool warms up after the first `Get`/`Put` cycle; the very first frame may produce allocations.
2. Use `NewWithFactory` only when factory work (e.g. slice pre-allocation) justifies the overhead — benchmark first.
3. `SlicePool.MinCap` is a hint, not a cap. If a caller appends beyond `minCap`, the grown buffer is retained on the next Put. Size-stable callers benefit from this auto-sizing.
4. Pools must not be `nil`-assigned or replaced at runtime; their identity is used by `sync.Pool`'s GC interaction.

## 7. Drawbacks & Alternatives

- **GC eviction between frames**: `sync.Pool` does not guarantee persistence across GC cycles. Acceptable in practice — the engine's pools are warm throughout active gameplay. Entity-level pools (requiring persistence) use the disabled-entity mechanism (§5.6).
- **Arena / bump allocator**: a per-frame linear allocator would guarantee zero-allocation without GC risk, at the cost of manual reset-and-reuse semantics. Deferred until profiling confirms pooling is insufficient.
- **Direct `sync.Pool` usage**: eliminates the wrapper but requires a type-assertion on every `Get` and risks cross-team misuse. The generic wrapper is a net win in correctness and expressibility at zero runtime cost.

## Canonical References

<!-- Downstream agents: read ALL files below before implementing or extending the pool package. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |
| `[POOL]` | `internal/ecs/pool/pool.go` | Authoritative implementation of `Pool[T]`, `SlicePool[T]`, `SliceBuf[T]` |
| `[BENCH]` | `internal/ecs/pool/bench_test.go` | Steady-state zero-allocation benchmark gates (must remain 0 alloc/op) |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-28 | Initial Stable — documents `Pool[T]`, `SlicePool[T]`, concurrency model and C27 invariant compliance. Implements l1-ecs-lifecycle-patterns.md §5.6. |
