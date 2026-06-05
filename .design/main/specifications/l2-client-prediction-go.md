# Client-Side Prediction — Go Implementation

**Version:** 0.1.0
**Status:** Draft
**Layer:** go
**Implements:** l1-client-prediction.md

## Overview

This specification defines the Go realization of [Client-Side Prediction](l1-client-prediction.md): the client simulates its locally-controlled entity immediately (no server round-trip), then reconciles against the server's authoritative state — rolling back and resimulating from buffered inputs on a misprediction, blending the visual correction so there is no teleport. It is the responsive-input model for fast-paced games; remote entities are displayed via [snapshot interpolation](l2-snapshot-interpolation-go.md).

The implementation is the direct consumer of the networking foundation primitives: it rolls back through `SnapshotManager.RestoreSnapshot`, replays through `InputBuffer` + `DeterministicSchedule.RunTick`, and applies server corrections through the command buffer — all defined in [l2-networking-system-go.md](l2-networking-system-go.md). It partitions the World by `NetworkAuthority`: `Predicted(localConn)` entities simulate+rollback, everything else interpolates. Everything is stdlib (C-003); `hash/crc32` provides fast mismatch detection.

## Related Specifications

- [l2-networking-system-go.md](l2-networking-system-go.md) — `SnapshotManager` rollback, `InputBuffer`, `RollbackCoordinator`, `DeterministicSchedule` (resimulation)
- [l2-replication-go.md](l2-replication-go.md) — server state delivery + `NetworkAuthority` for predicted entities
- [l2-transport-go.md](l2-transport-go.md) — input on ch 1 (ReliableUnordered), state on ch 0 (Unreliable)
- [l2-snapshot-interpolation-go.md](l2-snapshot-interpolation-go.md) — remote (non-predicted) entities use interpolation
- [l2-time-system-go.md](l2-time-system-go.md) — `FixedTime` drives deterministic simulation ticks
- [l2-input-system-go.md](l2-input-system-go.md) — raw input captured + serialized per tick
- [l2-command-system-go.md](l2-command-system-go.md) — server corrections applied as deferred commands

## 1. Motivation

Without prediction, every action waits a full RTT before the player sees it — sluggish at 80 ms, broken at 150 ms. Prediction runs the controlled entity's simulation locally so input feels instant, while the server stays authoritative: on the rare misprediction, the client snaps to server state and resimulates. The Go-specific work is the prediction history ring (with checksums for fast mismatch detection), the reconciliation system that drives the rollback-resimulate loop over the existing deterministic schedule, and a correction-smoothing component so the snap is visually blended.

## 2. Constraints & Assumptions

- **C-003 (stdlib only)**: `hash/crc32` for checksums, math `Lerp`/`Slerp` for correction blending. No third-party netcode library.
- Only `NetworkAuthority.Predicted(localConn)` entities are predicted; all others interpolate (INV-5).
- Prediction requires deterministic simulation for the predicted systems (reuses `DeterministicSchedule`); non-deterministic systems (particles, audio) are never rolled back.
- The client buffers inputs for the last N ticks (depth ≈ max RTT in ticks) to replay during resimulation.
- Misprediction correction is blended over time (default ~100 ms), never teleported.

## 3. Core Invariants (Layer 1 only)

This is a Layer 2 specification; the authoritative invariants live in [l1-client-prediction.md §3](l1-client-prediction.md). They are mapped to their Go realization in §4 below.

## 4. Invariant Compliance (Layer 2 only)

| L1 Invariant | Go Implementation |
| :--- | :--- |
| **INV-1** — server is always authoritative; prediction is visual-only | `ReconciliationSystem` always converges predicted entities to the server's confirmed state (snap then blend); the client never overrides a server confirmation. Prediction only fills the gap between confirmed ticks. |
| **INV-2** — predicted entities use the same deterministic systems as the server | The `PredictionSystem` and the rollback resimulation both run the **same `DeterministicSchedule.RunTick`** the server runs — one system order, fixed-point/seeded ops — so client and server execute identical code on identical input. |
| **INV-3** — compare server state to prediction; reconcile on mismatch | On server state for tick T, the system computes `crc32` of the predicted-entity components in `PredictionHistory[T]` and compares to the server's; equal → discard history ≤T (prediction correct); unequal → rollback. |
| **INV-4** — rollback-resim reproduces live results given the same inputs | Reduces to networking-system INV-2/4: `SnapshotManager.RestoreSnapshot(T)` restores exact state (entity-ID-stable via `EntityAllocator` pinning — the hot-reload mechanism), then `for t in T+1..current: DeterministicSchedule.RunTick(t, InputBuffer)` replays buffered inputs deterministically. This spec composes the parent's determinism guarantee rather than re-proving it. |
| **INV-5** — non-predicted entities are never rolled back | The prediction/rollback systems query only entities carrying `NetworkAuthority.Predicted(localConn)`; every other entity is outside the query and is handled solely by snapshot interpolation. INV-5 is a query-filter invariant, not runtime bookkeeping. |

> All five invariants are addressed. RFC promotion is blocked by the layer constraint: l1-client-prediction is Draft.

## 5. Detailed Design

### 5.1 Package Layout

```plaintext
internal/net/predict/
  authority.go     // NetworkAuthority component (Predicted/Interpolated/Authoritative)
  history.go       // PredictionHistory ring (per-tick predicted snapshot + crc32 checksum)
  predict.go       // PredictionSystem (FixedUpdate): capture input → record → send → simulate → record history
  reconcile.go     // ReconciliationSystem (PreUpdate): compare → rollback → resimulate
  smoothing.go     // CorrectionState component + blend-out system (no teleport)
  plugin.go        // PredictionPlugin: resources + systems
```

### 5.2 Prediction Loop (FixedUpdate)

```plaintext
PredictionSystem(world):
  input := captureLocalInput()                                  // input-system
  inputBuffer.RecordInput(currentTick, localPlayer, input)
  transport.Send(Server, ch1, serialize(input))                 // INV: input is reliable
  runPredictedSystems(world)                                    // DeterministicSchedule subset — INV-2
  predictionHistory.Record(currentTick, snapshot(predictedEntities)) // + crc32
  currentTick++
```

The client runs ahead of the server by ≈`RTT/2/fixedTimestep` ticks (simulating an unconfirmed future).

### 5.3 Prediction History

```plaintext
[REFERENCE]
type PredictionEntry struct {
    Tick     uint64
    Entities map[entity.EntityID]PredictedSnapshot
}
type PredictedSnapshot struct {
    Components map[component.ID][]byte
    Checksum   uint32           // crc32 — fast mismatch detection (INV-3)
}
type PredictionHistory struct { ring []PredictionEntry; capacity int } // default 64 (~1s @ 60Hz)
```

### 5.4 Reconciliation (PreUpdate)

```plaintext
ReconciliationSystem(world, cmd CommandBuffer):
  for serverState at tick T (via replication):
    entry := predictionHistory[T]
    if crc32(serverState.predicted) == entry.Checksum:
        predictionHistory.discardThrough(T); confirmedTick = T; continue   // correct
    // misprediction → rollback (INV-3/4):
    recordVisualPose(predictedEntities)                 // for smoothing
    snapshotManager.RestoreSnapshot(T)                  // entity-ID-stable
    applyServerState(cmd, T)                             // server values via commands (authoritative)
    for t := T+1; t <= currentTick; t++ {
        applyInput(inputBuffer.Get(t)); deterministicSchedule.RunTick(t)
        predictionHistory.Record(t, snapshot(...))      // overwrite stale prediction
    }
    addCorrectionState(predictedEntities, oldPose - newPose)   // smoothing offset
    confirmedTick = T; predictionHistory.discardBefore(T)
```

### 5.5 Misprediction Smoothing

```plaintext
[REFERENCE]
type CorrectionState struct {           // component on a corrected predicted entity
    VisualOffset   math.Vec3            // corrected − displayed position
    RotationOffset math.Quat
    BlendRemaining time.Duration
    BlendDuration  time.Duration        // default 100ms
}
```

A `Last`-schedule system decays `VisualOffset`/`RotationOffset` to zero over `BlendDuration` and writes the residual into the render-side display transform — the entity simulates at the corrected position immediately, but is *displayed* blending from the old position, so the correction is invisible to the player. Removed when the offset reaches zero.

## 6. Implementation Notes

1. `authority.go` `NetworkAuthority` (the predicted/interpolated partition gating INV-5).
2. `history.go` ring + crc32 checksum (INV-3 mismatch detection).
3. `predict.go` prediction loop over the `DeterministicSchedule` subset (INV-2).
4. `reconcile.go` rollback-resimulate via `SnapshotManager`+`InputBuffer`+`DeterministicSchedule` (INV-1/4) — characterization-test against a deterministic fixture where a forced server divergence triggers exactly one rollback that reproduces live state.
5. `smoothing.go` blend component + decay system; `plugin.go` wiring.

## 7. Drawbacks & Alternatives

- **Determinism is a hard prerequisite.** A single non-deterministic op in a predicted system causes constant mispredictions. Mitigation: predicted systems are an explicit opt-in subset of the `DeterministicSchedule`; the rollback test fixture surfaces drift early. The cost is real — prediction is the most demanding netcode model.
- **Per-tick prediction snapshots cost memory + CPU.** A 64-deep history of predicted-entity snapshots plus per-tick crc32 is the price of fast reconciliation. Bounded by the predicted-entity count (usually just the local player + held items), so it is small in practice.
- **Snap-then-blend vs. hard snap.** Blending hides corrections but can briefly show a visually "wrong" (old) position. Accepted — a 100 ms blend is far less jarring than a teleport, and the simulation itself is already correct.

## Canonical References

<!-- MANDATORY for Stable status. Stub — this L2 is Draft (L1 parent Draft, no
     implementation yet). Populate with on-disk source/test files when code lands. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-06-05 | Initial Draft: prediction loop over the DeterministicSchedule subset (INV-2), crc32 prediction history (INV-3), rollback-resimulate via SnapshotManager+InputBuffer (INV-1/4, composing networking-system determinism), NetworkAuthority query-filter partition (INV-5), correction-blend smoothing. Implements l1-client-prediction. |
