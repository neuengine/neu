# Master Task Index (Registry)

**Version:** 1.7.0
**Generated:** 2026-05-17
**Based on:** .design/main/PLAN.md v1.6.0
**Based on RULES:** .design/RULES.md v1.7.1 *(⚠ see RULES Parity note in Meta — RULES.md header lags its own Document History/body by one minor)*
**Execution Mode:** Parallel (per C3)
**Status:** Active
**Mode:** Phase 1 specs → `Stable` (Bootstrap deactivated for P1). Phase 2+ remain `[Bootstrap]` — per-phase `examples/` gate pending (P2 → `examples/ecs/framework/`; P3 → `examples/{async,asset,scene,math}/`).

## Overview

Tactical registry of all phases and their statuses. Atomic checklists live in per-phase workbooks under `tasks/phase-{N}.md`.

## Active Phases

| Phase | Description | Status |
| :--- | :--- | :--- |
| [Phase 1](tasks/phase-1.md) | ECS Core POC — world, entities, components, queries, scheduler, validating `examples/ecs/poc/` | Done |
| [Phase 2](tasks/phase-2.md) | Framework Primitives — hierarchy, time, input, state, change-detection, app/plugin (24 atomic tasks, Tracks A–G + T) | Done |
| [Phase 3](tasks/phase-3.md) | Assets, Math & Concurrency — task pool, asset server, scene, math (L1+L2 specs drafted; atomic decomposition deferred to Phase 2 ≥ 50%) | Todo |
| [Phase 4](tasks/phase-4.md) | Render Pipeline — render graph, mesh, materials, camera, post-processing | Hold |
| [Phase 5](tasks/phase-5.md) | Content Systems — audio, asset codecs, 2D, animation, tweening | Hold |
| [Phase 6](tasks/phase-6.md) | UI, Tooling & Quality — definition, window, UI, build, CLI, platform, AI, plugins, visual graph, examples, errors, benchmark, codegen | Hold |
| [Phase 7](tasks/phase-7.md) | Networking & Hot-Reload — profiling, transport, replication, sync, RPC, hot-reload | Hold |
| [Phase 8](tasks/phase-8.md) | Physics & Scripting — physics server, bodies, colliders, queries, joints, character, scripting | Hold |

## Archived Phases

| Phase | Description | Archive |
| :--- | :--- | :--- |
| Phase 0 (legacy) | POC implementation (pre-Bootstrap layout) | [archives/tasks/01-poc-implementation.md](archives/tasks/01-poc-implementation.md) |

## Cross-Phase Status Counters

- **Total atomic tasks (Phase 1)**: 27 (Tracks A–I + T) — Done
- **Total atomic tasks (Phase 2)**: 24 (Tracks A–G + T) — decomposed 2026-05-16; critical path E → F
- **Total atomic tasks (Phase 6)**: 46 (Tracks A–P + T) — Track P (Visual Graph System, 4 tasks) added 2026-05-14
- **Phases 3–5, 7–8**: structural workbooks, atomic decomposition deferred to per-phase `/magic.task {workspace} "decompose phase-N"` invocations.

## Meta Information

- **Last Updated**: 2026-05-17
- **Maintainer**: Core Team
- **RULES Parity (⚠ drift, non-blocking)**: `.design/RULES.md` header declares `**Version:** 1.7.1`, but its Document History records a `1.8.0` row (C31 — sync-docs Safety Gate) and C31 is present in the body (§7). The version-of-record (header) lags content by one minor. `magic.task` does not own `RULES.md` — recorded parity uses the header value (1.7.1). **Action required:** run `/magic.rule` to reconcile the RULES.md header to 1.8.0 (content already present, no rule change needed — header bump + Document-History consistency only).
- **Phase 1 Progress**: 27 / 27 (100%) — All tracks complete. Validation Track T done: `pkg/ecs` public API, race/fuzz/golden tests, `examples/ecs/poc/` end-to-end POC. C29 unblocked. Phase 1 → Done.
- **Pre-Planning Stabilization (2026-05-17)**: 17 P1 ECS Core specs promoted `Draft → Stable` via Trust Mode batch (C29 gate satisfied by T-1T05). Breakdown: 9 L1 concept specs + 8 L2 Go impl specs. Bootstrap Mode deactivated for Phase 1; Phase 2+ remain `[Bootstrap]` until `examples/ecs/framework/` validates the runtime. Stable: 17 / 84.
- **Phase 2 Atomic Decomposition (2026-05-16)**: 24 atomic tasks across Tracks A (Hierarchy), B (Time), C (Input), D (State), E (Change Detection — critical path, closes T-1D03 stub), F (App Framework — critical path tail), G (Multi-Repo surface, RFC-gated), T (Validation). Phase 2 is the active phase; `examples/ecs/framework/` is the Phase 2 promotion gate. Execute via `/magic.run main`.
- **Phase 6 Atomic Decomposition (2026-05-01)**: 42 atomic tasks across Tracks A–O + T. Tracks N (Plugin Distribution, 4 tasks) and O (AI API Plugin, 5 tasks) are new. Phase remains in `Hold` until Phase 1–3 reach `Stable`.
- **Phase 6 Track P Addition (2026-05-14)**: Track P (Visual Graph System, 4 tasks T-6P01..04) registered in PLAN.md to match prior phase-6.md workbook decomposition. Phase total: 46.
- **Registry Sync (2026-05-17)**: INDEX v2.24.0 → v2.25.0 (84 → 89 specs). 5 orphan L2 specs placed: `l2-multi-repo-architecture-go` → Phase 2 Track G (re-linked T-2G01/T-2G02 from L1 to the new L2 impl spec); `l2-{task,asset,scene,math}-system-go` → Phase 3 (under their L1 parents). All `Draft [Bootstrap]` — C29 keeps non-P1 specs Draft (no `examples/` validator yet). Pre-Planning Stabilization: 0 promoted (C29 gate for P2/P3 unmet — `examples/ecs/framework/` + P3 examples absent). No new atomic task IDs; Phase 3 atomic decomposition still deferred until Phase 2 ≥ 50% Done.
