# Deterministic Lockstep — Go Implementation

**Version:** 0.1.0
**Status:** Draft
**Layer:** go
**Implements:** l1-lockstep.md

## Overview

This specification defines the Go realization of [Deterministic Lockstep](l1-lockstep.md): all peers run the identical deterministic simulation and exchange only *inputs*, never state. A tick advances only once every peer's input for that tick has arrived, so bandwidth scales with player count, not world complexity — the only practical model for large-simulation genres (RTS, fighting games). It is the alternative architecture to the server-authoritative client pair (interpolation + prediction); it shares their rollback machinery but inverts what crosses the wire.

The implementation is built entirely on the networking foundation primitives ([l2-networking-system-go.md](l2-networking-system-go.md)): the `DeterministicSchedule` guarantees identical execution, the `InputBuffer` serializes/buffers inputs, the `SnapshotManager` produces the per-tick checksum for desync detection, and (for opt-in speculative execution) the same rollback path as [client-prediction](l2-client-prediction-go.md). Everything is stdlib (C-003); `hash/crc32` provides the desync checksum.

## Related Specifications

- [l2-networking-system-go.md](l2-networking-system-go.md) — `DeterministicSchedule` (identical exec), `InputBuffer`, `SnapshotManager` (checksum/late-join), per-tick RNG
- [l2-transport-go.md](l2-transport-go.md) — inputs + checksums on ch 1 (ReliableUnordered)
- [l2-time-system-go.md](l2-time-system-go.md) — `FixedTime` drives the constant tick rate
- [l2-input-system-go.md](l2-input-system-go.md) — input serialization for transmission
- [l2-client-prediction-go.md](l2-client-prediction-go.md) — speculative execution reuses the rollback-resimulate path
- [l2-replication-go.md](l2-replication-go.md) — not used in steady state; only for late-join full-state transfer

## 1. Motivation

Replicating 10,000 RTS units 30×/second is prohibitive; running the same deterministic simulation on every peer makes state *implicitly* synchronized, so only ~50–200 bytes of input per player per tick cross the wire. The Go-specific work is the all-peers-ready gate, the input-delay pipeline, the periodic checksum exchange for desync detection, and the opt-in speculative path that reuses prediction's rollback so a late input doesn't freeze everyone.

## 2. Constraints & Assumptions

- **C-003 (stdlib only)**: `hash/crc32` desync checksum, `encoding/binary` input framing. No third-party lockstep library.
- **All** gameplay systems must be registered with the `DeterministicSchedule`; non-deterministic systems (particles, UI, audio) run separately and are not part of the lockstep simulation.
- All peers share engine version + system-registration order + fixed timestep (version mismatch caught at the transport handshake).
- Pure lockstep stalls on any late input; input delay + opt-in speculative execution mitigate. A peer that exceeds `input_timeout` is disconnected (never an indefinite wait).
- Late join requires a full snapshot transfer (lockstep does not continuously replicate).
- Natural for P2P, but also works behind a relay server that aggregates/distributes inputs.

## 3. Core Invariants (Layer 1 only)

This is a Layer 2 specification; the authoritative invariants live in [l1-lockstep.md §3](l1-lockstep.md). They are mapped to their Go realization in §4 below.

## 4. Invariant Compliance (Layer 2 only)

| L1 Invariant | Go Implementation |
| :--- | :--- |
| **INV-1** — no tick advances until all peers' inputs for it are available | `LockstepScheduler.Step` checks `InputBuffer` for an entry from every `peers` connection at `currentTick + inputDelay`; only then does it call `DeterministicSchedule.RunTick(currentTick)` and advance. Missing any → it returns without advancing (a "waiting for players" state after a 200 ms stall). |
| **INV-2** — identical inputs + initial state → identical state at every tick | Composes networking-system INV-2: all gameplay runs through the one `DeterministicSchedule` (fixed order, seeded per-tick RNG, no wall-clock reads). This spec does not re-prove determinism; it requires every gameplay system to be registered in that schedule. |
| **INV-3** — only inputs cross the wire; no state replication in steady play | The send path serializes `SerializedInput` (from `InputBuffer`) and broadcasts on transport ch 1; no `ReplicationMessage` is sent during gameplay. Replication is invoked only for the late-join snapshot transfer. |
| **INV-4** — periodic desync detection halts + reports the diverged tick | Every `checksumInterval` ticks, `SnapshotManager.TakeChecksum(tick)` (crc32 of deterministic state) is exchanged on ch 1; a mismatch fires `DesyncDetected{tick}` and halts the scheduler (no further ticks), surfacing the diverged tick for a desync report. |
| **INV-5** — input delay bounded; timeout → disconnect, never an infinite wait | `inputDelay` (default 2 ticks ≈ 33 ms @ 60 Hz) is a fixed `uint8`; a peer whose input is absent past `inputTimeout` (default 5 s) is `transport.Disconnect`ed with reason Timeout — the gate then proceeds without it rather than blocking forever. |

> All five invariants are addressed. RFC promotion is blocked by the layer constraint: l1-lockstep is Draft.

## 5. Detailed Design

### 5.1 Package Layout

```plaintext
internal/net/lockstep/
  scheduler.go    // LockstepScheduler: all-peers-ready gate, RunTick, catch-up, checksum cadence
  inputflow.go    // local tag+broadcast / remote receive into InputBuffer
  speculative.go  // opt-in speculative execution (predict missing input → run → confirm/rollback)
  desync.go       // checksum exchange + DesyncDetected + halt
  latejoin.go     // full-state snapshot transfer for a joining peer
  plugin.go       // LockstepPlugin: resources + systems (replaces the standard FixedUpdate driver)
```

### 5.2 Lockstep Tick Loop

```plaintext
LockstepScheduler.Step():
  for time budget allows (catch-up):
    target := currentTick + inputDelay
    if !inputBuffer.HasAllPeers(target, peers):
        if stalled > 200ms { signal "waiting for players" }
        if peerLate > inputTimeout { transport.Disconnect(peer, Timeout) }   // INV-5
        return                                                               // INV-1: do not advance
    applyInputs(currentTick)
    deterministicSchedule.RunTick(currentTick)                               // INV-2
    if currentTick % checksumInterval == 0 {
        sendChecksum(snapshotManager.TakeChecksum(currentTick))              // INV-4
    }
    currentTick++
```

### 5.3 Input Flow (INV-3)

```plaintext
Local:  capture → SerializedInput → tag tick = currentTick + inputDelay →
        InputBuffer.RecordInput(...) → transport.Broadcast(ch1, serialize)
Remote: receive SerializedInput → InputBuffer.RecordInput(peer, tick, input)
```

Only `SerializedInput` is ever broadcast in steady state — no `ReplicationMessage`.

### 5.4 Speculative Execution (opt-in) — reuses prediction's rollback

```plaintext
SpeculativeConfig{ Enabled bool; MaxSpeculative uint8 (default 4) }

On a missing peer input at currentTick (Enabled):
  predicted := inputBuffer.PredictInput(tick, peer)   // repeat-last
  run the tick speculatively; mark it speculative in history
  on real input arrival:
    if real == predicted → confirm; done
    else → SnapshotManager.RestoreSnapshot(tick); re-run with real input
           (the SAME rollback path as client-prediction — SnapshotManager + DeterministicSchedule)
```

This is why lockstep depends on client-prediction: speculative execution *is* misprediction rollback, triggered by a missing remote input instead of a server divergence.

### 5.5 Desync Detection & Late Join

- **Desync** (`desync.go`): peers exchange the crc32 state checksum every `checksumInterval`; a mismatch halts the scheduler and emits `DesyncDetected{tick}` for the diagnostics desync report (l2-network-diagnostics-go). The game does not silently continue corrupt.
- **Late join** (`latejoin.go`): a joining peer receives a full `SnapshotManager` snapshot (the only time state crosses the wire) plus the input stream from the snapshot tick forward, then enters the lockstep loop.

## 6. Implementation Notes

1. `scheduler.go` all-peers-ready gate + RunTick + catch-up (INV-1/2/5) — the core loop.
2. `inputflow.go` local tag+broadcast / remote receive (INV-3).
3. `desync.go` checksum cadence + halt (INV-4) — characterization-test a forced divergence halts at the right tick.
4. `speculative.go` reusing the prediction rollback path (opt-in).
5. `latejoin.go` + `plugin.go` wiring (replaces the standard FixedUpdate driver when lockstep is active).

## 7. Drawbacks & Alternatives

- **100% determinism is mandatory.** A single non-deterministic op desyncs the whole session. Mitigation: all gameplay must register with the `DeterministicSchedule`, fixed-point/seeded ops only; the periodic checksum catches drift early with a precise tick. This is inherent to lockstep, not a design flaw.
- **Stall sensitivity.** One slow peer can freeze everyone. Input delay hides small jitter; speculative execution avoids freezes at the cost of occasional rollback; `inputTimeout` bounds the worst case by disconnecting. The trade-off is explicit and configurable.
- **No mid-game state recovery without late-join transfer.** Because state isn't replicated, a desynced or joining peer needs a full snapshot. Accepted — it is the price of input-only bandwidth, and the snapshot path already exists (SnapshotManager).

## Canonical References

<!-- MANDATORY for Stable status. Stub — this L2 is Draft (L1 parent Draft, no
     implementation yet). Populate with on-disk source/test files when code lands. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-06-05 | Initial Draft: all-peers-ready tick gate (INV-1), DeterministicSchedule composition (INV-2), input-only broadcast (INV-3), crc32 checksum desync-halt (INV-4), bounded input-delay/timeout-disconnect (INV-5), opt-in speculative execution reusing the prediction rollback path, late-join snapshot transfer. Implements l1-lockstep. |
