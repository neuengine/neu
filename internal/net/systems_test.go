package net

import (
	"testing"

	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app"
)

// netTestTransport is a NetworkTransport stub for systems and plugin tests.
// Distinct from the transport_test.go stubTransport (same package scope).
type netTestTransport struct {
	queued    []InboundPacket
	sent      []netTestSent
	broadcast []netTestBroadcast
}

type netTestSent struct {
	id      ConnectionID
	channel ChannelID
	payload []byte
}

type netTestBroadcast struct {
	channel ChannelID
	payload []byte
}

func (s *netTestTransport) Listen(_ SocketAddr) error                  { return nil }
func (s *netTestTransport) Connect(_ SocketAddr) (ConnectionID, error) { return 0, nil }
func (s *netTestTransport) Disconnect(_ ConnectionID, _ string)        {}
func (s *netTestTransport) Connections() []ConnectionID                { return nil }
func (s *netTestTransport) Stats(_ ConnectionID) ConnectionStats       { return ConnectionStats{} }
func (s *netTestTransport) Drain() []InboundPacket {
	pkts := s.queued
	s.queued = nil
	return pkts
}
func (s *netTestTransport) Send(id ConnectionID, ch ChannelID, p []byte) error {
	cp := make([]byte, len(p))
	copy(cp, p)
	s.sent = append(s.sent, netTestSent{id, ch, cp})
	return nil
}
func (s *netTestTransport) Broadcast(ch ChannelID, p []byte) {
	cp := make([]byte, len(p))
	copy(cp, p)
	s.broadcast = append(s.broadcast, netTestBroadcast{ch, cp})
}

// ─── NetworkReceive ───────────────────────────────────────────────────────────

func TestNetworkReceivePopulatesQueue(t *testing.T) {
	t.Parallel()
	w := world.NewWorld()
	tr := &netTestTransport{queued: []InboundPacket{
		{Connection: 1, Channel: 0, Payload: []byte("pkt")},
		{Connection: 2, Channel: 1, Payload: []byte("rpc")},
	}}
	world.SetResource(w, TransportResource{T: tr})
	world.SetResource(w, InboundQueue{})

	networkReceive(w)

	q, ok := world.Resource[InboundQueue](w)
	if !ok {
		t.Fatal("InboundQueue not found after networkReceive")
	}
	if len(q.Packets) != 2 {
		t.Fatalf("InboundQueue.Packets = %d, want 2", len(q.Packets))
	}
	if string(q.Packets[0].Payload) != "pkt" || string(q.Packets[1].Payload) != "rpc" {
		t.Errorf("unexpected payloads: %v", q.Packets)
	}
}

func TestNetworkReceiveNoTransport(t *testing.T) {
	t.Parallel()
	w := world.NewWorld()
	// No resources at all — must not panic.
	networkReceive(w)
}

func TestNetworkReceiveClearsOnDrain(t *testing.T) {
	t.Parallel()
	w := world.NewWorld()
	tr := &netTestTransport{queued: []InboundPacket{{Connection: 1, Channel: 0, Payload: []byte("x")}}}
	world.SetResource(w, TransportResource{T: tr})
	world.SetResource(w, InboundQueue{})

	networkReceive(w)
	// Second call: transport is empty now.
	networkReceive(w)

	q, _ := world.Resource[InboundQueue](w)
	if len(q.Packets) != 0 {
		t.Errorf("stale packet survived second drain: %v", q.Packets)
	}
}

// ─── NetworkSend ─────────────────────────────────────────────────────────────

func TestNetworkSendFlushesToTransport(t *testing.T) {
	t.Parallel()
	w := world.NewWorld()
	tr := &netTestTransport{}
	world.SetResource(w, TransportResource{T: tr})
	world.SetResource(w, OutboundQueue{Messages: []OutboundMessage{
		{Target: 1, Channel: 0, Payload: []byte("hello")},
		{Target: 0, Channel: 1, Payload: []byte("bc")}, // broadcast (Target==0)
	}})

	networkSend(w)

	if len(tr.sent) != 1 || string(tr.sent[0].payload) != "hello" || tr.sent[0].id != 1 {
		t.Errorf("sent = %+v", tr.sent)
	}
	if len(tr.broadcast) != 1 || string(tr.broadcast[0].payload) != "bc" {
		t.Errorf("broadcast = %+v", tr.broadcast)
	}
	q, _ := world.Resource[OutboundQueue](w)
	if len(q.Messages) != 0 {
		t.Errorf("OutboundQueue not drained: %d messages remain", len(q.Messages))
	}
}

func TestNetworkSendNoTransport(t *testing.T) {
	t.Parallel()
	w := world.NewWorld()
	world.SetResource(w, OutboundQueue{Messages: []OutboundMessage{
		{Target: 1, Channel: 0, Payload: []byte("x")},
	}})
	// Must not panic when TransportResource is absent.
	networkSend(w)
}

// ─── NetworkPlugin ────────────────────────────────────────────────────────────

func TestNetworkPluginRegistersResources(t *testing.T) {
	t.Parallel()
	a := app.NewApp()
	tr := &netTestTransport{}
	a.AddPlugin(NetworkPlugin{Transport: tr})
	w := a.World()

	if _, ok := world.Resource[TransportResource](w); !ok {
		t.Error("TransportResource not registered by NetworkPlugin")
	}
	if _, ok := world.Resource[*SnapshotManager](w); !ok {
		t.Error("*SnapshotManager not registered by NetworkPlugin")
	}
	if _, ok := world.Resource[*DeterministicSchedule](w); !ok {
		t.Error("*DeterministicSchedule not registered by NetworkPlugin")
	}
	if _, ok := world.Resource[*RollbackCoordinator](w); !ok {
		t.Error("*RollbackCoordinator not registered by NetworkPlugin")
	}
	if _, ok := world.Resource[*InputBuffer](w); !ok {
		t.Error("*InputBuffer not registered by NetworkPlugin")
	}
	if _, ok := world.Resource[InboundQueue](w); !ok {
		t.Error("InboundQueue not registered by NetworkPlugin")
	}
	if _, ok := world.Resource[OutboundQueue](w); !ok {
		t.Error("OutboundQueue not registered by NetworkPlugin")
	}
}

func TestNetworkPluginSystemsRunEachFrame(t *testing.T) {
	t.Parallel()
	a := app.NewApp()
	tr := &netTestTransport{queued: []InboundPacket{{Connection: 1, Channel: 0, Payload: []byte("tick")}}}
	a.AddPlugin(NetworkPlugin{Transport: tr})
	a.SetRunMode(app.RunOnce)
	if err := a.Run(); err != nil {
		t.Fatalf("App.Run: %v", err)
	}
	w := a.World()
	q, ok := world.Resource[InboundQueue](w)
	if !ok {
		t.Fatal("InboundQueue not found after Run")
	}
	// After one frame, networkReceive drained the stub → packet is in queue.
	if len(q.Packets) != 1 || string(q.Packets[0].Payload) != "tick" {
		t.Errorf("InboundQueue after one frame = %+v", q.Packets)
	}
}
