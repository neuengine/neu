# Project State

<!-- STATE.md — live project memory. Read FIRST in every workflow session. -->
<!-- Maximum 100 lines. Agent updates AFTER each completed action. -->

**Workspace:** main
**Updated:** 2026-05-19 06:46
**Phase:** 4 — Render Pipeline
**Status:** Active

## Current Position

- **Task:** T-4A02 Done — RenderGraph (Kahn-DAG, barriers, INV-1/2/3). Next: T-4A03.
- **Next Action:** Run /magic.run main → T-4A03 RenderSubApp + 4-phase extract (Collect/Extract/Prepare/Draw) + single-extract frame guard; A03 gates B03/C01

## Progress

```
Phase 1: [27/27] ████████ 100% ✓ Done
Phase 2: [24/24] ████████ 100% ✓ Done
Phase 3: [18/18] ████████ 100% ✓ Done
Phase 4: [ 2/19] █░░░░░░░  11% ▶ Active
Overall: [71/88] ███████░  81%
```

## Recent Decisions

<!-- Last 3-5 locked decisions. Older entries → archived to PLAN.md -->

- 2026-05-19 **Done:** T-4A02 — `internal/render/graph.go` + `pkg/render/phase.go`. `RenderGraph`: producer→consumer edges from shared RIDs, Kahn topo sort (sorted ready frontier → deterministic) reusing scheduler DAG pattern (C30); `ErrRenderGraphCycle` (errors.Is-compatible wrapper naming passes, mirrors `dagCycleError`), self-cycle rejected; `Barrier` transition list (golden-tested on diamond); INV-3 pin via in-package `*ResourceTracker` with external-vs-transient input distinction (transient produced in-graph exempt+pinned, external zero-ref ⇒ `ErrResourceReleased`); `ErrRenderGraphNotBuilt` guard. `RenderPhase` public enum. `-race` clean 7/7 + no T-4A01 regression. **Pattern:** `RenderPhase`→`pkg/render` (public, cross-spec); graph stays `internal/render` (consumers lighting/postpass are internal). Next: T-4A03.
- 2026-05-19 **Done:** T-4A01 — `pkg/render/backend.go` + `internal/render/{server,resources}.go`. `RID` = kind8|gen24|idx32 bit-pack (zero = nil). `Server`: sync `Allocate`, deferred `Initialize`/`Submit` with goroutine-bound inline fast-path (drains queue before inline cmd → global FIFO), `Drain` sole consumer, `Close`→`ErrRenderClosed`. `ResourceTracker`: refcount→0 records freed-frame; `EndFrame(f)` destroys only `freed < f` (never in-flight — INV-3); `Retain` cancels pending. `-race` clean (256-goroutine Submit dedup; deferred-delete timing). **Pattern:** internal/render imports pkg/render aliased `gpu` (RIDs caller-held → public). **Spec tightening (Bootstrap):** `Submit` returns `error` per §Error Handling. Track A root done — unblocks A02/A03/A04 then B/C/D/E.

<!-- Phase 1–3 decisions archived (Done; see PLAN.md Document History + archives/tasks/). -->
- 2026-04-30 **Pattern (carried):** ECS hot paths use `sync.Pool` with a `*SliceBuf[T]` wrapper for 0-alloc slice reuse; deferred mutations apply after all systems run, before tick boundary (single sync point). Reused by render: Server FIFO drain, ResourceTracker deferred-delete.

## Blockers

<!-- Empty if none. Format: [severity] description -->

<!-- (none) -->

## Blocking Constraints

<!-- Anti-patterns discovered through real failures. MANDATORY reading. -->
<!-- Agent MUST explicitly acknowledge each constraint before working. -->

- [C-001] **C29 Promotion Gate**: No P1 spec may be promoted Draft → Stable until `examples/ecs/poc/` validates the runtime end-to-end (T-1T05).
- [C-002] **STOP FACTOR (Phase ≥ 4 Hold)**: Phases 4–8 stay in `Hold` until Phase 1 POC is validated AND Phase 2 App Framework reaches `Stable`. No premature implementation work in those subsystems.
- [C-003] **C24 Stdlib Priority**: Engine core MUST have zero external Go deps. Any third-party package requires explicit justification recorded in an ADR.
- [C-004] **C27 GC Compensation**: Hot-path allocations (commands, events, transient views) MUST flow through `sync.Pool`. Validation Track verifies ≤1 alloc/op for `BenchmarkCommandFlush`.
- [C-005] **C28 Race Gate**: All concurrent tests MUST pass with `-race`; CI blocks otherwise.

## Session Continuity

**Last Session Ended:** 2026-04-25 13:27
**Handoff File:** none
**Bootstrap Mode:** true
