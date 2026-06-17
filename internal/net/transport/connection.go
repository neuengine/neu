package transport

import (
	"net"
	"time"

	netcore "github.com/neuengine/neu/internal/net"
)

// connState is the handshake phase of a connection.
type connState uint8

const (
	stateConnecting  connState = iota // handshake in progress
	stateConnected                    // handshake complete; all channels open
	stateDisconnected                 // closed (should be removed from the conn map)
)

// lossWindow tracks the last 128 sent/acked packets for PacketLoss estimation.
type lossWindow struct {
	seqs  [128]uint16
	acked [128]bool
	head  int // next write position
	n     int // filled slots (≤128)
}

func (w *lossWindow) sent(seq uint16) {
	if w.n < 128 {
		w.n++
	}
	w.seqs[w.head] = seq
	w.acked[w.head] = false
	w.head = (w.head + 1) % 128
}

func (w *lossWindow) onAck(a Ack) {
	for i := range w.n {
		idx := (w.head - 1 - i + 128) % 128
		if !w.acked[idx] && acked(a, w.seqs[idx]) {
			w.acked[idx] = true
		}
	}
}

func (w *lossWindow) loss() float32 {
	if w.n == 0 {
		return 0
	}
	var ac int
	for i := range w.n {
		idx := (w.head - 1 - i + 128) % 128
		if w.acked[idx] {
			ac++
		}
	}
	return 1 - float32(ac)/float32(w.n)
}

// chanState is per-channel reliability state for one connection.
type chanState struct {
	sender        *reliableSender
	receiver      *reliableReceiver
	lastSeq       uint16 // for Unreliable newest-wins tracking (received side)
	hasSeq        bool
	mode          netcore.DeliveryMode
	unreliableSeq uint16 // next outgoing seq for Unreliable frames (send side)
}

// connection holds all per-peer state. It is owned exclusively by the
// UDPTransport goroutine; no locking is needed on any of its fields.
type connection struct {
	addr      net.Addr
	id        netcore.ConnectionID
	remoteID  uint16 // peer's connID from their datagrams (informational)
	localSeq  uint16 // next outgoing packet sequence number
	channels  [256]chanState
	ackTrack  ackTracker
	lossWin   lossWindow
	mtu       int // negotiated MTU (bytes); starts at MaxDatagramSize
	mtuP      mtuProber
	lastRecv  time.Time
	lastSend  time.Time
	bytesSent uint64
	bytesRecv uint64
	rto       rtoEstimator

	state       connState
	connectResp chan connectResult // non-nil on client side until handshake resolves
	connectAt   time.Time         // when Connect() was called; used for handshake timeout
}

// newConnection allocates and wires up per-channel reliability state.
// All new connections start in stateConnecting; the caller transitions to
// stateConnected after the handshake completes.
func newConnection(id netcore.ConnectionID, addr net.Addr, mtu int, channels [256]netcore.ChannelConfig) *connection {
	c := &connection{
		id:        id,
		addr:      addr,
		mtu:       mtu,
		state:     stateConnecting,
		connectAt: time.Now(),
		lastRecv:  time.Now(),
	}
	for i, cfg := range channels {
		c.channels[i].mode = cfg.Delivery
		if cfg.Delivery != netcore.Unreliable {
			c.channels[i].sender = newReliableSender(256)
			c.channels[i].receiver = newReliableReceiver(cfg.Delivery)
		}
	}
	return c
}

// stats returns a point-in-time snapshot for NetworkTransport.Stats.
func (c *connection) stats() netcore.ConnectionStats {
	return netcore.ConnectionStats{
		RTT:           c.rto.srtt,
		RTTVariance:   c.rto.rttvar,
		BytesSent:     c.bytesSent,
		BytesReceived: c.bytesRecv,
		PacketLoss:    c.lossWin.loss(),
	}
}
