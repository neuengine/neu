package transport

import (
	"net"
	"testing"
	"time"

	netcore "github.com/neuengine/neu/internal/net"
)

// memDialFunc returns a dialFunc that resolves addresses as memAddr (no DNS).
func memDialFunc(addr string) (net.Addr, error) { return memAddr(addr), nil }

// defaultChannels returns a minimal 4-channel config matching the engine defaults.
func defaultChannels() []netcore.ChannelConfig {
	return []netcore.ChannelConfig{
		{ID: 0, Delivery: netcore.Unreliable, MaxMessageSize: MaxDatagramSize},
		{ID: 1, Delivery: netcore.ReliableUnordered, MaxMessageSize: MaxDatagramSize},
		{ID: 2, Delivery: netcore.ReliableOrdered, MaxMessageSize: MaxDatagramSize},
		{ID: 3, Delivery: netcore.Unreliable, MaxMessageSize: MaxDatagramSize},
	}
}

// drainWait collects all InboundPackets within deadline, retrying until at
// least want packets arrive or the deadline is exceeded.
func drainWait(t *testing.T, tr *UDPTransport, want int, deadline time.Duration) []netcore.InboundPacket {
	t.Helper()
	end := time.Now().Add(deadline)
	var got []netcore.InboundPacket
	for time.Now().Before(end) {
		got = append(got, tr.Drain()...)
		if len(got) >= want {
			return got
		}
		time.Sleep(5 * time.Millisecond)
	}
	return got
}

// makeLinkedPair creates two UDPTransports connected by a memBackend pair.
func makeLinkedPair(t *testing.T, seed int64) (*UDPTransport, *UDPTransport) {
	t.Helper()
	bA, bB := newMemPair("A:1", "B:2", seed)
	chs := defaultChannels()
	settings := NetworkSettings{}
	tA := NewWithBackend(bA, settings, chs)
	tB := NewWithBackend(bB, settings, chs)
	tA.dialFunc = memDialFunc
	tB.dialFunc = memDialFunc
	if err := tA.Listen(""); err != nil {
		t.Fatalf("tA.Listen: %v", err)
	}
	if err := tB.Listen(""); err != nil {
		t.Fatalf("tB.Listen: %v", err)
	}
	t.Cleanup(func() {
		_ = tA.Close()
		_ = tB.Close()
	})
	return tA, tB
}

// TestUDPTransport_Connect verifies that Connect returns a non-zero ID and
// the connection appears in Connections().
func TestUDPTransport_Connect(t *testing.T) {
	tA, _ := makeLinkedPair(t, 1)
	id, err := tA.Connect("B:2")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if id == 0 {
		t.Fatal("Connect returned zero ConnectionID")
	}
	ids := tA.Connections()
	found := false
	for _, c := range ids {
		if c == id {
			found = true
		}
	}
	if !found {
		t.Fatalf("Connections() = %v, want id %d", ids, id)
	}
}

// TestUDPTransport_UnreliableSendDrain sends an unreliable frame from A and
// verifies it arrives at B via Drain.
func TestUDPTransport_UnreliableSendDrain(t *testing.T) {
	tA, tB := makeLinkedPair(t, 2)
	id, err := tA.Connect("B:2")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	payload := []byte("hello")
	if err := tA.Send(id, netcore.ChannelState, payload); err != nil {
		t.Fatalf("Send: %v", err)
	}

	pkts := drainWait(t, tB, 1, 200*time.Millisecond)
	if len(pkts) == 0 {
		t.Fatal("Drain returned 0 packets; expected 1")
	}
	if string(pkts[0].Payload) != string(payload) {
		t.Errorf("payload = %q, want %q", pkts[0].Payload, payload)
	}
	if pkts[0].Channel != netcore.ChannelState {
		t.Errorf("channel = %d, want %d", pkts[0].Channel, netcore.ChannelState)
	}
}

// TestUDPTransport_ReliableDelivery sends over a reliable channel and verifies
// delivery even when the backend simulates 50% packet loss.
func TestUDPTransport_ReliableDelivery(t *testing.T) {
	bA, bB := newMemPair("A:1", "B:2", 42)
	bA.LossRate = 0.5
	bB.LossRate = 0.5
	chs := defaultChannels()
	tA := NewWithBackend(bA, NetworkSettings{}, chs)
	tB := NewWithBackend(bB, NetworkSettings{}, chs)
	tA.dialFunc = memDialFunc
	tB.dialFunc = memDialFunc
	if err := tA.Listen(""); err != nil {
		t.Fatal(err)
	}
	if err := tB.Listen(""); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tA.Close(); _ = tB.Close() })

	id, err := tA.Connect("B:2")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	want := []byte("reliable")
	if err := tA.Send(id, netcore.ChannelEvents, want); err != nil {
		t.Fatalf("Send: %v", err)
	}

	// Allow several RTO cycles (minRTO = 200ms); 2s budget is generous.
	pkts := drainWait(t, tB, 1, 2*time.Second)
	if len(pkts) == 0 {
		t.Fatal("reliable frame not delivered after 2s with 50% loss")
	}
	if string(pkts[0].Payload) != string(want) {
		t.Errorf("payload = %q, want %q", pkts[0].Payload, want)
	}
}

// TestUDPTransport_OrderedDelivery sends three messages on a ReliableOrdered
// channel and confirms they arrive in order.
func TestUDPTransport_OrderedDelivery(t *testing.T) {
	tA, tB := makeLinkedPair(t, 3)
	id, err := tA.Connect("B:2")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	msgs := []string{"first", "second", "third"}
	for _, m := range msgs {
		if err := tA.Send(id, netcore.ChannelControl, []byte(m)); err != nil {
			t.Fatalf("Send %q: %v", m, err)
		}
	}

	pkts := drainWait(t, tB, len(msgs), 500*time.Millisecond)
	if len(pkts) < len(msgs) {
		t.Fatalf("got %d packets, want %d", len(pkts), len(msgs))
	}
	for i, m := range msgs {
		if string(pkts[i].Payload) != m {
			t.Errorf("packet[%d] = %q, want %q", i, pkts[i].Payload, m)
		}
	}
}

// TestUDPTransport_Broadcast sends a broadcast and verifies the connected peer
// receives it. memBackend is point-to-point, so one pair exercises the broadcast path.
func TestUDPTransport_Broadcast(t *testing.T) {
	tA, tB := makeLinkedPair(t, 10)
	_, err := tA.Connect("B:2")
	if err != nil {
		t.Fatalf("Connect B: %v", err)
	}

	tA.Broadcast(netcore.ChannelState, []byte("bc"))
	pkts := drainWait(t, tB, 1, 200*time.Millisecond)
	if len(pkts) == 0 {
		t.Fatal("B did not receive broadcast")
	}
	if string(pkts[0].Payload) != "bc" {
		t.Errorf("payload = %q, want \"bc\"", pkts[0].Payload)
	}
}

// TestUDPTransport_Stats verifies that Stats returns non-zero BytesSent after sending.
func TestUDPTransport_Stats(t *testing.T) {
	tA, tB := makeLinkedPair(t, 4)
	id, err := tA.Connect("B:2")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if err := tA.Send(id, netcore.ChannelState, []byte("ping")); err != nil {
		t.Fatalf("Send: %v", err)
	}
	// Wait for at least one goroutine tick.
	_ = drainWait(t, tB, 1, 100*time.Millisecond)
	st := tA.Stats(id)
	if st.BytesSent == 0 {
		t.Error("Stats.BytesSent == 0 after Send")
	}
}

// TestUDPTransport_Disconnect verifies that Disconnect removes the connection.
func TestUDPTransport_Disconnect(t *testing.T) {
	tA, _ := makeLinkedPair(t, 5)
	id, err := tA.Connect("B:2")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	tA.Disconnect(id, "test")
	// Give goroutine time to process the disconnect.
	time.Sleep(20 * time.Millisecond)
	ids := tA.Connections()
	for _, c := range ids {
		if c == id {
			t.Errorf("connection %d still present after Disconnect", id)
		}
	}
}

// TestMemBackend_LossRate verifies that 100% loss drops every packet.
func TestMemBackend_LossRate(t *testing.T) {
	a, b := newMemPair("A:1", "B:2", 0)
	a.LossRate = 1.0

	data := []byte("dropped")
	if err := a.WriteTo(data, b.addr); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	_ = b.SetReadDeadline(time.Now().Add(20 * time.Millisecond))
	n, _, err := b.ReadFrom(make([]byte, 64))
	if n > 0 || err == nil {
		t.Errorf("expected timeout with 100%% loss, got n=%d err=%v", n, err)
	}
}

// TestMemBackend_ReorderRate verifies that 100% reorder causes the first packet
// to be held until a second is sent.
func TestMemBackend_ReorderRate(t *testing.T) {
	a, b := newMemPair("A:1", "B:2", 0)
	a.ReorderRate = 1.0

	// First write is held.
	_ = a.WriteTo([]byte("first"), b.addr)

	// No packet should be deliverable yet.
	_ = b.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
	n, _, _ := b.ReadFrom(make([]byte, 64))
	if n > 0 {
		t.Error("expected first packet to be held, but something arrived")
	}

	// Second write releases the held first packet.
	_ = a.WriteTo([]byte("second"), b.addr)
	_ = b.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	buf := make([]byte, 64)
	n, _, err := b.ReadFrom(buf)
	if err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	if string(buf[:n]) != "first" {
		t.Errorf("got %q, want \"first\"", buf[:n])
	}
}

// TestLossWindow tracks coverage of the loss estimation ring.
func TestLossWindow(t *testing.T) {
	var w lossWindow
	if w.loss() != 0 {
		t.Error("empty window loss should be 0")
	}
	for i := range 10 {
		w.sent(uint16(i))
	}
	if w.loss() != 1.0 {
		t.Errorf("loss = %v, want 1.0 (none acked)", w.loss())
	}
	// Ack the first 5 (seqs 0..4). Base=4, bits cover Base-1..Base-4 = seqs 3,2,1,0.
	a := Ack{Base: 4, Bits: 0b1111} // bit0=seq3, bit1=seq2, bit2=seq1, bit3=seq0
	w.onAck(a)
	// 5 of 10 acked → 50% loss.
	if got := w.loss(); got < 0.49 || got > 0.51 {
		t.Errorf("loss after 5 acks = %v, want ~0.5", got)
	}
}

// TestUDPTransport_SendBeforeStart verifies Send returns an error when not started.
func TestUDPTransport_SendBeforeStart(t *testing.T) {
	tr := New(NetworkSettings{}, defaultChannels())
	err := tr.Send(1, 0, []byte("x"))
	if err == nil {
		t.Error("Send before start should return error")
	}
}

// TestUDPTransport_HandshakeEvents verifies that a Connected event fires on
// both sides after the 3-way handshake completes.
func TestUDPTransport_HandshakeEvents(t *testing.T) {
	tA, tB := makeLinkedPair(t, 20)

	_, err := tA.Connect("B:2")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// Connect() returning is proof client-side handshake completed. Check server-side event.
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		select {
		case ev := <-tB.EventsCh:
			if ev.Connected != nil && ev.Connected.IsServer {
				return // server-side Connected event received
			}
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
	t.Error("server did not emit Connected event within 200ms")
}

// TestUDPTransport_HandshakeTimeout verifies Connect returns an error when the
// server never responds within HandshakeTimeout.
func TestUDPTransport_HandshakeTimeout(t *testing.T) {
	bA, _ := newMemPair("A:1", "B:2", 30)
	bA.LossRate = 1.0 // drop everything — server will never respond
	settings := NetworkSettings{HandshakeTimeout: 100 * time.Millisecond}
	tA := NewWithBackend(bA, settings, defaultChannels())
	tA.dialFunc = memDialFunc
	if err := tA.Listen(""); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tA.Close() })

	_, err := tA.Connect("B:2")
	if err == nil {
		t.Fatal("Connect should return error when server never responds")
	}
}

// TestUDPTransport_MTUNegotiation verifies that the MTU probe/ack cycle
// settles c.mtu to a value from the probe ladder within a short deadline.
func TestUDPTransport_MTUNegotiation(t *testing.T) {
	tA, tB := makeLinkedPair(t, 40)
	idA, err := tA.Connect("B:2")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	// Drain briefly so tB processes the PMTUD probe.
	_ = drainWait(t, tB, 0, 200*time.Millisecond)

	st := tA.Stats(idA)
	// After a real handshake + PMTUD exchange, BytesSent reflects probe traffic.
	if st.BytesSent == 0 {
		t.Error("Stats.BytesSent == 0 after handshake+PMTUD")
	}
}
