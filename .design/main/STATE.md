# Project State

<!-- STATE.md — live project memory. Read FIRST in every workflow session. -->
<!-- Maximum 100 lines. Agent updates AFTER each completed action. -->

**Workspace:** main
**Updated:** 2026-05-28
**Phase:** 4 — Render Pipeline
**Status:** Active

## Current Position

- **Task:** T-4D01 Done — `Material`/`MaterialParameters`/`AlphaMode.Phase()`/`StandardPBR`/`SpecKey` (2026-05-28).
- **Next Action:** T-4D02 (Track D): Light components (Point/Spot/Directional/Ambient/IBL) + `CascadeShadowConfig.Splits()` — depends on T-4D01 ✓

## Progress

```
Phase 1: [27/27] ████████ 100% ✓ Done
Phase 2: [24/24] ████████ 100% ✓ Done
Phase 3: [18/18] ████████ 100% ✓ Done
Phase 4: [10/19] █████░░░  53% ▶ Active  (Tracks A ✓ B ✓ C ✓ D01 ✓)
Overall: [79/88] ███████░  90%
```

## Recent Decisions

<!-- Last 3-5 locked decisions. Older entries → archived to PLAN.md -->

- 2026-05-28 **Done: T-4D01** — `pkg/render/material/{material,pbr,speckey}.go`: `AlphaMode` (5 variants) + `Phase()` total switch (INV-5); `MaterialParameters.Sanitize()` (idempotent clamp: metallic/roughness/occlusion→[0,1], all color components→≥0, slog.Debug on change); `Material` + `Validate()` (INV-1 ErrMaterialNoShader); `resolvePhase()` (PhaseHint boundary guard — isOpaqueSide rejects cross-bucket hints silently); `StandardPBR()` (white/dielectric/0.5rough); `SetFloat/SetColor/SetTexture` chaining setters; `SpecializationKey` + `SpecKey(VertexLayout)` → pipeline cache bridge; `assetIDHash` (fmt+FNV-1a; zero-ID→0, 0-alloc fast path). 13/13 tests PASS; `BenchmarkSpecKey` 0 B/op 0 allocs/op; go vet + modernize clean; C-003 stdlib+engine. Next: T-4D02 (Light components + IBL + shadow config).
- 2026-05-28 **Done: Tracks B + C** (5 tasks) — **Track B:** `pkg/render/mesh/{mesh,layout,primitives,skin}.go` + `pkg/render/image/{image,atlas,loaders}.go` + `internal/render/upload/staging.go`; Mesh INV-1/2/3 validated, FNV-1a VertexLayout deterministic hash, 6 primitives (Cube/Sphere/Plane/Cylinder/Capsule/Torus), shelf-pack DynamicAtlas INV-5, PNG/JPEG decode, C-027 StagingPool. **Track C:** `pkg/render/camera/{camera,projection,visibility,frustum,bundles}.go` + `internal/render/cameraupd/systems.go`; perspective/ortho Matrix() with ErrInvalidNearPlane/ErrDegenerateOrtho guards, FrustumFromViewProj (Gribb–Hartmann, inward-normal, normalised), 3-layer visibility (Visibility→InheritedVisibility→ViewVisibility), buildChildrenMap-based DFS propagation, ForBatched disjoint-index cull (10k parallel≡sequential), SortedActiveCameras (Order,EntityID). All tests PASS; `go vet` + modernize clean; C-003 stdlib+engine. D/E/T unblocked.
- 2026-05-19 **Done:** T-4A04 — `internal/render/{renderdata,feature,visibility}.go` + subapp feature dispatch. **Track A complete.** SoA `RenderDataHolder` (type-erased columns, `RegisterStatic/DynamicKey[T]`, `Slice()` aliases storage → GPU-bindable). `RenderFeature` 7-hook interface + contexts. `VisibilityGroup`: Gribb–Hartmann `buildFrustum`, frame-skip, **lock-free disjoint-index parallel cull** (`visible[DataIndex]`) + ordered compaction → parallel≡sequential (10k, `-race`). **QA-caught:** sequential bench 165B/1alloc — shared `test` closure captured `f` & escaped via conditional `ForBatched` arg; fixed by non-closure `cullBatch` method → SoA kernel **0 B/op 0 allocs/op** (C-027). **Bootstrap reconciliation:** `BenchmarkFrustumCullSoA` pins the SoA kernel (C-027 unit); parallel path's O(workers) allocs are `task.ForBatched`'s documented per-call cost (correctness proven by parallel≡seq test). Next: T-4B01 (Track B head).
- 2026-05-19 **Done:** T-4A03 — `internal/render/{phases,extract,subapp}.go`. `Stage` enum (Collect/Extract/Prepare/Draw — distinct from `gpu.RenderPhase`). `ExtractFn func(main,render *world.World)` mirrors `app.ExtractFn` (no pkg/app coupling) + ordered `extractRegistry`. `RenderSubApp` owns isolated `world.World`+server+tracker+graph; `RunFrame` = Collect→Extract(once, `ErrExtractReentry` INV-4 guard)→Prepare(drain placeholder, feature prep = T-4A04)→Draw(graph Build+Execute, Submit/Present, tracker.EndFrame); lazy server.Bind frame 1. `-race` clean (isolation via slices.Clone proven; 3-frame guard; cycle surfaces). **Pattern:** render world is a per-frame copy-not-share snapshot; App-plugin wiring deferred (RunFrame is app.SubApp-adapter-compatible). Next: T-4A04 (Track A tail).
- 2026-05-19 **Done:** T-4A02 — `internal/render/graph.go` + `pkg/render/phase.go`. `RenderGraph`: producer→consumer edges from shared RIDs, Kahn topo sort (sorted ready frontier → deterministic) reusing scheduler DAG pattern (C30); `ErrRenderGraphCycle` (errors.Is-compatible wrapper naming passes, mirrors `dagCycleError`), self-cycle rejected; `Barrier` transition list (golden-tested on diamond); INV-3 pin via in-package `*ResourceTracker` with external-vs-transient input distinction (transient produced in-graph exempt+pinned, external zero-ref ⇒ `ErrResourceReleased`); `ErrRenderGraphNotBuilt` guard. `RenderPhase` public enum. `-race` clean 7/7 + no T-4A01 regression. **Pattern:** `RenderPhase`→`pkg/render` (public, cross-spec); graph stays `internal/render` (consumers lighting/postpass are internal). Next: T-4A03.
- 2026-05-19 **Done:** T-4A01 — `pkg/render/backend.go` + `internal/render/{server,resources}.go`. `RID` = kind8|gen24|idx32 bit-pack (zero = nil). `Server`: sync `Allocate`, deferred `Initialize`/`Submit` with goroutine-bound inline fast-path (drains queue before inline cmd → global FIFO), `Drain` sole consumer, `Close`→`ErrRenderClosed`. `ResourceTracker`: refcount→0 records freed-frame; `EndFrame(f)` destroys only `freed < f` (never in-flight — INV-3); `Retain` cancels pending. `-race` clean (256-goroutine Submit dedup; deferred-delete timing). **Pattern:** internal/render imports pkg/render aliased `gpu` (RIDs caller-held → public). **Spec tightening (Bootstrap):** `Submit` returns `error` per §Error Handling. Track A root done — unblocks A02/A03/A04 then B/C/D/E.

<!-- Phase 1–3 decisions archived (Done; see PLAN.md Document History + archives/tasks/). -->
- 2026-04-30 **Pattern (carried):** ECS hot paths use `sync.Pool` with a `*SliceBuf[T]` wrapper for 0-alloc slice reuse; deferred mutations apply after all systems run, before tick boundary (single sync point). Reused by render: Server FIFO drain, ResourceTracker deferred-delete.

## Blockers

<!-- Empty if none. Format: [severity] description -->

<!-- (none) -->

## Blocking Constraints

<!-- Anti-patterns discovered through real failures. MANDATORY reading. -->
<!-- Agent MUST explicitly acknowledge each constraint before working. -->

- [C-001] **C29 Promotion Gate**: No P1 spec may be promoted Draft → Stable until `examples/ecs/poc/` validates the runtime end-to-end (T-1T05).
- [C-002] **STOP FACTOR (Phase ≥ 4 Hold)**: Phases 4–8 stay in `Hold` until Phase 1 POC is validated AND Phase 2 App Framework reaches `Stable`. No premature implementation work in those subsystems.
- [C-003] **C24 Stdlib Priority**: Engine core MUST have zero external Go deps. Any third-party package requires explicit justification recorded in an ADR.
- [C-004] **C27 GC Compensation**: Hot-path allocations (commands, events, transient views) MUST flow through `sync.Pool`. Validation Track verifies ≤1 alloc/op for `BenchmarkCommandFlush`.
- [C-005] **C28 Race Gate**: All concurrent tests MUST pass with `-race`; CI blocks otherwise.

## Session Continuity

**Last Session Ended:** 2026-04-25 13:27
**Handoff File:** none
**Bootstrap Mode:** true
