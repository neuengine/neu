---
phase: 2
name: "Framework Primitives"
status: Todo
subsystem: "internal/ecs, pkg/"
requires:
  - "Phase 1 ECS Core (World, Component, Query, Scheduler)"
provides:
  - "Hierarchy + transform propagation"
  - "Time abstractions (real, virtual, fixed)"
  - "Input layer (keyboard, mouse, gamepad, touch)"
  - "State machines + computed states"
  - "Tick-based change detection (Ref/Mut wrappers)"
  - "App / Plugin / SubApp assembly"
  - "Multi-repo extension surface (pkg/editor, pkg/protocol)"
key_files:
  created: []
  modified: []
patterns_established: []
duration_minutes: ~
bootstrap: true
---

# Stage 2 Tasks — Framework Primitives

**Phase:** 2
**Status:** Todo
**Strategic Goal:** Land general-purpose engine framework on top of the ECS core. Atomic decomposition deferred until Phase 1 ≥ 80% Done.

## High-Level Checklist

- [ ] [T-2A] Hierarchy: `ChildOf`, `Children`, `Transform`, `GlobalTransform`, propagation system. ([l1-hierarchy-system.md](../specifications/l1-hierarchy-system.md), [l2-hierarchy-system-go.md](../specifications/l2-hierarchy-system-go.md))
- [ ] [T-2B] Time: `gametime` package, `Time`, `RealTime`, `VirtualTime`, `FixedTime`, `Timer`, `Stopwatch`. ([l1-time-system.md](../specifications/l1-time-system.md), [l2-time-system-go.md](../specifications/l2-time-system-go.md))
- [ ] [T-2C] Input: `ButtonInput[T]`, `AxisInput[T]`, `KeyCode`, `MouseButton`, `GamepadButton`, picking. ([l1-input-system.md](../specifications/l1-input-system.md), [l2-input-system-go.md](../specifications/l2-input-system-go.md), [l2-input-system-go-codes.md](../specifications/l2-input-system-go-codes.md))
- [ ] [T-2D] State: `State[S]`, `NextState[S]`, `SubState`, `ComputedState`, `DespawnOnExit`. ([l1-state-system.md](../specifications/l1-state-system.md), [l2-state-system-go.md](../specifications/l2-state-system-go.md))
- [ ] [T-2E] Change Detection: `Tick`, `ComponentTicks`, `Ref[T]`, `Mut[T]`, `RemovedComponents[T]`. ([l1-change-detection.md](../specifications/l1-change-detection.md), [l2-change-detection-go.md](../specifications/l2-change-detection-go.md))
- [ ] [T-2F] App / Plugin / SubApp / RunMode / DefaultPlugins. ([l1-app-framework.md](../specifications/l1-app-framework.md), [l2-app-framework-go.md](../specifications/l2-app-framework-go.md))
- [ ] [T-2G] Multi-repo extension scaffolding: `pkg/editor/`, `pkg/protocol/` boundary interfaces. ([l1-multi-repo-architecture.md](../specifications/l1-multi-repo-architecture.md))
- [ ] [T-2T] Validation track: hierarchy fuzz, fixed-step determinism, app lifecycle integration test, `examples/ecs/framework/` extension example.

## Atomic Decomposition

> Pending. Run `/magic.task main "decompose phase-2"` once Phase 1 ≥ 80% Done.

## Exit Criteria

1. Every L1 + L2 in Phase 2 promoted `Draft → Stable`.
2. `examples/ecs/framework/` validates the App/Plugin lifecycle end-to-end.
3. `pkg/editor/` and `pkg/protocol/` expose stable interface surfaces (RFC `multi-repo-architecture` ratified).
