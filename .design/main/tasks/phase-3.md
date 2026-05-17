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

> Pending. Run `/magic.task main "decompose phase-3"` once Phase 2 ≥ 50% Done.

## Notes

- L2 Go specs for task/asset/scene/math are **drafted** (2026-05-17): [l2-task-system-go.md](../specifications/l2-task-system-go.md), [l2-asset-system-go.md](../specifications/l2-asset-system-go.md), [l2-scene-system-go.md](../specifications/l2-scene-system-go.md), [l2-math-system-go.md](../specifications/l2-math-system-go.md). All `Draft [Bootstrap]` — C29 holds them until `examples/{async,asset,scene,math}/` validates. Implement against the L2 contracts, not directly from L1.
- Phase 3 is the **last** Bootstrap phase that runs without the C29 unblock. Phases 4+ require POC validation.
- Atomic decomposition still deferred (see below) — run `/magic-task main "decompose phase-3"` once Phase 2 ≥ 50% Done.
