# Implementation Plan — Neu Engine

**Version:** 1.12.0
**Generated:** 2026-05-28
**Based on:** .design/main/INDEX.md v2.30.0
**Based on RULES:** .design/RULES.md v1.8.0
**Status:** Active
**Mode:** Specs `Stable` 48/96. **Phase 4 Done** (19/19 tasks) — C29 P4 gate closed by T-4T05; **8 render specs promoted Draft → Stable** (mesh-and-image, materials-and-lighting, camera-and-visibility, post-processing — L1+L2). **Quarantined:** `l1-render-core` stays **RFC** + `l2-render-core-go` Draft (layer-blocked) — RFC→Stable ratification deferred to `/magic.spec` (user choice 2026-05-28). **Phase 5 Hold** — full atomic decomposition complete (18 tasks, Tracks A–E + T; critical path {A‖D} → B → T; Track C externally gated on render-core Stable). Phase 5 unfreezes when render-core ratifies.

## Overview

Force-Bootstrap regeneration of the implementation plan. Every registered specification (89 total) is mapped to its target phase, ordered by the P1–P8 priority batches in `INDEX.md` and gated by:

- **STOP FACTOR**: phases ≥ 4 are frozen (`Hold`) until Phase 1 (POC) is validated by code in `examples/ecs/poc/` (C29).
- **Layer Order**: every L1 concept spec is scheduled before its L2 Go implementation within the same phase.
- **C29 Resolved (Phase 1)**: P1 ECS Core specs promoted `Draft → Stable` on 2026-05-17 via Pre-Planning Stabilization (T-1T05 satisfied the gate). Phase 2+ promotion deferred to `examples/ecs/framework/` completion.

Dependency analysis (Implements: chains):

- 19 hard L2→L1 edges, all 1:1, no chains, no cycles (5 new L2→L1 edges added 2026-05-17: math, multi-repo, task, asset, scene).
- 65 L1 specs are roots. `Related Specifications` cycles within a single phase are non-blocking (Circular Guard Semantic Split — Soft).

## Phase 1 — ECS Core POC (Done) `[Bootstrap]`

*Foundation runtime: world, entities, components, queries, scheduler. Outcome: a runnable POC in `examples/ecs/poc/` that exercises the full data path and unblocks C29. **Complete — 27/27 atomic tasks Done.***

- [x] **World System** ([l1-world-system.md](specifications/l1-world-system.md)) [L1] `Stable`
- [x] **World System (Go)** ([l2-world-system-go.md](specifications/l2-world-system-go.md)) [L2] `Stable`
- [x] **Entity System** ([l1-entity-system.md](specifications/l1-entity-system.md)) [L1] `Stable`
- [x] **Entity System (Go)** ([l2-entity-system-go.md](specifications/l2-entity-system-go.md)) [L2] `Stable`
- [x] **Component System** ([l1-component-system.md](specifications/l1-component-system.md)) [L1] `Stable`
- [x] **Component System (Go)** ([l2-component-system-go.md](specifications/l2-component-system-go.md)) [L2] `Stable`
- [x] **Query System** ([l1-query-system.md](specifications/l1-query-system.md)) [L1] `Stable`
- [x] **Query System (Go)** ([l2-query-system-go.md](specifications/l2-query-system-go.md)) [L2] `Stable`
- [x] **System Scheduling** ([l1-system-scheduling.md](specifications/l1-system-scheduling.md)) [L1] `Stable`
- [x] **System Scheduling (Go)** ([l2-system-scheduling-go.md](specifications/l2-system-scheduling-go.md)) [L2] `Stable`
- [x] **Command System** ([l1-command-system.md](specifications/l1-command-system.md)) [L1] `Stable`
- [x] **Command System (Go)** ([l2-command-system-go.md](specifications/l2-command-system-go.md)) [L2] `Stable`
- [x] **Event System** ([l1-event-system.md](specifications/l1-event-system.md)) [L1] `Stable`
- [x] **Event System (Go)** ([l2-event-system-go.md](specifications/l2-event-system-go.md)) [L2] `Stable`
- [x] **Type Registry** ([l1-type-registry.md](specifications/l1-type-registry.md)) [L1] `Stable`
- [x] **Type Registry (Go)** ([l2-type-registry-go.md](specifications/l2-type-registry-go.md)) [L2] `Stable`
- [x] **ECS Lifecycle Patterns** ([l1-ecs-lifecycle-patterns.md](specifications/l1-ecs-lifecycle-patterns.md)) [L1] `Stable`
- [x] **Object Pool (Go)** ([l2-pool-go.md](specifications/l2-pool-go.md)) [L2] `Stable` *(retroactive 2026-05-28 — C27 sync.Pool wrappers; code pre-existed in `internal/ecs/pool/`)*
- [x] **Entity View Cache (Go)** ([l2-view-go.md](specifications/l2-view-go.md)) [L2] `Stable` *(retroactive 2026-05-28 — T-1I01 reactive archetype cache; code pre-existed in `internal/ecs/view/`)*

## Phase 2 — Framework Primitives (Done) `[Bootstrap]`

*Hierarchy, time, input, state, change-detection, app/plugin assembly. Targets `pkg/` extension points and prepares the plugin surface for editor/tooling. Multi-repo architecture (RFC) gate. **Complete — 24/24 atomic tasks Done across Tracks A–G + Validation T (see [tasks/phase-2.md](tasks/phase-2.md)). Validated end-to-end by `examples/ecs/framework/`. 13 specs promoted Draft → Stable; multi-repo l1/l2 remain Draft (RFC ratification = Exit Criterion #4 via /magic.spec).***

- [x] **Hierarchy System** ([l1-hierarchy-system.md](specifications/l1-hierarchy-system.md)) [L1] `Stable`
- [x] **Hierarchy System (Go)** ([l2-hierarchy-system-go.md](specifications/l2-hierarchy-system-go.md)) [L2] `Stable`
- [x] **Time System** ([l1-time-system.md](specifications/l1-time-system.md)) [L1] `Stable`
- [x] **Time System (Go)** ([l2-time-system-go.md](specifications/l2-time-system-go.md)) [L2] `Stable`
- [x] **Input System** ([l1-input-system.md](specifications/l1-input-system.md)) [L1] `Stable`
- [x] **Input System (Go)** ([l2-input-system-go.md](specifications/l2-input-system-go.md)) [L2] `Stable`
- [x] **Input System Codes (Go)** ([l2-input-system-go-codes.md](specifications/l2-input-system-go-codes.md)) [L2] `Stable`
- [x] **State System** ([l1-state-system.md](specifications/l1-state-system.md)) [L1] `Stable`
- [x] **State System (Go)** ([l2-state-system-go.md](specifications/l2-state-system-go.md)) [L2] `Stable`
- [x] **Change Detection** ([l1-change-detection.md](specifications/l1-change-detection.md)) [L1] `Stable`
- [x] **Change Detection (Go)** ([l2-change-detection-go.md](specifications/l2-change-detection-go.md)) [L2] `Stable`
- [x] **App Framework** ([l1-app-framework.md](specifications/l1-app-framework.md)) [L1] `Stable`
- [x] **App Framework (Go)** ([l2-app-framework-go.md](specifications/l2-app-framework-go.md)) [L2] `Stable`
- [x] **Multi-Repo Architecture** ([l1-multi-repo-architecture.md](specifications/l1-multi-repo-architecture.md)) [L1] `Draft` *(surface delivered; RFC ratification pending — Exit Criterion #4)*
- [x] **Multi-Repo Architecture (Go)** ([l2-multi-repo-architecture-go.md](specifications/l2-multi-repo-architecture-go.md)) [L2] `Draft` *(Track G — T-2G01/T-2G02 surface delivered; promotion gated on RFC)*

## Phase 3 — Assets, Math & Concurrency (Done)

*Parallel task pool, asset server, scene serialization, math primitives. Last phase before the STOP FACTOR gate. **Complete — 18/18 atomic tasks Done across Tracks A (Task, critical-path head), B (Asset, consumes pool), C (Scene), D (Math), T (Validation/C29 gate); see [tasks/phase-3.md](tasks/phase-3.md). Validated end-to-end by `examples/{async,asset,scene,math}/` (T-3T05 C29 sign-off — `go test -race ./...` 25 pkgs PASS). All 8 P3 specs promoted Draft → Stable; Bootstrap deactivated for P3.***

- [x] **Task System** ([l1-task-system.md](specifications/l1-task-system.md)) [L1] `Stable`
- [x] **Task System (Go)** ([l2-task-system-go.md](specifications/l2-task-system-go.md)) [L2] `Stable`
- [x] **Asset System** ([l1-asset-system.md](specifications/l1-asset-system.md)) [L1] `Stable`
- [x] **Asset System (Go)** ([l2-asset-system-go.md](specifications/l2-asset-system-go.md)) [L2] `Stable`
- [x] **Scene System** ([l1-scene-system.md](specifications/l1-scene-system.md)) [L1] `Stable`
- [x] **Scene System (Go)** ([l2-scene-system-go.md](specifications/l2-scene-system-go.md)) [L2] `Stable`
- [x] **Math System** ([l1-math-system.md](specifications/l1-math-system.md)) [L1] `Stable`
- [x] **Math System (Go)** ([l2-math-system-go.md](specifications/l2-math-system-go.md)) [L2] `Stable`

## Phase 4 — Render Pipeline (Done)

*Render graph, mesh/image, materials, camera, post-processing. **Done — 19/19 atomic tasks complete across Tracks A (Render Core, 4), B (Mesh & Image, 3), C (Camera & Visibility, 2), D (Materials & Lighting, 3), E (Post-Processing, 2), T (Validation, 5); see [tasks/phase-4.md](tasks/phase-4.md).** C29 P4 gate closed by T-4T05 (`examples/{3d,camera,shader}/` validated, 36/36 pkgs PASS, 0-alloc hot paths). **8 specs promoted Draft → Stable** (4 L1 + 4 L2). **Quarantined:** `l1-render-core` (RFC) + `l2-render-core-go` (Draft, layer-blocked) — RFC→Stable ratification deferred to `/magic.spec` (user decision 2026-05-28); their tasks T-4A01..04 are Done, only spec promotion is held.*

- [x] **Render Core** ([l1-render-core.md](specifications/l1-render-core.md)) [L1] `RFC` *(tasks Done; RFC→Stable ratification pending `/magic.spec` — quarantined from promotion)*
- [x] **Render Core (Go)** ([l2-render-core-go.md](specifications/l2-render-core-go.md)) [L2] `Draft` *(T-4A01..04 Done; layer-blocked by RFC parent — promotion held)*
- [x] **Mesh & Image** ([l1-mesh-and-image.md](specifications/l1-mesh-and-image.md)) [L1] `Stable` *(Track B — T-4B01/B02/B03 Done; promoted 2026-05-28)*
- [x] **Mesh & Image (Go)** ([l2-mesh-and-image-go.md](specifications/l2-mesh-and-image-go.md)) [L2] `Stable` *(Track B; promoted 2026-05-28)*
- [x] **Camera & Visibility** ([l1-camera-and-visibility.md](specifications/l1-camera-and-visibility.md)) [L1] `Stable` *(Track C — T-4C01/C02 Done; promoted 2026-05-28)*
- [x] **Camera & Visibility (Go)** ([l2-camera-and-visibility-go.md](specifications/l2-camera-and-visibility-go.md)) [L2] `Stable` *(Track C; promoted 2026-05-28)*
- [x] **Materials & Lighting** ([l1-materials-and-lighting.md](specifications/l1-materials-and-lighting.md)) [L1] `Stable` *(Track D — T-4D01/D02/D03 Done; promoted 2026-05-28)*
- [x] **Materials & Lighting (Go)** ([l2-materials-and-lighting-go.md](specifications/l2-materials-and-lighting-go.md)) [L2] `Stable` *(Track D; promoted 2026-05-28)*
- [x] **Post-Processing** ([l1-post-processing.md](specifications/l1-post-processing.md)) [L1] `Stable` *(Track E — T-4E01/E02 Done; promoted 2026-05-28)*
- [x] **Post-Processing (Go)** ([l2-post-processing-go.md](specifications/l2-post-processing-go.md)) [L2] `Stable` *(Track E; promoted 2026-05-28)*

## Phase 5 — Content Systems `[Hold]` `[Bootstrap]`

*Audio, asset format codecs, 2D rendering, animation graphs, tweening. **Atomic decomposition complete (2026-05-28) — 18 tasks across Tracks A (Audio, 3), B (Asset Formats, 3), C (2D Rendering, 2), D (Animation, 3), E (Tweening, 2), T (Validation, 5); see [tasks/phase-5.md](tasks/phase-5.md). Critical path: {A‖D} → B → T** (glTF/audio loaders consume the AnimationClip + AudioSource asset types; Track E independent). **Hold:** unfreezes after Phase 4 Render Core reaches `Stable`. Tracks A/B/D/E are render-core-independent (gate only on now-Stable Mesh/Camera/Asset/Math); **Track C (2D) is the sole render-core-coupled track** — blocked until `l1-render-core` ratifies RFC → Stable via `/magic.spec`. L2 Go contracts for P5 specs are not yet authored (recommended `/magic.spec` pre-execution step).*

- [ ] **Audio System** ([l1-audio-system.md](specifications/l1-audio-system.md)) [L1] `[Bootstrap]` *(Track A: T-5A01..03)*
- [ ] **Asset Formats** ([l1-asset-formats.md](specifications/l1-asset-formats.md)) [L1] `[Bootstrap]` *(Track B: T-5B01..03 — joins A+D)*
- [ ] **2D Rendering** ([l1-2d-rendering.md](specifications/l1-2d-rendering.md)) [L1] `[Bootstrap]` *(Track C: T-5C01..02 — gated on render-core Stable)*
- [ ] **Animation System** ([l1-animation-system.md](specifications/l1-animation-system.md)) [L1] `[Bootstrap]` *(Track D: T-5D01..03 — largest track)*
- [ ] **Tweening System** ([l1-tweening-system.md](specifications/l1-tweening-system.md)) [L1] `[Bootstrap]` *(Track E: T-5E01..02 — independent)*

## Phase 6 — UI, Tooling & Quality `[Hold]` `[Bootstrap]`

*Definition layer, window/UI, diagnostics, build & CLI tooling, platform abstraction, AI assistant surface, examples framework, compatibility policy, error taxonomy, benchmark suite, codegen.*

- [ ] **Definition System** ([l1-definition-system.md](specifications/l1-definition-system.md)) [L1]
- [ ] **Definition Integration** ([l1-definition-integration.md](specifications/l1-definition-integration.md)) [L1] `[Bootstrap]`
- [ ] **Window System** ([l1-window-system.md](specifications/l1-window-system.md)) [L1]
- [ ] **Diagnostic System** ([l1-diagnostic-system.md](specifications/l1-diagnostic-system.md)) [L1]
- [ ] **UI System** ([l1-ui-system.md](specifications/l1-ui-system.md)) [L1]
- [ ] **Build Tooling** ([l1-build-tooling.md](specifications/l1-build-tooling.md)) [L1]
- [ ] **CLI Tooling** ([l1-cli-tooling.md](specifications/l1-cli-tooling.md)) [L1]
- [ ] **Code Documentation** ([l1-code-documentation.md](specifications/l1-code-documentation.md)) [L1] `[Bootstrap]`
- [ ] **Code Documentation (Go)** ([l2-code-documentation-go.md](specifications/l2-code-documentation-go.md)) [L2] `[Bootstrap]`
- [ ] **Platform System** ([l1-platform-system.md](specifications/l1-platform-system.md)) [L1]
- [ ] **AI Assistant System** ([l1-ai-assistant-system.md](specifications/l1-ai-assistant-system.md)) [L1]
- [ ] **Plugin Distribution** ([l1-plugin-distribution.md](specifications/l1-plugin-distribution.md)) [L1] `[Bootstrap]`
- [ ] **AI API Plugin** ([l1-ai-api-plugin.md](specifications/l1-ai-api-plugin.md)) [L1] `[Bootstrap]`
- [ ] **Visual Graph System** ([l1-visual-graph-system.md](specifications/l1-visual-graph-system.md)) [L1] `[Bootstrap]`
- [ ] **Visual Graph Editor Bridge (Go)** ([l2-visual-graph-editor-bridge.md](specifications/l2-visual-graph-editor-bridge.md)) [L2] `[Bootstrap]`
- [ ] **Examples Framework** ([l1-examples-framework.md](specifications/l1-examples-framework.md)) [L1]
- [ ] **Compatibility Policy** ([l1-compatibility-policy.md](specifications/l1-compatibility-policy.md)) [L1]
- [ ] **Error Core** ([l1-error-core.md](specifications/l1-error-core.md)) [L1]
- [ ] **Benchmark Spec** ([l2-benchmark-spec.md](specifications/l2-benchmark-spec.md)) [L2-test]
- [ ] **Codegen Tools** ([l2-codegen-tools.md](specifications/l2-codegen-tools.md)) [L2-tool]

## Phase 7 — Networking & Hot-Reload `[Hold]` `[Bootstrap]`

*Profiling protocol, multiplayer stack (transport, replication, sync models), RPC, network diagnostics, hot-reload orchestrator.*

- [ ] **Profiling Protocol** ([l1-profiling-protocol.md](specifications/l1-profiling-protocol.md)) [L1]
- [ ] **Networking System** ([l1-networking-system.md](specifications/l1-networking-system.md)) [L1]
- [ ] **Transport** ([l1-transport.md](specifications/l1-transport.md)) [L1]
- [ ] **Replication** ([l1-replication.md](specifications/l1-replication.md)) [L1]
- [ ] **Snapshot Interpolation** ([l1-snapshot-interpolation.md](specifications/l1-snapshot-interpolation.md)) [L1]
- [ ] **Client Prediction** ([l1-client-prediction.md](specifications/l1-client-prediction.md)) [L1]
- [ ] **Lockstep** ([l1-lockstep.md](specifications/l1-lockstep.md)) [L1]
- [ ] **RPC** ([l1-rpc.md](specifications/l1-rpc.md)) [L1]
- [ ] **Network Diagnostics** ([l1-network-diagnostics.md](specifications/l1-network-diagnostics.md)) [L1]
- [ ] **Hot Reload** ([l1-hot-reload.md](specifications/l1-hot-reload.md)) [L1]

## Phase 8 — Physics & Scripting `[Hold]` `[Bootstrap]`

*Physics server, rigid bodies, colliders, queries, joints, collision events, physics materials, character controller, scripting bridge.*

- [ ] **Physics System** ([l1-physics-system.md](specifications/l1-physics-system.md)) [L1]
- [ ] **Rigid Body** ([l1-rigid-body.md](specifications/l1-rigid-body.md)) [L1]
- [ ] **Collider** ([l1-collider.md](specifications/l1-collider.md)) [L1]
- [ ] **Physics Query** ([l1-physics-query.md](specifications/l1-physics-query.md)) [L1]
- [ ] **Joints** ([l1-joints.md](specifications/l1-joints.md)) [L1]
- [ ] **Collision Events** ([l1-collision-events.md](specifications/l1-collision-events.md)) [L1]
- [ ] **Physics Materials** ([l1-physics-materials.md](specifications/l1-physics-materials.md)) [L1]
- [ ] **Character Controller** ([l1-character-controller.md](specifications/l1-character-controller.md)) [L1]
- [ ] **Scripting System** ([l1-scripting-system.md](specifications/l1-scripting-system.md)) [L1]

## Backlog

<!-- Empty: Bootstrap regeneration mapped every registered spec to a phase. New Draft additions land here. -->

## Phase Gating Matrix

| Phase | Status | Unfreezes when |
| :--- | :--- | :--- |
| 1 — ECS Core POC | Done | — (27/27 complete) |
| 2 — Framework | Done | — (24/24 complete; 13 specs Stable; multi-repo RFC pending) |
| 3 — Assets, Math & Concurrency | Done | — (18/18 complete; 8 P3 specs Stable; C29 P3 gate satisfied by T-3T05) |
| 4 — Render Pipeline | Done | — (19/19 complete; C29 P4 gate closed by T-4T05; 8 specs Stable; render-core RFC pending `/magic.spec` ratification) |
| 5 — Content Systems | Hold | `l1-render-core` ratifies RFC → Stable (Track C / 2D only — Tracks A/B/D/E render-core-independent); 18 tasks decomposed 2026-05-28 |
| 6 — UI, Tooling & Quality | Hold | Phase 1–3 Stable |
| 7 — Networking & Hot-Reload | Hold | App Framework + Scheduler Stable |
| 8 — Physics & Scripting | Hold | Render Core + Phase 3 Math Stable |

## Planning Audit (`@role:planner`)

- **Phase 1 (Done)**: 27 atomic tasks across 9 tracks; critical path B → C → D held. Retained for historical audit.
- **Phase 2 Optimism Bias**: 24 atomic tasks across 8 tracks (A–G + T). Tracks A–D file-independent and parallelizable; Track E is small but highest-risk (reaches into Phase 1 `query/filter.go`).
- **Phase 2 Hidden Dependencies**: Track E (Change Detection) is the critical path — T-2E02 replaces the Phase 1 `T-1D03` accept-all stub and unblocks every `Changed`-filtered system. Track F's `DefaultPlugins` (T-2F03) joins on A03/B03/C03/D03/E03.
- **Phase 2 Cascade Risk**: If Track E slips, the framework example (T-2T04) cannot prove its `Changed`-filter acceptance → Phase 2 gate stays closed → Phases 4–8 remain `Hold` (C-002). Mitigation: schedule Track E first; strongest contributor owns it.
- **Phase 3 Optimism Bias**: 18 atomic tasks across Tracks A–D + T. Track D (Math) is the largest (`l2-math-system-go.md` 435 lines, plus SIMD-accel parity) — sized at 4 tasks (T-3D01..04), not the default 2–3, to avoid under-estimation of the linear-algebra + primitives + color/curves + SIMD surface.
- **Phase 3 Hidden Dependencies**: Tracks are **not** fully parallel — Track B's async asset loader (T-3B02) consumes Track A's `ComputePool` (T-3A03), so A03 gates B02. Tracks C (Scene) and D (Math) are file-independent and parallelizable; Scene reuses Phase 1 `typereg` (Done, no new dependency). Critical path: **A → B**.
- **Phase 3 Cascade Risk**: If Track A slips, T-3B02 (async loader) and T-3T01 (`examples/async/`) are blocked → the C29 P3 gate (`examples/{async,asset,scene,math}/`) stays closed → P3 specs cannot promote Draft → Stable and Phases 4–8 remain `Hold` (C-002). Mitigation: schedule Track A first; strongest contributor owns the work-stealing deque (T-3A01).
- **Phase 4 Optimism Bias**: 19 atomic tasks across Tracks A–E + T. Track A (Render Core) is sized at **4** tasks (not the default 2–3) — `l2-render-core-go.md` is the largest P4 surface (281 lines: server/RID, Kahn graph, SubApp+4-phase extract, RenderFeature+SoA) and is the hard dependency for every other track; under-sizing it is the dominant schedule risk.
- **Phase 4 Hidden Dependencies**: Tracks are **not** fully parallel. T-4A01 (server/RID) gates T-4B03 (image upload) and T-4C01 (camera RenderTarget); T-4A02 (graph) gates T-4D03 (shadow pass) and T-4E01 (post chain); T-4A04 (SoA cull) gates T-4C02. Tracks B (mesh) and C (camera) are mutually file-independent and parallelize once A03 lands. D (materials) joins B+C; E (post) is the tail joining D. The render server (A01→A03) is the true critical path, not a single track.
- **Phase 4 Cascade Risk**: If Track A slips, **every** downstream track and all three C29 examples (`examples/{3d,camera,shader}/`, T-4T01–03) are blocked → the C29 P4 gate stays closed → P4 specs cannot promote Draft → Stable and Phase 5 (Content Systems) stays `Hold` (depends on Render Core `Stable`). Mitigation: schedule Track A first; the strongest contributor owns the RID+command-queue server (T-4A01) and the Kahn-DAG graph (T-4A02). Render-world isolation (T-4T04) must be proven early — a leak there invalidates the extract-pattern invariant the whole phase rests on.
- **Phase 5 Optimism Bias**: 18 atomic tasks across Tracks A–E + T. Track D (Animation) is the largest functional surface (skeletal + morph + blend-graph + events + reflection-driven property-path resolution + parallel eval — 10 design sections) — sized at 3 tasks; the reflection-path write-accessor caching (T-5D01) and crossfade dual-state evaluation (T-5D02) are the under-sizing risks. Audio (Track A) is rich (bus graph DAG + effect factory/instance + driver abstraction) but the headless stub backend keeps the validation surface small.
- **Phase 5 Hidden Dependencies**: Tracks are **not** 5-way parallel. Asset Formats (Track B) is a *consumer*, not a root — the glTF loader (T-5B03) produces Animation/Mesh/Material sub-assets so it joins Track D's `AnimationClip` (T-5D01); audio loaders (T-5B02) consume Track A's `AudioSource` (T-5A01). Critical path is **{A‖D} → B**, not "B first". Track E (Tweening) is the only genuinely independent track. Track C (2D) depends on render-core SoA infra (`RenderDataHolder`/`VisibilityGroup`) — it is *externally* gated, not internally.
- **Phase 5 Cascade Risk**: `l1-render-core` staying RFC blocks **only** Track C (2D) and the 2D portion of the C29 P5 gate (T-5T04) — the phase-level `Hold` condition ("Render Core Stable") is therefore *over-broad*: Tracks A/B/D/E could execute against the now-Stable Mesh/Camera/Asset/Math specs the moment the hold is administratively relaxed. Mitigation options: (a) ratify render-core RFC → Stable via `/magic.spec` to unblock everything uniformly; or (b) split the P5 gate so the render-core-independent cohort (audio/animation/tweening/formats) promotes ahead of 2D. Also: P5 specs are L1-only — authoring L2 Go contracts via `/magic.spec` is the recommended pre-execution step before `/magic.run`.
- **C29 Cascade Risk (standing)**: Phases 4–8 are blocked on per-phase `examples/` gates; if a gate slips, the upper plan freezes. Each phase carries an explicit Validation Track (T-*) scoping the minimal gate.

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 1.0.0 | 2026-04-25 | Force-Bootstrap regeneration; 76 specs mapped across 8 phases |
| 1.1.0 | 2026-05-01 | Added `l1-plugin-distribution` + `l1-ai-api-plugin` to Phase 6 (UI, Tooling & Quality); INDEX v2.22.0 |
| 1.2.0 | 2026-05-14 | Added `l1-visual-graph-system` to Phase 6 (Track P already decomposed in `tasks/phase-6.md` as T-6P01..04); INDEX v2.23.0 |
| 1.3.0 | 2026-05-15 | Registry sync to INDEX v2.24.0 (84 specs); 5 orphan specs placed; full Draft cohort re-mapped |
| 1.4.0 | 2026-05-16 | Phase 1 → Done (27/27); Phase 2 promoted Active with 24-task atomic decomposition (Tracks A–G + T); corrected stale spec count (79 → 84) |
| 1.5.0 | 2026-05-17 | Pre-Planning Stabilization: 17 P1 specs Draft → Stable (C29 via T-1T05); Bootstrap deactivated for P1 |
| 1.6.0 | 2026-05-17 | Registry sync to INDEX v2.25.0 (89 specs); placed 5 orphan L2 specs — `l2-multi-repo-architecture-go` → Phase 2 (Track G); `l2-{task,asset,scene,math}-system-go` → Phase 3. 19 hard L2→L1 edges, acyclic. All new specs `[Bootstrap]` (C29 keeps non-P1 Draft) |
| 1.7.0 | 2026-05-17 | Phase 2 → Done (24/24); Pre-Planning Stabilization promoted 13 P2 specs Draft → Stable (C29-style gate via `examples/ecs/framework/`); multi-repo l1/l2 stay Draft (RFC-gated, Exit Criterion #4). Phase 3 → Active with atomic decomposition. INDEX v2.26.0 (Stable 30/89) |
| 1.8.0 | 2026-05-18 | Phase 3 atomic decomposition realized in `tasks/phase-3.md` (18 tasks: Tracks A:3 Task / B:3 Asset / C:3 Scene / D:4 Math / T:5 Validation) — deferral gate satisfied (Phase 2 = 100% Done). Resolved PLAN↔workbook drift (1.7.0 prose claimed decomposition complete while workbook was still "Pending"). Pre-Planning Stabilization: 0 promoted (C29 P3 `examples/` gate unmet — Bootstrap retained). Engine Snapshot synced (`.design/INDEX.md` **Engine Version:** → 2.1.27). INDEX v2.26.0 unchanged (Stable 30/89) |
| 1.9.0 | 2026-05-18 | Post-Run Replan (rules/magic.md §5): Phase 3 → **Done** (18/18); resolved PLAN↔STATE mechanical drift (PLAN listed Phase 3 Active with unchecked items while STATE/TASKS/git = Done). Pre-Planning Stabilization promoted 8 P3 specs Draft → Stable (4 L1 + 4 L2: task, asset, scene, math) — C29 P3 gate satisfied by T-3T05 (`examples/{async,asset,scene,math}/` green). Bootstrap deactivated for P3. Phase 4 STOP FACTOR [C-002] lifted; remains `Hold` on Release Cond. #3 (L2 render specs absent) — decomposition deferred per TASKS.md per-phase policy. INDEX v2.27.0 (Stable 38/89), RULES parity v1.8.0 |
| 1.10.0 | 2026-05-19 | Scoped+Guided `/magic.task main "decompose phase-4"`. Phase 4 `Hold` → **Active** — all 3 Hold Release Conditions met (POC validated + App Framework Stable + 5 L2 render specs authored 2026-05-18 via `/magic.spec`). Placed 5 orphan L2 render specs into Phase 4 under their L1 parents (resolves Pre-flight ORPHANED_SPEC ×5 + SYNC_GAP). Atomic decomposition: 19 tasks across Tracks A:4 Render Core / B:3 Mesh&Image / C:2 Camera / D:3 Materials / E:2 Post / T:5 Validation — critical path A → {B‖C} → D → E → T. Pre-Planning Stabilization: 0 promoted (C29 P4 `examples/{3d,camera,shader}/` gate unmet — Bootstrap retained). 5 new hard L2→L1 edges (24 total), acyclic. INDEX sync v2.27.0 → v2.28.0 (94 specs, Stable 38/94). |
| 1.11.0 | 2026-05-28 | Registry sync (backfilled — header bumped without history row): INDEX v2.28.0 → v2.29.0 (94 → 96 specs). Placed 2 retroactive P1 L2 specs `l2-pool-go` + `l2-view-go` (Stable, debt-recovery — code pre-existed in `internal/ecs/{pool,view}/`) into Phase 1. `l1-render-core` promoted Draft → RFC (v0.5.0 → v0.6.0: +`Destroy`, +RID layout, +Canonical Refs, Q4/Q5 resolved). Pre-Planning Stabilization: 0 newly promoted (C29 P4 gate still unmet at that point). Stable 40/96. |
| 1.12.0 | 2026-05-28 | `/magic.task` full planning. **Pre-Planning Stabilization: 8 P4 render specs Draft → Stable** (4 L1 + 4 L2: mesh-and-image, materials-and-lighting, camera-and-visibility, post-processing) — C29 P4 gate closed by T-4T05 (`examples/{3d,camera,shader}/` validated). **Phase 4 → Done** (19/19 tasks). Per user decision, `l1-render-core` kept **RFC** + `l2-render-core-go` Draft (layer-blocked) — RFC→Stable ratification deferred to `/magic.spec`; tasks already Done so promotion-quarantine only (C12.1, not task-quarantine). **Phase 5 full atomic decomposition** (explicit user request): 18 tasks across Tracks A:3 Audio / B:3 Asset Formats / C:2 2D / D:3 Animation / E:2 Tweening / T:5 Validation — critical path {A‖D} → B → T; Track C externally gated on render-core Stable. Phase 5 stays `Hold`. INDEX v2.29.0 → v2.30.0 (Stable 40 → 48 / 96). RULES parity v1.8.0. |
