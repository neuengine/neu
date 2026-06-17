package transport

import (
	"encoding/binary"
	"fmt"
	"time"

	netcore "github.com/neuengine/neu/internal/net"
)

// protoVersion is the handshake protocol version; a mismatch yields ConnectReject.
const protoVersion uint16 = 1

// Handshake message types carried as the first byte of a ch-2 (ChannelControl) payload.
const (
	msgConnectRequest uint8 = 0x01 // client → server: [type, ver_lo, ver_hi]
	msgConnectAccept  uint8 = 0x02 // server → client: [type]
	msgConnectReject  uint8 = 0x03 // server → client: [type, reason]
	msgConnectAck     uint8 = 0x04 // client → server: [type]
)

// isHandshakeMsg reports whether b is a handshake message type tag.
func isHandshakeMsg(b uint8) bool { return b >= msgConnectRequest && b <= msgConnectAck }

// hasConnectRequest reports whether any frame in frames carries a ConnectRequest
// on ChannelControl. Used by processInbound to gate server-side connection creation.
func hasConnectRequest(frames []Frame) bool {
	for _, f := range frames {
		if f.Channel == uint8(netcore.ChannelControl) && len(f.Payload) > 0 && f.Payload[0] == msgConnectRequest {
			return true
		}
	}
	return false
}

// ─── send helpers ─────────────────────────────────────────────────────────────

// sendConnectRequest queues a ConnectRequest onto c's ch-2 reliable sender.
// Called by the goroutine after allocating a client-side connection.
func (t *UDPTransport) sendConnectRequest(c *connection) {
	var buf [3]byte
	buf[0] = msgConnectRequest
	binary.LittleEndian.PutUint16(buf[1:], protoVersion)
	t.queueControl(c, buf[:])
}

// sendConnectAccept queues a ConnectAccept onto c's ch-2 reliable sender.
func (t *UDPTransport) sendConnectAccept(c *connection) {
	t.queueControl(c, []byte{msgConnectAccept})
}

// sendConnectReject queues a ConnectReject onto c's ch-2 reliable sender.
func (t *UDPTransport) sendConnectReject(c *connection, reason uint8) {
	t.queueControl(c, []byte{msgConnectReject, reason})
}

// sendConnectAck queues a ConnectAck onto c's ch-2 reliable sender.
func (t *UDPTransport) sendConnectAck(c *connection) {
	t.queueControl(c, []byte{msgConnectAck})
}

// queueControl enqueues payload on c's ChannelControl (ch 2) reliable sender.
func (t *UDPTransport) queueControl(c *connection, payload []byte) {
	s := c.channels[netcore.ChannelControl].sender
	if s != nil {
		s.queue(payload)
	}
}

// ─── state machine ────────────────────────────────────────────────────────────

// processHandshakeFrame routes a delivered ch-2 payload through the connection
// handshake state machine. It is called from processInbound on the goroutine.
func (t *UDPTransport) processHandshakeFrame(
	conns map[netcore.ConnectionID]*connection,
	addrToID map[string]netcore.ConnectionID,
	c *connection,
	payload []byte,
	now time.Time,
) {
	if len(payload) == 0 || c.state == stateDisconnected {
		return
	}
	switch payload[0] {
	case msgConnectRequest:
		// Server side: validate version and accept/reject.
		if c.state != stateConnecting {
			return
		}
		if len(payload) < 3 {
			t.disconnectWithReason(conns, addrToID, c.id, netcore.DisconnectProtocolError)
			return
		}
		if ver := binary.LittleEndian.Uint16(payload[1:]); ver != protoVersion {
			t.sendConnectReject(c, 0x01) // version mismatch; tick will flush before removal
			return
		}
		t.sendConnectAccept(c)

	case msgConnectAccept:
		// Client side: server accepted; complete handshake.
		if c.state != stateConnecting || c.connectResp == nil {
			return
		}
		t.sendConnectAck(c)
		t.markConnected(c, now)
		c.connectResp <- connectResult{id: c.id}
		c.connectResp = nil
		select {
		case t.EventsCh <- ConnEvent{Connected: &netcore.Connected{
			Connection: c.id,
			RemoteAddr: netcore.SocketAddr(c.addr.String()),
			IsServer:   false,
		}}:
		default:
		}

	case msgConnectReject:
		// Client side: server rejected; fail Connect().
		if c.state != stateConnecting || c.connectResp == nil {
			return
		}
		c.connectResp <- connectResult{err: fmt.Errorf("transport: connect rejected")}
		c.connectResp = nil
		t.disconnectWithReason(conns, addrToID, c.id, netcore.DisconnectRejected)

	case msgConnectAck:
		// Server side: client acknowledged; connection open.
		if c.state != stateConnecting {
			return
		}
		t.markConnected(c, now)
		select {
		case t.EventsCh <- ConnEvent{Connected: &netcore.Connected{
			Connection: c.id,
			RemoteAddr: netcore.SocketAddr(c.addr.String()),
			IsServer:   true,
		}}:
		default:
		}
	}
}

// markConnected transitions c to stateConnected and starts PMTUD.
func (t *UDPTransport) markConnected(c *connection, now time.Time) {
	c.state = stateConnected
	t.startMTUProbe(c, now)
}

// disconnectWithReason removes c from the connection maps, signals any waiting
// Connect() call with an error, and enqueues a Disconnected event.
func (t *UDPTransport) disconnectWithReason(
	conns map[netcore.ConnectionID]*connection,
	addrToID map[string]netcore.ConnectionID,
	id netcore.ConnectionID,
	reason netcore.DisconnectReason,
) {
	c, ok := conns[id]
	if !ok {
		return
	}
	if c.connectResp != nil {
		c.connectResp <- connectResult{err: fmt.Errorf("transport: connection closed: %v", reason)}
		c.connectResp = nil
	}
	delete(addrToID, c.addr.String())
	delete(conns, id)
	select {
	case t.EventsCh <- ConnEvent{Disconnected: &netcore.Disconnected{
		Connection: id,
		RemoteAddr: netcore.SocketAddr(c.addr.String()),
		Reason:     reason,
	}}:
	default:
	}
}
