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
- [ ] **T-7J02 — Snapshot encode + transactional restore + entity-ID pinning.** `snapshot.go`: serialize `World` via the `DynamicScene` codec into the snapshot body + `dropped.manifest` for non-serializable types (INV-2) + app-state capture. `restore.go`: decode into a **staging** scene, apply to the live `World` only on full success (INV-1 — no half-mutated state); `IdentityRemapper` pins each `EntityID` at its exact index+generation and rebuilds the `EntityAllocator` free-list from the index gaps (INV-5). *Skeptic flag: this is the heaviest task (touches `EntityAllocator` internals) — split `.1` snapshot / `.2` restore at execution if it grows.*
- [ ] **T-7J03 — Shader hot-swap + orchestrator + plugin.** `shader.go`: compile-into-new-handle then double-buffered swap, keep old handle on compile error (INV-3); headless backend = validating no-op. `orchestrator.go` (editor-side `FileWatcher` host + debounce + `os/exec` `go build` + mode routing) + `scope.go` (AST change-scope classifier: SystemOnly/ComponentType/ResourceType/PluginAPI/CoreEngine). `plugin.go` `HotReloadPlugin` + `--hot-reload=<path>` flag handling + `cmd/hot-reload-daemon` (standalone terminal-UI orchestrator).
- [ ] **T-7J04 — Validation (Track J).** Snapshot→restore round-trip preserves every `EntityID` (INV-5 characterization test); corrupt-decode aborts before any World mutation → clean start (INV-1); shader compile-failure keeps the old handle bound (INV-3); `TestNoHotReloadInRelease` go-list/AST guard asserts the package is absent without the `editor` tag (INV-4); dropped-component manifest surfaced, never silent (INV-2).

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
