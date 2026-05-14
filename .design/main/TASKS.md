# Master Task Index (Registry)

**Version:** 1.3.0
**Generated:** 2026-05-14
**Based on:** .design/main/PLAN.md v1.2.0
**Based on RULES:** .design/RULES.md v1.7.1
**Execution Mode:** Parallel (per C3)
**Status:** Active
**Mode:** `[Bootstrap]` — full Draft cohort, C29 unblock pending

## Overview

Tactical registry of all phases and their statuses. Atomic checklists live in per-phase workbooks under `tasks/phase-{N}.md`.

## Active Phases

| Phase | Description | Status |
| :--- | :--- | :--- |
| [Phase 1](tasks/phase-1.md) | ECS Core POC — world, entities, components, queries, scheduler, validating `examples/ecs/poc/` | Done |
| [Phase 2](tasks/phase-2.md) | Framework Primitives — hierarchy, time, input, state, change-detection, app/plugin | Todo |
| [Phase 3](tasks/phase-3.md) | Assets, Math & Concurrency — task pool, asset server, scene, math | Todo |
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

- **Total atomic tasks (Phase 1)**: 27 (Tracks A–I + T)
- **Total atomic tasks (Phase 6)**: 46 (Tracks A–P + T) — Track P (Visual Graph System, 4 tasks) added 2026-05-14
- **Phases 2–5, 7–8**: structural workbooks, atomic decomposition deferred to per-phase `/magic.task {workspace} "decompose phase-N"` invocations.

## Meta Information

- **Last Updated**: 2026-05-14
- **Maintainer**: Core Team
- **Phase 1 Progress**: 27 / 27 (100%) — All tracks complete. Validation Track T done: `pkg/ecs` public API, race/fuzz/golden tests, `examples/ecs/poc/` end-to-end POC. C29 unblocked. Phase 1 → Done.
- **Phase 6 Atomic Decomposition (2026-05-01)**: 42 atomic tasks across Tracks A–O + T. Tracks N (Plugin Distribution, 4 tasks) and O (AI API Plugin, 5 tasks) are new. Phase remains in `Hold` until Phase 1–3 reach `Stable`.
- **Phase 6 Track P Addition (2026-05-14)**: Track P (Visual Graph System, 4 tasks T-6P01..04) registered in PLAN.md to match prior phase-6.md workbook decomposition. Phase total: 46.
