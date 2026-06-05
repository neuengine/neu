---
phase: 7
name: "Networking & Hot-Reload"
status: Active
subsystem: "pkg/diag/profiling, internal/hotreload, pkg/protocol, cmd/hot-reload-daemon (active slice); pkg/net (deferred)"
requires:
  - "Phase 2 App Framework Stable (satisfied)"
  - "Phase 1 Scheduler Stable (satisfied)"
  - "Phase 6 pkg/diag span seed + asset/shader hot-reload bridge + pkg/protocol (satisfied)"
provides:
  - "Profiling protocol: build-tag spans, pooled zero-alloc, pprof + Chrome TEF exporters (Tracy deferred)"
  - "Hot-reload: code restart with transactional state snapshot + entity-ID pinning + shader hot-swap"
  - "DEFERRED: multiplayer boundaries, UDP transport, replication, sync models, RPC, network diagnostics"
key_files:
  created: []
  modified: []
patterns_established: []
duration_minutes: ~
bootstrap: true
hold_reason: "Hot-reload + profiling slice UNFROZEN (App Framework + Scheduler Stable). Multiplayer stack stays Bootstrap pending its L2 specs."
---

# Stage 7 Tasks — Networking & Hot-Reload

**Phase:** 7
**Status:** Active (hot-reload + profiling slice)
**RULES:** v1.9.0

## Active Slice — Atomic Tasks

The hot-reload + profiling starting slice, decomposed from the two Draft L2 contracts authored
via `/magic.spec`. Both tracks are `[Bootstrap]` (L1 parents Draft → tentative until Stable) and
**parallelizable** — the only cross-link is a soft `Reload:{phase}` span (no hard dependency).
Per CLAUDE.md: every new Go file ≥80% coverage, `-race` on concurrent tests, `gofmt`/`vet`/`modernize` clean.

### Track A — Profiling Protocol ([l2-profiling-protocol-go.md](../specifications/l2-profiling-protocol-go.md) → [l1-profiling-protocol.md](../specifications/l1-profiling-protocol.md))

Package `pkg/diag/profiling/`. Generalizes the build-tag span seed already in `l2-diagnostic-system-go` (T-6C03).

- [x] **T-7A01 — Untagged data + API surface.** `pkg/diag/profiling/{span,config,exporter}.go` (all-build): `Span` (pure data + read accessors), `SpanCategory`/`ExporterType` enums + `String`, `KeyValue`, `SpanID`, `ProfilingConfig` resource + `DefaultProfilingConfig`, `ProfileExporter` interface + `NopExporter` + `MultiExporter` (error-joining fan-out). _Changes: untagged data/config/interface surface; 8 unit tests._
- [x] **T-7A02 — Dual-file build-tag span machinery.** `profile_off.go` (`//go:build !profiling` — inlinable no-op stubs, authored first) + `profile_on.go` (`//go:build profiling` — `sync.Pool` spans, `context.Context` parent chain via `ctxKey`, monotonic `nowNanos`, `MarkFrame` atomic counter, `MinSpanDuration` filter, `Set{Config,Exporter,ThreadIDFunc}`). **Reconciliation: used the engine's existing `profiling` tag (matching `pkg/diag/span.go`), not the spec's `profile`.** INV-1/2/3/4. _Changes: live + no-op span paths; zero-alloc verified._
- [x] **T-7A03 — Stdlib exporters + plugin wiring.** `exporter_chrome.go` (Trace Event Format JSON, `encoding/json`), `exporter_pprof.go` (span-timing aggregator; true pprof sample-labels noted as dispatch-level follow-up), `gpu.go` (untagged `GPUSpan`/`GPUTimingCollector` stub), `internal/profiling/plugin{,_off}.go` (`ProfilingPlugin`: injected exporter + `Last`-schedule `MarkFrame`), scheduler `System:{name}` auto-wrap via build-tagged `dispatchRun` (`dispatch_profile.go`/`dispatch_noprofile.go`) — **non-breaking single-site extraction, System interface unchanged**. **Tracy REJECTED — C32 ADR #3 added (RULES 1.9.0→1.10.0).** _Changes: 2 stdlib exporters + plugin + scheduler span hook._
- [x] **T-7A04 — Validation (Track A).** `TestSpanPoolZeroAlloc` (pool path `AllocsPerRun==0`, INV-2), `TestBeginEndBoundedAlloc` (full round-trip ≤1 — the `context.WithValue` wrapper), `TestInactiveZeroAlloc` (no-op path 0 allocs in default build, INV-1), `TestChromeExporterTEF` (TEF golden), parent-linkage/min-duration/frame-mark/disabled-short-circuit. **Both `profiling` and default tag configs build + test green.** Coverage: profiling 96.1%, internal/profiling 80.0%. _Changes: zero-alloc + TEF + dual-tag conformance._

### Track J — Hot-Reload ([l2-hot-reload-go.md](../specifications/l2-hot-reload-go.md) → [l1-hot-reload.md](../specifications/l1-hot-reload.md))

Package `internal/hotreload/` — all `//go:build editor`. Reuses the `DynamicScene` codec (`l2-scene-system-go`), `TypeRegistry`, the stdlib dev `FileWatcher` (`l2-asset-system-go`), `EntityAllocator`, and `pkg/protocol`.

- [x] **T-7J01 — Wire contract + snapshot format.** Protocol Kinds (`HotReloadPrepare/Ready/Failed`, `ShaderError`, `ShaderReloaded`, `ReloadMetrics`) **already present + Decode-wired + round-trip-tested** in `pkg/protocol` (Phase-6 multi-repo surface) — verified, no change needed. New: `internal/ecs/typereg` `RegisterAlias(oldName, type)` migration hook (additive; `ResolveByName` reads the alias map; rejects unregistered-type + name-collision, idempotent; 97.7% cov) + `internal/hotreload/format.go` (`//go:build editor`): `SnapshotHeader` (engine+format-version `CompatibleWith` guard, INV-1), `AppState`/`CameraSnapshot` (portable float arrays, no render-pkg dep)/`TimeSnapshot`, `Snapshot` (reuses `scene.SerializedScene` as the World payload), `DroppedComponent` (INV-2). 100% cov, JSON round-trip + compatibility tests. _Changes: RegisterAlias + editor snapshot format structs._
- [x] **T-7J02 — Snapshot encode + transactional restore + entity-ID pinning.** `format.go` revised to a **self-contained ID-faithful encoding** (reconciliation: no `DynamicScene→SerializedScene` serializer exists + SerializedScene is positional/drops the generation → it can't satisfy INV-5; uses per-entity `{ID, []ComponentSnapshot{type, json}}` reusing the `scene.WorldReader` extraction + typereg + JSON coercion). `snapshot.go` `BuildSnapshot` (capture via `EachArchetype`, INV-2 dropped-manifest for types absent from the typereg). `restore.go` `EncodeSnapshot`/`DecodeSnapshot` (corrupt → fail at decode, before any mutation, INV-1) + `ApplySnapshot` (header `CompatibleWith` guard INV-1; **staging-then-apply**; per-component drop INV-2). New `EntityAllocator.PlaceAt`/`RebuildFreeList` (additive; **caught + fixed a gen-0-gap→null-sentinel bug**). **INV-5 insight: pinning IDs makes remapping unnecessary** — the "IdentityRemapper" is the identity function. Cov entity 100%, hotreload 89.7%. _Changes: ID-pinning snapshot/restore + allocator place/rebuild._
- [x] **T-7J03 — Shader hot-swap + orchestrator + plugin.** `shader.go` `ShaderReloader` (compile-into-NEW-handle, keep old on error INV-3, double-buffered `ReleasePending`; injected `ShaderCompiler`+`EventSink` seams → decoupled from render/transport). `scope.go` `ClassifyChange` (go/parser + go/printer type-shape diff → SystemOnly vs ComponentType; parse-failure → SystemOnly fallback). `orchestrator.go` `RouteFile` (ext→ReloadMode) + `ReloadOrchestrator` (injected `BuildFunc` seam over `os/exec` go build). `plugin.go` `HotReloadPlugin` (`Last`-schedule shader release) + `SnapshotPathFromArgs` (`--hot-reload=` flag). `cmd/hot-reload-daemon` (editor main + `!editor` stub so `go build ./...` stays green). _Changes: shader swap + AST classifier + orchestrator + plugin + daemon._
- [x] **T-7J04 — Validation (Track J).** `internal/releaseguard` `TestNoHotReloadInRelease` (go-list guard: `internal/hotreload` contributes 0 Go files without the `editor` tag, INV-4) + daemon-stub-buildable guard. INV-5 round-trip ID preservation, INV-1 version-abort + corrupt-decode, INV-3 compile-keeps-old + double-buffer, INV-2 drop (capture + restore) — all covered across `snapshot_test`/`feature_test`. **Both default + editor + profiling + combined tag builds green; 84 default pkgs.** _Changes: INV-4 release guard + invariant test suite._

**Active slice COMPLETE (8/8): Track A profiling + Track J hot-reload.** Phase stays Active for the deferred multiplayer stack below (L1-only — author L2 via `/magic.spec` first).

## Deferred — Multiplayer Stack & Network Diagnostics (Bootstrap, L1-only)

These remain **L1 concept specs with no L2 contract** → not task-decomposable. Author each L2 via `/magic.spec`
(dependency order transport → replication → {sync models} → rpc → diagnostics) before `/magic.task` decomposes them.

- [ ] [T-7B] Networking system: snapshot/rollback primitives, fixed-step sync. ([l1-networking-system.md](../specifications/l1-networking-system.md))
- [ ] [T-7C] UDP transport: channels, reliability, lifecycle, MTU. ([l1-transport.md](../specifications/l1-transport.md))
- [ ] [T-7D] Replication: markers, entity mapping, visibility, deltas, priority. ([l1-replication.md](../specifications/l1-replication.md))
- [ ] [T-7E] Snapshot interpolation: server snapshots + client buffer + adaptive delay. ([l1-snapshot-interpolation.md](../specifications/l1-snapshot-interpolation.md))
- [ ] [T-7F] Client prediction: input prediction, reconciliation, rollback smoothing. ([l1-client-prediction.md](../specifications/l1-client-prediction.md))
- [ ] [T-7G] Lockstep: deterministic, input delay, speculative exec, desync detect. ([l1-lockstep.md](../specifications/l1-lockstep.md))
- [ ] [T-7H] RPC: typed send/receive, event integration, rate limiting. ([l1-rpc.md](../specifications/l1-rpc.md))
- [ ] [T-7I] Network diagnostics: metrics, alerts, overlay, desync reports. ([l1-network-diagnostics.md](../specifications/l1-network-diagnostics.md))
- [ ] [T-7T] Validation (deferred): lossy-network sim suite, deterministic lockstep checksum, rollback fuzz.
