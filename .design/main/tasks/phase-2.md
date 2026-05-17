---
phase: 2
name: "Framework Primitives"
status: Todo
subsystem: "internal/ecs, pkg/app, pkg/editor, pkg/protocol"
requires:
  - "Phase 1 ECS Core (World, Component, Query, Scheduler, Command, Event)"
provides:
  - "Hierarchy + transform propagation"
  - "Time abstractions (real, virtual, fixed)"
  - "Input layer (keyboard, mouse, gamepad, touch)"
  - "State machines + computed states"
  - "Tick-based change detection (Ref/Mut wrappers) — closes T-1D03 scaffold"
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
**Strategic Goal:** Land a general-purpose engine framework on top of the ECS core, validated end-to-end by `examples/ecs/framework/`. Closes the Phase 1 `T-1D03` change-detection scaffold and exposes the editor/tooling plugin surface.

## Track Overview

| Track | Domain | Critical-Path | Tasks |
| :--- | :--- | :---: | :--- |
| A | Hierarchy (`internal/ecs/hierarchy/`) | — | T-2A01..03 |
| B | Time (`internal/ecs/gametime/`) | — | T-2B01..03 |
| C | Input (`internal/ecs/input/`) | — | T-2C01..03 |
| D | State (`internal/ecs/state/`) | — | T-2D01..03 |
| E | Change Detection (`internal/ecs/changedetect/`, closes T-1D03) | **Yes** | T-2E01..03 |
| F | App Framework (`pkg/app/`) | Yes | T-2F01..03 |
| G | Multi-Repo Surface (`pkg/editor/`, `pkg/protocol/`) | — | T-2G01..02 |
| T | Validation (`examples/ecs/framework/`, cross-cutting) | — | T-2T01..04 |

Critical path: **E → F** (App `DefaultPlugins` integrates every plugin; Change Detection unblocks all `Changed`-filtered systems). Tracks A, B, C, D are file-independent and parallelizable. Track G is RFC-gated surface scaffolding (no editor implementation).

> **Bootstrap:** every task carries `[Bootstrap]` — the Phase 2 spec cohort stays `Draft` until `examples/ecs/framework/` validates the runtime (mirrors the C29 gate that Phase 1's `examples/ecs/poc/` satisfied).
> **RFC Gate (Track G):** `l1-multi-repo-architecture.md` is RFC. T-2G tasks build only the engine-side boundary surface; spec ratification + `Stable` promotion is an Exit Criterion, not a task deliverable.

## Atomic Checklist

### Track A — Hierarchy

- [ ] [T-2A01] `ChildOf` / `Children` relationship components + maintenance ops (attach, detach, reparent, recursive despawn). — `internal/ecs/hierarchy/relationship.go` [Bootstrap]
- [ ] [T-2A02] `Transform` / `GlobalTransform` components + propagation system (dirty-flag, depth-ordered, root→leaf). — `internal/ecs/hierarchy/transform.go` [Bootstrap]
- [ ] [T-2A03] Traversal helpers (`iter.Seq` descendants/ancestors) + `HierarchyPlugin` schedule wiring. — `internal/ecs/hierarchy/{traverse,plugin}.go` [Bootstrap]

### Track B — Time

- [ ] [T-2B01] `Time` / `RealTime` / `VirtualTime` clock model (delta, elapsed, scale, pause). — `internal/ecs/gametime/{clock,virtual}.go` [Bootstrap]
- [ ] [T-2B02] `FixedTime` + fixed-timestep accumulator (sub-stepping, max-catch-up clamp). — `internal/ecs/gametime/fixed.go` [Bootstrap]
- [ ] [T-2B03] `Timer` / `Stopwatch` (one-shot, repeating, finished/just-finished) + `TimePlugin`. — `internal/ecs/gametime/{timer,plugin}.go` [Bootstrap]

### Track C — Input

- [ ] [T-2C01] Generic `ButtonInput[T]` / `AxisInput[T]` state containers (pressed / just-pressed / just-released, per-frame clear). — `internal/ecs/input/{button,axis}.go` [Bootstrap]
- [ ] [T-2C02] Code reference types — `KeyCode` / `MouseButton` / `GamepadButton` / `GamepadAxis` / `Touch` enum tables. — `internal/ecs/input/codes.go` [Bootstrap]
- [ ] [T-2C03] Input ingestion systems (event→state, frame clear) + picking ray helper + `InputPlugin`. — `internal/ecs/input/{system,picking,plugin}.go` [Bootstrap]

### Track D — State

- [ ] [T-2D01] `State[S]` / `NextState[S]` resources + transition system (`OnEnter` / `OnExit` schedule sets). — `internal/ecs/state/{state,transition}.go` [Bootstrap]
- [ ] [T-2D02] `SubState` + `ComputedState` derivation; `DespawnOnExit` cleanup. — `internal/ecs/state/{substate,computed}.go` [Bootstrap]
- [ ] [T-2D03] `StatePlugin` wiring + `in_state` run-condition integration. — `internal/ecs/state/plugin.go` [Bootstrap]

### Track E — Change Detection (Critical Path · closes T-1D03)

- [x] [T-2E01] Per-row `ComponentTicks` (added/changed) in `Table` + `SparseSet`; world `LastChangeTick` / `IncrementChangeTick` integration. — `internal/ecs/changedetect/ticks.go` + `internal/ecs/component/{table,sparseset}.go` [Bootstrap]
- [x] [T-2E02] `Ref[T]` / `Mut[T]` wrappers (`IsAdded` / `IsChanged`); **replace the Phase 1 `passesPerRow` stub** with real Added/Changed tick comparison in `internal/ecs/query/filter.go`. — `internal/ecs/changedetect/wrappers.go` + `internal/ecs/query/filter.go` [Bootstrap]
- [x] [T-2E03] `RemovedComponents[T]` (event-backed) + `ChangeDetectionPlugin`. — `internal/ecs/changedetect/{removed,plugin}.go` [Bootstrap]

### Track F — App Framework (Critical Path)

- [ ] [T-2F01] `App` builder + `Plugin` / `PluginGroup` interfaces (build/finish lifecycle, dependency order, duplicate guard). — `pkg/app/{app,plugin}.go` [Bootstrap]
- [ ] [T-2F02] `SubApp` + `RunMode` (once / loop / headless) + main↔sub extract/apply step. — `pkg/app/{subapp,runmode}.go` [Bootstrap]
- [ ] [T-2F03] `DefaultPlugins` group (wires Hierarchy + Time + Input + State + ChangeDetection) + `App.Run`. — `pkg/app/defaultplugins.go` [Bootstrap]

### Track G — Multi-Repo Extension Surface (RFC-gated, surface only)

- [ ] [T-2G01] `pkg/protocol/` versioned IPC message contracts + serialization boundary. — `pkg/protocol/` [Bootstrap]
- [ ] [T-2G02] `pkg/editor/` engine-side boundary interfaces (extension points; **no** editor implementation, **no** `internal/` imports). — `pkg/editor/` [Bootstrap]

### Track T — Validation

- [ ] [T-2T01] `FuzzHierarchyReparent` — random spawn/reparent/despawn; invariants: no parent cycles, `Children`↔`ChildOf` consistency.
- [ ] [T-2T02] Fixed-step determinism golden test — identical input ⇒ identical state hash across 2 runs over 600 fixed steps.
- [ ] [T-2T03] App lifecycle integration test — `DefaultPlugins`, 100 ticks, state-transition + input-injection + hierarchy-mutation round-trip.
- [ ] [T-2T04] **Framework gate** — `examples/ecs/framework/` end-to-end; `Document History` updated in every Phase 2 spec (C26).

## Detailed Tracking

### [T-2A01] ChildOf / Children relationship

- **Spec:** [l2-hierarchy-system-go.md](../specifications/l2-hierarchy-system-go.md), [l1-hierarchy-system.md](../specifications/l1-hierarchy-system.md)
- **Status:** Todo [Bootstrap]
- **Assignment:** Agent
- **Verify:** `go test ./internal/ecs/hierarchy/ -run TestParentChild` — table-driven attach/detach/reparent + recursive-despawn cases pass; orphaned `ChildOf` cleaned.
- **Handoff:** Required by T-2A02 (propagation walks the relationship), T-2T01 (fuzz).
- **Notes:** Reuse Phase 1 `internal/ecs/entity/hierarchy.go` `ChildOf` if present (T-1G02 introduced a minimal `ChildOf`); promote to full relationship pair without breaking observer bubbling.

### [T-2A02] Transform propagation

- **Spec:** [l1-hierarchy-system.md](../specifications/l1-hierarchy-system.md), [l2-hierarchy-system-go.md](../specifications/l2-hierarchy-system-go.md)
- **Status:** Todo [Bootstrap]
- **Verify:** `go test ./internal/ecs/hierarchy/ -run TestPropagation` — deterministic world-space `GlobalTransform` across a 3-level chain; dirty-flag skips clean subtrees.
- **Handoff:** Consumed by T-2F03 (DefaultPlugins schedules propagation), T-2T02.
- **Notes:** Depth-ordered traversal (root→leaf) is mandatory for single-pass correctness; no recursion into already-clean subtrees (perf, C28 baseline).

### [T-2A03] Traversal + HierarchyPlugin

- **Spec:** [l2-hierarchy-system-go.md](../specifications/l2-hierarchy-system-go.md)
- **Status:** Todo [Bootstrap]
- **Verify:** `go test ./internal/ecs/hierarchy/` ≥ 95% pkg coverage, `-race` clean.
- **Handoff:** `HierarchyPlugin` consumed by T-2F03.

### [T-2B01] Real / Virtual clock model

- **Spec:** [l2-time-system-go.md](../specifications/l2-time-system-go.md), [l1-time-system.md](../specifications/l1-time-system.md)
- **Status:** Todo [Bootstrap]
- **Verify:** `go test ./internal/ecs/gametime/ -run TestVirtualScaling` — scale=0 (pause), scale=2 (fast), elapsed monotonic invariants pass.
- **Notes:** `gametime` package name (avoids stdlib `time` collision). Pure-data clocks (C24 §1 POD components).

### [T-2B02] Fixed-timestep accumulator

- **Spec:** [l1-time-system.md](../specifications/l1-time-system.md), [l2-time-system-go.md](../specifications/l2-time-system-go.md)
- **Status:** Todo [Bootstrap]
- **Verify:** `go test ./internal/ecs/gametime/ -run TestFixedStepDeterminism` — N accumulated steps deterministic for fixed dt; spiral-of-death clamp honored.
- **Handoff:** Required by T-2T02 (golden determinism), Phase 1 SubApp fixed schedule.

### [T-2B03] Timer / Stopwatch + TimePlugin

- **Spec:** [l2-time-system-go.md](../specifications/l2-time-system-go.md)
- **Status:** Todo [Bootstrap]
- **Verify:** `go test ./internal/ecs/gametime/` ≥ 95% pkg coverage, `-race` clean; `Timer.JustFinished()` true exactly one frame.

### [T-2C01] ButtonInput / AxisInput

- **Spec:** [l2-input-system-go.md](../specifications/l2-input-system-go.md)
- **Status:** Todo [Bootstrap]
- **Verify:** `go test ./internal/ecs/input/ -run TestButtonState` — press→just-pressed (1 frame)→pressed→release→just-released edges correct after frame clear.
- **Notes:** Generic over comparable code type `T`; `sync.Pool` not required (per-frame reused state map, C27 N/A for fixed-size state).

### [T-2C02] Code reference tables

- **Spec:** [l2-input-system-go-codes.md](../specifications/l2-input-system-go-codes.md)
- **Status:** Todo [Bootstrap]
- **Verify:** `go test ./internal/ecs/input/ -run TestCodeTables` — stringer round-trip; per-enum count assertion matches the spec reference table.

### [T-2C03] Input systems + picking + InputPlugin

- **Spec:** [l1-input-system.md](../specifications/l1-input-system.md), [l2-input-system-go.md](../specifications/l2-input-system-go.md)
- **Status:** Todo [Bootstrap]
- **Verify:** `go test ./internal/ecs/input/` ≥ 95% pkg coverage, `-race` clean; picking ray helper unit-tested against a known camera/viewport.
- **Handoff:** `InputPlugin` consumed by T-2F03.

### [T-2D01] State / NextState + transitions

- **Spec:** [l2-state-system-go.md](../specifications/l2-state-system-go.md)
- **Status:** Todo [Bootstrap]
- **Verify:** `go test ./internal/ecs/state/ -run TestTransition` — `OnExit(old)` runs before `OnEnter(new)`; no-op when `NextState == State`.

### [T-2D02] SubState / ComputedState / DespawnOnExit

- **Spec:** [l1-state-system.md](../specifications/l1-state-system.md), [l2-state-system-go.md](../specifications/l2-state-system-go.md)
- **Status:** Todo [Bootstrap]
- **Verify:** `go test ./internal/ecs/state/ -run TestComputedState` — derived state recomputes on source change; `DespawnOnExit` entities removed at transition.

### [T-2D03] StatePlugin + in_state condition

- **Spec:** [l2-state-system-go.md](../specifications/l2-state-system-go.md)
- **Status:** Todo [Bootstrap]
- **Verify:** `go test ./internal/ecs/state/` ≥ 95% pkg coverage, `-race` clean; `in_state(X)` run-condition gates a system on/off across a transition.
- **Handoff:** `StatePlugin` + `in_state` consumed by T-2F03; integrates Phase 1 `scheduler.RunCondition` (T-1E03).

### [T-2E01] ComponentTicks integration

- **Spec:** [l2-change-detection-go.md](../specifications/l2-change-detection-go.md)
- **Status:** Done [Bootstrap]
- **Verify:** `go test ./internal/ecs/changedetect/ -run TestComponentTicks` — PASS (8 subtests); added-tick set on insert, changed-tick advances only on explicit mutation, reads side-effect free.
- **Handoff:** **Critical path.** Required by T-2E02 (filter rewrite), T-2F03.
- **Notes:** Extends Phase 1 `Table`/`SparseSet` (T-1B02) with per-row tick storage; world tick model from T-1C01.
- **Changes:** New leaf pkg `internal/ecs/changedetect/ticks.go` — `Tick`, `ComponentTicks` (Added/Changed, `NewComponentTicks`/`IsAdded`/`IsChanged`/`SetChanged`), `ColumnTicks` aggregate (`Observe`/`Reset`/`MayHaveChanged`/`MayHaveAdded`) for O(1) column skip; leaf (no ecs-core imports) to avoid world→component→changedetect→world cycle. `component.Table` + `SparseSet` extended with dense tick arrays parallel to rows (tracked for tag columns too), swap-and-pop mirrored, plus `Ticks`/`TicksByID`/`StampAddedByID`/`StampChangedByID`/`SetTicksByID`/`ColumnTicks[ByID]` (Table) and `Ticks`/`StampAdded`/`StampChanged`/`SetTicks`/`ColumnTicks` (SparseSet). Verify: changedetect 100% cov, component 97.2% (≥ pre-existing 96.6%), full Phase 1 suite 12/12 green, `-race` clean. World-side auto-stamp + archetype-migration tick-carry deliberately deferred to T-2E02/E03 (no scope creep).

### [T-2E02] Ref/Mut wrappers + closes T-1D03 scaffold

- **Spec:** [l1-change-detection.md](../specifications/l1-change-detection.md), [l2-change-detection-go.md](../specifications/l2-change-detection-go.md)
- **Status:** Done [Bootstrap]
- **Verify:** `go test ./internal/ecs/query -run TestAddedChangedFilter` — PASS (3 subtests: Added selectivity, Changed selectivity, baseline catch-up after ClearTrackers). The Phase 1 accept-all `passesPerRow` stub is eliminated; scaffold test `TestAddedAndChangedScaffoldBehavior` replaced and `TestQuery2/3CountWithPerRowFilter` updated to assert real tick semantics.
- **Handoff:** Unblocks every Phase 2+ system that relies on `Changed`/`Added` filters (cascade — see Planning Audit).
- **Notes:** Designed consumer of T-1D03's deferred seam (STATE.md 2026-04-29 pattern note).
- **Changes:** `changedetect/wrappers.go` — `Ref[T]` (read-only, no-mark) + `Mut[T]` (auto-marks Changed on `Value()`; `prevChanged` captured at construction so `IsChanged` is call-order independent; `SetChanged`/`BypassChangeDetection`). `query/filter.go` — `passesPerRow` rewritten to strict-`>` per-row `Added`/`Changed` comparison vs `world.LastChangeTick()` baseline, via `rowTicks` helper dispatching Table (`TicksByID`) vs SparseSet (`ss.Ticks`) like `fetchComponent`; AND semantics across records. **Bootstrap deviation from planner's 2-file key_files** (Planning Audit pre-flagged Track E "reaches into Phase 1 files"): the stub's `(w, perRow)` signature was structurally insufficient, so all 8 call sites in `query/{query1,query2,query3,par}.go` extended to `(w, arch, row, perRow)` (one-line guards unchanged); and `world/entity_ops.go` wired tick stamping (`addEntityToArchetype` carry param + `captureTableTicks`/`idSet`: Added on fresh insert, history carried verbatim across archetype migration, Changed on in-place overwrite and overwrite-during-migration) — without it the comparison is untestable (all slots `{0,0}`). Verify: changedetect 100% cov, query 95.8% (≥95% gate; residual = unreachable defensive nil-guards), component 97.2%, world 96.1%, full Phase 1 suite 12/12 green, `-race` clean on all affected pkgs.

### [T-2E03] RemovedComponents + ChangeDetectionPlugin

- **Spec:** [l2-change-detection-go.md](../specifications/l2-change-detection-go.md)
- **Status:** Done [Bootstrap]
- **Verify:** `go test ./internal/ecs/changedetect/` ≥ 95% pkg coverage, `-race` clean; `RemovedComponents[T]` drained per tick (double-buffered like Phase 1 EventBus T-1G01).
- **Handoff:** `ChangeDetectionPlugin` consumed by T-2F03.
- **Changes:** `changedetect/removed.go` — `RemovedComponents[T]` (Append/Iter/Len/DrainBefore with two-frame retention window INV-5; underflow guard for lastTick < 2; GC-safe clear on pruned tail) + `RemovedRegistry` (type-erased `[]func(Tick)` drain callbacks, avoids generics at world boundary). `changedetect/plugin.go` — `ChangeDetectionPlugin` stub. `world/world.go` — `removedCallbacks map[component.ID]func(entity.Entity, changedetect.Tick)` field + `ClearTrackers` updated to drain registered `RemovedRegistry` via `Resource[*changedetect.RemovedRegistry]` double-pointer pattern. `world/removed.go` — `RegisterRemovedComponents[T]` (idempotent; wires drain callback into shared registry; registers per-component removal hook) + `ensureRemovedRegistry`. `world/entity_ops.go` — removal hooks fired in both `RemoveByID` and `Remove[T]` before archetype migration. Verify: changedetect 100% coverage, `-race` clean.

### [T-2F01] App + Plugin / PluginGroup

- **Spec:** [l2-app-framework-go.md](../specifications/l2-app-framework-go.md)
- **Status:** Todo [Bootstrap]
- **Verify:** `go test ./pkg/app/ -run TestPluginLifecycle` — `Build` then `Finish` ordering; duplicate-plugin registration guarded; plugin dependency order respected.
- **Notes:** `pkg/app` is a **public** package (C24 boundary) — must not leak `internal/ecs` types beyond the documented façade (reuse `pkg/ecs` aliases from T-1T01).

### [T-2F02] SubApp + RunMode

- **Spec:** [l1-app-framework.md](../specifications/l1-app-framework.md), [l2-app-framework-go.md](../specifications/l2-app-framework-go.md)
- **Status:** Todo [Bootstrap]
- **Verify:** `go test ./pkg/app/ -run TestSubAppExtract` — sub-app world isolation; extract step copies only declared resources; `RunMode` once/loop/headless each terminate correctly.

### [T-2F03] DefaultPlugins + App.Run

- **Spec:** [l2-app-framework-go.md](../specifications/l2-app-framework-go.md)
- **Status:** Todo [Bootstrap]
- **Verify:** `go test ./pkg/app/` ≥ 90% pkg coverage, `-race` clean; `DefaultPlugins` boots Hierarchy+Time+Input+State+ChangeDetection and a 10-tick `App.Run` completes with all plugin schedules firing.
- **Handoff:** **Critical path tail** — blocks on T-2A03, T-2B03, T-2C03, T-2D03, T-2E03. Consumed by T-2T03/T-2T04.
- **Notes:** Cascade risk — if any plugin track slips, T-2F03 and the validation track stall (see Planning Audit).

### [T-2G01] pkg/protocol IPC contracts

- **Spec:** [l1-multi-repo-architecture.md](../specifications/l1-multi-repo-architecture.md) *(RFC)*
- **Status:** Todo [Bootstrap]
- **Verify:** `go build ./pkg/protocol/` + `go test ./pkg/protocol/ -run TestProtocolRoundTrip` — versioned message encode/decode stable; backward-compat field rule documented.
- **Notes:** RFC-gated — surface only. Spec ratification is an Exit Criterion, not part of this task.

### [T-2G02] pkg/editor boundary interfaces

- **Spec:** [l1-multi-repo-architecture.md](../specifications/l1-multi-repo-architecture.md) *(RFC)*
- **Status:** Todo [Bootstrap]
- **Verify:** `go build ./pkg/editor/` + architecture-guard test asserting **zero** `internal/` imports in `pkg/editor/`.
- **Notes:** Engine-side extension points only (no editor implementation — that lives in the extracted `.design-editor/` repo per INDEX note).

### [T-2T01] Hierarchy fuzz

- **Goal:** Verify T-2A relationship/propagation invariants under random churn.
- **Method:** `go test -run x -fuzz FuzzHierarchyReparent -fuzztime 30s ./internal/ecs/hierarchy/` — no crashes; assert no parent cycles + `Children`↔`ChildOf` bijection.
- **Status:** Todo [Bootstrap]

### [T-2T02] Fixed-step determinism golden

- **Goal:** Verify T-2B fixed-step is deterministic (network/replay precondition).
- **Method:** `go test -run TestFixedStepGolden ./internal/ecs/gametime/` — two identical-input runs over 600 fixed steps produce identical state hashes.
- **Status:** Todo [Bootstrap]

### [T-2T03] App lifecycle integration

- **Goal:** Verify T-2F integrates all plugin tracks.
- **Method:** `go test -race ./pkg/app/ -run TestAppLifecycleIntegration` — `DefaultPlugins`, 100 ticks, state transition + input injection + hierarchy mutation round-trip.
- **Status:** Todo [Bootstrap]

### [T-2T04] examples/ecs/framework — Phase 2 gate

- **Spec:** [l1-examples-framework.md](../specifications/l1-examples-framework.md), all Phase 2 specs (referenced from `Document History`)
- **Status:** Todo [Bootstrap]
- **Goal:** End-to-end validation exercising every Phase 2 spec. Output is the gate that lets `magic.spec` promote the Phase 2 cohort `Draft → Stable`.
- **Method:** `go run ./examples/ecs/framework` deterministic; `go test -race ./examples/ecs/framework` passes.
- **Acceptance:**
  1. App with `DefaultPlugins` runs ≥ 100 ticks headless.
  2. A state machine drives ≥ 2 transitions; `DespawnOnExit` observed.
  3. Hierarchy of ≥ 3 levels propagates `GlobalTransform` correctly.
  4. `Changed`/`Added` filters demonstrably select only mutated rows (proves T-2E02 closed the T-1D03 scaffold).
  5. `Document History` updated in every Phase 2 spec referencing the framework example path.

## Planning Audit (`@role:planner`)

- **Optimism Bias:** 24 atomic tasks across 8 tracks. Tracks A–D are genuinely independent (separate `internal/ecs/*` dirs) and parallelizable. Track E is smaller in file count but **highest risk** — it reaches back into Phase 1 `query/filter.go` and `component/{table,sparseset}.go`.
- **Hidden Dependencies:** Track E (Change Detection) is the critical path: T-2E02 mutates a Phase-1 file (`query/filter.go`) and unblocks every `Changed`-filtered system. Track F's `DefaultPlugins` (T-2F03) is a join point — it cannot complete until A03/B03/C03/D03/E03 all land.
- **Cascade Risk:** If Track E slips, the framework example (T-2T04) cannot prove Acceptance #4, and Phase 2 cannot satisfy its Exit Criteria → C29-style gate stays closed → Phases 4–8 remain `Hold` (C-002). Mitigation: schedule Track E first alongside A/B/C/D; treat E as the critical path the strongest contributor owns.
- **RFC Cascade (Track G):** `l1-multi-repo-architecture` is RFC. T-2G builds surface only; if the RFC is rejected or reshaped during review, T-2G01/02 interfaces churn but no other track depends on them — blast radius is contained to `pkg/editor` + `pkg/protocol`.

## Exit Criteria

1. Every Phase 2 L1 + L2 spec validated by `examples/ecs/framework/` (C29-style gate; promotion `Draft → Stable` deferred to `magic.spec` post-validation).
2. Phase 1 `T-1D03` change-detection scaffold replaced with real tick comparison (T-2E02) — no remaining accept-all stub in `query/filter.go`.
3. `examples/ecs/framework/` validates the App/Plugin lifecycle end-to-end (`go test -race` clean).
4. `pkg/editor/` and `pkg/protocol/` expose stable interface surfaces; `l1-multi-repo-architecture` RFC ratified (governance via `/magic.spec`).
