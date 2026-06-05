---
phase: 7
name: "Networking & Hot-Reload"
status: Active
subsystem: "pkg/diag/profiling, internal/hotreload, cmd/hot-reload-daemon (DONE); internal/net{,/transport,/replication,/rpc,/interp,/predict,/lockstep,/netdiag} (decomposed, Tracks B-I+T)"
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

## Networking Stack — Atomic Tasks (Bootstrap; all 8 L2s authored 2026-06-05)

Decomposed from the complete 8-spec networking L2 tier. All `[Bootstrap]` (Draft L1 parents → tentative until Stable).
**Deep sequential dependency chain** (NOT parallel like Tracks A/J): `B → C → {D ‖ H} → {E ‖ F ‖ G} → I`, T validates throughout.
Per CLAUDE.md: every new Go file ≥80% cov, `-race` on concurrent tests (transport goroutine + reliability), `gofmt`/`vet`/`modernize` clean.
**Skeptic flags:** transport reliability (T-7C01) + DeterministicSchedule (T-7B02) are the high-risk critical-path items — a determinism bug blocks both prediction (F) and lockstep (G); split into `.1/.2` at execution if they grow.

### Track B — Networking System ([l2-networking-system-go.md](../specifications/l2-networking-system-go.md) → [l1-networking-system.md](../specifications/l1-networking-system.md)) · `internal/net`

- [x] **T-7B01 — Transport abstraction + handles.** `internal/net/transport.go`: `NetworkTransport` interface + `ConnectionID`/`ChannelID`/`DeliveryMode`/`ChannelConfig`/`ConnectionStats`/`InboundPacket`/`Connected`/`Disconnected`/`DisconnectReason` + default channel IDs (INV-5 — gameplay sees handles + events, never sockets). **Reconciliation: `SocketAddr` is a `string` so package `net` stays free of the stdlib `net` import (no shadow clash) + keeps the abstraction protocol-agnostic.** 100% cov. _Changes: net abstraction + handle/event types._
- [x] **T-7B02 — Deterministic primitives** *(.1 + .2 done).* **.1:** `internal/net/input.go` `InputBuffer` (128-tick ring; record/get/predict-repeat-last/`HasAllPeers` lockstep gate; ring-aliasing cleared; deterministic `EncodeInput`/`DecodeInput`, INV-4). **.2:** `deterministic.go` `DeterministicSchedule` (explicit-order systems + per-tick `rand/v2.PCG` reseed `RngSeed^tick`, no DAG parallelism → INV-2) + `snapshot.go` `SnapshotManager` (tick + CRC32 + ring; **deterministic encode — entities by ID, components by type name — so peers get byte-identical checksums for desync detection**; entity-ID-stable restore, INV-3). **Refactor delivered: extracted the World→bytes snapshot/restore core to the shared non-editor `internal/worldsnap` package** (`Capture`/`Restore`; the `EntityAllocator.PlaceAt`/`RebuildFreeList` pinning was already non-editor) — hot-reload re-wired to consume it (editor-only header/AppState wrapper kept; all hotreload tests + `TestNoHotReloadInRelease` green). worldsnap 89.1%, internal/net 99.0% cov. _Changes: InputBuffer + DeterministicSchedule + SnapshotManager + worldsnap extraction._
- [ ] **T-7B03 — Pipeline + plugin.** `rollback.go` (reference `RollbackCoordinator`, ServiceRegistry-replaceable) + `systems.go` (`NetworkReceive` PreUpdate / `NetworkSend` PostUpdate — INV-1 no socket in schedules) + `plugin.go` (`NetworkPlugin` SubApp: default channels, transport injection, resources).

### Track C — Transport ([l2-transport-go.md](../specifications/l2-transport-go.md) → [l1-transport.md](../specifications/l1-transport.md)) · `internal/net/transport`

- [x] **T-7C01 — Packet codec + reliability.** `internal/net/transport/packet.go` (`Encode`/`Decode`: 8-byte header + optional ACK block + length-prefixed frames; protocol-id reject; **every length bounds-checked against the remaining buffer → no panic on hostile UDP**; `FuzzDecode` 1.7M execs/0 crashes) + `reliability.go` (`seqGreater` wraparound; Jacobson/Karels `rtoEstimator` clamped [200ms,2s]; `ackTracker` 32-bit piggyback bitfield; `reliableSender` sliding window + RTO retransmit + first-transmit-only RTT sampling + window-full backpressure; `reliableReceiver` 64-wide dedup bitfield + ReliableOrdered reorder-release — INV-1/2/3/6). **Reconciliation:** package `transport` aliases `internal/net` as `netcore` (avoids the stdlib-`net` shadow). 95.1% cov. _Changes: wire codec + user-space reliability core._
- [ ] **T-7C02 — UDP loop + backend.** `udp.go` (`UDPTransport` network goroutine on IO pool, channel ECS boundary — INV-5) + `connection.go` (per-conn seqs/window/stats/MTU) + `backend.go` (`SocketBackend` iface + `net.UDPConn` default + `memBackend` for tests).
- [ ] **T-7C03 — Lifecycle + MTU.** `handshake.go` (ConnectRequest/Accept/Reject/Ack over ch 2, version check) + heartbeat/timeout (INV-4) + `mtu.go` (PMTUD 1400→1200→576) + `settings.go`.

### Track D — Replication ([l2-replication-go.md](../specifications/l2-replication-go.md) → [l1-replication.md](../specifications/l1-replication.md)) · `internal/net/replication`

- [ ] **T-7D01 — Markers + EntityMap.** `markers.go` (`Replicated`/`ServerOnly`/`ClientOnly` tags + `ReplicationRule` + `ReplicationConfig`, INV-1 whitelist) + `entitymap.go` (bijective map + `Remap` reusing the scene EntityID-field walk + placeholder/deferred resolution, INV-2/3).
- [ ] **T-7D02 — Visibility + messages + delta.** `visibility.go` (`VisibilityPolicy` ReplicateAll/Grid/Custom + `VisibilitySet`, INV-4) + `message.go` (`ReplicationMessage` codec + channel assignment) + `delta.go` (`ClientAckState` over change-detection `ChangedTick` + optional `DeltaSerializer`).
- [ ] **T-7D03 — Send/receive + priority + plugin.** `send.go` (read-only, INV-5) + `receive.go` (command-buffer-applied, INV-5; despawn propagation INV-6) + `priority.go` (accumulator + bandwidth budget) + `plugin.go` (default rules).

### Track H — RPC ([l2-rpc-go.md](../specifications/l2-rpc-go.md) → [l1-rpc.md](../specifications/l1-rpc.md)) · `internal/net/rpc`

- [ ] **T-7H01 — Registry + send + receive.** `registry.go` (`RpcTypeID↔reflect.Type`, `RegisterRpc[T]`) + `sender.go` (`Send[T]` + direction-validated `RpcTarget`, Except→no echo INV-1) + `receive.go` (`RpcReceiveSystem` PreUpdate, unknown-ID drop+log INV-3, `EntityMap` remap, typed-event INV-4) + `message.go` (+ fuzz decoder).
- [ ] **T-7H02 — Rate limit + plugin.** `ratelimit.go` (token bucket global + per-type) + `plugin.go`.

### Track E — Snapshot Interpolation ([l2-snapshot-interpolation-go.md](../specifications/l2-snapshot-interpolation-go.md) → [l1-snapshot-interpolation.md](../specifications/l1-snapshot-interpolation.md)) · `internal/net/interp`

- [ ] **T-7E01 — Buffer + interpolate.** `buffer.go` (tick-sorted ring, out-of-order/dup discard INV-3) + `interpolate.go` (render-clock bracket, `t=clamp(...,0,1)` + bounded extrapolation INV-1/2/4, type-driven Lerp/Slerp into render-only state).
- [ ] **T-7E02 — Adaptive delay + plugin.** `adaptive.go` (render-delay controller vs buffer fill) + `config.go` + `plugin.go` (consume replication snapshots).

### Track F — Client Prediction ([l2-client-prediction-go.md](../specifications/l2-client-prediction-go.md) → [l1-client-prediction.md](../specifications/l1-client-prediction.md)) · `internal/net/predict`

- [ ] **T-7F01 — Authority + history + predict loop.** `authority.go` (`NetworkAuthority` partition, INV-5 query-filter) + `history.go` (ring + crc32, INV-3) + `predict.go` (FixedUpdate loop over the `DeterministicSchedule` subset, INV-2).
- [ ] **T-7F02 — Reconcile + rollback + smoothing.** `reconcile.go` (compare → `SnapshotManager.RestoreSnapshot` + `InputBuffer` + `DeterministicSchedule.RunTick` resimulate, INV-1/4) + `smoothing.go` (`CorrectionState` blend, no teleport) + `plugin.go`.

### Track G — Lockstep ([l2-lockstep-go.md](../specifications/l2-lockstep-go.md) → [l1-lockstep.md](../specifications/l1-lockstep.md)) · `internal/net/lockstep`

- [ ] **T-7G01 — Scheduler + input flow.** `scheduler.go` (all-peers-ready gate, RunTick, catch-up, checksum cadence — INV-1/2/5) + `inputflow.go` (local tag+broadcast / remote receive — INV-3).
- [ ] **T-7G02 — Desync + speculative + late join.** `desync.go` (crc32 checksum exchange + halt + `DesyncDetected`, INV-4) + `speculative.go` (opt-in — reuses Track F rollback path) + `latejoin.go` (snapshot transfer) + `plugin.go`.

### Track I — Network Diagnostics ([l2-network-diagnostics-go.md](../specifications/l2-network-diagnostics-go.md) → [l1-network-diagnostics.md](../specifications/l1-network-diagnostics.md)) · `internal/net/netdiag`

- [ ] **T-7I01 — Metrics + gated collection.** `paths.go` (`"net/..."` `DiagnosticPath` constants) + `collect.go` (per-category `Last`-schedule systems, `HasAnyReader`-gated zero-cost INV-1, read-only INV-2, standard `DiagnosticsStore` API INV-3).
- [ ] **T-7I02 — Alerts + overlay + desync report.** `alerts.go` (`NetworkAlertConfig` thresholds → `pkg/protocol.NetworkAlert`, INV-4) + `overlay.go` (diag-overlay reuse; headless resource) + `desyncreport.go` + `plugin.go`.

### Track T — Networking Validation

- [ ] **T-7T01 — Transport conformance.** `FuzzPacketDecode` (hostile/truncated datagrams never panic) + a `memBackend` lossy-network sim (drop/reorder/dup) asserting Unreliable tolerates gaps + Reliable delivers exactly-once-in-order.
- [ ] **T-7T02 — Determinism + rollback equivalence.** Deterministic lockstep checksum match across two in-proc peers; client-prediction rollback reproduces live state given the same inputs (a forced divergence triggers exactly one rollback to the correct state).
- [ ] **T-7T03 — Replication round-trip.** Server→client EntityMap bijection + visibility gating (out-of-visibility entity never delivered) + whitelist (unmarked component never serialized) + despawn propagation.
- [ ] **T-7T04 — End-to-end.** `examples/networking` (in-proc server+client over `memBackend`, hash-stable ×20 via `cmd/examplecheck`) exercising the full pipeline.
