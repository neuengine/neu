---
phase: 4
name: "Render Pipeline"
status: Active
subsystem: "pkg/render, internal/render"
requires:
  - "Phase 1 ECS Core"
  - "Phase 2 App Framework Stable"
  - "Phase 3 Task/Asset/Math Stable (ComputePool, Handle[A], Mat4/Frustum)"
provides:
  - "Render graph + extract pattern + render world"
  - "Mesh / image / texture atlas primitives"
  - "Material system + PBR + lighting + shadows"
  - "Camera + visibility + frustum culling"
  - "Post-processing chain (AA, tonemapping, bloom)"
key_files:
  created: []
  modified: []
patterns_established: []
duration_minutes: ~
bootstrap: true
hold_reason: ""
---

# Stage 4 Tasks — Render Pipeline

**Phase:** 4
**Status:** Active
**Strategic Goal:** Backend-agnostic render graph with Bevy-style extract pattern + render-world isolation. First phase under the full C29 unblock — promotion requires validating `examples/{3d,camera,shader}/`.

## High-Level Checklist

- [ ] [T-4A] Render core: RID+command-queue server, RenderBackend, Kahn-DAG graph, SubApp extract, 4-phase schedule, RenderFeature, SoA. ([l1-render-core.md](../specifications/l1-render-core.md) → [l2-render-core-go.md](../specifications/l2-render-core-go.md))
- [ ] [T-4B] Mesh & image: attribute-map Mesh, layout hash, primitives, skinning/morph, image decode, atlases. ([l1-mesh-and-image.md](../specifications/l1-mesh-and-image.md) → [l2-mesh-and-image-go.md](../specifications/l2-mesh-and-image-go.md))
- [ ] [T-4C] Camera & visibility: Camera/projections, 3-layer visibility, frustum cull. ([l1-camera-and-visibility.md](../specifications/l1-camera-and-visibility.md) → [l2-camera-and-visibility-go.md](../specifications/l2-camera-and-visibility-go.md))
- [ ] [T-4D] Materials & lighting: PBR material, lights, IBL, shadows, clustering. ([l1-materials-and-lighting.md](../specifications/l1-materials-and-lighting.md) → [l2-materials-and-lighting-go.md](../specifications/l2-materials-and-lighting-go.md))
- [ ] [T-4E] Post-processing: canonical effect chain, tonemap boundary, AA, custom passes. ([l1-post-processing.md](../specifications/l1-post-processing.md) → [l2-post-processing-go.md](../specifications/l2-post-processing-go.md))
- [ ] [T-4T] Validation: `examples/{3d,camera,shader}/`, golden-image diff harness, render-world isolation race tests, backend-conformance suite. Gate: C29 — unblocks Phase 4 Draft → Stable.

## Atomic Decomposition

> Decomposed 2026-05-19 (all 3 Hold Release Conditions met — POC validated, App Framework Stable, L2 render specs authored 2026-05-18). 19 atomic tasks across Tracks A–E + T.
> **Execution Mode:** Parallel (C3). **Critical path:** A (render core) → {B mesh ‖ C camera} → D materials → E post-processing → T validation. Tracks B and C are file-independent and parallelizable once A03 (server) lands.
> **Bootstrap:** all 10 P4 specs `Draft [Bootstrap]` — C29 holds Stable promotion until `examples/{3d,camera,shader}/` validate (T-4T05).

### Track A — Render Core (`internal/render/`, `pkg/render/`) — critical-path head

- [x] **T-4A01** — RID + command-queue server, `RenderBackend` interface, two-phase resource create, `resourceTracker` (refcount + deferred-delete). Files: `pkg/render/backend.go`, `internal/render/{server,resources}.go`. Spec: [l2-render-core-go.md](../specifications/l2-render-core-go.md) §Server/§Type Definitions. `[Bootstrap]`
  Verify: ✅ `go test -race ./internal/render/...` ok 1.045s (TestServer_ConcurrentSubmitDrain: 256-goroutine Submit dedup, all run on bound Drain goroutine; TestServer_InlineFIFO; TestResourceTracker_DeferredDeleteOneFrameAfterRelease: freed frame 0 survives EndFrame(0), destroyed exactly once at EndFrame(1), idempotent at EndFrame(2)) + `go vet` clean + `go list -deps ./pkg/render` stdlib+self only (C-003).
  Changes: `pkg/render/backend.go` — `RID` (kind8|gen24|idx32 bit-pack, zero=nil), `ResourceKind`, `RenderBackend` 10-method interface, minimal descriptors. `internal/render/server.go` — `Server` (sync RID `Allocate`, deferred `Initialize`/`Submit`, goroutine-bound inline fast-path preserving global FIFO, `Drain`, `Close`→`ErrRenderClosed`). `internal/render/resources.go` — `ResourceTracker` (refcount; refcount→0 records freed-frame; `EndFrame(f)` destroys only `freed < f` → never in-flight, INV-3; Retain cancels pending). **Spec tightening (Bootstrap):** `Server.Submit` returns `error` (§Error Handling mandates `ErrRenderClosed`, "never silently drops"). **Defect caught pre-QA:** `go vet` flagged value-receiver lock copy on test backend → fixed to pointer receivers.
- [x] **T-4A02** — `RenderGraph` (Kahn topological sort reusing scheduler DAG pattern, barrier insertion, cycle → `ErrRenderGraphCycle`), `RenderPass` interface, pin/unpin of pass I/O RIDs (INV-3). Files: `internal/render/graph.go`, `pkg/render/phase.go`. `[Bootstrap]`
  Verify: ✅ `go test -race ./internal/render/... -run TestRenderGraph` 7/7 PASS (acyclic order + Execute walk; A↔B + self-cycle ⇒ ErrRenderGraphCycle naming passes; diamond barrier list == golden; never-retained & released external input ⇒ ErrResourceReleased; transient produced in-graph exempt + pinned; execute-before-build guard; deterministic across 5 rebuilds) + full `internal/render` `-race` suite green (no T-4A01 regression) + `go vet` clean + C-003 stdlib+self only.
  Changes: `pkg/render/phase.go` — `RenderPhase` public enum (None/Opaque/AlphaMask/Transparent/UI) + String. `internal/render/graph.go` — `RenderPass` interface (Name/Phase/Inputs/Outputs/Execute), `RenderGraph` (producer→consumer edges from shared RIDs; Kahn sorted-ready-frontier deterministic order; `ErrRenderGraphCycle`/`ErrResourceReleased`/`ErrRenderGraphNotBuilt` with `errors.Is`-compatible wrapper mirroring scheduler `dagCycleError`), `Barrier` transition list, INV-3 pin via in-package `*ResourceTracker` with external-vs-transient input distinction (transient produced in-graph is exempt + pinned, external zero-ref ⇒ released). **Spec reconciliation (Bootstrap):** `RenderPhase` placed in `pkg/render` (public — referenced by `pkg/render/material` cross-spec), graph stays `internal/render` per §Go Package; consistent because graph consumers (lighting/postpass) are internal subpackages.
- [x] **T-4A03** — `RenderSubApp` (own World+schedule, plug into App after main Update), `ExtractFn` registry, four-phase schedule (Collect/Extract/Prepare/Draw) with single-extract frame guard (INV-4). Files: `internal/render/{subapp,extract,phases}.go`. Requires: T-4A01. `[Bootstrap]`
  Verify: ✅ `go test -race ./internal/render/... -run 'TestExtractIsolation|TestFrameGuard|TestRunFrame'` PASS (isolation: post-frame main mutation invisible to render snapshot via slices.Clone; INV-4: exactly 1 extract/frame ×3, stage order Collect<Extract<Prepare<Draw, passes only StageDraw; graph cycle surfaces from RunFrame) + full `internal/render` `-race` green (no T-4A01/02 regression) + `go vet` clean + C-003 (stdlib + engine `world`/`pkg/render`).
  Changes: `internal/render/phases.go` — `Stage` enum (Collect/Extract/Prepare/Draw, distinct from `gpu.RenderPhase`). `internal/render/extract.go` — `ExtractFn func(main,render *world.World)` (mirrors `app.ExtractFn`, no pkg/app coupling) + ordered `extractRegistry`. `internal/render/subapp.go` — `RenderSubApp` owning isolated `world.World`+server+tracker+graph; `RunFrame` drives Collect→Extract(once, INV-4 `ErrExtractReentry` guard)→Prepare(drain placeholder, feature prep = T-4A04)→Draw(graph.Build+Execute, Submit/Present, tracker.EndFrame); lazy server.Bind on first frame; `Trace()`/`PassStage()` test introspection. **Scope note:** App-plugin wiring is forward integration (spec §Go Package scopes T-4A03 to internal/render; `RunFrame` signature is app.SubApp-adapter-compatible).
- [ ] **T-4A04** — `RenderFeature` + `RenderObject` proxy (NOT ECS entity), `VisibilityGroup` parallel frustum cull via `task.ForBatched`, SoA `RenderDataHolder` (Static/Dynamic keys, GPU-bindable slices). Files: `internal/render/{feature,visibility,renderdata}.go`. Requires: T-4A03. `[Bootstrap]`
  Verify: `go test -race ./internal/render/... -run 'TestVisibilityGroup|TestRenderDataSoA'` — 10k objects, parallel cull race-clean & identical to sequential reference; `[]Mat4` static-key slice contiguous; `BenchmarkFrustumCullSoA` 0 B/op 0 allocs/op (C-027). Track A complete.

### Track B — Mesh & Image (`pkg/render/mesh`, `pkg/render/image`) — consumes Track A server

- [ ] **T-4B01** — `Mesh`, `VertexAttribute`, `IndexBuffer`, `SubMesh`, `VertexLayout` FNV hash, `Validate()` (INV-1 position, INV-2 index-in-range, INV-3 skin length). Files: `pkg/render/mesh/{mesh,layout}.go`. Spec: [l2-mesh-and-image-go.md](../specifications/l2-mesh-and-image-go.md) §Type Definitions. `[Bootstrap]`
  Verify: `go test -race ./pkg/render/mesh/... -run 'TestMeshValidate|TestLayoutHash'` — missing-position/oob-index/skin-mismatch each return the sentinel; identical attribute set ⇒ identical layout hash across runs (determinism — pipeline-cache key).
- [ ] **T-4B02** — Primitive generators (Cube/Sphere/Plane/Cylinder/Capsule/Torus) + skinning/morph attribute components. Files: `pkg/render/mesh/{primitives,skin}.go`. Requires: T-4B01. `[Bootstrap]`
  Verify: `go test -race ./pkg/render/mesh/... -run 'TestPrimitive'` — each generator's vertex/index counts + AABB match golden fixture; normals unit-length ≤1e-6; `BenchmarkSphereGen` 0 allocs/op steady.
- [ ] **T-4B03** — `Image` (format/sampler/mip), stdlib PNG/JPEG decode on `IOPool` + placeholder-RID swap, `TextureAtlasLayout`/`TextureAtlas`/`DynamicAtlas` shelf-pack (INV-4 format, INV-5 non-overlap), staging upload via server. Files: `pkg/render/image/{image,atlas,loaders}.go`, `internal/render/upload/staging.go`. Requires: T-4A01. `[Bootstrap]`
  Verify: `go test -race ./pkg/render/image/... ./internal/render/upload/...` — invalid format ⇒ ErrImageFormatInvalid; overlapping atlas region ⇒ ErrAtlasOverlap; placeholder bound synchronously then swapped after IOPool completion (race-clean). Track B complete.

### Track C — Camera & Visibility (`pkg/render/camera`) — depends on A; parallel with B

- [ ] **T-4C01** — `Camera`/`RenderTarget`/`Viewport`/`ClearColorConfig`, `Perspective`/`Orthographic` `Matrix()` with near>0 guard (INV-5), `Camera2D`/`Camera3D` bundles, `Frustum` 6-plane extraction. Files: `pkg/render/camera/{camera,projection,bundles,frustum}.go`. Spec: [l2-camera-and-visibility-go.md](../specifications/l2-camera-and-visibility-go.md). Requires: T-4A01. `[Bootstrap]`
  Verify: `go test -race ./pkg/render/camera/... -run 'TestProjection|TestFrustumExtract'` — perspective near≤0 ⇒ ErrInvalidNearPlane; ortho zero-area ⇒ ErrDegenerateOrtho; frustum planes normalized; `BenchmarkProjMatrix` 0 allocs/op.
- [ ] **T-4C02** — 3-layer visibility (`Visibility`→`InheritedVisibility`→`ViewVisibility`) reusing Phase-2 hierarchy walk, `CameraUpdateSystems` set (PostUpdate, ordered), `task.ForBatched` frustum cull (INV-1,2,3,4). Files: `pkg/render/camera/visibility.go`, `internal/render/cameraupd/systems.go`. Requires: T-4C01, T-4A04. `[Bootstrap]`
  Verify: `go test -race ./pkg/render/camera/... ./internal/render/cameraupd/...` — parent Hidden ⇒ all descendants ViewVisibility false (DFS table); frustum no-false-negative property test; equal-Order cameras sort by EntityID (deterministic); 10k-entity parallel cull `-race` clean == sequential. Track C complete.

### Track D — Materials & Lighting (`pkg/render/material`, `pkg/render/light`) — joins B + C

- [ ] **T-4D01** — `Material`/`MaterialParameters`, `AlphaMode.Phase()` total map (INV-5), `StandardPBR` + idempotent `Sanitize()` clamp (INV-2 INV-1 shader), `SpecKey` → render-core pipeline cache. Files: `pkg/render/material/{material,pbr,speckey}.go`. Spec: [l2-materials-and-lighting-go.md](../specifications/l2-materials-and-lighting-go.md). Requires: T-4A02, T-4B01. `[Bootstrap]`
  Verify: `go test -race ./pkg/render/material/... -run 'TestAlphaPhase|TestSanitize|TestSpecKey'` — every AlphaMode maps to L1 phase; PhaseHint cannot cross opaque/transparent; Sanitize idempotent & clamps metallic/roughness/occlusion∈[0,1]; nil shader ⇒ ErrMaterialNoShader.
- [ ] **T-4D02** — Light components (Point/Spot/Directional/Ambient), IBL (`EnvironmentMapLight`/`IrradianceVolume`), `CascadeShadowConfig.Splits()` near→max coverage (INV-3). Files: `pkg/render/light/{light,ibl,shadow}.go`. Requires: T-4D01. `[Bootstrap]`
  Verify: `go test -race ./pkg/render/light/... -run 'TestCascadeCoverage|TestLight'` — config under-covering MaxDistance ⇒ ErrCascadeCoverage; logarithmic split sums verified; shadow map-count per light type matches L1 §4.6 table.
- [ ] **T-4D03** — Light clustering (`task.ForBatched` froxel binning) + shadow-pass render-graph construction (INV-4: graph edge shadow-pass Before lighting-pass). Files: `internal/render/lighting/{cluster,shadowpass}.go`. Requires: T-4D02, T-4A02. `[Bootstrap]`
  Verify: `go test -race ./internal/render/lighting/...` — light outside a tile contributes to zero froxels; every shadow-caster yields a shadow→lighting graph edge (INV-4); `-race` clean; `BenchmarkClusterLights` 0 allocs/op steady. Track D complete.

### Track E — Post-Processing (`pkg/render/postprocess`) — tail, joins D

- [ ] **T-4E01** — Canonical `EffectSlot` order table, per-effect pure-data settings, `Tonemapper`/`ColorGrading`, `BuildPostChain` omit-disabled + RID auto-chain (INV-1 tonemap-last, INV-2 deterministic order, INV-3 zero-cost disable). Files: `pkg/render/postprocess/{stack,settings,tonemap,colorgrade}.go`, `internal/render/postpass/builder.go`. Spec: [l2-post-processing-go.md](../specifications/l2-post-processing-go.md). Requires: T-4A02, T-4C01. `[Bootstrap]`
  Verify: `go test -race ./pkg/render/postprocess/... ./internal/render/postpass/...` — shuffled settings insertion ⇒ identical node order; color-altering node after tonemap ⇒ ErrPostOrder; disabling an effect removes exactly its node & reconnects RIDs (golden topology); Tonemapper.Apply matches reference curves ≤1e-4.
- [ ] **T-4E02** — Pooled ping-pong targets (HDR/LDR), `FullscreenMaterial` custom pass, AA mutual-exclusion (`ErrAAConflict`), render-world isolation (INV-4). Files: `pkg/render/postprocess/custom.go`, `internal/render/postpass/pingpong.go`. Requires: T-4E01. `[Bootstrap]`
  Verify: `go test -race ./internal/render/postpass/...` — every disallowed AA combination ⇒ ErrAAConflict (SMAA preferred); mid-frame settings mutation does not affect extracted snapshot; `BenchmarkBuildPostChain` 0 B/op steady (pooled targets). Track E complete.

### Track T — Validation (C29 Phase 4 gate — unblocks P4 Draft → Stable)

- [ ] **T-4T01** — `examples/3d/` — mesh + PBR material + lights scene; golden-image diff harness (deterministic software-backend frame hash). Files: `examples/3d/{main,main_test,go.mod}.go`. Requires: T-4A04, T-4B03, T-4D03. `[Bootstrap]`
  Verify: `go run ./examples/3d` prints PASS; `go test -race ./examples/3d/...` — rendered frame hash == golden within tolerance, stable across 20 runs.
- [ ] **T-4T02** — `examples/camera/` — multi-camera (split-screen + render-to-texture) + visibility/frustum-cull determinism demo. Files: `examples/camera/{main,main_test,go.mod}.go`. Requires: T-4C02. `[Bootstrap]`
  Verify: `go run ./examples/camera` PASS; `go test -race ./examples/camera/...` — visible-set deterministic across runs; ordered cameras composite by (Order,EntityID).
- [ ] **T-4T03** — `examples/shader/` — custom `FullscreenMaterial` + post-process stack (bloom→tonemap→FXAA) demo. Files: `examples/shader/{main,main_test,go.mod}.go`. Requires: T-4E02. `[Bootstrap]`
  Verify: `go run ./examples/shader` PASS; `go test -race ./examples/shader/...` — disabled effect produces byte-identical output to omitting it (zero-cost INV-3).
- [ ] **T-4T04** — Render-world isolation race test + backend-conformance suite (software rasteriser passes full `RenderBackend` contract, golden-image). Files: `internal/render/conformance_test.go`, `internal/render/isolation_test.go`. Requires: T-4A03. `[Bootstrap]`
  Verify: `go test -race ./internal/render/... -run 'TestBackendConformance|TestExtractIsolation'` — all backend methods exercised, golden scene matches; main-world mutation during Extract provably invisible to render world.
- [ ] **T-4T05** — C29 Phase 4 gate sign-off. All three examples build+run green; full workspace `go test -race ./...` clean; stdlib-only (C-003); benchmarks 0 allocs/op (C-027). Specs eligible Draft → Stable in next `/magic.task` Pre-Planning Stabilization. Requires: T-4T01..04, all A–E tracks.
  Verify: `go build ./examples/{3d,camera,shader}/...` clean; `go test -race ./...` 0 failures; `go list -deps ./pkg/render/... ./internal/render/...` stdlib-only; bench `-benchmem` shows 0 allocs/op on hot paths.

## Hold Release Conditions (all satisfied 2026-05-19)

1. ✅ `examples/ecs/poc/` deterministic + benchmarks within baseline (Phase 1 Done).
2. ✅ Phase 2 `l1/l2-app-framework` promoted to `Stable`.
3. ✅ L2 Go specs for render core authored — `l2-{render-core,mesh-and-image,materials-and-lighting,camera-and-visibility,post-processing}-go.md` (INDEX v2.28.0, 2026-05-18).

## Notes

- L2 Go specs for all 5 render subsystems are **drafted** (2026-05-18): [l2-render-core-go.md](../specifications/l2-render-core-go.md), [l2-mesh-and-image-go.md](../specifications/l2-mesh-and-image-go.md), [l2-materials-and-lighting-go.md](../specifications/l2-materials-and-lighting-go.md), [l2-camera-and-visibility-go.md](../specifications/l2-camera-and-visibility-go.md), [l2-post-processing-go.md](../specifications/l2-post-processing-go.md). All `Draft [Bootstrap]` — C29 holds them until `examples/{3d,camera,shader}/` validate. Implement against the L2 contracts, not directly from L1.
- Phase 4 is the **first** phase under the full C29 unblock (no Bootstrap shortcut for promotion — examples are mandatory).
- Critical path A → {B‖C} → D → E → T. Track A `render-core` is the foundation; mis-sizing it cascades to every other track (see PLAN.md Planning Audit). Execute via `/magic.run main`. T-4T05 closing the C29 P4 gate makes P4 specs eligible for Draft → Stable in the next `/magic.task` Pre-Planning Stabilization.
