package transport

import (
	"time"

	netcore "github.com/neuengine/neu/internal/net"
)

// mtuLadder is the PMTUD probe-size sequence (bytes), tried largest-first.
var mtuLadder = [3]int{1400, 1200, 576}

const (
	msgMTUProbe uint8 = 0x05
	msgMTUAck   uint8 = 0x06
)

// mtuProber holds per-connection Path-MTU discovery state.
// All fields are goroutine-owned; no locking required.
type mtuProber struct {
	probeSeq uint8      // sequence tag for the in-flight probe
	step     int        // current index into mtuLadder
	sentAt   time.Time  // when the current probe was sent
	done     bool       // true once the negotiated MTU is settled
}

// startMTUProbe begins the PMTUD ladder for c immediately after handshake.
func (t *UDPTransport) startMTUProbe(c *connection, now time.Time) {
	c.mtuP = mtuProber{step: 0}
	t.sendMTUProbe(c, now)
}

// sendMTUProbe emits a datagram of exactly mtuLadder[c.mtuP.step] bytes.
// The payload is [msgMTUProbe, probeSeq, zero-padding...].
func (t *UDPTransport) sendMTUProbe(c *connection, now time.Time) {
	step := c.mtuP.step
	if step >= len(mtuLadder) {
		return
	}
	targetSize := mtuLadder[step]
	// Predictable overhead: header(8) + ACK(6) + frameHead(5) = 19.
	const overhead = headerSize + ackSize + frameHead
	payloadSize := targetSize - overhead
	if payloadSize < 2 {
		c.mtu = MaxDatagramSize
		c.mtuP.done = true
		return
	}
	c.mtuP.probeSeq++
	c.mtuP.sentAt = now

	payload := make([]byte, payloadSize)
	payload[0] = msgMTUProbe
	payload[1] = c.mtuP.probeSeq
	// [2:] remains zero padding.

	// Use an explicit empty Ack to keep overhead predictable.
	emptyAck := &Ack{}
	frame := Frame{Channel: uint8(netcore.ChannelState), MsgSeq: 0, Payload: payload}
	data := Encode(uint16(c.id), c.localSeq, 0, emptyAck, []Frame{frame})
	c.lossWin.sent(c.localSeq)
	c.localSeq++
	c.lastSend = now
	c.bytesSent += uint64(len(data))
	_ = t.backend.WriteTo(data, c.addr)
}

// tickMTU advances the PMTUD ladder for c when the current probe has timed out.
func (t *UDPTransport) tickMTU(c *connection, now time.Time, timeout time.Duration) {
	if c.mtuP.done || c.mtuP.sentAt.IsZero() {
		return
	}
	if now.Sub(c.mtuP.sentAt) < timeout {
		return
	}
	c.mtuP.step++
	if c.mtuP.step >= len(mtuLadder) {
		c.mtu = mtuLadder[len(mtuLadder)-1]
		c.mtuP.done = true
		return
	}
	t.sendMTUProbe(c, now)
}

// processMTUProbe handles an incoming probe frame and echoes back an MTUAck.
func (t *UDPTransport) processMTUProbe(c *connection, probeSeq uint8, now time.Time) {
	ack := c.ackTrack.ack()
	payload := [2]byte{msgMTUAck, probeSeq}
	frame := Frame{Channel: uint8(netcore.ChannelState), MsgSeq: 0, Payload: payload[:]}
	data := Encode(uint16(c.id), c.localSeq, 0, ack, []Frame{frame})
	c.localSeq++
	c.lastSend = now
	c.bytesSent += uint64(len(data))
	_ = t.backend.WriteTo(data, c.addr)
}

// processMTUAck handles an incoming MTUAck and settles the negotiated MTU.
func (t *UDPTransport) processMTUAck(c *connection, probeSeq uint8) {
	if c.mtuP.done || probeSeq != c.mtuP.probeSeq {
		return // stale or unexpected
	}
	c.mtu = mtuLadder[c.mtuP.step]
	c.mtuP.done = true
}
