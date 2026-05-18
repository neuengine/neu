---
phase: 3
name: "Assets, Math & Concurrency"
status: Todo
subsystem: "pkg/math, pkg/asset, internal/scene, internal/task"
requires:
  - "Phase 1 ECS Core"
  - "Phase 2 App/Plugin assembly"
provides:
  - "Parallel task pool (work-stealing)"
  - "Asset server + handles + hot-reload IO abstraction"
  - "Scene serialization + entity remapping"
  - "Math primitives (vectors, matrices, quaternions, color)"
key_files:
  created: []
  modified: []
patterns_established: []
duration_minutes: ~
bootstrap: true
---

# Stage 3 Tasks — Assets, Math & Concurrency

**Phase:** 3
**Status:** Todo
**Strategic Goal:** Final foundation phase before the STOP FACTOR gate. After Phase 3 the upper render/physics/network stack can be unblocked.

## High-Level Checklist

- [ ] [T-3A] Task pool: worker pools, scoped tasks, parallel iteration (work-stealing). ([l1-task-system.md](../specifications/l1-task-system.md) → [l2-task-system-go.md](../specifications/l2-task-system-go.md))
- [ ] [T-3B] Asset server: loaders, handles, hot-reload, IO abstraction. ([l1-asset-system.md](../specifications/l1-asset-system.md) → [l2-asset-system-go.md](../specifications/l2-asset-system-go.md))
- [ ] [T-3C] Scene system: serialization, dynamic scenes, entity remapping. ([l1-scene-system.md](../specifications/l1-scene-system.md) → [l2-scene-system-go.md](../specifications/l2-scene-system-go.md))
- [ ] [T-3D] Math: vectors, matrices, quaternions, colors, geometric primitives, `simd/archsimd` accel. ([l1-math-system.md](../specifications/l1-math-system.md) → [l2-math-system-go.md](../specifications/l2-math-system-go.md))
- [ ] [T-3T] Validation: parallel-iter determinism (deterministic seed), asset hot-reload roundtrip, scene save/load fixture, math correctness vs. reference impl. Gate: `examples/{async,asset,scene,math}/` (C29 — unblocks P3 Draft → Stable).

## Atomic Decomposition

> Decomposed 2026-05-18 (Phase 2 = 100% Done — deferral gate satisfied). 18 atomic tasks across Tracks A–D + T.
> **Execution Mode:** Parallel (C3). **Critical path:** A (Task pool) → B (Asset async loader). Tracks C, D file-independent and parallelizable. Track T joins all on the C29 P3 gate.

### Track A — Task System (`pkg/task/`) — critical-path head

> **Path correction (2026-05-18):** workbook drafted `internal/task/`; the authoritative L2 spec §Go Package mandates **`pkg/task/`** (public, no-ECS, mirrors `pkg/math`). Implemented per spec.

- [x] **T-3A01** — Chase-Lev work-stealing deque + ComputePool/IOPool core. Files: `pkg/task/{deque,pool}.go`. Spec: [l2-task-system-go.md](../specifications/l2-task-system-go.md) §Type Definitions/§Performance Strategy. `[Bootstrap]`
  Verify: ✅ `go test -race ./pkg/task/...` ok (8×CPU steal-storm: every task consumed exactly once) + `BenchmarkDequePushPop` 61 ns/op **0 B/op 0 allocs/op** (C-004/C-027) + build/vet clean, stdlib-only (C-003).
  Changes: Chase-Lev deque with `[]atomic.Pointer[tcell]` slots + `sync.Pool` cell recycling (race-clean *and* 0-alloc steady-state); `ComputePool` fixed-N workers (INV-1) + per-worker deque + priority deques + random-victim stealing + parked-worker wake; `IOPool` semaphore-bounded elastic; `Shutdown(ctx)` drains then joins (INV-4); panic isolation per worker.
- [x] **T-3A02** — Scoped tasks: `RunScope`/`Scope.Spawn` (INV-2), `TaskHandle[T]` (Spawn/Poll/BlockOn/Detach/Err), `ForBatched` atomic-claim dispatcher, `ParChunkMap`/`ParSplatMap`. Files: `pkg/task/{scope,handle,dispatch}.go`. `[Bootstrap]`
  Verify: ✅ `go test -race ./pkg/task/...` ok 24.8s (TestRunScope join/borrow/**nested-no-deadlock**) + `TestForBatchedGoldenDeterminism` ×100 stable + `BenchmarkForBatched` **0 B/op 0 allocs/op** + deque bench regression 0-alloc.
  Changes: scope-owned work-stealing deque registered with the pool — RunScope join drains **only its own** children inline (livelock-free nested scopes; single-worker deadlock-free). `TaskHandle[T]` state machine (pending/running/finished/detached) + `PanicError` re-panic on `BlockOn`. `ForBatched` one-atomic-add-per-batch claim, runner count capped at batch count. **Bug found & fixed (run.md §3.5):** initial `tryRunOne` cooperative wait recursively absorbed unrelated blocking tasks → livelock under nested `RunScope` (caught by `TestRunScopeNestedNoDeadlock`, 100s timeout). Also hardened a latent T-3A01 hazard: external `submitPrio` now serialized (`submitMu`) — concurrent `pushBottom` violated the Chase-Lev single-owner contract.
- [ ] **T-3A03** — `MainThreadExecutor` (INV-3, `LockOSThread`/`PollMainThread`), `TaskPlugin` App wiring, cooperative `BlockOn` steal (§4.9). Files: `pkg/task/mainthread.go` + plugin registration.
  Verify: `go test -race ./...` green + `go list -deps ./pkg/task/...` resolves to stdlib only (C-003) + `go vet ./...` clean + `PollMainThread` from non-main goroutine panics (INV-3 test).

### Track B — Asset System (`pkg/asset/`) — consumes Track A pool

- [ ] **T-3B01** — `Handle[A]`, `Assets[A]` store, `AssetID`, generational invalidation. Files: `pkg/asset/{handle,store}.go`. Spec: [l2-asset-system-go.md](../specifications/l2-asset-system-go.md) §Handle/§Store.
  Verify: `go test -race -run TestHandle ./pkg/asset/...` (freed handle → `!ok` on re-resolve) + store package coverage ≥ 95%.
- [ ] **T-3B02** — `AssetLoader` registry + `fs.FS` VFS + async load path via `ComputePool`. Files: `pkg/asset/{loader,server,vfs}.go`. Requires: T-3A03.
  Verify: `go test -race ./pkg/asset/...` (concurrent load of same path dedups to a single loader invocation; load runs off the caller goroutine — asserted via goroutine-id capture).
- [ ] **T-3B03** — `ContentManager` refcount + poll-based stdlib dev watcher hot-reload (no fsnotify — C24). Files: `pkg/asset/{content,watch}.go`.
  Verify: `go test -race -run TestHotReload ./pkg/asset/...` (mutate fixture → handle observes new content within 1 poll interval; refcount → 0 evicts asset).

### Track C — Scene System (`internal/scene/`) — uses Phase 1 typereg (Done)

- [ ] **T-3C01** — `StaticScene` gob + interned binary codec. Files: `internal/scene/{static,codec}.go`. Spec: [l2-scene-system-go.md](../specifications/l2-scene-system-go.md) §StaticScene.
  Verify: `go test -race ./internal/scene/...` (gob round-trip == source) + interned codec output smaller than naive gob on the 1k-entity golden fixture (size assertion).
- [ ] **T-3C02** — `DynamicScene` reflection extract + `SceneSpawner`. Files: `internal/scene/{dynamic,spawner}.go`.
  Verify: `go test -run TestDynamicScene ./internal/scene/...` (extract→spawn yields a component-equal world via typereg; no per-type codegen — C24).
- [ ] **T-3C03** — Two-pass entity remap + hierarchy retention on spawn. Files: `internal/scene/remap.go`.
  Verify: `go test -race -run TestRemap ./internal/scene/...` (cross-referencing entities remap correctly; `ChildOf` links survive spawn into a populated world).

### Track D — Math System (`pkg/math/`) — fully independent

- [ ] **T-3D01** — `Vec2/3/4`, `Mat2/3/4`, `Quat` core algebra. Files: `pkg/math/{vec,mat,quat}.go`. Spec: [l2-math-system-go.md](../specifications/l2-math-system-go.md) §Linear.
  Verify: `go test ./pkg/math/...` (assoc/inverse/normalization identities within 1e-6 of the embedded reference table) + `go test -bench=. -benchmem ./pkg/math/` 0 allocs/op for value ops.
- [ ] **T-3D02** — `Affine3`, `Isometry`, `Dir` + geometric primitives (`Ray`/`Aabb`/`Sphere`/`Plane`). Files: `pkg/math/{affine,isometry,primitives}.go`.
  Verify: `go test -run TestPrimitives ./pkg/math/...` (ray-aabb / sphere-plane intersections vs hand-computed fixtures).
- [ ] **T-3D03** — `Color` (linear/sRGB/HSL), `Curves` (Bezier/Hermite), `TransformInterpolator`. Files: `pkg/math/{color,curve,interp}.go`.
  Verify: `go test -run TestColor ./pkg/math/...` (sRGB↔linear round-trip ≤ 1 ULP; sampled C1-continuity assertion on curves).
- [ ] **T-3D04** — `simd/archsimd` acceleration with pure-Go fallback parity. Files: `pkg/math/simd/{simd_amd64.go,simd_fallback.go}`.
  Verify: `go test -run TestSIMDParity ./pkg/math/simd/...` (SIMD == scalar bit-for-bit on a fuzz-seed corpus) + `GOARCH=amd64 go build ./...` and fallback `go build ./...` both succeed.

### Track T — Validation (C29 P3 gate — unblocks P3 Draft → Stable)

- [ ] **T-3T01** — `examples/async/` parallel-iter determinism demo (deterministic seed). Verify: `go run ./examples/async` output stable across 50 runs (golden diff); `-race` clean.
- [ ] **T-3T02** — `examples/asset/` hot-reload roundtrip demo + `examples/asset/README.md`. Verify: `go run ./examples/asset` reloads a mutated fixture and prints the new value.
- [ ] **T-3T03** — `examples/scene/` save/load fixture demo. Verify: `go run ./examples/scene` save→load→re-save is byte-stable.
- [ ] **T-3T04** — `examples/math/` correctness-vs-reference demo + bench summary. Verify: `go run ./examples/math` prints parity PASS; `go test -bench=. ./pkg/math/...` recorded in the example README.
- [ ] **T-3T05** — C29 P3 gate sign-off. Verify: all four `examples/{async,asset,scene,math}/` build & run green + `node .magic/scripts/executor.js check-prerequisites --json` ok; documents the Draft→Stable promotion trigger consumed by the next `/magic.task` Pre-Planning Stabilization.

## Notes

- L2 Go specs for task/asset/scene/math are **drafted** (2026-05-17): [l2-task-system-go.md](../specifications/l2-task-system-go.md), [l2-asset-system-go.md](../specifications/l2-asset-system-go.md), [l2-scene-system-go.md](../specifications/l2-scene-system-go.md), [l2-math-system-go.md](../specifications/l2-math-system-go.md). All `Draft [Bootstrap]` — C29 holds them until `examples/{async,asset,scene,math}/` validates. Implement against the L2 contracts, not directly from L1.
- Phase 3 is the **last** Bootstrap phase that runs without the C29 unblock. Phases 4+ require POC validation.
- Atomic decomposition complete (2026-05-18). Execute via `/magic.run main`. T-3T05 closing the C29 P3 gate makes P3 specs eligible for Draft → Stable in the next `/magic.task` Pre-Planning Stabilization.
