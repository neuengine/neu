package net

import "testing"

func TestDeliveryModeString(t *testing.T) {
	t.Parallel()
	cases := map[DeliveryMode]string{
		Unreliable:        "Unreliable",
		ReliableUnordered: "ReliableUnordered",
		ReliableOrdered:   "ReliableOrdered",
		DeliveryMode(99):  "Unknown",
	}
	for m, want := range cases {
		if got := m.String(); got != want {
			t.Errorf("DeliveryMode(%d).String() = %q, want %q", m, got, want)
		}
	}
}

func TestDisconnectReasonString(t *testing.T) {
	t.Parallel()
	cases := map[DisconnectReason]string{
		DisconnectGraceful:      "Graceful",
		DisconnectTimeout:       "Timeout",
		DisconnectProtocolError: "ProtocolError",
		DisconnectRejected:      "Rejected",
		DisconnectLocalClose:    "LocalClose",
		DisconnectReason(99):    "Unknown",
	}
	for r, want := range cases {
		if got := r.String(); got != want {
			t.Errorf("DisconnectReason(%d).String() = %q, want %q", r, got, want)
		}
	}
}

func TestDefaultChannelAssignments(t *testing.T) {
	t.Parallel()
	// The default channel IDs match the l1-transport §4.2 table.
	if ChannelState != 0 || ChannelEvents != 1 || ChannelControl != 2 || ChannelSnapshot != 3 {
		t.Errorf("default channel IDs drifted: state=%d events=%d control=%d snapshot=%d",
			ChannelState, ChannelEvents, ChannelControl, ChannelSnapshot)
	}
}

// stubTransport verifies the NetworkTransport interface is satisfiable and its
// handle/event types compose as expected (a compile-time + behavioural check).
type stubTransport struct {
	inbound []InboundPacket
	conns   []ConnectionID
}

func (s *stubTransport) Listen(SocketAddr) error { return nil }
func (s *stubTransport) Connect(SocketAddr) (ConnectionID, error) {
	id := ConnectionID(len(s.conns) + 1)
	s.conns = append(s.conns, id)
	return id, nil
}
func (s *stubTransport) Disconnect(ConnectionID, string) {}
func (s *stubTransport) Connections() []ConnectionID     { return s.conns }
func (s *stubTransport) Send(id ConnectionID, ch ChannelID, p []byte) error {
	s.inbound = append(s.inbound, InboundPacket{Connection: id, Channel: ch, Payload: p})
	return nil
}
func (s *stubTransport) Broadcast(ch ChannelID, p []byte) {
	for _, id := range s.conns {
		_ = s.Send(id, ch, p)
	}
}
func (s *stubTransport) Drain() []InboundPacket {
	out := s.inbound
	s.inbound = nil
	return out
}
func (s *stubTransport) Stats(ConnectionID) ConnectionStats { return ConnectionStats{} }

func TestNetworkTransportInterface(t *testing.T) {
	t.Parallel()
	var tr NetworkTransport = &stubTransport{}
	id, _ := tr.Connect("127.0.0.1:7777")
	if id != 1 {
		t.Fatalf("Connect returned %d, want 1", id)
	}
	if err := tr.Send(id, ChannelEvents, []byte("hi")); err != nil {
		t.Fatalf("Send: %v", err)
	}
	got := tr.Drain()
	if len(got) != 1 || got[0].Channel != ChannelEvents || string(got[0].Payload) != "hi" {
		t.Errorf("Drain = %+v, want one ChannelEvents packet 'hi'", got)
	}
	// Events compose with handle types.
	ev := Disconnected{Connection: id, RemoteAddr: "127.0.0.1:7777", Reason: DisconnectTimeout}
	if ev.Reason.String() != "Timeout" || ev.Connection != id || ev.RemoteAddr != "127.0.0.1:7777" {
		t.Errorf("event composed wrong: %+v", ev)
	}
}
