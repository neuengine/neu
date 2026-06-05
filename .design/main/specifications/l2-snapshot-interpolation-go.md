# Snapshot Interpolation — Go Implementation

**Version:** 0.1.0
**Status:** Draft
**Layer:** go
**Implements:** l1-snapshot-interpolation.md

## Overview

This specification defines the Go realization of [Snapshot Interpolation](l1-snapshot-interpolation.md): the client buffers authoritative server snapshots delivered by replication and renders a slightly delayed, smoothly interpolated view — never simulating, predicting, or rolling back. It is the simple, robust client-side rendering model for spectators, slow-paced games, and the display path for *remote* (non-predicted) entities under client-prediction.

The implementation reuses shipped infrastructure: replication ([l2-replication-go.md](l2-replication-go.md)) delivers the per-entity component snapshots; the math system ([l2-math-system-go.md](l2-math-system-go.md)) provides `Lerp`/`Slerp`; `gametime` ([l2-time-system-go.md](l2-time-system-go.md)) `VirtualTime` drives the render clock. Interpolated values are written to render-only display state (an `OffsetTransform`-style component), never the authoritative `Transform` — so interpolation is a pure display concern. Everything is stdlib (C-003).

## Related Specifications

- [l2-replication-go.md](l2-replication-go.md) — delivers the buffered per-entity component snapshots
- [l2-networking-system-go.md](l2-networking-system-go.md) — `SnapshotManager` snapshot/delta semantics
- [l2-transport-go.md](l2-transport-go.md) — snapshots on ch 0/3 (unreliable), spawns/despawns on ch 1
- [l2-time-system-go.md](l2-time-system-go.md) — `VirtualTime` drives the render-delay clock
- [l2-math-system-go.md](l2-math-system-go.md) — `Lerp`/`Slerp` for component interpolation
- [l2-client-prediction-go.md](l2-client-prediction-go.md) — predicted local entity; remote entities use this model

## 1. Motivation

Many games (spectator, turn-based, strategy) and most entities (everything the local player doesn't control) need no prediction — receive state, buffer, interpolate, display. The Go-specific work is a tick-sorted snapshot ring, a render clock deliberately behind the server, and an adaptive delay that tracks buffer fill against network jitter — all as ordinary ECS systems consuming replication output, writing only render-side components.

## 2. Constraints & Assumptions

- **C-003 (stdlib only)**: math `Lerp`/`Slerp`, `time` for the render clock. No third-party interpolation library.
- The client does not simulate; it displays interpolated server state.
- Snapshots arrive over unreliable channels — gaps/reorder are normal; the buffer tolerates them.
- Interpolation writes render-only display state, leaving any authoritative components untouched.
- Extrapolation past the latest snapshot is opt-in and bounded.

## 3. Core Invariants (Layer 1 only)

This is a Layer 2 specification; the authoritative invariants live in [l1-snapshot-interpolation.md §3](l1-snapshot-interpolation.md). They are mapped to their Go realization in §4 below.

## 4. Invariant Compliance (Layer 2 only)

| L1 Invariant | Go Implementation |
| :--- | :--- |
| **INV-1** — display is always interpolated between two confirmed snapshots, never predicted | `InterpolateSystem` (in the render-extract phase) finds the two buffered snapshots bracketing `renderTime` and writes `Lerp/Slerp(a, b, t)` into a render-only display component. It never calls a simulation system; the client holds no authoritative copy of remote entities. |
| **INV-2** — interpolation begins only with ≥2 buffered snapshots | The system checks `buffer.Len() >= 2` and that two entries bracket `renderTime`; below that it writes the latest received snapshot directly (snap, no interpolation) — a documented warm-up state. |
| **INV-3** — out-of-order/duplicate snapshots discarded by tick | On arrival, `SnapshotBuffer.Insert` drops any snapshot whose `tick <= latestBufferedTick`; otherwise it inserts in sorted position and evicts the oldest when full. |
| **INV-4** — interpolation `t` clamped to [0,1]; extrapolation bounded | `t = clamp((renderTime - a.ts) / (b.ts - a.ts), 0, 1)`. When `renderTime` exceeds the newest snapshot and extrapolation is enabled, a *separate* factor capped at `maxExtrapolation` (default ~0.5) is used; otherwise the newest snapshot is held. |

> All four invariants are addressed. RFC promotion is blocked by the layer constraint: l1-snapshot-interpolation is Draft.

## 5. Detailed Design

### 5.1 Package Layout

```plaintext
internal/net/interp/
  buffer.go       // SnapshotBuffer (tick-sorted ring), SnapshotEntry, insert/evict/bracket
  interpolate.go  // InterpolateSystem: render-clock, bracket lookup, Lerp/Slerp, clamp/extrapolate
  config.go       // InterpolationConfig (renderDelay, capacity, targetFill, maxExtrapolation)
  adaptive.go     // adaptive render-delay controller (tracks buffer fill vs target)
  plugin.go       // InterpolationPlugin: resources + the render-extract system
```

### 5.2 Snapshot Buffer

```plaintext
[REFERENCE]
type SnapshotEntry struct {
    Tick       uint64
    Timestamp  time.Duration                       // server-side time of this tick
    Entities   map[entity.EntityID]ComponentSnapshot
    ReceivedAt time.Duration
}
type SnapshotBuffer struct { ring []SnapshotEntry; capacity int; renderDelay time.Duration }
Insert(e):  if e.Tick <= latestTick { return }     // INV-3
            sorted insert; evict oldest if full
Bracket(renderTime) (a, b SnapshotEntry, ok bool)  // a.ts <= renderTime < b.ts
```

### 5.3 Interpolation Timing

```plaintext
renderTime = latestServerTime - renderDelay
a, b, ok := buffer.Bracket(renderTime)
if !ok || buffer.Len() < 2 { displayLatest(); return }      // INV-2 warm-up
t := clamp((renderTime - a.Timestamp) / (b.Timestamp - a.Timestamp), 0, 1)  // INV-4
for entity present in both a and b:
    display[entity] = lerpComponents(a[entity], b[entity], t)   // Lerp pos, Slerp rot — INV-1
```

**Adaptive render delay** (`adaptive.go`): a controller nudges `renderDelay` toward keeping the buffer near `targetFill` snapshots ahead of `renderTime` — increase on chronic underfill (jitter), decrease on overfill — bounding the latency/smoothness trade-off automatically.

### 5.4 Component Interpolation

Per-type interpolation is metadata-driven: `Transform.Translation` uses `Lerp`, `Transform.Rotation` uses `Slerp`, scalar fields `Lerp`; a type with no registered interpolator is snapped (held at `a`). This mirrors the tweening system's easing-by-type approach and writes only the render-side display component.

## 6. Implementation Notes

1. `buffer.go` tick-sorted ring + `Bracket` (INV-3 discard; characterization-test insert/evict/bracket/gap).
2. `interpolate.go` render-clock + clamp + Lerp/Slerp (INV-1/2/4) — the core system.
3. `config.go` + `adaptive.go` delay controller.
4. `plugin.go` wiring; consume replication's per-entity snapshots.

## 7. Drawbacks & Alternatives

- **Deliberate latency.** Interpolation trades ~2× snapshot-interval of input-to-display delay for smoothness — unacceptable for the local player (hence client-prediction for the controlled entity). This model is correct for remote entities and spectators by design.
- **Buffer memory per entity.** A 32-deep buffer × visible entities costs memory; bounded by the visibility set (replication) and far cheaper than the alternative of client-side simulation of remote entities.
- **Snap on warm-up / gaps.** Below 2 snapshots or across a buffer gap the display snaps. Accepted as a brief, rare artifact; the adaptive delay minimizes gap frequency.

## Canonical References

<!-- MANDATORY for Stable status. Stub — this L2 is Draft (L1 parent Draft, no
     implementation yet). Populate with on-disk source/test files when code lands. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-06-05 | Initial Draft: tick-sorted snapshot ring (INV-3), render-clock interpolation with clamp/bounded-extrapolation (INV-1/2/4), adaptive render delay, type-driven Lerp/Slerp into render-only display state. Implements l1-snapshot-interpolation. |
