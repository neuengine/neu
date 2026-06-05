# Replication — Go Implementation

**Version:** 0.1.0
**Status:** Draft
**Layer:** go
**Implements:** l1-replication.md

## Overview

This specification defines the Go realization of the [Replication](l1-replication.md) system: it synchronizes whitelisted entity/component state from a server World to client Worlds over the transport layer, answering *what* (markers + rules), *to whom* (visibility), and *how often* (priority/frequency). It is the keystone of the networking consumer tier — the three sync models (interpolation, prediction, lockstep) all build on its primitives.

The implementation reuses shipped engine infrastructure rather than a parallel stack: the change-detection system ([l2-change-detection-go.md](l2-change-detection-go.md)) provides dirty tracking (no second dirty-flag mechanism); the scene system ([l2-scene-system-go.md](l2-scene-system-go.md)) + type registry ([l2-type-registry-go.md](l2-type-registry-go.md)) provide reflection serialization and the EntityID-field remap walk; the command system ([l2-command-system-go.md](l2-command-system-go.md)) applies inbound state as deferred mutations (so replication never mutates the source World, INV-5); the transport ([l2-transport-go.md](l2-transport-go.md)) delivers messages. Everything is stdlib (C-003).

## Related Specifications

- [l2-networking-system-go.md](l2-networking-system-go.md) — parent boundary; SnapshotManager/InputBuffer/DeterministicSchedule
- [l2-transport-go.md](l2-transport-go.md) — channels 0/3 (unreliable state), 1 (reliable spawn/despawn/events)
- [l2-change-detection-go.md](l2-change-detection-go.md) — `ChangedTick`/`Changed[T]` drive `OnChange` rules + delta
- [l2-scene-system-go.md](l2-scene-system-go.md) — `SerializedComponent` + the EntityID-field remap walk
- [l2-type-registry-go.md](l2-type-registry-go.md) — binary component serialization + reflection metadata
- [l2-command-system-go.md](l2-command-system-go.md) — inbound state applied as deferred commands (INV-5)
- [l2-rpc-go.md](l2-rpc-go.md) — consumes the `EntityMap` this spec owns for payload remapping

## 1. Motivation

Every multiplayer project re-solves *what to sync*, *to whom*, and *how often*. Doing it once at the engine level — on a whitelist model, with pluggable visibility and priority-accumulator bandwidth control — gives netcode plugins a clean interface. The Go-specific challenge is doing this *as ECS systems within the standard schedule* (not on a thread), reading through the existing change-detection ticks and writing through the existing command buffer, so replication is just another well-behaved system pair rather than a privileged subsystem.

## 2. Constraints & Assumptions

- **C-003 (stdlib only)**: type-registry binary serialization + `encoding/binary` framing. No third-party serialization library.
- Whitelist model: nothing replicates without an explicit `Replicated` marker + a `ReplicationRule` (INV-1).
- Authority-agnostic: replication moves state; *who may write* is a policy layer above.
- Runs as ECS systems in the standard schedule (`PostUpdate` send, `PreUpdate` receive), reading the transport's drained queue — not on a separate thread.
- Server and client `EntityAllocator`s are independent; the `EntityMap` bridges their ID spaces (INV-2/3).
- Delta tracking reuses change-detection `ChangedTick` — no parallel dirty state.

## 3. Core Invariants (Layer 1 only)

This is a Layer 2 specification; the authoritative invariants live in [l1-replication.md §3](l1-replication.md). They are mapped to their Go realization in §4 below.

## 4. Invariant Compliance (Layer 2 only)

| L1 Invariant | Go Implementation |
| :--- | :--- |
| **INV-1** — only marked components are sent | `Replicated`/`ServerOnly`/`ClientOnly` are zero-sized ECS **tag components**; `ReplicationConfig` resource holds `map[component.ID]ReplicationRule`. The send system iterates only entities carrying `Replicated`, and for each, only components whose rule is `Replicate` (skipping `ServerOnly`). An unmarked component is never reached by the serializer. |
| **INV-2** — bijective server↔client entity mapping within visibility | `EntityMap` (per-connection resource on server, global on client) holds `serverToClient`/`clientToServer` maps; `Map(serverID)` returns the existing client `EntityID` or allocates a fresh client entity (Bevy `SceneEntityMapper` pattern) recording both directions. Bijection is maintained by `Unmap` on visibility-leave/despawn. |
| **INV-3** — no server EntityID leaks into client gameplay | `EntityMap.Remap` walks every `EntityID` field of a `ReflectedComponent` using type-registry metadata (the **same reflection walk** `SceneSpawner.Spawn` uses) and substitutes the client mapping; an unmapped reference becomes `EntityID` placeholder + a queued deferred resolution applied when the referenced entity later arrives. Game code only ever sees client IDs. |
| **INV-4** — a client never receives data outside its visibility set | `VisibilityPolicy` (interface) computes a per-connection `VisibilitySet{entities, added, removed}` each tick; built-ins `ReplicateAll`/`GridVisibility`/`CustomVisibility`. The send system serializes a component only when its entity is in that connection's `VisibilitySet.entities`; an entity entering sends a full snapshot, leaving sends `EntityDespawn` + `Unmap`. |
| **INV-5** — replication never mutates the source World | Send systems are **read-only** over the World (queries + change ticks) and produce outbound transport messages. Inbound messages are applied exclusively through `command.CommandBuffer` (deferred spawn/insert/despawn), flushed at the standard apply point — never a direct World write. |
| **INV-6** — server despawn → despawn message to relevant clients | A despawned `Replicated` entity (detected via `RemovedComponents[Replicated]` / a despawn observer) enqueues an `EntityDespawn` on channel 1 (ReliableUnordered) to every connection whose `VisibilitySet` contained it, then `Unmap`s it per connection. |

> All six invariants are addressed. RFC promotion is blocked by the layer constraint: l1-replication is Draft.

## 5. Detailed Design

### 5.1 Package Layout

```plaintext
internal/net/replication/
  markers.go        // Replicated/ServerOnly/ClientOnly tag components, ReplicationRule, ReplicationConfig
  entitymap.go      // EntityMap (Map/Unmap/Remap, placeholder + deferred resolution)
  visibility.go     // VisibilityPolicy interface + ReplicateAll/GridVisibility/CustomVisibility, VisibilitySet
  message.go        // ReplicationMessage variants + binary encode/decode, channel assignment
  delta.go          // ClientAckState, ChangedTick delta selection, optional DeltaSerializer (Diff/Patch)
  priority.go       // ReplicationPriority accumulator + bandwidth budget
  send.go           // ReplicationSend system (PostUpdate): visibility → delta → priority → serialize → transport
  receive.go        // ReplicationReceive system (PreUpdate): decode → EntityMap → command apply (INV-5)
  plugin.go         // ReplicationPlugin: default rules, resources, systems
```

### 5.2 Markers and Rules

```plaintext
[REFERENCE]
type Replicated struct{}   // zero-sized tag
type ServerOnly struct{}   // zero-sized tag
type ClientOnly struct{}   // zero-sized tag

type ReplicationRule uint8 // Replicate | ServerOnly | ClientOnly | OnChange | OnInterval(n) | OnEvent
type ReplicationConfig struct { rules map[component.ID]ReplicationRule }
```

Defaults registered by `ReplicationPlugin`: `Transform → Replicate+OnChange`, `GlobalTransform → ServerOnly` (recomputed client-side), `Name → Replicate+OnEvent`.

### 5.3 EntityMap (the shared remap primitive)

```plaintext
[REFERENCE]
type EntityMap struct {
    serverToClient map[entity.EntityID]entity.EntityID
    clientToServer map[entity.EntityID]entity.EntityID
    deferred       []pendingRef   // unresolved references awaiting their target
}
Map(serverID) clientID            // existing or freshly allocated client entity
Unmap(serverID)                   // remove both directions; despawn client entity via command
Remap(rc ReflectedComponent)      // walk EntityID fields (typereg metadata) → client IDs; placeholder if unmapped
```

`EntityMap` is owned by this package and consumed by [l2-rpc-go.md](l2-rpc-go.md) for payload remapping — a clean one-way `rpc → replication` edge.

### 5.4 Send Pipeline (PostUpdate, server)

```plaintext
ReplicationSend(world):
  for each connection:
    policy.Update(world, visibilitySets)            // INV-4 — compute who sees what
    for each entity in visibilitySet.added:          send EntitySpawn (full snapshot)
    for each entity in visibilitySet.removed:        send EntityDespawn; EntityMap.Unmap
    pending := collectChangedComponents(world, ack)  // INV-1 markers + ChangedTick > acked (delta)
    sort pending by priority accumulator             // §5.6
    serialize until bandwidth budget exhausted; enqueue on the rule's channel
```

### 5.5 Receive Pipeline (PreUpdate, client) — INV-5

```plaintext
ReplicationReceive(world, cmd CommandBuffer):
  for pkt := range transport.Drain()[replication channels]:
    msg := decode(pkt)
    switch msg:
      EntitySpawn:    clientE := EntityMap.Map(msg.serverEntity)
                      for each comp: Remap; cmd.Insert(clientE, comp)     // deferred (INV-5)
      ComponentUpdate: clientE := EntityMap.Map(...); cmd.Insert(...)
      ComponentRemove: cmd.Remove(...)
      EntityDespawn:  cmd.Despawn(EntityMap.Map(...)); EntityMap.Unmap(...)
  // cmd flushed at the standard command apply point
```

### 5.6 Delta, Priority, Bandwidth

- **Delta**: `ClientAckState.lastAckedTick[entity]` vs each component's `ChangedTick` (change-detection) selects only newer components; optional per-type `DeltaSerializer{Diff,Patch}` for field-level diffs (slice/struct-bitmask built-ins); absent → full component.
- **Priority accumulator**: `effective = base + accumulator`; sort descending, serialize within budget, reset sent / `accumulator += base` for deferred (no starvation).
- **Bandwidth budget**: `transport.NetworkSettings.MaxSendRate × replicationBudgetFraction` (default 0.8); the remaining 20% reserved for reliable/heartbeat.

## 6. Implementation Notes

1. `markers.go` + `ReplicationConfig` (the whitelist, INV-1) — the gate everything else respects.
2. `entitymap.go` + `Remap` reusing the scene EntityID-field walk (+ deferred resolution) — the INV-2/3 core; characterization-test the bijection + placeholder resolution.
3. `message.go` binary codec (+ fuzz the decoder) + channel assignment.
4. `delta.go` over change-detection ticks + `priority.go` accumulator.
5. `send.go`/`receive.go` systems (receive applies via command buffer, INV-5) + `visibility.go` policies.
6. `plugin.go` wiring + default rules.

## 7. Drawbacks & Alternatives

- **Reflection-based serialization cost.** Walking component fields via the type registry per entity per tick is slower than generated codegen marshalers. Accepted for v1 (correctness + zero new deps); a future codegen path (l2-codegen-tools) could emit per-type marshalers behind the same `DeltaSerializer` interface.
- **Per-connection VisibilitySet memory.** A server with many clients holds an `EntitySet` per connection. `GridVisibility` bounds the working set; the cost is inherent to interest management and far cheaper than replicating everything to everyone.
- **EntityMap unbounded growth.** Long sessions accumulate mappings; `Unmap` on visibility-leave/despawn reclaims them, but a pathological churn pattern could grow the deferred queue. Bounded by the visibility set size; a TTL on deferred refs is a future hardening.

## Canonical References

<!-- MANDATORY for Stable status. Stub — this L2 is Draft (L1 parent Draft, no
     implementation yet). Populate with on-disk source/test files when code lands. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-06-05 | Initial Draft: whitelist markers + rules (INV-1), bijective EntityMap + reflection remap reusing the scene EntityID walk (INV-2/3), pluggable visibility (INV-4), read-only send + command-applied receive (INV-5), despawn propagation (INV-6), change-detection delta + priority-accumulator bandwidth control. Implements l1-replication. |
