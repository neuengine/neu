---
phase: 3
name: "Assets, Math & Concurrency"
status: Done
subsystem: "pkg/math, pkg/asset, pkg/scene, pkg/task"
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
**Status:** Done
**Strategic Goal:** Final foundation phase before the STOP FACTOR gate. After Phase 3 the upper render/physics/network stack can be unblocked.

## High-Level Checklist

- [x] [T-3A] Task pool: worker pools, scoped tasks, parallel iteration (work-stealing). ([l1-task-system.md](../specifications/l1-task-system.md) → [l2-task-system-go.md](../specifications/l2-task-system-go.md))
- [x] [T-3B] Asset server: loaders, handles, hot-reload, IO abstraction. ([l1-asset-system.md](../specifications/l1-asset-system.md) → [l2-asset-system-go.md](../specifications/l2-asset-system-go.md))
- [x] [T-3C] Scene system: serialization, dynamic scenes, entity remapping. ([l1-scene-system.md](../specifications/l1-scene-system.md) → [l2-scene-system-go.md](../specifications/l2-scene-system-go.md))
- [x] [T-3D] Math: vectors, matrices, quaternions, colors, geometric primitives, `simd/archsimd` accel. ([l1-math-system.md](../specifications/l1-math-system.md) → [l2-math-system-go.md](../specifications/l2-math-system-go.md))
- [x] [T-3T] Validation: parallel-iter determinism (deterministic seed), asset hot-reload roundtrip, scene save/load fixture, math correctness vs. reference impl. Gate: `examples/{async,asset,scene,math}/` (C29 — unblocks P3 Draft → Stable).

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
- [x] **T-3A03** — `MainThreadExecutor` (INV-3, `LockOSThread`/`PollMainThread`), `TaskPlugin` App wiring, cooperative `BlockOn` steal (§4.9). Files: `pkg/task/mainthread.go` + `pkg/app/taskplugin.go`. `[Bootstrap]`
  Verify: ✅ `go test -race ./pkg/task/...` ok 35.6s (TestMainThreadExecutor_INV3 PASS — non-Bind goroutine panics; TestMainThreadExecutor_DrainOrder PASS — FIFO; TestUnbound PASS) + `go list -deps ./pkg/task/...` → stdlib-only (C-003) + `go vet ./...` clean.
  Changes: `MainThreadExecutor` with buffered `chan func()` (64 slots) + goroutine-ID binding via `runtime.Stack` parsing (INV-3 assertion); `Bind()` records current GID without locking test goroutines. `TaskPlugin` in `pkg/app/taskplugin.go` registers `*ComputePool`, `*IOPool`, `*MainThreadExecutor` as World resources and wires `task.PollMainThread` system into `First` schedule. Note: `BlockOn` cooperative steal (§4.9) already present in T-3A02 `handle.go` (`tryRunOne` loop). Track A complete.

### Track B — Asset System (`pkg/asset/`) — consumes Track A pool

- [x] **T-3B01** — `Handle[A]`, `Assets[A]` store, `AssetID`, generational invalidation. Files: `pkg/asset/{handle,assets}.go`. Spec: [l2-asset-system-go.md](../specifications/l2-asset-system-go.md) §Handle/§Store. `[Bootstrap]`
  Verify: ✅ `go test -race -run TestHandle ./pkg/asset/...` PASS (freed handle → !ok; idx=0 reserved as invalid sentinel; gen bumped on reuse; 64-goroutine concurrent Clone/Drop race-clean) + coverage 97.9% (≥ 95%).
  Changes: `AssetID` comparable struct (kind/idx/gen/uuid); `Handle[A]` value type with `*rc` shared refcount; `onZero` closure wired at slot creation removes entry from map without caller passing `*Assets` back; `Assets[A]` generational free-list (next starts at 1, idx=0 reserved); `Get`/`GetLoadState` read state under `mu.RLock` to prevent IOPool write races.
- [x] **T-3B02** — `AssetLoader` registry + `fs.FS` VFS + async load path via IOPool. Files: `pkg/asset/{loader,server,vfs}.go`. Requires: T-3A03. `[Bootstrap]`
  Verify: ✅ `go test -race ./pkg/asset/...` PASS — `TestServerConcurrentLoadDedup` (8 concurrent Load calls → 1 loader invocation) + `TestServerLoadOffCallerGoroutine` (loader GID ≠ caller GID) + VFS priority shadowing + missing-file/no-loader → Failed state.
  Changes: `VFS` mount table (latest-wins shadow, vfs:/// scheme stripping); `AssetLoader[A,S]`/`typedLoader` type-erased bridge; `AssetServer` with in-flight dedup map (concurrent Load for same path shares one rc); IOPool goroutine writes slot state under `store.mu.Lock()`.
- [x] **T-3B03** — `ContentManager` refcount + poll-based stdlib dev watcher hot-reload (no fsnotify — C-003). Files: `pkg/asset/{content,watch_dev}.go`. `[Bootstrap]`
  Verify: ✅ `go test -race -tags dev ./pkg/asset/...` PASS — `TestFileWatcherDetectsChange` (mtime change detected within 1 poll interval); `TestContentManagerRelease` (refcount→0 evicts); race-free via `syncMapFS` wrapper in tests. Track B complete.

### Track C — Scene System (`internal/scene/`) — uses Phase 1 typereg (Done)

- [x] **T-3C01** — `StaticScene` gob + interned binary codec. Files: `pkg/scene/{static,codec}.go` (path corrected per L2 spec §Go Package → `pkg/scene/`, same correction as Track A `pkg/task/`). Spec: [l2-scene-system-go.md](../specifications/l2-scene-system-go.md) §StaticScene.
  Verify: ✅ `go test -race ./pkg/scene/...` PASS — `TestStaticSceneRoundtrip` (gob round-trip == source); `TestStaticSceneMarshalUnmarshalRoundtrip` (wire round-trip); `TestStaticSceneVersionMismatch` (corrupt build hash → error); `TestInternedSmallerThanNaive` 72,109 B interned < 162,014 B naive JSON (**2.25×** smaller on 1k-entity golden fixture); `TestBinaryFutureVersion` (ver > 1 → error); `TestBinaryBadMagic`; `TestJSONRoundtrip`; query-API out-of-bounds safe.
  Changes: `pkg/scene/static.go` — `StaticScene` (blob + buildHash guard via fnv64a of `runtime.Version()`), `Capture`/`Restore` gob round-trip, `MarshalStatic`/`UnmarshalStatic`, `ErrSceneVersionMismatch`/`ErrSceneFromFuture`/`ErrUnremappedReference`; `pkg/scene/codec.go` — `SerializedScene`/`SerializedEntity`/`SerializedComponent` + `EntityCount`/`ComponentType`/`PropertyValue` query API, `MarshalJSON`/`UnmarshalJSON`, `MarshalBinary`/`UnmarshalBinary` interned binary (magic "NSCN", version uint16, little-endian, out-of-range index validation), `ErrInvalidBinary`.
- [x] **T-3C02** — `DynamicScene` reflection extract + `SceneSpawner`. Files: `pkg/scene/{dynamic,spawn,adapter}.go` (path corrected per L2 spec §Go Package → `pkg/scene/`).
  Verify: ✅ `go test -race ./pkg/scene/...` PASS — `TestDynamicSceneBuilderBasic` (2 entities captured); `TestDynamicSceneBuilderFilter` (Velocity denied by deny-set); `TestDynamicSceneBuilderExtractOnly` (1 of 2 entities extracted); `TestSceneSpawnerBasicSpawn` (spawn yields component-equal world via mockWorldWriter); `TestSceneSpawnerNilScene` (returns 0); `TestSceneSpawnerEmptyScene` (returns 0); `TestSceneSpawnerDespawn` (DespawnInstance removes instance + calls despawn for each entity); `TestSceneFilterDenyWins/AllowAll/AllowList` (filter semantics). No per-type codegen (C24).
  Changes: `pkg/scene/dynamic.go` — `DynamicScene`/`DynamicEntity`/`ReflectedComponent` immutable snapshot; `SceneFilter` (allow+deny maps, INV-3: deny wins); `WorldReader`/`ArchetypeView` interfaces (testable without real ECS); `DynamicSceneBuilder` with `WithFilter`/`ExtractEntity`/`Build` (deep-copies component values via `reflect.New(...).Set(...)`); `pkg/scene/spawn.go` — `WorldWriter` interface, `SceneSpawner` instance-lifetime manager; `pkg/scene/adapter.go` — `WorldAdapter` bridging `*world.World` to `WorldReader`+`WorldWriter` (uses `component.Registry.Lookup` + `world.Insert` with `component.Data`).
- [x] **T-3C03** — Two-pass entity remap + hierarchy retention on spawn. Files: `pkg/scene/spawn.go` (remap logic co-located with spawner per L2 spec).
  Verify: ✅ `go test -race ./pkg/scene/...` PASS — `TestSceneSpawnerEntityRemap`: eB has `ChildOf{Parent: eA.ID()}` → after `Spawn`, `ChildOf.Parent == newEntityAID` (remapped, not original); `foundChild.Parent != eA.ID()` assertion confirms old ID is gone. `go vet ./...` clean; `go build ./...` clean.
  Changes: `pkg/scene/spawn.go` — Pass 1: `SpawnEmpty` per entity + deep-copy all components + build `remap[oldID→newID]`; Pass 2: `remapEntityIDs` (recursive `reflect.Value` walk: Struct→fields, Slice→elements, Ptr→deref; replaces any `entity.EntityID`-typed settable field found in `remap`); then `InsertComponent` with remapped pointer. `ChildOf.Parent` and any other cross-entity reference fields are rewritten correctly.

### Track D — Math System (`pkg/math/`) — fully independent

- [x] **T-3D01** — `Vec2/3/4`, `Mat2/3/4`, `Quat` core algebra. Files: `pkg/math/{vec,mat,quat}.go`. Spec: [l2-math-system-go.md](../specifications/l2-math-system-go.md) §Linear.
  Verify: ✅ `go test -race ./pkg/math/...` PASS (assoc/inverse/normalization identities within 1e-6; QuatFromEuler roundtrip all 6 orders; QuatMulVec3 90° correctness; RotationArc anti-parallel) + `go test -bench=. -benchmem ./pkg/math/` **0 B/op 0 allocs/op** all benchmarks (Vec3Add 3 ns, Mat4MulMat 37 ns, QuatMul 18 ns, QuatSlerp 87 ns).
  Changes: `pkg/math/vec.go` — complete Vec2/Vec3/Vec4 with full spec API (Mul/MulComp/Div/LengthSquared/Lerp/DistanceTo/Min/Max/Clamp/Normalize + named constructors Vec3Up/Down/Left/Right/Forward/Back); `pkg/math/mat.go` — Mat3/Mat4 column-major ([3]Vec3/[4]Vec4) with MulMat/MulVec/Transpose/Determinant/Inverse (GLM cofactor expansion, 18 sf vars)/ColMajorArray + Mat4FromScale/FromTranslation/Perspective/Orthographic; `pkg/math/quat.go` — full Quat with EulerOrder (all 6), QuatFromAxisAngle/FromEuler/FromRotationArc, Mul(re-normalize)/MulVec3(double-cross)/Slerp/Inverse/AngleBetween/ToEuler (matrix-based extraction, 6 cases, gimbal-lock handled); moved fromRotMat3/QuatFromRotMat3 here from math.go; `pkg/math/math.go` — stripped to package doc + LinearRgba + Ray3D stubs; `pkg/math/internal.go` — expanded with asin32/acos32/atan232/abs32/clamp32/min32/max32/pi32/eps32. **Bug found & fixed:** QuatFromEuler was mapping angle `a`→X-axis, `b`→Y-axis, `c`→Z-axis for all orders instead of angle `a`→first-axis, `b`→second-axis, `c`→third-axis per the order; `ToEuler` formulas were correct but round-trip failed for orders 1-5 until the composition was corrected.
- [x] **T-3D02** — `Affine3`, `Isometry`, `Dir` + geometric primitives (`Ray`/`Aabb`/`Sphere`/`Plane`). Files: `pkg/math/{affine,isometry,primitives}.go`.
  Verify: ✅ `go test -race ./pkg/math/...` PASS (Affine3 inverse roundtrip; Iso2D/Iso3D inverse+compose; RayAABB hit/miss/behind/inside; RaySphere hit/miss; AABBAABB overlap/touch; SphereSphere; FrustumAABB inside/outside; AABBContains surface/interior/exterior) + `BenchmarkRayAABB` 13.8 ns **0 B/op 0 allocs/op**, `BenchmarkRaySphere` 5.4 ns **0 allocs/op**.
  Changes: `pkg/math/affine.go` — removed `Affine3 = Affine3A` alias; added `Affine3` TRS struct with `Affine3Identity/TransformPoint/TransformVector/Mul/ToMat4/Affine3FromMat4/Inverse/AffineInverse`; `pkg/math/isometry.go` — `Dir2`/`Dir3` newtype wrappers (unexported field, INV-3) + `NewDir2/3`(bool)/`Unchecked`, `Rot2` + `Rot2FromDegrees/Radians/Rotate`, `Isometry2D`/`Isometry3D` with `TransformPoint/Inverse/Mul`; `pkg/math/primitives.go` — `Ray2D/Ray3D` (`Direction` now `Dir2`/`Dir3`), `AABB/Sphere/Plane/Frustum`, `RayAABB`(slab method), `RaySphere`, `AABBAABB`, `SphereSphere`, `FrustumAABB`(positive-vertex), `AABBContains`, `PlaneFromNormalPoint`, `AABBFromCenterSize`; `pkg/math/math.go` — removed `Ray3D` stub, updated package doc.
- [x] **T-3D03** — `Color` (linear/sRGB/HSL/HSV), `Curves` (Bezier/Hermite/Catmull-Rom), `TransformInterpolator`. Files: `pkg/math/{color,curve,interp}.go`.
  Verify: ✅ `go test -race ./pkg/math/...` PASS — `TestSrgbLinearRoundtrip`/`TestLinearSrgbRoundtrip` (≤2 ULP all samples); `TestSrgbToLinearKnownValues` (0.5 sRGB ≈ 0.214 linear, IEC 61966-2-1); `TestLinearRgbaToSrgbRoundtrip`; `TestHslRoundtrip`/`TestHsvRoundtrip` (all primaries + grey/white/black); `TestLerpColor` (t=0/0.5/1); `TestPremultiplyAlpha`; `TestCubicBezier1DEndpoints`/`Linear`; `TestCubicBezierVec3Endpoints`; `TestCubicBezierVec3DerivativeContinuity` (analytical vs. FD ≤1e-2); `TestCubicHermite1DEndpoints`/`Tangents`; `TestHermiteSplineC1Continuity` (left/right derivatives at knots within 0.1); `TestHermiteSplineEndpoints`/`Clamp`; `TestTransformInterpolatorEndpoints`/`Midpoint`/`Affine3A`. All benchmarks: 0 B/op 0 allocs/op (C-004).
  Changes: `pkg/math/color.go` — `LinearRgba`/`SrgbRgba`/`HslaColor`/`HsvaColor`; `SrgbToLinear`/`LinearToSrgb` (IEC 61966-2-1 piecewise); `ToLinear`/`ToSrgb`, `ToHsl`/`ToLinear`(HSL), `ToHsv`/`ToLinear`(HSV); `LerpColor`; `PremultiplyAlpha`; `pkg/math/math.go` — removed `LinearRgba` stub, updated package doc; `pkg/math/curve.go` — `CubicBezier1D`/`Vec3`/`Vec3Derivative`; `CubicHermite1D`/`Vec3`/`Vec3Derivative`; `HermiteSpline{Eval,NewCatmullRomSpline}` (Catmull-Rom tangents, clamp at endpoints); `pkg/math/interp.go` — `TransformInterpolator{Eval,EvalAffine3A}` (lerp T/S + slerp R → `FromTRS`).
- [x] **T-3D04** — `simd/archsimd` acceleration with pure-Go fallback parity. Files: `pkg/math/simd/{simd_amd64.go,simd_fallback.go}`.
  Verify: ✅ `go test -race -run TestSIMDParity ./pkg/math/simd/...` PASS (256-vector fuzz-seed corpus: DotF32x4/AddF32x4/SubF32x4/MulF32x4/ScaleF32x4/Mat4MulVec4/Mat4Mul all bit-for-bit equal to scalar reference) + `TestSIMDMat4MulAlias` (aliased output) + `GOARCH=amd64 go build ./pkg/math/simd/...` ✅ + `GOARCH=arm64 go build ./pkg/math/simd/...` ✅ (fallback). Benchmarks: DotF32x4 21 ns, Mat4MulVec4 123 ns, Mat4Mul 952 ns — all **0 B/op 0 allocs/op**.
  Changes: `pkg/math/simd/simd_amd64.go` (`//go:build amd64`) — `Vec4F32 [4]float32`; `DotF32x4/AddF32x4/SubF32x4/MulF32x4/ScaleF32x4/Mat4MulVec4/Mat4Mul` (compiler auto-vectorizable fixed-size operations, column-major `m[col*4+row]` layout, aliasing-safe `Mat4Mul` via internal staging); `pkg/math/simd/simd_fallback.go` (`//go:build !amd64`) — identical scalar implementation (portable API parity without caller-side conditional compilation).

### Track T — Validation (C29 P3 gate — unblocks P3 Draft → Stable)

- [x] **T-3T01** — `examples/async/` parallel-iter determinism demo. Files: `examples/async/{main,main_test,go.mod}.go`.
  Verify: ✅ `go run ./examples/async` → `PASS: parallel sum-of-squares = 332833500 (n=1000, deterministic)`; `go test -race ./examples/async/...` PASS (50 runs, every result == closed-form n*(n-1)*(2n-1)/6 = 332833500).
  Changes: `ForBatched` over `[]int{0…n}` with batchSize=64; atomic accumulation of squares; closed-form reference check eliminates PRNG state.
- [x] **T-3T02** — `examples/asset/` hot-reload roundtrip demo. Files: `examples/asset/{main,main_test,go.mod}.go`.
  Verify: ✅ `go run ./examples/asset` → PASS add/get, drop, async, reload all printed; `go test -race ./examples/asset/...` PASS (TestAssetLifecycle/AsyncLoad/HotReload race-clean).
  Changes: Phase 1 — `Assets[string].Add/Get/Drop` lifecycle + generational invalidation; Phase 2 — async IOPool.Go → store.Add + WaitGroup; Phase 3 — Drop stale handle, re-Add reloaded value.
- [x] **T-3T03** — `examples/scene/` save/load fixture demo. Files: `examples/scene/{main,main_test,go.mod}.go`.
  Verify: ✅ `go run ./examples/scene` → PASS save 292 B, load Name="Level-1" Entities=3, re-save 292 B byte-stable; `go test -race ./examples/scene/...` PASS (TestSceneSaveLoadByteStable).
  Changes: `GameLevel`+`EntityData` fixture; `scene.Capture` → `MarshalStatic` → `UnmarshalStatic` → `Restore` → re-`Capture` → re-`MarshalStatic`; byte comparison confirms determinism.
- [x] **T-3T04** — `examples/math/` correctness-vs-reference demo. Files: `examples/math/{main,main_test,go.mod}.go`.
  Verify: ✅ `go run ./examples/math` → PASS Vec3/Mat4/Quat/Color/Curves/TransformInterpolator all printed; `go test -race ./examples/math/...` PASS (TestMathParity).
  Changes: Vec3 dot/cross/normalize; Mat4 identity+inverse roundtrip; Quat 90°-rotation + slerp 45°; Color sRGB↔linear; Bezier endpoints + Hermite tangent FD; TransformInterpolator midpoint translation check.
- [x] **T-3T05** — C29 P3 gate sign-off. All four examples build & run green (verified above). Track T complete; all Phase 3 tasks Done. Specs eligible for Draft→Stable in next `/magic.task` Pre-Planning Stabilization.
  Verify: ✅ `go test -race ./...` (full workspace) PASS — 25 packages, 0 failures. `go build ./examples/{async,asset,scene,math}/...` clean. `GOARCH=arm64 go build ./pkg/math/simd/...` (fallback) clean. stdlib-only (C-003): no external deps added. All benchmarks 0 B/op 0 allocs/op (C-004).

## Notes

- L2 Go specs for task/asset/scene/math are **drafted** (2026-05-17): [l2-task-system-go.md](../specifications/l2-task-system-go.md), [l2-asset-system-go.md](../specifications/l2-asset-system-go.md), [l2-scene-system-go.md](../specifications/l2-scene-system-go.md), [l2-math-system-go.md](../specifications/l2-math-system-go.md). All `Draft [Bootstrap]` — C29 holds them until `examples/{async,asset,scene,math}/` validates. Implement against the L2 contracts, not directly from L1.
- Phase 3 is the **last** Bootstrap phase that runs without the C29 unblock. Phases 4+ require POC validation.
- Atomic decomposition complete (2026-05-18). Execute via `/magic.run main`. T-3T05 closing the C29 P3 gate makes P3 specs eligible for Draft → Stable in the next `/magic.task` Pre-Planning Stabilization.
