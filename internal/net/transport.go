// Package net defines the engine's transport-agnostic networking abstraction:
// the NetworkTransport interface plus its handle types, delivery modes,
// connection events, and statistics. Gameplay code only ever sees these handles
// and the ECS events derived from them — never a socket (l1-transport INV-5).
//
// Concrete transports (UDP in internal/net/transport, and future
// WebSocket/WebRTC) implement NetworkTransport and are injected by NetworkPlugin
// via the ServiceRegistry, so this package never imports a concrete transport
// (one-way dependency). SocketAddr is a string ("host:port") rather than a
// *net.UDPAddr so this package stays free of the stdlib net import and remains
// protocol-agnostic. [Bootstrap]
package net

import "time"

// ConnectionID is an opaque handle for a peer connection.
type ConnectionID uint32

// ChannelID selects one of the (up to 256) pre-configured delivery channels.
type ChannelID uint8

// SocketAddr is a transport-agnostic peer address ("host:port").
type SocketAddr string

// DeliveryMode is a channel's fixed delivery guarantee, chosen at registration.
type DeliveryMode uint8

const (
	// Unreliable: fire-and-forget; may drop or reorder. Newest-wins state.
	Unreliable DeliveryMode = iota
	// ReliableUnordered: exactly-once, any order. Events, RPC.
	ReliableUnordered
	// ReliableOrdered: exactly-once, in send order (head-of-line blocking).
	ReliableOrdered
)

// String returns the delivery mode name.
func (d DeliveryMode) String() string {
	switch d {
	case Unreliable:
		return "Unreliable"
	case ReliableUnordered:
		return "ReliableUnordered"
	case ReliableOrdered:
		return "ReliableOrdered"
	default:
		return "Unknown"
	}
}

// Default channel assignments registered by NetworkPlugin (l1-transport §4.2).
const (
	// ChannelState carries high-frequency unreliable state (positions).
	ChannelState ChannelID = 0
	// ChannelEvents carries reliable-unordered events and RPC.
	ChannelEvents ChannelID = 1
	// ChannelControl carries reliable-ordered control (chat, handshake).
	ChannelControl ChannelID = 2
	// ChannelSnapshot carries high-frequency unreliable snapshot deltas.
	ChannelSnapshot ChannelID = 3
)

// ChannelConfig declares one channel's delivery guarantee and fragmentation
// threshold, agreed between peers at handshake time.
type ChannelConfig struct {
	ID             ChannelID
	Delivery       DeliveryMode
	MaxMessageSize int // bytes before fragmentation; default 1200
}

// InboundPacket is one delivered channel message. It is internal to the
// networking layer — consumed by replication/RPC, never exposed to game code.
type InboundPacket struct {
	Payload    []byte
	Connection ConnectionID
	Channel    ChannelID
}

// ConnectionStats is a point-in-time snapshot of a connection's health,
// returned by value from NetworkTransport.Stats.
type ConnectionStats struct {
	RTT           time.Duration
	RTTVariance   time.Duration
	BytesSent     uint64
	BytesReceived uint64
	PacketLoss    float32 // fraction 0..1 over the last 128 packets
	SendRate      float32 // bytes/sec
	ReceiveRate   float32 // bytes/sec
}

// DisconnectReason classifies why a connection closed.
type DisconnectReason uint8

const (
	// DisconnectGraceful: the peer closed cleanly.
	DisconnectGraceful DisconnectReason = iota
	// DisconnectTimeout: no heartbeat within the timeout window.
	DisconnectTimeout
	// DisconnectProtocolError: malformed packet or version mismatch.
	DisconnectProtocolError
	// DisconnectRejected: the server refused the connection.
	DisconnectRejected
	// DisconnectLocalClose: we called Disconnect.
	DisconnectLocalClose
)

// String returns the reason name.
func (r DisconnectReason) String() string {
	switch r {
	case DisconnectGraceful:
		return "Graceful"
	case DisconnectTimeout:
		return "Timeout"
	case DisconnectProtocolError:
		return "ProtocolError"
	case DisconnectRejected:
		return "Rejected"
	case DisconnectLocalClose:
		return "LocalClose"
	default:
		return "Unknown"
	}
}

// Connected is an ECS event emitted when a connection completes its handshake.
type Connected struct {
	RemoteAddr SocketAddr
	Connection ConnectionID
	IsServer   bool // true if we accepted this connection as a server
}

// Disconnected is an ECS event emitted when a connection closes.
type Disconnected struct {
	RemoteAddr SocketAddr
	Connection ConnectionID
	Reason     DisconnectReason
}

// NetworkTransport is the transport-agnostic service registered in the
// ServiceRegistry. Concrete implementations (UDP, WebSocket) live in
// sub-packages and are injected by NetworkPlugin. All methods are called from
// the ECS main thread; the implementation does its own off-thread I/O and
// returns results via Drain (l1-transport INV-5).
type NetworkTransport interface {
	// Listen binds a server socket on addr.
	Listen(addr SocketAddr) error
	// Connect initiates a client connection to addr, returning its handle.
	Connect(addr SocketAddr) (ConnectionID, error)
	// Disconnect closes a connection with a human-readable reason.
	Disconnect(id ConnectionID, reason string)
	// Connections returns all live connection handles.
	Connections() []ConnectionID
	// Send enqueues payload to one connection on the given channel.
	Send(id ConnectionID, channel ChannelID, payload []byte) error
	// Broadcast enqueues payload to every connection on the given channel.
	Broadcast(channel ChannelID, payload []byte)
	// Drain returns all packets received since the last call (one frame's worth).
	Drain() []InboundPacket
	// Stats returns a connection's current statistics by value.
	Stats(id ConnectionID) ConnectionStats
}
