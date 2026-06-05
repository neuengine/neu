# Transport — Go Implementation

**Version:** 0.1.0
**Status:** Draft
**Layer:** go
**Implements:** l1-transport.md

## Overview

This specification defines the Go realization of the [Transport](l1-transport.md) layer: a UDP-based message transport with user-space reliability, per-channel delivery guarantees, connection lifecycle management, and Path-MTU discovery. It implements the `NetworkTransport` abstraction owned by [l2-networking-system-go.md](l2-networking-system-go.md), so gameplay code never sees a socket (transport INV-5).

The implementation uses only the standard library — `net.UDPConn` for the socket, `encoding/binary` for the wire codec, `hash/crc32`/sequence bitfields for integrity — honoring the engine's zero-dependency rule (C-003). It runs on a dedicated goroutine borrowed from the task system's IO pool ([l2-task-system-go.md](l2-task-system-go.md)), exchanging data with the ECS loop exclusively through buffered channels so the main loop never blocks on the network (INV-5 of transport).

## Related Specifications

- [l2-networking-system-go.md](l2-networking-system-go.md) — owns the `NetworkTransport` interface + handle types this spec implements; injects the UDP transport via ServiceRegistry
- [l2-task-system-go.md](l2-task-system-go.md) — IO pool supplies the network goroutine
- [l2-event-system-go.md](l2-event-system-go.md) — `Connected`/`Disconnected` lifecycle events delivered on the bus
- [l2-platform-system-go.md](l2-platform-system-go.md) — `SocketBackend` varies per platform (POSIX/WASM/console)
- [l2-diagnostic-system-go.md](l2-diagnostic-system-go.md) — `ConnectionStats` (RTT, loss, bandwidth) surfaced as diagnostics

## 1. Motivation

Every netcode model needs the same primitive: send bytes A→B with configurable reliability, without TCP's head-of-line blocking. Go's `net` package gives a raw `UDPConn`; everything above it — framing, ACKs, reordering, heartbeats, MTU — must be built in user space. This spec builds that once, on stdlib, behind an interface so the higher layers (replication, RPC, snapshots) focus on game state rather than packet headers.

The hard Go-specific constraint is the goroutine boundary: the socket read/write loop runs off-thread, but the ECS loop is single-threaded and must not block on I/O. The design therefore is a classic producer/consumer over buffered channels — the network goroutine fills an inbound channel, the `PreUpdate` system drains it; the `PostUpdate` system fills an outbound channel, the network goroutine flushes it.

## 2. Constraints & Assumptions

- **C-003 (stdlib only)**: `net` (UDP socket), `encoding/binary` (wire codec), `time` (RTO/heartbeat), `hash/crc32` if a packet checksum is added. No third-party networking library (a QUIC/WebRTC dependency would require a C32 ADR).
- UDP exclusively for game traffic; MTU conservatively 1200 bytes; larger messages are fragmented over the `ReliableOrdered` path and reassembled.
- One `*UDPTransport` per process; multiple connections (server hosting N clients) keyed by `ConnectionID`.
- The transport goroutine is the *only* goroutine touching the socket; all cross-thread handoff is via channels (share-by-communicating, not shared mutexes on the hot path).
- No encryption in v1 (DTLS is a future ADR); no congestion control in v1 (rate limiting only).
- The `SocketBackend` interface abstracts the concrete socket so non-POSIX targets (WASM) can supply their own; the default is `net.UDPConn`.

## 3. Core Invariants (Layer 1 only)

This is a Layer 2 specification; the authoritative invariants live in [l1-transport.md §3](l1-transport.md). They are mapped to their Go realization in §4 below.

## 4. Invariant Compliance (Layer 2 only)

| L1 Invariant | Go Implementation |
| :--- | :--- |
| **INV-1** — `Unreliable` may drop/reorder, no retransmit | Channel 0/3 frames are sent once with a per-channel `msg_seq`; the receiver discards any frame whose seq is older than the highest seen beyond a 64-gap tolerance. No ACK bookkeeping, no resend queue. |
| **INV-2** — `Reliable` (ordered) exactly-once in order, else connection lost | A per-channel sliding window (256) of unACKed frames; piggybacked 32-bit ACK bitfield on every datagram; retransmit after `RTO = SRTT + 4·RTTVAR` (EWMA via `time.Duration` math). A frame unACKed past the max retransmit budget closes the connection with `Disconnected{reason: Timeout}`. The receiver buffers out-of-order frames in a reorder map keyed by `msg_seq` and releases them contiguously. |
| **INV-3** — `ReliableUnordered` exactly-once, any order | Same ACK/retransmit machinery as INV-2 but delivered immediately on arrival; a 256-wide received-`msg_seq` bitfield per channel drops duplicates silently. |
| **INV-4** — heartbeat every interval; timeout → `Disconnected` | A `time.Ticker`-free tick check inside the network goroutine's loop: if `now - lastSent ≥ heartbeatInterval` send an empty keepalive on channel 2; if `now - lastRecv ≥ timeoutDuration` transition the connection to `Disconnected` and enqueue a `Disconnected{reason: Timeout}` event. |
| **INV-5** — never block the ECS main loop | `Send`/`Broadcast` enqueue onto a buffered `chan outbound` (non-blocking; on overflow the lowest-priority Unreliable frames are dropped, Reliable queued); `Drain()` returns whatever the network goroutine has pushed onto `chan []InboundPacket`. The socket read/write loop lives on an IO-pool goroutine; no schedule calls `net` directly. |
| **INV-6** — monotonic per-channel sequence numbers | The transport holds an `atomic`-free per-`(ConnectionID, ChannelID)` `uint16` counter (mutated only on the network goroutine); each `MessageFrame` stamps `msg_seq++`, each datagram stamps a connection-level `packet_seq++`. Receivers use both for loss detection and reordering. |

> All six invariants are addressed. RFC promotion is blocked by the layer constraint: l1-transport is Draft.

## 5. Detailed Design

### 5.1 Package Layout

```plaintext
internal/net/                      // the abstraction (owned by l2-networking-system-go)
  transport.go                     // NetworkTransport interface, ConnectionID, ChannelID,
                                   // DeliveryMode, ChannelConfig, InboundPacket, events
internal/net/transport/            // THIS spec — the UDP realization
  udp.go            // UDPTransport: NetworkTransport impl, the network goroutine loop
  packet.go         // UDPDatagram + MessageFrame encode/decode (encoding/binary)
  reliability.go    // sliding window, ACK bitfield, RTO/EWMA, reorder + dup buffers
  connection.go     // per-connection state: seqs, window, stats, MTU, last-recv/sent
  handshake.go      // ConnectRequest/Accept/Reject/Ack over ReliableOrdered ch 2
  mtu.go            // Path-MTU discovery probe ladder (1400 → 1200 → 576)
  backend.go        // SocketBackend interface + net.UDPConn default impl
  settings.go       // NetworkSettings (heartbeat, timeout, max_send_rate, window sizes)
```

`internal/net` (the interface) and `internal/net/transport` (the impl) are split so a future `WebSocketTransport`/`WebRTCTransport` satisfies the same interface without touching the UDP code (the L1's pluggable-transport table). `transport → net` is the only edge; `net` never imports `transport` (the App injects the impl).

### 5.2 NetworkTransport Realization

```plaintext
[REFERENCE] the interface (defined in internal/net, implemented here):

type NetworkTransport interface {
    Listen(addr SocketAddr) error
    Connect(addr SocketAddr) (ConnectionID, error)
    Disconnect(id ConnectionID, reason string)
    Connections() []ConnectionID
    Send(id ConnectionID, channel ChannelID, payload []byte) error
    Broadcast(channel ChannelID, payload []byte)
    Drain() []InboundPacket            // called once per frame by NetworkPlugin
    Stats(id ConnectionID) ConnectionStats
}

UDPTransport struct (sketch):
    conn        SocketBackend                 // net.UDPConn by default
    settings    NetworkSettings
    channels    [256]ChannelConfig            // fixed delivery mode per ChannelID
    conns       map[ConnectionID]*connection  // network-goroutine-owned
    inbound     chan []InboundPacket          // goroutine → ECS (Drain reads)
    outbound    chan outboundMsg              // ECS → goroutine (Send writes)
    events      chan ConnEvent                // lifecycle → event bus
    done        chan struct{}                 // shutdown
```

### 5.3 Network Goroutine Loop

```plaintext
run() (on an IO-pool goroutine):
  for {
    select {
    case <-done: flush + close socket; return
    case msg := <-outbound:  enqueue into the target connection's send buffer (batched)
    default:
      // 1. read available datagrams (SetReadDeadline → non-blocking-ish)
      for each datagram: decode header (reject bad protocol_id), parse frames,
          run reliability (ACK, dedup, reorder), append delivered frames to a batch.
      // 2. per connection: process ACKs, retransmit timed-out reliable frames,
      //    send heartbeat if idle, check timeout → emit Disconnected.
      // 3. flush batched outbound frames into ≤MTU datagrams (message batching).
      // 4. push the inbound batch onto `inbound`; push events onto `events`.
    }
  }
```

The loop is the single owner of all `*connection` state, so per-connection sequence counters, windows, and buffers need no mutex — the channel boundary is the synchronization (INV-5 / share-by-communicating).

### 5.4 Packet Codec

`encoding/binary` (little-endian) encodes the 8-byte datagram header (`protocol_id`, `connection_id`, `packet_seq`, `flags`, `channel_count`) followed by N `MessageFrame`s (`channel_id`, `msg_seq`, `payload_len`, `payload`). Decode validates `protocol_id` first (foreign-packet rejection → `ProtocolError`) and bounds-checks every length against the datagram size before slicing (no panic on a truncated/hostile packet).

### 5.5 Reliability Layer

```plaintext
[REFERENCE]
sender (per reliable channel):
    window  []pendingFrame   // unACKed, cap = settings.WindowSize (256)
    srtt, rttvar time.Duration
    onAck(bitfield uint32, base seq): mark acked, sample RTT, recompute RTO
    tick(now): retransmit frames whose lastSent+RTO < now

receiver (per reliable channel):
    recvBits  [256]bool ring   // duplicate detection by msg_seq
    reorder   map[uint16][]byte // ReliableOrdered only
    expected  uint16            // next in-order seq to release
```

RTO uses the standard Jacobson/Karels EWMA (`SRTT = (1-α)·SRTT + α·sample`, `RTTVAR = (1-β)·RTTVAR + β·|SRTT-sample|`, `RTO = SRTT + 4·RTTVAR`) — pure `time.Duration` arithmetic, no float.

### 5.6 Connection Lifecycle, Stats, MTU

- `handshake.go` runs the ConnectRequest/Accept/Reject/Ack exchange over `ReliableOrdered` channel 2 before game channels open; a `client_version`/`protocol_id` mismatch yields `ConnectReject` → `Disconnected{reason: Rejected}`.
- `ConnectionStats` (RTT, RTTVAR, packet_loss over a 128-packet ring, byte counters, send/receive rate) is updated on the network goroutine and copied out by value in `Stats()`.
- `mtu.go` runs the PMTUD probe ladder (1400 → 1200 → 576) on connect and stores the negotiated MTU per connection for fragmentation decisions.
- Send-rate limiting throttles Unreliable channels first when the outbound queue exceeds `max_send_rate`; Reliable frames are queued, never dropped.

## 6. Implementation Notes

1. `internal/net` interface + handle/event/`ConnectionStats` types (the contract both transports share).
2. `packet.go` codec + fuzz the decoder (hostile/truncated datagrams must never panic — CLAUDE.md fuzz mandate).
3. `connection.go` + `reliability.go` (window/ACK/RTO/reorder/dedup) — the core, table-tested against simulated loss/reorder.
4. `udp.go` goroutine loop + `backend.go` (`net.UDPConn` default; a `memBackend` for deterministic tests without real sockets).
5. `handshake.go` + `mtu.go` + `settings.go`.
6. Wire into the App via the `NetworkPlugin` defined in l2-networking-system-go (ServiceRegistry injection).

## 7. Drawbacks & Alternatives

- **Reinventing reliability over UDP.** A QUIC library (`quic-go`) would give congestion control, encryption, and stream multiplexing for free — but it is a heavy third-party dependency violating C-003, pulls in TLS, and its stream model fights the per-channel-unreliable game-traffic pattern. Rejected pending a C32 ADR; the hand-rolled ACK layer is small and game-tuned. Documented as the fallback if encryption becomes mandatory (DTLS/QUIC ADR).
- **Goroutine-owned connection state vs. sharded locks.** Single-goroutine ownership is simplest and lock-free on the hot path, but caps a single transport's throughput to one core. For the engine's target (one server hosting tens of clients) this is ample; sharding by connection across IO-pool goroutines is a future scaling option.
- **`SetReadDeadline` polling vs. blocking read.** Polling adds a small spin; a blocking read on a separate goroutine with a wake channel is the alternative. Polling is chosen for a single, simple loop that also services heartbeats/retransmits on the same tick.

## Canonical References

<!-- MANDATORY for Stable status. Stub — this L2 is Draft (L1 parent Draft, no
     implementation yet). Populate with on-disk source/test files when code lands. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-06-05 | Initial Draft: stdlib UDP transport — goroutine/channel ECS boundary (INV-5), per-channel delivery modes + user-space reliability (window/ACK/RTO/reorder/dedup), handshake, heartbeat/timeout, PMTUD, SocketBackend seam. Implements l1-transport. |
