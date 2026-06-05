# Networking System — Go Implementation

**Version:** 0.1.0
**Status:** Draft
**Layer:** go
**Implements:** l1-networking-system.md

## Overview

This specification defines the Go realization of the [Networking System](l1-networking-system.md) — the engine's *boundary* with multiplayer infrastructure. The engine is not a netcode framework; it ships deterministic primitives that netcode plugins build on: a deterministic fixed-timestep schedule, a snapshot/rollback manager, an input buffer, and the `NetworkTransport` abstraction (realized by [l2-transport-go.md](l2-transport-go.md)).

The implementation reuses shipped engine infrastructure rather than inventing new machinery: the DAG scheduler ([l2-system-scheduling-go.md](l2-system-scheduling-go.md)) and `gametime.FixedTime` ([l2-time-system-go.md](l2-time-system-go.md)) drive deterministic ticks; the scene serialization codec ([l2-scene-system-go.md](l2-scene-system-go.md)) — the *same binary format* the hot-reload snapshot uses ([l2-hot-reload-go.md](l2-hot-reload-go.md)) — backs world snapshots; the task IO pool ([l2-task-system-go.md](l2-task-system-go.md)) hosts the transport goroutine off the main loop. Everything is stdlib (C-003); `hash/crc32` provides the desync checksum.

## Related Specifications

- [l2-transport-go.md](l2-transport-go.md) — the concrete UDP realization of the `NetworkTransport` interface this spec owns
- [l2-system-scheduling-go.md](l2-system-scheduling-go.md) — `DeterministicSchedule` is a fixed-order sequential executor
- [l2-time-system-go.md](l2-time-system-go.md) — `FixedTime` drives the deterministic tick rate
- [l2-scene-system-go.md](l2-scene-system-go.md) — world-serialization codec backing snapshots
- [l2-hot-reload-go.md](l2-hot-reload-go.md) — shares the snapshot binary lineage (entity-ID-stable capture)
- [l2-task-system-go.md](l2-task-system-go.md) — IO pool for the transport goroutine
- [l2-event-system-go.md](l2-event-system-go.md) — network events delivered on the standard bus
- [l2-app-framework-go.md](l2-app-framework-go.md) — `NetworkPlugin` as a SubApp; ServiceRegistry for transport injection

## 1. Motivation

Multiplayer needs deterministic simulation, state sync, and latency compensation — all of which fight a normal engine's variable-rate, parallel execution. Rather than bake in one netcode model, the engine exposes the *primitives* so rollback, lockstep, and client-server libraries can be plugins. The Go realization's job is to make those primitives (a) deterministic on demand, (b) snapshot-able efficiently, and (c) network-isolated from the game loop — reusing the ECS pieces that already exist instead of a parallel stack.

The defining reuse: the L1 states that snapshot serialization "uses the same binary format as scene saves and hot-reload." The Go realization makes that literal — `SnapshotManager` serializes through the `scene` codec, the exact mechanism the just-shipped hot-reload restore pins entity IDs with, so a network rollback and a code hot-reload share one battle-tested capture path.

## 2. Constraints & Assumptions

- **C-003 (stdlib only)**: scene codec + `hash/crc32` (desync checksum) + `encoding/binary` (input serialization). No third-party netcode or serialization library.
- **Not editor-gated**: networking is a runtime (shipping) feature, so the `SnapshotManager` reuses the *non-editor* `pkg/scene` codec directly — it does not import `internal/hotreload` (which is `//go:build editor`). The two share the codec, not the package.
- Determinism is **opt-in** per system (`AddDeterministicSystems`); only those run in the fixed-order `DeterministicSchedule`. Particles/audio stay in the regular `Update`.
- No network I/O on the main thread — the transport goroutine (l2-transport-go) is the only socket owner; the ECS exchanges via channels.
- The engine provides a *reference* `RollbackCoordinator`; a netcode plugin may replace it wholesale via ServiceRegistry (the engine offers primitives, not a mandate).
- Entity-ID stability across snapshot/restore reuses the same mechanism hot-reload uses (`EntityAllocator` placement) so cached handles survive a rollback.

## 3. Core Invariants (Layer 1 only)

This is a Layer 2 specification; the authoritative invariants live in [l1-networking-system.md §3](l1-networking-system.md). They are mapped to their Go realization in §4 below.

## 4. Invariant Compliance (Layer 2 only)

| L1 Invariant | Go Implementation |
| :--- | :--- |
| **INV-1** — no network I/O on the main thread / `First`..`Last` schedules | All socket work is on the transport's IO-pool goroutine (l2-transport-go). The ECS touches the network only through two systems: `NetworkReceive` (`PreUpdate`, drains the transport's inbound channel → ECS events) and `NetworkSend` (`PostUpdate`, collects outbound → transport channel). No schedule calls `net`. |
| **INV-2** — deterministic systems are platform/framerate independent | `DeterministicSchedule` wraps a `SequentialExecutor` (no parallelism) over an explicit `[]SystemID` order; the per-tick RNG is seeded from `rng_seed ^ tick` (a `math/rand/v2` source re-seeded each tick); deterministic systems read only `tick_number` + `fixed_timestep`, never wall-clock. Reuses the existing scheduler + `gametime.FixedTime` fixed-step loop. |
| **INV-3** — snapshots are self-contained | `SnapshotManager.TakeSnapshot` captures the World through the `scene` codec into a `SnapshotHandle{tick, data, checksum, entity_count}`; `RestoreSnapshot` rebuilds purely from `data` (no external state), pinning entity IDs via `EntityAllocator` placement (the hot-reload INV-5 mechanism). A `hash/crc32` checksum enables desync detection. |
| **INV-4** — input replay == live execution | `InputBuffer` stores `SerializedInput{tick, player, data, checksum}` in a 128-tick ring; `data` is a platform-independent `encoding/binary` packing (button bitfields + fixed-point axes), so replaying recorded input through the `DeterministicSchedule` reproduces the live result bit-for-bit. `PredictInput` returns last-known input (repeat-last), overridable by a plugin. |
| **INV-5** — transport never leaks protocol types into gameplay | Gameplay sees only handle types (`ConnectionID`, `ChannelID`) and ECS events (`Connected`/`Disconnected`); the concrete `*transport.UDPTransport` is injected behind the `NetworkTransport` interface via `ServiceRegistry` and never named in game code. `InboundPacket` is internal — consumed by replication/RPC, not exposed. |

> All five invariants are addressed. RFC promotion is blocked by the layer constraint: l1-networking-system is Draft.

## 5. Detailed Design

### 5.1 Package Layout

```plaintext
internal/net/
  transport.go        // NetworkTransport interface + ConnectionID/ChannelID/DeliveryMode/
                      // ChannelConfig/InboundPacket/Connected/Disconnected (the abstraction;
                      // implemented by internal/net/transport — l2-transport-go)
  deterministic.go    // DeterministicSchedule, AddDeterministicSystems, per-tick RNG
  snapshot.go         // SnapshotManager (ring buffer, TakeSnapshot/RestoreSnapshot/GetDelta), CRC32
  input.go            // InputBuffer (ring), SerializedInput pack/unpack, PredictInput
  rollback.go         // RollbackCoordinator reference impl (replaceable via ServiceRegistry)
  systems.go          // NetworkReceive (PreUpdate) + NetworkSend (PostUpdate)
  plugin.go           // NetworkPlugin (SubApp), wires transport + systems + resources
```

### 5.2 Deterministic Schedule

```plaintext
[REFERENCE]
type DeterministicSchedule struct {
    FixedTimestep time.Duration   // from gametime.FixedTime
    Order         []scheduler.System
    RngSeed       uint64
    Tick          uint64          // monotonic
}
RunTick(tick uint64, inputs *InputBuffer):
    seedRNG(RngSeed ^ tick)
    for _, sys := range Order { sys.Run(world) }   // SequentialExecutor, fixed order
```

Built atop the Stable scheduler: `Order` is a pre-sorted system list run by a `SequentialExecutor` (no DAG parallelism → determinism). `gametime.FixedTime` already provides the fixed-step accumulator loop; the deterministic schedule plugs into its `FixedUpdate` tick.

### 5.3 Snapshot Manager (reuses the scene codec)

```plaintext
[REFERENCE]
type SnapshotManager struct {
    ring    []SnapshotHandle   // configurable, default 16
    codec   sceneCodec         // pkg/scene serialize/deserialize
}
TakeSnapshot(tick) SnapshotHandle:  serialize World → data; crc32(data) → checksum
RestoreSnapshot(h):                 deserialize h.data → World, pinning entity IDs
GetDelta(from, to) DeltaSnapshot:   per-component diff → changed/removed/added only
```

The ring buffer bounds rollback memory (16 ticks ≈ 266 ms at 60 Hz). `GetDelta` is the network-sync primitive — replication (a later spec) sends deltas, not full snapshots. Entity-ID-stable restore is the same `EntityAllocator` placement the hot-reload restore uses, so a rollback does not invalidate cached `Entity` handles.

### 5.4 Input Buffer & Rollback Coordinator

`InputBuffer` is a 128-entry ring of `map[PlayerID]SerializedInput`; `SerializedInput.data` is an `encoding/binary` packing (buttons → bitfield, axes → fixed-point) so it is platform-stable (INV-4). The `RollbackCoordinator` is the reference rollback-resimulate loop (on late remote input: restore the snapshot at that tick, re-run the `DeterministicSchedule` to `current_tick`, re-snapshot each tick), explicitly replaceable by a netcode plugin via `ServiceRegistry`.

### 5.5 Network Pipeline Systems

```plaintext
NetworkReceive (PreUpdate):  for _, pkt := range transport.Drain() { decode → ECS event }
NetworkSend   (PostUpdate):  collect outbound game messages → transport.Send/Broadcast
```

Two systems bracket the frame; combined with the transport goroutine's channels, this is the full producer/consumer pipeline that keeps the socket off the game loop (INV-1).

### 5.6 NetworkPlugin

A SubApp-style plugin that: registers the default channels (0 Unreliable, 1 ReliableUnordered, 2 ReliableOrdered, 3 Unreliable), injects the concrete `NetworkTransport` (UDP by default) into the `ServiceRegistry`, installs `SnapshotManager`/`InputBuffer`/`RollbackCoordinator` resources, and adds the receive/send systems. Opt-in (not in DefaultPlugins); headless servers and clients share it.

## 6. Implementation Notes

1. `transport.go` interface + handle/event types — the contract l2-transport-go implements.
2. `snapshot.go` over the scene codec + CRC32 + ring (the highest-reuse primitive; characterization-test round-trip + delta).
3. `input.go` deterministic pack/unpack (+ fuzz the decoder) + `deterministic.go` schedule.
4. `rollback.go` reference coordinator (table-test the late-input rollback path against a deterministic fixture).
5. `systems.go` + `plugin.go` wiring; inject the UDP transport.

## 7. Drawbacks & Alternatives

- **Reference RollbackCoordinator may be thrown away.** Shipping a reference netcode loop risks implying the engine endorses one model. Mitigation: it is ServiceRegistry-replaceable and documented as a starting point; the durable value is the primitives (snapshot/input/deterministic schedule), not the coordinator.
- **Full-snapshot ring vs. always-delta.** A 16-deep full-snapshot ring trades memory for simplicity (any tick is restorable directly). An always-delta chain would save memory but make restore O(depth). The ring is chosen for bounded, predictable rollback cost; `GetDelta` still serves the network path.
- **Sharing the scene codec couples networking to scene-system evolution.** A codec change must keep snapshot/restore stable. Accepted — the alternative (a parallel network serialization format) duplicates a large, tested subsystem and violates the L1's explicit "same binary format" requirement.

## Canonical References

<!-- MANDATORY for Stable status. Stub — this L2 is Draft (L1 parent Draft, no
     implementation yet). Populate with on-disk source/test files when code lands. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-06-05 | Initial Draft: deterministic fixed-step schedule (reuses scheduler + FixedTime), scene-codec-backed SnapshotManager (ring + CRC32 + entity-ID-stable restore, shared lineage with hot-reload), binary InputBuffer, reference RollbackCoordinator, transport-abstraction injection (INV-5), PreUpdate/PostUpdate network pipeline. Implements l1-networking-system. |
