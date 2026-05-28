# Master Task Index (Registry)

**Version:** 1.13.0
**Generated:** 2026-05-28
**Based on:** .design/main/PLAN.md v1.13.0
**Based on RULES:** .design/RULES.md v1.8.0
**Execution Mode:** Parallel (per C3)
**Status:** Active
**Mode:** Specs `Stable` 50/96. **Phase 4 Done** (19/19) — **all 10 render specs Stable** (`l1-render-core` ratified RFC → Stable + `l2-render-core-go` Draft → Stable via `/magic.spec` 2026-05-28; RFC count → 0). **Phase 5 Ready** — gate cleared (render-core Stable); full atomic decomposition complete (18 tasks, Tracks A–E + T; critical path {A‖D} → B → T; Track C now unblocked). Recommended next: author L2 Go contracts for P5 via `/magic.spec`, then `/magic.run main`.

## Overview

Tactical registry of all phases and their statuses. Atomic checklists live in per-phase workbooks under `tasks/phase-{N}.md`.

## Active Phases

| Phase | Description | Status |
| :--- | :--- | :--- |
| [Phase 1](tasks/phase-1.md) | ECS Core POC — world, entities, components, queries, scheduler, validating `examples/ecs/poc/` | Done |
| [Phase 2](archives/tasks/phase-2.md) | Framework Primitives — hierarchy, time, input, state, change-detection, app/plugin (24 atomic tasks, Tracks A–G + T) | Done |
| [Phase 3](tasks/phase-3.md) | Assets, Math & Concurrency — task pool, asset server, scene, math (18 atomic tasks, Tracks A–D + T; decomposed 2026-05-18) | Done |
| [Phase 4](tasks/phase-4.md) | Render Pipeline — render graph, mesh, materials, camera, post-processing (19 atomic tasks, Tracks A–E + T; decomposed 2026-05-19) | Done |
| [Phase 5](tasks/phase-5.md) | Content Systems — audio, asset codecs, 2D, animation, tweening (18 atomic tasks, Tracks A–E + T; decomposed 2026-05-28) | Ready |
| [Phase 6](tasks/phase-6.md) | UI, Tooling & Quality — definition, window, UI, build, CLI, platform, AI, plugins, visual graph, examples, errors, benchmark, codegen | Hold |
| [Phase 7](tasks/phase-7.md) | Networking & Hot-Reload — profiling, transport, replication, sync, RPC, hot-reload | Hold |
| [Phase 8](tasks/phase-8.md) | Physics & Scripting — physics server, bodies, colliders, queries, joints, character, scripting | Hold |

## Archived Phases

| Phase | Description | Archive |
| :--- | :--- | :--- |
| Phase 0 (legacy) | POC implementation (pre-Bootstrap layout) | [archives/tasks/01-poc-implementation.md](archives/tasks/01-poc-implementation.md) |

## Cross-Phase Status Counters

- **Total atomic tasks (Phase 1)**: 27 (Tracks A–I + T) — Done *(+2 retroactive Stable L2 specs: `l2-pool-go`, `l2-view-go` — debt-recovery, no new atomic tasks required)*
- **Total atomic tasks (Phase 2)**: 24 (Tracks A–G + T) — decomposed 2026-05-16; critical path E → F
- **Total atomic tasks (Phase 3)**: 18 (Tracks A:3 / B:3 / C:3 / D:4 / T:5) — **Done 2026-05-18** (18/18; critical path A → B held; C29 P3 gate closed by T-3T05)
- **Total atomic tasks (Phase 4)**: 19 (Tracks A:4 / B:3 / C:2 / D:3 / E:2 / T:5) — **Done 2026-05-28** (19/19; critical path A → {B‖C} → D → E → T held; C29 P4 gate closed by T-4T05; 8 specs Stable, render-core RFC quarantined)
- **Total atomic tasks (Phase 5)**: 18 (Tracks A:3 Audio / B:3 Asset Formats / C:2 2D / D:3 Animation / E:2 Tweening / T:5 Validation) — decomposed 2026-05-28; critical path {A‖D} → B → T; Track C gated on render-core Stable
- **Total atomic tasks (Phase 6)**: 46 (Tracks A–P + T) — Track P (Visual Graph System, 4 tasks) added 2026-05-14
- **Phases 7–8**: structural workbooks, atomic decomposition deferred to per-phase `/magic.task {workspace} "decompose phase-N"` invocations.

## Meta Information

- **Last Updated**: 2026-05-28
- **Maintainer**: Core Team
- **Render Core Ratification (2026-05-28, `/magic.spec`)**: `l1-render-core` **RFC → Stable** + `l2-render-core-go` **Draft → Stable** — completes the ratification deferred from the prior `/magic.task`. Evidence: Phase 4 complete (19/19) + C29 P4 gate closed by T-4T05; L1 Canonical References already filled (8 files), Q4/Q5 resolved, Q1–Q3 annotated as non-blocking forward-looking refinements; L2 Canonical References populated with 11 verified source files + 2 conformance/isolation tests. **Phase 4 = 10/10 specs Stable** (RFC count → 0). **Phase 5 gate ("Render Core Stable") cleared** → Phase 5 `Hold` → `Ready` (Track C / 2D unblocked). INDEX v2.30.0 → v2.31.0 (Stable 48 → 50 / 96), PLAN/TASKS v1.12.0 → v1.13.0. No engine (`.magic/`) files modified — no C14 bump. Next: author L2 Go contracts for P5 specs via `/magic.spec`, then activate Phase 5 via `/magic.task` / `/magic.run main`.
- **Phase 4 Stabilization + Phase 5 Decomposition (2026-05-28)**: `/magic.task` full planning. **Pre-Planning Stabilization promoted 8 P4 render specs Draft → Stable** (4 L1 + 4 L2: mesh-and-image, materials-and-lighting, camera-and-visibility, post-processing) — C29 P4 gate satisfied by T-4T05 (`examples/{3d,camera,shader}/` validated, 36/36 pkgs PASS, all hot-path benchmarks 0 B/op 0 allocs/op). **Phase 4 → Done** (19/19). Per user decision, `l1-render-core` retained **RFC** + `l2-render-core-go` Draft (layer-blocked by RFC parent) — RFC→Stable ratification deferred to `/magic.spec`; this is a *promotion-quarantine* only (tasks T-4A01..04 are Done; C12.1 stabilization exception — no task moved to Backlog). **Phase 5 full atomic decomposition** (explicit user request, overriding the deferred per-phase policy): 18 tasks written to `tasks/phase-5.md` across Tracks A (Audio, 3) / B (Asset Formats, 3) / C (2D, 2) / D (Animation, 3) / E (Tweening, 2) / T (Validation, 5). Critical path {A‖D} → B → T (glTF/audio loaders consume AnimationClip + AudioSource types); Track C externally gated on render-core Stable. P5 specs are L1-only — L2 Go contracts pending `/magic.spec`. Phase 5 stays `Hold`. INDEX v2.29.0 → v2.30.0 (Stable 40 → 48 / 96), PLAN v1.11.0 → v1.12.0. No engine (`.magic/`) files modified — no C14 bump.
- **Registry Sync (2026-05-28)**: INDEX v2.28.0 → v2.29.0 (94 → 96 specs). Resolved ORPHANED_SPEC ×2 + SYNC_GAP: placed `l2-pool-go` and `l2-view-go` (Stable, debt-recovery P1 L2 specs) into Phase 1. `l1-render-core` promoted Draft → RFC (v0.5.0 → v0.6.0). PLAN.md v1.10.0 → v1.11.0. Pre-Planning Stabilization: 0 newly promoted (pool/view already Stable from magic.spec session; Phase 4-8 Draft specs not yet MVC-promotable without C29 gate). No new atomic task IDs — retroactive specs require no implementation work.
- **Phase 4 Atomic Decomposition (2026-05-19)**: 19 atomic tasks written to `tasks/phase-4.md` — Track A (Render Core, 4: server/RID+backend, Kahn-DAG graph, SubApp+4-phase extract, RenderFeature+SoA cull — critical-path head), Track B (Mesh & Image, 3: mesh/layout, primitives/skin, image/atlas/upload — consumes A server), Track C (Camera & Visibility, 2: camera/projection/frustum, 3-layer visibility/cull), Track D (Materials & Lighting, 3: material/PBR/speckey, lights/IBL/shadow, clustering/shadow-pass), Track E (Post-Processing, 2: effect-chain/tonemap, ping-pong/custom/AA), Track T (Validation, 5: examples/{3d,camera,shader} + isolation/conformance + C29 gate sign-off). Scoped+Guided `/magic.task main "decompose phase-4"` — all 3 Hold Release Conditions met (POC + App Framework Stable + 5 L2 specs authored 2026-05-18). Resolved Pre-flight ORPHANED_SPEC ×5 + SYNC_GAP (placed L2 render specs into Phase 4, INDEX v2.27.0 → v2.28.0). Pre-Planning Stabilization: 0 promoted (C29 P4 gate unmet — Bootstrap retained). Execute via `/magic.run main`.
- **Phase 3 Atomic Decomposition (2026-05-18)**: 18 atomic tasks written to `tasks/phase-3.md` — Track A (Task System, 3: deque/pool, scope/handle, executor/plugin — critical-path head), Track B (Asset System, 3: handle/store, loader/VFS/async, content/watch — consumes A's pool), Track C (Scene System, 3: static/codec, dynamic/spawner, remap), Track D (Math System, 4: linear, affine/primitives, color/curves, SIMD-parity), Track T (Validation, 5: async/asset/scene/math examples + C29 gate sign-off). Deferral condition (Phase 2 ≥ 50% Done) satisfied — Phase 2 is 100% Done. Resolved PLAN↔workbook drift: PLAN v1.7.0 prose claimed decomposition complete while the workbook still read "Pending". Pre-Planning Stabilization: 0 specs promoted (C29 P3 `examples/` gate unmet — Bootstrap retained for P3+). Execute via `/magic.run main`.
- **RULES Parity (✅ resolved 2026-05-18)**: `.design/RULES.md` header now reads `**Version:** 1.8.0`, consistent with its Document History (C31 — sync-docs Safety Gate) and §7 body. Prior header/content drift (1.7.1 lagging 1.8.0) reconciled. Recorded parity in this header: RULES v1.8.0.
- **Post-Run Replan (2026-05-18, rules/magic.md §5)**: Phase 3 → **Done** (18/18 atomic tasks across Tracks A–D + T). Resolved PLAN↔STATE mechanical drift — PLAN.md listed Phase 3 `Active` with unchecked items while STATE.md/TASKS.md/git history all recorded Done. Pre-Planning Stabilization promoted **8 P3 specs Draft → Stable** (4 L1 + 4 L2: task, asset, scene, math) — C29 P3 gate satisfied by T-3T05 (`examples/{async,asset,scene,math}/` build+race green, full workspace `go test -race ./...` 25 pkgs PASS). Bootstrap deactivated for P3. INDEX v2.26.0 → v2.27.0 (Stable 30 → 38 / 89). Phase 4 STOP FACTOR [C-002] lifted (POC validated + App Framework Stable + Phase 3 Done); Phase 4 stays `Hold` on Release Condition #3 (L2 render specs absent) — atomic decomposition deferred per the per-phase policy above.
- **Phase 1 Progress**: 27 / 27 (100%) — All tracks complete. Validation Track T done: `pkg/ecs` public API, race/fuzz/golden tests, `examples/ecs/poc/` end-to-end POC. C29 unblocked. Phase 1 → Done.
- **Pre-Planning Stabilization (2026-05-17)**: 17 P1 ECS Core specs promoted `Draft → Stable` via Trust Mode batch (C29 gate satisfied by T-1T05). Breakdown: 9 L1 concept specs + 8 L2 Go impl specs. Bootstrap Mode deactivated for Phase 1; Phase 2+ remain `[Bootstrap]` until `examples/ecs/framework/` validates the runtime. Stable: 17 / 84.
- **Phase 2 Atomic Decomposition (2026-05-16)**: 24 atomic tasks across Tracks A (Hierarchy), B (Time), C (Input), D (State), E (Change Detection — critical path, closes T-1D03 stub), F (App Framework — critical path tail), G (Multi-Repo surface, RFC-gated), T (Validation). Phase 2 is the active phase; `examples/ecs/framework/` is the Phase 2 promotion gate. Execute via `/magic.run main`.
- **Phase 6 Atomic Decomposition (2026-05-01)**: 42 atomic tasks across Tracks A–O + T. Tracks N (Plugin Distribution, 4 tasks) and O (AI API Plugin, 5 tasks) are new. Phase remains in `Hold` until Phase 1–3 reach `Stable`.
- **Phase 6 Track P Addition (2026-05-14)**: Track P (Visual Graph System, 4 tasks T-6P01..04) registered in PLAN.md to match prior phase-6.md workbook decomposition. Phase total: 46.
- **Registry Sync (2026-05-17)**: INDEX v2.24.0 → v2.25.0 (84 → 89 specs). 5 orphan L2 specs placed: `l2-multi-repo-architecture-go` → Phase 2 Track G (re-linked T-2G01/T-2G02 from L1 to the new L2 impl spec); `l2-{task,asset,scene,math}-system-go` → Phase 3 (under their L1 parents). All `Draft [Bootstrap]` — C29 keeps non-P1 specs Draft (no `examples/` validator yet). Pre-Planning Stabilization: 0 promoted (C29 gate for P2/P3 unmet — `examples/ecs/framework/` + P3 examples absent). No new atomic task IDs; Phase 3 atomic decomposition still deferred until Phase 2 ≥ 50% Done.
