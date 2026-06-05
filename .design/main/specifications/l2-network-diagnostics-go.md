# Network Diagnostics — Go Implementation

**Version:** 0.1.0
**Status:** Draft
**Layer:** go
**Implements:** l1-network-diagnostics.md

## Overview

This specification defines the Go realization of [Network Diagnostics](l1-network-diagnostics.md): the observability layer over the entire networking stack. It collects per-connection statistics (RTT, loss, bandwidth), replication costs, prediction/interpolation health, lockstep stalls, RPC rates, and desync events — feeding them all into the shipped `DiagnosticsStore` under `"net/"`-prefixed paths, renderable via the existing debug overlay and configurable alerts. It is the capstone of the networking tier: it *reads* every subsystem specced before it and *adds* no new infrastructure.

The implementation reuses shipped diagnostics wholesale: the `DiagnosticsStore` + its `HasAnyReader` zero-cost gate and `RingBuffer` ([l2-diagnostic-system-go.md](l2-diagnostic-system-go.md)), the debug overlay ([the diag overlay + bitmap font]), the build-tag profiling spans ([l2-profiling-protocol-go.md](l2-profiling-protocol-go.md)), and the existing `pkg/protocol.NetworkAlert` wire type. Everything is stdlib (C-003); collection is pure observation.

## Related Specifications

- [l2-diagnostic-system-go.md](l2-diagnostic-system-go.md) — `DiagnosticsStore`, `Diagnostic`, `HasAnyReader` gate, debug overlay
- [l2-transport-go.md](l2-transport-go.md) — `ConnectionStats` source (RTT, loss, bandwidth, queue, peer count)
- [l2-replication-go.md](l2-replication-go.md) — replication stats (entities, bytes, deferred updates, EntityMap size)
- [l2-client-prediction-go.md](l2-client-prediction-go.md) — misprediction frequency, rollback depth, input RTT
- [l2-snapshot-interpolation-go.md](l2-snapshot-interpolation-go.md) — buffer fill, render delay, extrapolation, starvation
- [l2-lockstep-go.md](l2-lockstep-go.md) — input delay, stall count/duration, speculative ticks, desync events
- [l2-rpc-go.md](l2-rpc-go.md) — RPC sent/received/dropped rates
- [l2-profiling-protocol-go.md](l2-profiling-protocol-go.md) — networking spans on the profiling timeline

## 1. Motivation

Network bugs are the hardest to diagnose — "the game felt laggy" could be RTT, loss, bandwidth saturation, mispredictions, buffer starvation, or an undetected desync. Ad-hoc `fmt.Println` misses the real cause. A structured layer makes every networking issue visible, measurable, and traceable. The Go-specific work is a set of per-category collection systems that read each subsystem's stats and push them into the standard store behind the zero-cost reader gate — never inventing a second metric store.

## 2. Constraints & Assumptions

- **C-003 (stdlib only)**: reuses `pkg/diag` + `pkg/protocol`; no third-party metrics/telemetry library (a Prometheus/OpenTelemetry exporter would be a C32 ADR).
- All metrics register in the standard `DiagnosticsStore`; collection is opt-in per category, zero-overhead when disabled.
- Debug overlay rendering depends on the render pipeline; headless servers expose metrics via a queryable resource (no render).
- All paths are `DiagnosticPath` strings prefixed `"net/"` for namespace isolation.
- Diagnostics never modify game/transport/replication state — pure read.

## 3. Core Invariants (Layer 1 only)

This is a Layer 2 specification; the authoritative invariants live in [l1-network-diagnostics.md §3](l1-network-diagnostics.md). They are mapped to their Go realization in §4 below.

## 4. Invariant Compliance (Layer 2 only)

| L1 Invariant | Go Implementation |
| :--- | :--- |
| **INV-1** — zero overhead when no readers registered | Each collection system early-returns on `store.HasAnyReader()` for its category paths — the exact zero-cost gate `l2-diagnostic-system-go` established (collection cost is nil until an overlay/test subscribes). Inherits diagnostic-system INV-1. |
| **INV-2** — collection never modifies game/transport/replication state | Collection systems take read-only views: `transport.Stats(id)` returns a value copy, replication/prediction/interp/lockstep stats are read from their resources, RPC counters are read atomics. No system writes back to any networking state. |
| **INV-3** — all metrics use the standard DiagnosticsStore API | Every metric is a `pkgdiag.Diagnostic` registered at plugin build and pushed via `store.Push(path, value)`; no parallel metric store, no custom aggregation. Paths are `"net/{category}/{name}"`. |
| **INV-4** — alert thresholds are configurable, no hardcoded magic numbers | A `NetworkAlertConfig` resource holds per-metric warning/critical thresholds; an alert system compares the latest sample and emits `pkg/protocol.NetworkAlert{Metric, Value, Level}` (Warning/Critical) when crossed. Defaults live in the config, overridable by the game — no literals in the comparison code. |

> All four invariants are addressed. RFC promotion is blocked by the layer constraint: l1-network-diagnostics is Draft.

## 5. Detailed Design

### 5.1 Package Layout

```plaintext
internal/net/netdiag/
  paths.go        // "net/..." DiagnosticPath constants per category
  collect.go      // per-category collection systems (Last schedule, HasAnyReader-gated)
  alerts.go       // NetworkAlertConfig + threshold comparison → pkg/protocol.NetworkAlert
  overlay.go      // network debug overlay (reuses the diag overlay primitive); headless = resource only
  desyncreport.go // DesyncDetected → structured desync report (diverged tick, peer checksums)
  plugin.go       // NetworkDiagnosticsPlugin: registers metrics + collection + alert systems
```

### 5.2 Metric Categories

```plaintext
[REFERENCE] DiagnosticPath constants (registered at build, pushed each Last frame when read):

net/connection/{rtt, rtt_variance, packet_loss, send_rate, receive_rate, send_queue, peer_count}
net/replication/{entities, bytes_sent, bytes_received, updates_deferred, entity_map_size}
net/prediction/{mispredictions, rollback_depth, corrections, input_rtt}
net/interpolation/{buffer_fill, render_delay, extrapolating, buffer_starved}
net/lockstep/{input_delay, stall_count, stall_duration, speculative_ticks, desync_events}
net/rpc/{sent, received, dropped}
```

Each category has a dedicated collection system that reads its source subsystem's stats and pushes the values — registered independently so a game enables only the categories relevant to its netcode model (a lockstep game has no `net/prediction/*` source).

### 5.3 Collection (Last schedule, gated)

```plaintext
collectConnection(world, store):
    if !store.HasAnyReader(connectionPaths...) { return }   // INV-1 zero-cost
    for id := range transport.Connections():
        s := transport.Stats(id)                            // value copy — INV-2 read-only
        store.Push("net/connection/rtt", float64(s.RTT))
        store.Push("net/connection/packet_loss", float64(s.PacketLoss))
        ... // remaining metrics
```

### 5.4 Alerts and Desync Reports

- **Alerts** (`alerts.go`): `NetworkAlertConfig{ thresholds map[DiagnosticPath]{Warn, Crit float64} }`; the alert system reads the latest sample for each configured path and emits `pkg/protocol.NetworkAlert` on a crossing. Thresholds default in the config and are game-overridable (INV-4).
- **Desync reports** (`desyncreport.go`): a `DesyncDetected{tick}` event (from lockstep/networking-system) is turned into a structured report — diverged tick, per-peer checksums, the last N input frames — surfaced in the overlay and as a `net/lockstep/desync_events` increment.

### 5.5 Overlay & Profiling

The network overlay reuses the shipped diag-overlay primitive (laid-out text → `DebugOverlay` resource), formatting the key `"net/"` metrics; headless servers skip rendering and expose the store as a queryable resource. Networking spans (`"AssetLoad"`-style: `"net/replication"`, `"net/lockstep:tick"`) ride the `pkg/diag/profiling` timeline when the `profiling` build tag is active — no separate tracing path.

## 6. Implementation Notes

1. `paths.go` constants + `plugin.go` metric registration (the `"net/"` namespace).
2. `collect.go` per-category systems, each `HasAnyReader`-gated (INV-1/2/3) — test that collection is nil-cost without a reader and correct with one.
3. `alerts.go` configurable thresholds → `NetworkAlert` (INV-4).
4. `desyncreport.go` + `overlay.go` (reuse the diag overlay; headless resource path).

## 7. Drawbacks & Alternatives

- **Coupling to every networking subsystem.** netdiag reads transport/replication/prediction/interp/lockstep/rpc — a change to any stat surface touches it. Accepted: it is *the* observability layer by design, and it reads value-copy stats (no tight coupling to internals). Per-category independence means a missing subsystem just disables its category.
- **No external metrics export (Prometheus/OTel).** Keeping everything in the in-process `DiagnosticsStore` avoids a third-party dependency (C-003) but means no out-of-process scraping. A future exporter could read the store and emit OTLP behind a C32 ADR, without changing collection.
- **Overlay depends on the render pipeline.** Headless servers get the queryable-resource path instead; the overlay is a client convenience, not the source of truth.

## Canonical References

<!-- MANDATORY for Stable status. Stub — this L2 is Draft (L1 parent Draft, no
     implementation yet). Populate with on-disk source/test files when code lands. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-06-05 | Initial Draft: "net/"-namespaced metrics over the shipped DiagnosticsStore + HasAnyReader zero-cost gate (INV-1/3), read-only per-category collection (INV-2), configurable thresholds → pkg/protocol.NetworkAlert (INV-4), desync reports, overlay reuse + profiling spans. Implements l1-network-diagnostics. |
