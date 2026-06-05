# Network RPC — Go Implementation

**Version:** 0.1.0
**Status:** Draft
**Layer:** go
**Implements:** l1-rpc.md

## Overview

This specification defines the Go realization of [Network RPC](l1-rpc.md): typed, one-shot message passing between server and clients over the transport layer. Where replication continuously syncs *state*, RPC delivers discrete *actions* ("cast spell", "request respawn") as typed ECS events — sending an RPC on one peer fires a typed event on the receiver, with no manual deserialization in gameplay code.

The implementation reuses shipped infrastructure: the event system ([l2-event-system-go.md](l2-event-system-go.md)) delivers received RPCs as typed events; the type registry ([l2-type-registry-go.md](l2-type-registry-go.md)) serializes payloads; the transport ([l2-transport-go.md](l2-transport-go.md)) carries them on ChannelID 1 (ReliableUnordered) by default; the replication `EntityMap` ([l2-replication-go.md](l2-replication-go.md)) remaps `EntityID` payload fields across the server/client ID boundary. Everything is stdlib (C-003).

## Related Specifications

- [l2-event-system-go.md](l2-event-system-go.md) — received RPCs are delivered as typed ECS events
- [l2-transport-go.md](l2-transport-go.md) — RPC wire delivery (ChannelID 1 default, configurable)
- [l2-replication-go.md](l2-replication-go.md) — `EntityMap` for payload EntityID remapping
- [l2-networking-system-go.md](l2-networking-system-go.md) — network message pipeline (send/receive systems)
- [l2-type-registry-go.md](l2-type-registry-go.md) — RPC payload serialization
- [l2-command-system-go.md](l2-command-system-go.md) — server-side RPC handlers use commands for deferred mutation

## 1. Motivation

Many gameplay actions are one-shot events, not persistent state — a spell cast, a kill-feed trigger, a chat message. Encoding them as temporary components or generic byte blobs is error-prone and untyped. RPC gives a clean Go-typed interface: define a struct, register it, `Send` it, receive it as a typed event. The Go-specific work is the type↔wire-ID registry (generics for `Send[T]`/`EventWriter[T]`), direction validation, and reusing the existing event bus for delivery so RPC handlers are ordinary `EventReader[T]` systems.

## 2. Constraints & Assumptions

- **C-003 (stdlib only)**: type-registry serialization, `encoding/binary` framing. No third-party RPC framework (no gRPC/Cap'n Proto — those would require a C32 ADR and fight the UDP/event model).
- RPCs are fire-and-forget; reliability is the transport channel's job, not RPC's.
- Payloads must be type-registry-serializable (no closures/function pointers).
- RPCs have a direction (Client→Server, Server→Client, Server→AllClients, Server→Group); request-response is composed from two unidirectional RPCs.
- Handlers run as ECS event readers in the standard schedule — never on the transport thread (INV-4).
- RPC does not authenticate/authorize; that is game policy.

## 3. Core Invariants (Layer 1 only)

This is a Layer 2 specification; the authoritative invariants live in [l1-rpc.md §3](l1-rpc.md). They are mapped to their Go realization in §4 below.

## 4. Invariant Compliance (Layer 2 only)

| L1 Invariant | Go Implementation |
| :--- | :--- |
| **INV-1** — a registered RPC produces a typed event on the receiver; sender never receives its own | `RpcRegistry` maps `RpcTypeID (uint16) ↔ reflect.Type`. `Send[T](target, payload)` serializes + enqueues on transport; it never loops back into the local event bus. `RpcReceiveSystem` deserializes inbound payloads and calls `EventWriter[T].Send`, so only the *remote* peer sees the event. `RpcTarget.Except(conn)` supports "all but sender" to avoid server-side echo. |
| **INV-2** — delivery order follows the transport channel | RPC carries no ordering logic of its own; it is enqueued on the `RpcDefinition.Channel` (default 1 = ReliableUnordered → guaranteed, unordered; channel 2 = ReliableOrdered → guaranteed, in-order). The guarantee is exactly the transport channel's (l2-transport-go INV-2/3). |
| **INV-3** — unknown/unregistered type IDs are dropped + logged, never crash | `RpcReceiveSystem` looks up the inbound `rpc_type_id` in `RpcRegistry`; a miss is `slog.Warn`-logged and the packet skipped. Payload length is bounds-checked before deserialization, so a malformed/hostile RPC cannot panic the receiver. |
| **INV-4** — handlers run in the ECS schedule, not on the transport thread | `RpcReceiveSystem` runs in `PreUpdate` (after `transport.Drain()`), converting wire messages to events; gameplay handlers are ordinary `EventReader[T]` systems in the standard schedule. No RPC code touches the socket goroutine. |

> All four invariants are addressed. RFC promotion is blocked by the layer constraint: l1-rpc is Draft.

## 5. Detailed Design

### 5.1 Package Layout

```plaintext
internal/net/rpc/
  registry.go    // RpcRegistry, RpcDefinition, RpcDirection, RegisterRpc[T]
  sender.go      // RpcSender, Send[T], RpcTarget (Server/Client/AllClients/Group/Except)
  receive.go     // RpcReceiveSystem (PreUpdate): decode → registry → direction check → EntityMap → EventWriter[T]
  message.go     // RpcMessage wire framing (type_id + payload) encode/decode
  ratelimit.go   // RpcRateLimit (global + per-type, server-side)
  plugin.go      // RpcPlugin: registers default RPCs, receive system, rate limiter
```

### 5.2 Registry and Registration

```plaintext
[REFERENCE]
type RpcTypeID uint16
type RpcDirection uint8 // ClientToServer | ServerToClient | ServerToAllClients | ServerToGroup

type RpcDefinition struct {
    TypeID    RpcTypeID
    Name      string
    Direction RpcDirection
    Channel   net.ChannelID         // default 1 (ReliableUnordered)
    Type      reflect.Type          // type-registry-backed serialization
}

func RegisterRpc[T any](app App, cfg RpcConfig)  // assigns a compact TypeID, exchanged at handshake
```

`RpcTypeID`s are agreed during the connection handshake (alongside channel config), so both peers share the type↔ID mapping.

### 5.3 Sending

```plaintext
[REFERENCE]
type RpcTarget interface{} // Server | Client(ConnectionID) | AllClients | Group([]ConnectionID) | Except(ConnectionID)

Send[T](target RpcTarget, payload T):
  1. typeID := registry.IDFor(reflect.TypeFor[T]())
  2. data := typereg.Marshal(payload)
  3. EntityMap.Remap(payload fields) — local IDs → remote IDs
  4. enqueue RpcMessage{typeID, data} on RpcDefinition.Channel for the resolved connection(s)
```

`Send` validates the target against the RPC's registered `Direction` (a client may only target `Server`); an illegal direction is a logged error, not a panic.

### 5.4 Receiving

```plaintext
RpcReceiveSystem(world, writers) [PreUpdate, after Drain]:
  for pkt := range transport.Drain()[rpc channels]:
    msg := decode(pkt)                       // bounds-checked
    def, ok := registry.ByID(msg.TypeID)
    if !ok { slog.Warn("unknown rpc"); continue }   // INV-3
    if !directionValidForReceiver(def) { continue } // e.g. client drops ClientToServer
    payload := typereg.Unmarshal(def.Type, msg.Data)
    EntityMap.Remap(payload fields)          // remote IDs → local IDs (INV: no foreign ID leak)
    writers.For(def.Type).Send(payload)      // typed ECS event (INV-1/4)
```

### 5.5 Request-Response & Rate Limiting

- **Request-response** is a convention (two unidirectional RPCs + a client-generated `request_id` correlation token); the engine does not enforce pairing. A future `RpcFuture` could auto-correlate.
- **Rate limiting** (`RpcRateLimit` resource, server-side): a token bucket per connection (global default 100 RPC/s) plus optional per-`RpcTypeID` limits, reusing the same limiter pattern as the AI-API plugin. Over-limit inbound RPCs are dropped + counted (a diagnostic), protecting the server from a flooding client.

## 6. Implementation Notes

1. `registry.go` `RegisterRpc[T]` + `RpcDefinition` (the type↔ID contract both peers share).
2. `message.go` wire framing (+ fuzz the decoder for INV-3 robustness).
3. `sender.go` `Send[T]` + target resolution + direction validation.
4. `receive.go` `RpcReceiveSystem` (unknown-ID drop INV-3, EntityMap remap, typed-event emit INV-1/4).
5. `ratelimit.go` + `plugin.go` wiring.

## 7. Drawbacks & Alternatives

- **Generics + reflection for `Send[T]`/typed delivery.** A fully codegen RPC layer (à la gRPC) would be faster and schema-checked, but is a heavy third-party dependency that fights the UDP/event model and violates C-003. The generics-over-type-registry approach is zero-dep and integrates with the existing event bus; a codegen marshaler is a future optimization behind the same API.
- **`request_id` correlation is unenforced.** Leaving request-response as a convention keeps the core simple but pushes correlation bookkeeping to the game. Documented; `RpcFuture` is the named future extension.
- **Per-connection rate limiter state.** A token bucket per connection per type costs memory under many clients/types; bounded by the registered RPC count and connection count, and far cheaper than the DoS it prevents.

## Canonical References

<!-- MANDATORY for Stable status. Stub — this L2 is Draft (L1 parent Draft, no
     implementation yet). Populate with on-disk source/test files when code lands. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-06-05 | Initial Draft: type↔wire-ID RpcRegistry, generic `Send[T]` + direction-validated targets, `RpcReceiveSystem` (unknown-ID drop INV-3, EntityMap remap, typed-event delivery INV-1/4) on transport channels (INV-2), token-bucket rate limiting. Implements l1-rpc. |
