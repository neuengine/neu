package rpc

import (
	"encoding/json"
	"testing"

	"github.com/neuengine/neu/internal/ecs/event"
	"github.com/neuengine/neu/internal/ecs/world"
	netcore "github.com/neuengine/neu/internal/net"
)

// ─── transport stub ───────────────────────────────────────────────────────────

type rpcTestTransport struct {
	sent      []rpcSentMsg
	broadcast []rpcBroadcastMsg
	conns     []netcore.ConnectionID
}

type rpcSentMsg struct {
	id      netcore.ConnectionID
	channel netcore.ChannelID
	payload []byte
}

type rpcBroadcastMsg struct {
	channel netcore.ChannelID
	payload []byte
}

func (s *rpcTestTransport) Listen(_ netcore.SocketAddr) error { return nil }
func (s *rpcTestTransport) Connect(_ netcore.SocketAddr) (netcore.ConnectionID, error) {
	return 0, nil
}
func (s *rpcTestTransport) Disconnect(_ netcore.ConnectionID, _ string) {}
func (s *rpcTestTransport) Connections() []netcore.ConnectionID         { return s.conns }
func (s *rpcTestTransport) Stats(_ netcore.ConnectionID) netcore.ConnectionStats {
	return netcore.ConnectionStats{}
}
func (s *rpcTestTransport) Drain() []netcore.InboundPacket { return nil }
func (s *rpcTestTransport) Send(id netcore.ConnectionID, ch netcore.ChannelID, p []byte) error {
	cp := make([]byte, len(p))
	copy(cp, p)
	s.sent = append(s.sent, rpcSentMsg{id, ch, cp})
	return nil
}
func (s *rpcTestTransport) Broadcast(ch netcore.ChannelID, p []byte) {
	cp := make([]byte, len(p))
	copy(cp, p)
	s.broadcast = append(s.broadcast, rpcBroadcastMsg{ch, cp})
}

// ─── test message types ───────────────────────────────────────────────────────

type MoveCmd struct{ X, Y float32 }
type FireCmd struct{ Weapon string }

// ─── registry ─────────────────────────────────────────────────────────────────

func TestRpcRegistrySequentialIDs(t *testing.T) {
	t.Parallel()
	reg := NewRpcRegistry()
	w := world.NewWorld()

	def1, err := RegisterRpc[MoveCmd](reg, w, DirClientToServer, 0)
	if err != nil {
		t.Fatalf("RegisterRpc[MoveCmd]: %v", err)
	}
	def2, err := RegisterRpc[FireCmd](reg, w, DirServerToClient, 1)
	if err != nil {
		t.Fatalf("RegisterRpc[FireCmd]: %v", err)
	}
	if def1.TypeID == 0 || def2.TypeID == 0 {
		t.Error("TypeID must not be zero")
	}
	if def2.TypeID <= def1.TypeID {
		t.Errorf("TypeIDs not sequential: %d → %d", def1.TypeID, def2.TypeID)
	}
}

func TestRpcRegistryByID(t *testing.T) {
	t.Parallel()
	reg := NewRpcRegistry()
	w := world.NewWorld()
	def, _ := RegisterRpc[MoveCmd](reg, w, DirBidirectional, 0)

	got, ok := reg.ByID(def.TypeID)
	if !ok || got.TypeID != def.TypeID {
		t.Errorf("ByID(%d) = %+v, %v", def.TypeID, got, ok)
	}
	_, ok = reg.ByID(9999)
	if ok {
		t.Error("ByID(9999) should return false")
	}
}

func TestRpcRegistryByType(t *testing.T) {
	t.Parallel()
	reg := NewRpcRegistry()
	w := world.NewWorld()
	def, _ := RegisterRpc[MoveCmd](reg, w, DirClientToServer, 0)

	got, ok := reg.ByType(def.GoType)
	if !ok || got != def {
		t.Errorf("ByType returned wrong definition")
	}
}

func TestRpcRegistryDuplicateReturnsError(t *testing.T) {
	t.Parallel()
	reg := NewRpcRegistry()
	w := world.NewWorld()
	if _, err := RegisterRpc[MoveCmd](reg, w, DirClientToServer, 0); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if _, err := RegisterRpc[MoveCmd](reg, w, DirClientToServer, 0); err == nil {
		t.Error("second RegisterRpc for same type should return error")
	}
}

func TestRpcRegistryLen(t *testing.T) {
	t.Parallel()
	reg := NewRpcRegistry()
	w := world.NewWorld()
	if reg.Len() != 0 {
		t.Errorf("Len() on empty registry = %d, want 0", reg.Len())
	}
	RegisterRpc[MoveCmd](reg, w, DirClientToServer, 0)
	RegisterRpc[FireCmd](reg, w, DirServerToClient, 0)
	if reg.Len() != 2 {
		t.Errorf("Len() = %d, want 2", reg.Len())
	}
}

// ─── sender ───────────────────────────────────────────────────────────────────

func TestSendTargetClient(t *testing.T) {
	t.Parallel()
	reg := NewRpcRegistry()
	w := world.NewWorld()
	RegisterRpc[MoveCmd](reg, w, DirServerToClient, 2)
	tr := &rpcTestTransport{}
	s := NewRpcSender(reg, tr)

	if err := Send(s, TargetClient(5), MoveCmd{X: 1, Y: 2}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(tr.sent) != 1 || tr.sent[0].id != 5 || tr.sent[0].channel != 2 {
		t.Errorf("sent = %+v", tr.sent)
	}
	if len(tr.broadcast) != 0 {
		t.Errorf("unexpected broadcast: %+v", tr.broadcast)
	}
}

func TestSendTargetAllClients(t *testing.T) {
	t.Parallel()
	reg := NewRpcRegistry()
	w := world.NewWorld()
	RegisterRpc[FireCmd](reg, w, DirServerToClient, 0)
	tr := &rpcTestTransport{}
	s := NewRpcSender(reg, tr)

	if err := Send(s, TargetAllClients{}, FireCmd{Weapon: "laser"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(tr.broadcast) != 1 {
		t.Errorf("broadcast count = %d, want 1", len(tr.broadcast))
	}
	if len(tr.sent) != 0 {
		t.Errorf("unexpected unicast: %+v", tr.sent)
	}
}

func TestSendTargetServer(t *testing.T) {
	t.Parallel()
	reg := NewRpcRegistry()
	w := world.NewWorld()
	RegisterRpc[MoveCmd](reg, w, DirClientToServer, 0)
	tr := &rpcTestTransport{}
	s := NewRpcSender(reg, tr)

	if err := Send(s, TargetServer{}, MoveCmd{X: 3, Y: 4}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(tr.broadcast) != 1 {
		t.Errorf("TargetServer should broadcast: got %d broadcasts", len(tr.broadcast))
	}
}

func TestSendTargetGroup(t *testing.T) {
	t.Parallel()
	reg := NewRpcRegistry()
	w := world.NewWorld()
	RegisterRpc[MoveCmd](reg, w, DirServerToClient, 0)
	tr := &rpcTestTransport{}
	s := NewRpcSender(reg, tr)

	ids := []netcore.ConnectionID{1, 2, 3}
	if err := Send(s, TargetGroup{IDs: ids}, MoveCmd{}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(tr.sent) != 3 {
		t.Errorf("TargetGroup: sent %d, want 3", len(tr.sent))
	}
}

func TestSendTargetExcept(t *testing.T) {
	t.Parallel()
	reg := NewRpcRegistry()
	w := world.NewWorld()
	RegisterRpc[FireCmd](reg, w, DirServerToClient, 0)
	tr := &rpcTestTransport{conns: []netcore.ConnectionID{1, 2, 3}}
	s := NewRpcSender(reg, tr)

	if err := Send(s, TargetExcept(2), FireCmd{Weapon: "sword"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(tr.sent) != 2 {
		t.Errorf("TargetExcept: sent %d, want 2 (excluding conn 2)", len(tr.sent))
	}
	for _, sv := range tr.sent {
		if sv.id == 2 {
			t.Error("TargetExcept sent to excluded connection 2")
		}
	}
}

func TestSendUnregisteredTypeReturnsError(t *testing.T) {
	t.Parallel()
	reg := NewRpcRegistry()
	tr := &rpcTestTransport{}
	s := NewRpcSender(reg, tr)

	type UnknownCmd struct{ Val int }
	if err := Send(s, TargetServer{}, UnknownCmd{Val: 1}); err == nil {
		t.Error("Send with unregistered type should return error")
	}
}

// ─── receive system ───────────────────────────────────────────────────────────

func TestRpcReceiveDispatchesKnownType(t *testing.T) {
	t.Parallel()
	reg := NewRpcRegistry()
	w := world.NewWorld()
	def, err := RegisterRpc[MoveCmd](reg, w, DirClientToServer, 0)
	if err != nil {
		t.Fatalf("RegisterRpc: %v", err)
	}

	payload, _ := json.Marshal(MoveCmd{X: 10, Y: 20})
	pkt := EncodeRpcMessage(def.TypeID, payload)
	world.SetResource(w, netcore.InboundQueue{Packets: []netcore.InboundPacket{
		{Connection: 1, Channel: 0, Payload: pkt},
	}})

	sys := NewRpcReceiveSystem(reg)
	sys.Run(w)

	reader := event.NewEventReader[MoveCmd](w)
	var events []MoveCmd
	for e := range reader.All() {
		events = append(events, e)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	if events[0].X != 10 || events[0].Y != 20 {
		t.Errorf("event = %+v, want {X:10 Y:20}", events[0])
	}
}

func TestRpcReceiveUnknownTypeDropped(t *testing.T) {
	t.Parallel()
	reg := NewRpcRegistry()
	w := world.NewWorld()
	pkt := EncodeRpcMessage(99, []byte(`{}`))
	world.SetResource(w, netcore.InboundQueue{Packets: []netcore.InboundPacket{
		{Connection: 1, Channel: 0, Payload: pkt},
	}})

	sys := NewRpcReceiveSystem(reg)
	// Must not panic; unknown typeID logged and dropped (INV-3).
	sys.Run(w)
}

func TestRpcReceiveNoQueueNoPanic(t *testing.T) {
	t.Parallel()
	reg := NewRpcRegistry()
	w := world.NewWorld()
	sys := NewRpcReceiveSystem(reg)
	sys.Run(w)
}

func TestRpcReceiveMultipleMessagesInPayload(t *testing.T) {
	t.Parallel()
	reg := NewRpcRegistry()
	w := world.NewWorld()
	def, _ := RegisterRpc[MoveCmd](reg, w, DirClientToServer, 0)

	p1, _ := json.Marshal(MoveCmd{X: 1})
	p2, _ := json.Marshal(MoveCmd{X: 2})
	msg := append(EncodeRpcMessage(def.TypeID, p1), EncodeRpcMessage(def.TypeID, p2)...)
	world.SetResource(w, netcore.InboundQueue{Packets: []netcore.InboundPacket{
		{Connection: 1, Channel: 0, Payload: msg},
	}})

	sys := NewRpcReceiveSystem(reg)
	sys.Run(w)

	reader := event.NewEventReader[MoveCmd](w)
	var events []MoveCmd
	for e := range reader.All() {
		events = append(events, e)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
}

func TestRpcReceiveMultiplePackets(t *testing.T) {
	t.Parallel()
	reg := NewRpcRegistry()
	w := world.NewWorld()
	def, _ := RegisterRpc[FireCmd](reg, w, DirServerToClient, 0)

	p1, _ := json.Marshal(FireCmd{Weapon: "gun"})
	p2, _ := json.Marshal(FireCmd{Weapon: "bow"})
	world.SetResource(w, netcore.InboundQueue{Packets: []netcore.InboundPacket{
		{Connection: 1, Channel: 0, Payload: EncodeRpcMessage(def.TypeID, p1)},
		{Connection: 2, Channel: 0, Payload: EncodeRpcMessage(def.TypeID, p2)},
	}})

	sys := NewRpcReceiveSystem(reg)
	sys.Run(w)

	reader := event.NewEventReader[FireCmd](w)
	var events []FireCmd
	for e := range reader.All() {
		events = append(events, e)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
}
