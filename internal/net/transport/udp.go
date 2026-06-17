package transport

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	netcore "github.com/neuengine/neu/internal/net"
)

// ConnEvent is a lifecycle notification pushed by the goroutine to the ECS layer.
// Exactly one of Connected or Disconnected is set per event.
type ConnEvent struct {
	Connected    *netcore.Connected
	Disconnected *netcore.Disconnected
}

// outboundMsg carries a Send or Broadcast payload to the goroutine.
// id == 0 means broadcast to all connections.
type outboundMsg struct {
	payload []byte
	id      netcore.ConnectionID
	channel netcore.ChannelID
}

type connectReq struct {
	addr net.Addr
	resp chan connectResult
}

type connectResult struct {
	id  netcore.ConnectionID
	err error
}

type disconnectReq struct{ id netcore.ConnectionID }

type statsReq struct {
	id   netcore.ConnectionID
	resp chan netcore.ConnectionStats
}

type listReq struct{ resp chan []netcore.ConnectionID }

// pendingDisconnect collects connections that need to be removed during tickConns.
type pendingDisconnect struct {
	id     netcore.ConnectionID
	reason netcore.DisconnectReason
}

// UDPTransport implements netcore.NetworkTransport over UDP. A single goroutine
// owns all connection state; the ECS main thread communicates only via channels
// (INV-5 — the main loop is never blocked on I/O or connection state).
type UDPTransport struct {
	mu       sync.Mutex
	backend  SocketBackend
	started  bool
	settings NetworkSettings
	channels [256]netcore.ChannelConfig
	nextID   netcore.ConnectionID // guarded by goroutine only; 0 is reserved

	// dialFunc resolves a SocketAddr string to a net.Addr. Defaults to
	// net.ResolveUDPAddr; override in tests to avoid DNS lookups.
	dialFunc func(addr string) (net.Addr, error)

	// ECS ↔ goroutine channels.
	outboundCh   chan outboundMsg
	connectCh    chan connectReq
	disconnectCh chan disconnectReq
	statsCh      chan statsReq
	listCh       chan listReq

	// Goroutine → ECS.
	InboundCh chan []netcore.InboundPacket // Drain reads from here
	EventsCh  chan ConnEvent               // NetworkPlugin reads from here

	doneCh chan struct{} // closed when goroutine exits
	stopCh chan struct{} // closed to signal goroutine stop
}

// New creates an UDPTransport that binds its socket lazily on Listen or the
// first Connect call. channels configures delivery mode per channel ID.
func New(settings NetworkSettings, channels []netcore.ChannelConfig) *UDPTransport {
	return newTransport(nil, settings, channels)
}

// NewWithBackend creates an UDPTransport with a pre-bound backend (for tests).
func NewWithBackend(backend SocketBackend, settings NetworkSettings, channels []netcore.ChannelConfig) *UDPTransport {
	return newTransport(backend, settings, channels)
}

func newTransport(backend SocketBackend, settings NetworkSettings, channels []netcore.ChannelConfig) *UDPTransport {
	s := settings.withDefaults()
	t := &UDPTransport{
		backend:  backend,
		settings: s,
		dialFunc: func(addr string) (net.Addr, error) {
			return net.ResolveUDPAddr("udp", addr)
		},
		outboundCh:   make(chan outboundMsg, s.SendQueueDepth),
		connectCh:    make(chan connectReq),
		disconnectCh: make(chan disconnectReq, 16),
		statsCh:      make(chan statsReq),
		listCh:       make(chan listReq),
		InboundCh:    make(chan []netcore.InboundPacket, s.RecvQueueDepth),
		EventsCh:     make(chan ConnEvent, 64),
		doneCh:       make(chan struct{}),
		stopCh:       make(chan struct{}),
	}
	t.nextID = 1 // 0 is broadcast sentinel in outboundMsg
	for _, ch := range channels {
		t.channels[ch.ID] = ch
	}
	return t
}

// ─── NetworkTransport interface ───────────────────────────────────────────────

// Listen binds the socket on addr (if not already bound) and starts the I/O goroutine.
func (t *UDPTransport) Listen(addr netcore.SocketAddr) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.started {
		return nil
	}
	if t.backend == nil {
		b, err := newUDPConnBackend(string(addr))
		if err != nil {
			return err
		}
		t.backend = b
	}
	return t.start()
}

// Connect initiates a handshake with addr. It blocks until the server accepts
// or rejects, or until HandshakeTimeout elapses. The goroutine is started
// automatically if Listen was not called.
func (t *UDPTransport) Connect(addr netcore.SocketAddr) (netcore.ConnectionID, error) {
	if err := t.ensureStarted(); err != nil {
		return 0, err
	}
	t.mu.Lock()
	dial := t.dialFunc
	t.mu.Unlock()
	peerAddr, err := dial(string(addr))
	if err != nil {
		return 0, fmt.Errorf("transport: resolve %q: %w", addr, err)
	}
	resp := make(chan connectResult, 1)
	select {
	case t.connectCh <- connectReq{addr: peerAddr, resp: resp}:
	case <-t.doneCh:
		return 0, fmt.Errorf("transport: closed")
	}
	// Block until the goroutine resolves the handshake (or times out).
	res := <-resp
	return res.id, res.err
}

// Disconnect enqueues a close for id. It is asynchronous and non-blocking.
func (t *UDPTransport) Disconnect(id netcore.ConnectionID, _ string) {
	select {
	case t.disconnectCh <- disconnectReq{id: id}:
	default:
	}
}

// Connections returns a snapshot of live connection handles (including connecting ones).
func (t *UDPTransport) Connections() []netcore.ConnectionID {
	if !t.isStarted() {
		return nil
	}
	resp := make(chan []netcore.ConnectionID, 1)
	select {
	case t.listCh <- listReq{resp: resp}:
	case <-t.doneCh:
		return nil
	}
	return <-resp
}

// Send enqueues payload on channel for id. Returns ErrSendQueueFull if the
// outbound channel has no room (INV-5 — never blocks).
func (t *UDPTransport) Send(id netcore.ConnectionID, channel netcore.ChannelID, payload []byte) error {
	if !t.isStarted() {
		return fmt.Errorf("transport: not started")
	}
	cp := make([]byte, len(payload))
	copy(cp, payload)
	select {
	case t.outboundCh <- outboundMsg{id: id, channel: channel, payload: cp}:
		return nil
	default:
		return ErrSendQueueFull
	}
}

// Broadcast enqueues payload on channel for every live connection (INV-5 — non-blocking).
func (t *UDPTransport) Broadcast(channel netcore.ChannelID, payload []byte) {
	if !t.isStarted() {
		return
	}
	cp := make([]byte, len(payload))
	copy(cp, payload)
	select {
	case t.outboundCh <- outboundMsg{id: 0, channel: channel, payload: cp}:
	default:
	}
}

// Drain returns all packets received since the previous call. Call once per frame.
func (t *UDPTransport) Drain() []netcore.InboundPacket {
	var out []netcore.InboundPacket
	for {
		select {
		case batch := <-t.InboundCh:
			out = append(out, batch...)
		default:
			return out
		}
	}
}

// Stats returns connection statistics by value. Blocks until the goroutine responds.
func (t *UDPTransport) Stats(id netcore.ConnectionID) netcore.ConnectionStats {
	if !t.isStarted() {
		return netcore.ConnectionStats{}
	}
	resp := make(chan netcore.ConnectionStats, 1)
	select {
	case t.statsCh <- statsReq{id: id, resp: resp}:
	case <-t.doneCh:
		return netcore.ConnectionStats{}
	}
	return <-resp
}

// Close stops the goroutine and closes the backend socket.
func (t *UDPTransport) Close() error {
	t.mu.Lock()
	if !t.started {
		t.mu.Unlock()
		return nil
	}
	t.mu.Unlock()
	close(t.stopCh)
	<-t.doneCh
	return t.backend.Close()
}

// ErrSendQueueFull is returned by Send when the outbound channel is full.
var ErrSendQueueFull = errors.New("transport: send queue full")

// ─── Internal helpers ─────────────────────────────────────────────────────────

func (t *UDPTransport) isStarted() bool {
	t.mu.Lock()
	s := t.started
	t.mu.Unlock()
	return s
}

func (t *UDPTransport) ensureStarted() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.started {
		return nil
	}
	if t.backend == nil {
		b, err := newUDPConnBackend(":0")
		if err != nil {
			return err
		}
		t.backend = b
	}
	return t.start()
}

// start launches the goroutine. Must be called with t.mu held.
func (t *UDPTransport) start() error {
	t.started = true
	go t.run()
	return nil
}

// ─── Goroutine loop ───────────────────────────────────────────────────────────

func (t *UDPTransport) run() {
	defer close(t.doneCh)

	conns := make(map[netcore.ConnectionID]*connection, t.settings.MaxConnections)
	addrToID := make(map[string]netcore.ConnectionID, t.settings.MaxConnections)
	readBuf := make([]byte, t.settings.ReadBufSize)
	var pending []netcore.InboundPacket

	for {
		// Poll for an incoming datagram with a short deadline to remain responsive
		// to outbound messages without burning a goroutine on blocking reads.
		_ = t.backend.SetReadDeadline(time.Now().Add(time.Millisecond))
		n, addr, err := t.backend.ReadFrom(readBuf)
		now := time.Now()
		if err == nil {
			pkts := t.processInbound(conns, addrToID, readBuf[:n], addr, now)
			pending = append(pending, pkts...)
		}

		// Drain all pending admin requests in this tick.
	admin:
		for {
			select {
			case <-t.stopCh:
				return
			case req := <-t.connectCh:
				// Register connection and start handshake. resp is held by the
				// goroutine; connectResult is sent when the handshake resolves.
				id := t.addConn(conns, addrToID, req.addr, req.resp)
				t.sendConnectRequest(conns[id])
			case req := <-t.disconnectCh:
				t.disconnectWithReason(conns, addrToID, req.id, netcore.DisconnectLocalClose)
			case req := <-t.statsCh:
				if c, ok := conns[req.id]; ok {
					req.resp <- c.stats()
				} else {
					req.resp <- netcore.ConnectionStats{}
				}
			case req := <-t.listCh:
				ids := make([]netcore.ConnectionID, 0, len(conns))
				for id := range conns {
					ids = append(ids, id)
				}
				req.resp <- ids
			default:
				break admin
			}
		}

		// Tick reliability first: retransmits and ConnectAck must depart before
		// any game frames queued by Send() in the same goroutine turn.
		t.tickConns(conns, addrToID, now)

		// Flush queued outbound messages.
		t.flushOutbound(conns)

		// Push accumulated inbound to the ECS-side channel.
		if len(pending) > 0 {
			select {
			case t.InboundCh <- pending:
				pending = nil
			default:
				// ECS hasn't drained yet; accumulate (bounded by frame rate).
			}
		}
	}
}

// addConn registers a new connection entry. resp is non-nil for client-initiated
// connections (held until handshake resolves); nil for server-accepted connections.
func (t *UDPTransport) addConn(
	conns map[netcore.ConnectionID]*connection,
	addrToID map[string]netcore.ConnectionID,
	addr net.Addr,
	resp chan connectResult,
) netcore.ConnectionID {
	id := t.nextID
	t.nextID++
	if t.nextID == 0 {
		t.nextID = 1
	}
	c := newConnection(id, addr, MaxDatagramSize, t.channels)
	c.connectResp = resp
	conns[id] = c
	addrToID[addr.String()] = id
	return id
}

func (t *UDPTransport) flushOutbound(conns map[netcore.ConnectionID]*connection) {
	for {
		select {
		case msg := <-t.outboundCh:
			if msg.id == 0 {
				for _, c := range conns {
					t.enqueueFrame(c, msg.channel, msg.payload)
				}
			} else if c, ok := conns[msg.id]; ok {
				t.enqueueFrame(c, msg.channel, msg.payload)
			}
		default:
			return
		}
	}
}

// enqueueFrame queues payload on channel ch for connection c. During handshake
// (stateConnecting), only ChannelControl frames are allowed so game data cannot
// leak before the peer is ready. Unreliable frames are written immediately with
// a monotone per-channel seq so the receiver's newest-wins dedup works correctly.
// Reliable frames are queued for tickConns to emit.
func (t *UDPTransport) enqueueFrame(c *connection, channelID netcore.ChannelID, payload []byte) {
	if c.state == stateConnecting && channelID != netcore.ChannelControl {
		return
	}
	ch := &c.channels[channelID]
	if ch.mode == netcore.Unreliable {
		seq := ch.unreliableSeq
		ch.unreliableSeq++
		frame := Frame{Channel: uint8(channelID), MsgSeq: seq, Payload: payload}
		t.sendFrames(c, []Frame{frame})
		return
	}
	if ch.sender != nil {
		ch.sender.queue(payload)
	}
}

// tickConns handles per-connection periodic work: reliable retransmissions,
// heartbeat keepalives, disconnect timeouts, handshake timeouts, and PMTUD.
func (t *UDPTransport) tickConns(
	conns map[netcore.ConnectionID]*connection,
	addrToID map[string]netcore.ConnectionID,
	now time.Time,
) {
	var toDisconnect []pendingDisconnect

	for id, c := range conns {
		// Handshake timeout: client-side only (connectResp non-nil).
		if c.state == stateConnecting && c.connectResp != nil {
			if now.Sub(c.connectAt) >= t.settings.HandshakeTimeout {
				toDisconnect = append(toDisconnect, pendingDisconnect{id, netcore.DisconnectTimeout})
				continue
			}
		}

		// Disconnect timeout: drop silent connections.
		if c.state == stateConnected && !c.lastRecv.IsZero() {
			if now.Sub(c.lastRecv) >= t.settings.DisconnectTimeout {
				toDisconnect = append(toDisconnect, pendingDisconnect{id, netcore.DisconnectTimeout})
				continue
			}
		}

		// Reliable retransmissions (both stateConnecting for handshake and stateConnected).
		// We track which channel each set of due frames came from so we can record the
		// actual datagram PacketSeq after sending — channel msg seqs and datagram seqs
		// diverge once handshake/probe frames precede game frames (INV-5 side-effect).
		type chanDue struct {
			idx    int
			frames []Frame
		}
		var dues []chanDue
		var frames []Frame
		for i := range c.channels {
			s := c.channels[i].sender
			if s == nil {
				continue
			}
			due := s.due(now)
			if len(due) == 0 {
				continue
			}
			dues = append(dues, chanDue{i, due})
			for _, f := range due {
				frames = append(frames, Frame{Channel: uint8(i), MsgSeq: f.MsgSeq, Payload: f.Payload})
			}
		}
		if len(frames) > 0 {
			datagramSeq := c.localSeq // captured before sendFrames increments it
			t.sendFrames(c, frames)
			for _, cd := range dues {
				s := c.channels[cd.idx].sender
				for _, f := range cd.frames {
					s.setDatagramSeq(f.MsgSeq, datagramSeq)
				}
			}
		}

		if c.state != stateConnected {
			continue
		}

		// Heartbeat: send ACK-only keepalive when idle (INV-4).
		if !c.lastSend.IsZero() && now.Sub(c.lastSend) >= t.settings.HeartbeatInterval {
			t.sendFrames(c, nil) // ACK-only datagram
		}

		// PMTUD: advance probe ladder on timeout.
		t.tickMTU(c, now, t.settings.MTUProbeTimeout)
	}

	for _, d := range toDisconnect {
		t.disconnectWithReason(conns, addrToID, d.id, d.reason)
	}
}

// sendFrames encodes and writes a datagram, piggybacking the ACK state.
// Passing an empty or nil frames slice sends an ACK-only keepalive.
func (t *UDPTransport) sendFrames(c *connection, frames []Frame) {
	ack := c.ackTrack.ack()
	data := Encode(uint16(c.id), c.localSeq, 0, ack, frames)
	c.lossWin.sent(c.localSeq)
	c.localSeq++
	c.lastSend = time.Now()
	c.bytesSent += uint64(len(data))
	_ = t.backend.WriteTo(data, c.addr)
}

// processInbound decodes a datagram and delivers channel frames to callers.
// Server-side connections are created only when a ConnectRequest is received.
func (t *UDPTransport) processInbound(
	conns map[netcore.ConnectionID]*connection,
	addrToID map[string]netcore.ConnectionID,
	data []byte,
	addr net.Addr,
	now time.Time,
) []netcore.InboundPacket {
	hdr, ack, frames, err := Decode(data)
	if err != nil {
		return nil
	}

	id, known := addrToID[addr.String()]
	if !known {
		// Server-side accept: only create a connection for a valid ConnectRequest.
		if !hasConnectRequest(frames) || len(conns) >= t.settings.MaxConnections {
			return nil
		}
		id = t.addConn(conns, addrToID, addr, nil) // nil resp = server side
		conns[id].remoteID = hdr.ConnectionID
	}

	c := conns[id]
	c.ackTrack.onReceived(hdr.PacketSeq)
	c.lastRecv = now
	c.bytesRecv += uint64(len(data))

	if ack != nil {
		for i := range c.channels {
			if c.channels[i].sender != nil {
				c.channels[i].sender.onAck(*ack, now)
			}
		}
		c.lossWin.onAck(*ack)
	}

	var delivered []netcore.InboundPacket

	// Pass 1: ChannelControl (ch 2) reliable frames first.
	// ConnectAck and other handshake messages must advance the state machine
	// before game frames in the same datagram are evaluated.
	for _, f := range frames {
		if f.Channel != uint8(netcore.ChannelControl) {
			continue
		}
		ch := &c.channels[f.Channel]
		if ch.receiver == nil {
			continue
		}
		payloads, _ := ch.receiver.receive(f.MsgSeq, f.Payload)
		for _, p := range payloads {
			if len(p) > 0 && isHandshakeMsg(p[0]) {
				t.processHandshakeFrame(conns, addrToID, c, p, now)
				continue
			}
			if c.state == stateConnected {
				delivered = append(delivered, netcore.InboundPacket{
					Connection: c.id,
					Channel:    netcore.ChannelControl,
					Payload:    p,
				})
			}
		}
	}

	// Pass 2: Unreliable and non-control Reliable frames.
	for _, f := range frames {
		if f.Channel == uint8(netcore.ChannelControl) {
			continue // handled in pass 1
		}
		ch := &c.channels[f.Channel]
		switch ch.mode {
		case netcore.Unreliable:
			// PMTUD probes and acks are intercepted before the newest-wins seq
			// update so they do not consume seq numbers in the game-frame dedup.
			if len(f.Payload) >= 2 {
				if f.Payload[0] == msgMTUProbe {
					t.processMTUProbe(c, f.Payload[1], now)
					continue
				}
				if f.Payload[0] == msgMTUAck {
					t.processMTUAck(c, f.Payload[1])
					continue
				}
			}
			if ch.hasSeq && !seqGreater(f.MsgSeq, ch.lastSeq) {
				continue // newest-wins; drop stale out-of-order game frame
			}
			ch.lastSeq, ch.hasSeq = f.MsgSeq, true
			if c.state != stateConnected {
				continue // drop unreliable game frames before handshake completes
			}
			cp := make([]byte, len(f.Payload))
			copy(cp, f.Payload)
			delivered = append(delivered, netcore.InboundPacket{
				Connection: c.id,
				Channel:    netcore.ChannelID(f.Channel),
				Payload:    cp,
			})
		default:
			if ch.receiver == nil {
				continue
			}
			// Skip the receiver entirely while connecting: calling receive() marks the
			// sequence as seen. If the frame were dropped due to stateConnecting, the
			// retransmit would be deduplicated and the frame would be lost permanently.
			if c.state != stateConnected {
				continue
			}
			payloads, _ := ch.receiver.receive(f.MsgSeq, f.Payload)
			for _, p := range payloads {
				delivered = append(delivered, netcore.InboundPacket{
					Connection: c.id,
					Channel:    netcore.ChannelID(f.Channel),
					Payload:    p, // receiver already copies for ReliableOrdered
				})
			}
		}
	}
	return delivered
}
