# Project State

<!-- STATE.md — live project memory. Read FIRST in every workflow session. -->
<!-- Maximum 100 lines. Agent updates AFTER each completed action. -->

**Workspace:** main
**Updated:** 2026-05-28 16:42
**Phase:** 5 — Content Systems
**Status:** Ready

## Current Position

- **Task:** `/magic.spec` ratification Done — **render-core RFC → Stable** (+ L2). **Phase 4 = 10/10 specs Stable**, RFC count → 0. **Phase 5 gate cleared** (Hold → Ready); 18 tasks decomposed, Track C (2D) now unblocked.
- **Next Action:** Author L2 Go contracts for the 5 P5 specs via `/magic.spec` (parity with P1–P4), then activate Phase 5 via `/magic.task` and execute `/magic.run main`. Critical path {A‖D} → B → T.

## Progress

```
Phase 1: [27/27] ████████ 100% ✓ Done
Phase 2: [24/24] ████████ 100% ✓ Done
Phase 3: [18/18] ████████ 100% ✓ Done
Phase 4: [19/19] ████████ 100% ✓ Done  ← all 10 specs Stable (render-core ratified)
Phase 5: [0/18]  ░░░░░░░░   0%   Ready ← decomposed; gate cleared, awaiting activation
Overall: [88/106] ██████░░  83% (phases 1–5 decomposed; Phase 6 = 46 tasks, far-future Hold)
```

## Recent Decisions

<!-- Last 3-5 locked decisions. Older entries → archived to PLAN.md -->

- 2026-05-28 **`/magic.spec` — Render Core ratified Stable** — `l1-render-core` **RFC → Stable** + `l2-render-core-go` **Draft → Stable**, completing the ratification deferred in the prior `/magic.task`. Justification: Phase 4 complete + C29 gate (T-4T05); L1 Canonical Refs already filled + Q4/Q5 resolved; Q1–Q3 annotated non-blocking (forward-looking refinements — ref-counting tracker chosen per §4.6 and validated 0-alloc). L2 Canonical Refs populated with 11 source + 2 test files (all verified on disk via glob — Path Validity check). **Phase 4 = 10/10 Stable, RFC count → 0.** Phase 5 gate ("Render Core Stable") cleared → `Hold` → `Ready`, Track C (2D) unblocked. INDEX v2.31.0 (Stable 50/96), PLAN/TASKS v1.13.0. Post-Update Review (Critic): invariants intact, no impl code in spec (Go contracts permitted), links accurate. **Remaining recommended step:** author L2 Go contracts for the 5 P5 specs (still L1-only).
- 2026-05-28 **`/magic.task` — P4 Stabilization + P5 Decomposition** — Pre-Planning Stabilization promoted **8 P4 render specs Draft → Stable** (l1/l2 × mesh-and-image, materials-and-lighting, camera-and-visibility, post-processing) — C29 P4 gate closed by T-4T05. **Phase 4 → Done** (19/19). **User decision:** `l1-render-core` kept **RFC** + `l2-render-core-go` Draft (layer-blocked) — RFC→Stable ratification deferred to `/magic.spec`. This is a *promotion-quarantine* only — tasks T-4A01..04 are Done (C12.1 exception, nothing moved to Backlog). **Phase 5 full atomic decomposition** (18 tasks → `tasks/phase-5.md`): Track A:3 Audio / B:3 Asset Formats / C:2 2D / D:3 Animation / E:2 Tweening / T:5 Validation. Critical path **{A‖D} → B → T** (glTF/audio loaders consume AnimationClip + AudioSource types); Track E independent; **Track C externally gated** on render-core Stable. Skeptic flags: Animation (D) under-sizing risk; P5 specs are L1-only (L2 Go contracts pending /magic.spec); phase `Hold` condition over-broad (only Track C truly needs render-core). INDEX v2.30.0 (Stable 48/96), PLAN/TASKS v1.12.0.
- 2026-05-28 **PHASE 4 COMPLETE — T-4T01..05 Done** — Track T validation: `examples/3d/` (Cube+PBR+DirectionalLight+2-cascade shadow, frame hash stable 20 runs), `examples/camera/` (3-camera Order-sort determinism 20 runs), `examples/shader/` (post-process Bloom→Tonemap→FXAA, AA conflict detection, INV-3 golden topology). `internal/render/conformance_test.go` (recordingBackend, all 10 RenderBackend methods). `internal/render/isolation_test.go` (PostProcessStack value-copy isolation, slice-backing isolation, multi-frame isolation). T-4T05 C29 gate: 36/36 pkgs PASS; `go build ./examples/{3d,camera,shader}/...` OK; C-003 stdlib-only; BenchmarkFrustumCullSoA/ClusterLights/BuildPostChain/SpecKey all 0 B/0 allocs. P4 specs eligible Draft→Stable via next /magic.task.
- 2026-05-28 **Done: T-4E02 — Track E complete** — `pkg/render/postprocess/custom.go`: FullscreenMaterial{Shader Handle[material.Shader], InputTex []RID, Params map[string]ShaderValue, InsertAfter}. `internal/render/postpass/pingpong.go`: PingPongPool (2 HDR + 2 LDR pre-alloc RIDs, Reset=index-rewind C-027, 0-alloc), CheckAAConflict (FXAA+SMAA→ErrAAConflict, SMAA preferred). INV-4 proven by PostProcessStack value semantics. BenchmarkBuildPostChain 0 B/op 0 allocs/op. 24/24 PASS.
- 2026-05-28 **Done: T-4E01** — `pkg/render/postprocess/{stack,settings,tonemap,colorgrade}.go` + `internal/render/postpass/builder.go`: EffectSlot 10-slot iota enum (index=order, INV-2), IsHDR(), PostProcessStack Enable/Disable/EnabledSlots (canonical); per-effect settings structs; Tonemapper 5-op Apply() (Reinhard/ReinhardLuminance/ACES-Narkowicz/AgX-smoothstep/TonyMcMapface-fallback); ColorGrading; BuildPostChain([]EffectSlot→slices.Sort→validateOrder INV-1→postPass chain INV-3); ErrPostOrder/ErrAAConflict sentinels. 16/16 PASS; reference values ≤5e-4.
- 2026-05-28 **Done: T-4D03 — Track D complete** — `internal/render/lighting/{cluster,shadowpass}.go`: LightRef{Sphere,ShadowRID,Kind}, Froxel (pre-computed TileX/Y/Z), ClusterGrid + Reset() (C-027), ClusterLights (nil→sequential 0-alloc, pool→ForBatched disjoint-write); tileOverlapsLight (VP→clip→NDC projection, 2D circle-vs-AABB, directional fast-path); ShadowCaster, shadowMapPass (Outputs=[shadowRID]), lightingPass (Inputs=all shadow RIDs), BuildShadowPasses (graph edges → topo-sort enforces INV-4). BenchmarkClusterLights 0 B/op 0 allocs/op steady; parallel≡sequential; right-handed +Z behind-camera test. 7/7 PASS.
- 2026-05-28 **Done: T-4D02** — `pkg/render/light/{light,ibl,shadow}.go`: PointLight/SpotLight/DirectionalLight/AmbientLight (pure data, optional Shadow pointers); CubeShadow.FaceCount()=6, SingleShadow.MapCount()=1 (L1 §4.6 table); CascadeShadowConfig.Splits() — log: `near×(max/near)^(i/Count)` (INV-3 by construction), manual: INV-3 coverage guard → ErrCascadeCoverage, Count clamped 1..4; EnvironmentMapLight/IrradianceVolume (SH probes). Bootstrap: ManualSplits field (L2 spec `Splits` field renamed to avoid method/field name conflict). 14/14 tests PASS; go vet + modernize clean; C-003.
- 2026-05-28 **Done: T-4D01** — `pkg/render/material/{material,pbr,speckey}.go`: `AlphaMode` (5 variants) + `Phase()` total switch (INV-5); `MaterialParameters.Sanitize()` (idempotent clamp: metallic/roughness/occlusion→[0,1], all color components→≥0, slog.Debug on change); `Material` + `Validate()` (INV-1 ErrMaterialNoShader); `resolvePhase()` (PhaseHint boundary guard — isOpaqueSide rejects cross-bucket hints silently); `StandardPBR()` (white/dielectric/0.5rough); `SetFloat/SetColor/SetTexture` chaining setters; `SpecializationKey` + `SpecKey(VertexLayout)` → pipeline cache bridge; `assetIDHash` (fmt+FNV-1a; zero-ID→0, 0-alloc fast path). 13/13 tests PASS; `BenchmarkSpecKey` 0 B/op 0 allocs/op; go vet + modernize clean; C-003 stdlib+engine. Next: T-4D02 (Light components + IBL + shadow config).
- 2026-05-28 **Done: Tracks B + C** (5 tasks) — **Track B:** `pkg/render/mesh/{mesh,layout,primitives,skin}.go` + `pkg/render/image/{image,atlas,loaders}.go` + `internal/render/upload/staging.go`; Mesh INV-1/2/3 validated, FNV-1a VertexLayout deterministic hash, 6 primitives (Cube/Sphere/Plane/Cylinder/Capsule/Torus), shelf-pack DynamicAtlas INV-5, PNG/JPEG decode, C-027 StagingPool. **Track C:** `pkg/render/camera/{camera,projection,visibility,frustum,bundles}.go` + `internal/render/cameraupd/systems.go`; perspective/ortho Matrix() with ErrInvalidNearPlane/ErrDegenerateOrtho guards, FrustumFromViewProj (Gribb–Hartmann, inward-normal, normalised), 3-layer visibility (Visibility→InheritedVisibility→ViewVisibility), buildChildrenMap-based DFS propagation, ForBatched disjoint-index cull (10k parallel≡sequential), SortedActiveCameras (Order,EntityID). All tests PASS; `go vet` + modernize clean; C-003 stdlib+engine. D/E/T unblocked.
<!-- Phase 4 Track A (T-4A01..04, 2026-05-19) decisions archived → PLAN.md Document History v1.12.0 (Phase 4 Done). -->
<!-- Phase 1–3 decisions archived (Done; see PLAN.md Document History + archives/tasks/). -->
- 2026-04-30 **Pattern (carried):** ECS hot paths use `sync.Pool` with a `*SliceBuf[T]` wrapper for 0-alloc slice reuse; deferred mutations apply after all systems run, before tick boundary (single sync point). Reused by render: Server FIFO drain, ResourceTracker deferred-delete.

## Blockers

<!-- Empty if none. Format: [severity] description -->

<!-- [resolved 2026-05-28] render-core RFC blocker — ratified RFC → Stable via /magic.spec; Phase 5 gate cleared. -->
- [low] **Phase 5 specs are L1-only.** No `l2-*-go.md` contracts for audio/asset-formats/2d/animation/tweening. Recommended `/magic.spec` step before `/magic.run` so tasks implement against a Go-level contract (parity with Phases 1–4).

## Blocking Constraints

<!-- Anti-patterns discovered through real failures. MANDATORY reading. -->
<!-- Agent MUST explicitly acknowledge each constraint before working. -->

- [C-001] **C29 Promotion Gate**: No P1 spec may be promoted Draft → Stable until `examples/ecs/poc/` validates the runtime end-to-end (T-1T05).
- [C-002] **STOP FACTOR (Phase ≥ 4 Hold)**: Phases 4–8 stay in `Hold` until Phase 1 POC is validated AND Phase 2 App Framework reaches `Stable`. No premature implementation work in those subsystems.
- [C-003] **C24 Stdlib Priority**: Engine core MUST have zero external Go deps. Any third-party package requires explicit justification recorded in an ADR.
- [C-004] **C27 GC Compensation**: Hot-path allocations (commands, events, transient views) MUST flow through `sync.Pool`. Validation Track verifies ≤1 alloc/op for `BenchmarkCommandFlush`.
- [C-005] **C28 Race Gate**: All concurrent tests MUST pass with `-race`; CI blocks otherwise.

## Session Continuity

**Last Session Ended:** 2026-05-28 16:42
**Handoff File:** none
**Bootstrap Mode:** true (P5+ remain `[Bootstrap]`; P1–P4 fully deactivated — all 10 P4 specs Stable)
