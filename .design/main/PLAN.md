# Implementation Plan — Neu Engine

**Version:** 1.5.0
**Generated:** 2026-05-17
**Based on:** .design/main/INDEX.md v2.24.0
**Based on RULES:** .design/RULES.md v1.7.1
**Status:** Active
**Mode:** Phase 1 specs `Stable` (Bootstrap deactivated for P1 — 17/84 specs promoted 2026-05-17). Phase 2+ remain `[Bootstrap]` pending `examples/ecs/framework/` gate.

## Overview

Force-Bootstrap regeneration of the implementation plan. Every registered specification (84 total) is mapped to its target phase, ordered by the P1–P8 priority batches in `INDEX.md` and gated by:

- **STOP FACTOR**: phases ≥ 4 are frozen (`Hold`) until Phase 1 (POC) is validated by code in `examples/ecs/poc/` (C29).
- **Layer Order**: every L1 concept spec is scheduled before its L2 Go implementation within the same phase.
- **C29 Resolved (Phase 1)**: P1 ECS Core specs promoted `Draft → Stable` on 2026-05-17 via Pre-Planning Stabilization (T-1T05 satisfied the gate). Phase 2+ promotion deferred to `examples/ecs/framework/` completion.

Dependency analysis (Implements: chains):

- 14 hard L2→L1 edges, all 1:1, no chains, no cycles.
- 62 L1 specs are roots. `Related Specifications` cycles within a single phase are non-blocking (Circular Guard Semantic Split — Soft).

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

## Phase 2 — Framework Primitives (Active) `[Bootstrap]`

*Hierarchy, time, input, state, change-detection, app/plugin assembly. Targets `pkg/` extension points and prepares the plugin surface for editor/tooling. Multi-repo architecture (RFC) gate. **Atomic decomposition complete — 24 tasks across Tracks A–G + Validation T (see [tasks/phase-2.md](tasks/phase-2.md)). Critical path: E → F.***

- [ ] **Hierarchy System** ([l1-hierarchy-system.md](specifications/l1-hierarchy-system.md)) [L1] `[Bootstrap]`
- [ ] **Hierarchy System (Go)** ([l2-hierarchy-system-go.md](specifications/l2-hierarchy-system-go.md)) [L2] `[Bootstrap]`
- [ ] **Time System** ([l1-time-system.md](specifications/l1-time-system.md)) [L1] `[Bootstrap]`
- [ ] **Time System (Go)** ([l2-time-system-go.md](specifications/l2-time-system-go.md)) [L2] `[Bootstrap]`
- [ ] **Input System** ([l1-input-system.md](specifications/l1-input-system.md)) [L1] `[Bootstrap]`
- [ ] **Input System (Go)** ([l2-input-system-go.md](specifications/l2-input-system-go.md)) [L2] `[Bootstrap]`
- [ ] **Input System Codes (Go)** ([l2-input-system-go-codes.md](specifications/l2-input-system-go-codes.md)) [L2] `[Bootstrap]`
- [ ] **State System** ([l1-state-system.md](specifications/l1-state-system.md)) [L1] `[Bootstrap]`
- [ ] **State System (Go)** ([l2-state-system-go.md](specifications/l2-state-system-go.md)) [L2] `[Bootstrap]`
- [ ] **Change Detection** ([l1-change-detection.md](specifications/l1-change-detection.md)) [L1] `[Bootstrap]`
- [ ] **Change Detection (Go)** ([l2-change-detection-go.md](specifications/l2-change-detection-go.md)) [L2] `[Bootstrap]`
- [ ] **App Framework** ([l1-app-framework.md](specifications/l1-app-framework.md)) [L1] `[Bootstrap]`
- [ ] **App Framework (Go)** ([l2-app-framework-go.md](specifications/l2-app-framework-go.md)) [L2] `[Bootstrap]`
- [ ] **Multi-Repo Architecture** ([l1-multi-repo-architecture.md](specifications/l1-multi-repo-architecture.md)) [L1] *(RFC — surface for review)*

## Phase 3 — Assets, Math & Concurrency `[Bootstrap]`

*Parallel task pool, asset server, scene serialization, math primitives. Last phase before the STOP FACTOR gate.*

- [ ] **Task System** ([l1-task-system.md](specifications/l1-task-system.md)) [L1] `[Bootstrap]`
- [ ] **Asset System** ([l1-asset-system.md](specifications/l1-asset-system.md)) [L1] `[Bootstrap]`
- [ ] **Scene System** ([l1-scene-system.md](specifications/l1-scene-system.md)) [L1] `[Bootstrap]`
- [ ] **Math System** ([l1-math-system.md](specifications/l1-math-system.md)) [L1] `[Bootstrap]`

## Phase 4 — Render Pipeline `[Hold]` `[Bootstrap]`

*Render graph, mesh/image, materials, camera, post-processing. **Hold:** unfreezes once Phase 1 POC validated (C29) AND Phase 2 App Framework `Stable`.*

- [ ] **Render Core** ([l1-render-core.md](specifications/l1-render-core.md)) [L1]
- [ ] **Mesh & Image** ([l1-mesh-and-image.md](specifications/l1-mesh-and-image.md)) [L1]
- [ ] **Materials & Lighting** ([l1-materials-and-lighting.md](specifications/l1-materials-and-lighting.md)) [L1]
- [ ] **Camera & Visibility** ([l1-camera-and-visibility.md](specifications/l1-camera-and-visibility.md)) [L1]
- [ ] **Post-Processing** ([l1-post-processing.md](specifications/l1-post-processing.md)) [L1]

## Phase 5 — Content Systems `[Hold]` `[Bootstrap]`

*Audio, asset format codecs, 2D rendering, animation graphs, tweening. **Hold:** unfreezes after Phase 4 Render Core `Stable`.*

- [ ] **Audio System** ([l1-audio-system.md](specifications/l1-audio-system.md)) [L1]
- [ ] **Asset Formats** ([l1-asset-formats.md](specifications/l1-asset-formats.md)) [L1]
- [ ] **2D Rendering** ([l1-2d-rendering.md](specifications/l1-2d-rendering.md)) [L1]
- [ ] **Animation System** ([l1-animation-system.md](specifications/l1-animation-system.md)) [L1]
- [ ] **Tweening System** ([l1-tweening-system.md](specifications/l1-tweening-system.md)) [L1]

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
| 2 — Framework | Active | — (current; Phase 1 100% Done) |
| 3 — Assets, Math & Concurrency | Todo | Phase 1 Done; Phase 2 ≥ 50% |
| 4 — Render Pipeline | Hold | C29 unblocked (POC validated) AND App Framework Stable |
| 5 — Content Systems | Hold | Render Core Stable |
| 6 — UI, Tooling & Quality | Hold | Phase 1–3 Stable |
| 7 — Networking & Hot-Reload | Hold | App Framework + Scheduler Stable |
| 8 — Physics & Scripting | Hold | Render Core + Phase 3 Math Stable |

## Planning Audit (`@role:planner`)

- **Phase 1 (Done)**: 27 atomic tasks across 9 tracks; critical path B → C → D held. Retained for historical audit.
- **Phase 2 Optimism Bias**: 24 atomic tasks across 8 tracks (A–G + T). Tracks A–D file-independent and parallelizable; Track E is small but highest-risk (reaches into Phase 1 `query/filter.go`).
- **Phase 2 Hidden Dependencies**: Track E (Change Detection) is the critical path — T-2E02 replaces the Phase 1 `T-1D03` accept-all stub and unblocks every `Changed`-filtered system. Track F's `DefaultPlugins` (T-2F03) joins on A03/B03/C03/D03/E03.
- **Phase 2 Cascade Risk**: If Track E slips, the framework example (T-2T04) cannot prove its `Changed`-filter acceptance → Phase 2 gate stays closed → Phases 4–8 remain `Hold` (C-002). Mitigation: schedule Track E first; strongest contributor owns it.
- **C29 Cascade Risk (standing)**: Phases 4–8 are blocked on per-phase `examples/` gates; if a gate slips, the upper plan freezes. Each phase carries an explicit Validation Track (T-*) scoping the minimal gate.

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 1.0.0 | 2026-04-25 | Force-Bootstrap regeneration; 76 specs mapped across 8 phases |
| 1.1.0 | 2026-05-01 | Added `l1-plugin-distribution` + `l1-ai-api-plugin` to Phase 6 (UI, Tooling & Quality); INDEX v2.22.0 |
| 1.2.0 | 2026-05-14 | Added `l1-visual-graph-system` to Phase 6 (Track P already decomposed in `tasks/phase-6.md` as T-6P01..04); INDEX v2.23.0 |
| 1.3.0 | 2026-05-15 | Registry sync to INDEX v2.24.0 (84 specs); 5 orphan specs placed; full Draft cohort re-mapped |
| 1.4.0 | 2026-05-16 | Phase 1 → Done (27/27); Phase 2 promoted Active with 24-task atomic decomposition (Tracks A–G + T); corrected stale spec count (79 → 84) |
