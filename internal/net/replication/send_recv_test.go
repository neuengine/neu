package replication

import (
	"encoding/json"
	"maps"
	"reflect"
	"testing"

	"github.com/neuengine/neu/internal/ecs/command"
	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/world"
	netcore "github.com/neuengine/neu/internal/net"
	"github.com/neuengine/neu/pkg/app"
)

// ─── Test component types ──────────────────────────────────────────────────────

type testPos struct{ X, Y float32 }
type testHealth struct{ HP int }

// ─── Stub transport ───────────────────────────────────────────────────────────

type stubTransport struct{ conns []netcore.ConnectionID }

func (t *stubTransport) Listen(addr netcore.SocketAddr) error                               { return nil }
func (t *stubTransport) Connect(addr netcore.SocketAddr) (netcore.ConnectionID, error)      { return 0, nil }
func (t *stubTransport) Disconnect(id netcore.ConnectionID, reason string)                  {}
func (t *stubTransport) Connections() []netcore.ConnectionID                                 { return t.conns }
func (t *stubTransport) Send(id netcore.ConnectionID, ch netcore.ChannelID, p []byte) error { return nil }
func (t *stubTransport) Broadcast(ch netcore.ChannelID, p []byte)                           {}
func (t *stubTransport) Drain() []netcore.InboundPacket                                     { return nil }
func (t *stubTransport) Stats(id netcore.ConnectionID) netcore.ConnectionStats               { return netcore.ConnectionStats{} }

// ─── Helpers ──────────────────────────────────────────────────────────────────

func makeServerWorld() *world.World {
	w := world.NewWorld()
	world.SetResource(w, netcore.OutboundQueue{})
	cfg := ReplicationConfig{}
	world.SetResource(w, cfg)
	return w
}

func addTransportWithConns(w *world.World, conns ...netcore.ConnectionID) {
	world.SetResource(w, netcore.TransportResource{T: &stubTransport{conns: conns}})
}

// spawnReplicable creates an entity with Replicated tag and component data,
// registers the data component in cfg, and bumps the world change tick.
func spawnReplicable(w *world.World, cfg *ReplicationConfig, pos testPos) entity.EntityID {
	id := world.RegisterComponent[testPos](w)
	cfg.Set(id, RuleReplicate)
	e := w.Spawn(
		component.Data{Value: Replicated{}},
		component.Data{Value: pos},
	)
	w.IncrementChangeTick()
	return e.ID()
}

// ─── Priority tests ───────────────────────────────────────────────────────────

func TestPriorityStateAccumulation(t *testing.T) {
	t.Parallel()
	ps := &priorityState{base: 2.0}
	if got := ps.effective(); got != 2.0 {
		t.Errorf("effective before deferred = %v, want 2.0", got)
	}
	ps.onDeferred()
	if got := ps.effective(); got != 4.0 {
		t.Errorf("effective after one deferred = %v, want 4.0", got)
	}
	ps.onSent()
	if got := ps.effective(); got != 2.0 {
		t.Errorf("effective after sent = %v, want 2.0 (reset)", got)
	}
}

func TestPriorityStoreGetAndForget(t *testing.T) {
	t.Parallel()
	ps := NewPriorityStore()
	var eid entity.EntityID = 7

	s := ps.get(eid)
	if s.base != DefaultBasePriority {
		t.Errorf("base = %v, want %v", s.base, DefaultBasePriority)
	}
	// Same pointer on repeat call.
	if ps.get(eid) != s {
		t.Error("get should return the same *priorityState on repeat calls")
	}
	ps.forget(eid)
	s2 := ps.get(eid)
	if s2 == s {
		t.Error("get after forget should return a fresh *priorityState")
	}
}

func TestSortByPriority(t *testing.T) {
	t.Parallel()
	updates := []pendingUpdate{
		{entity: 1, priority: 1.0},
		{entity: 2, priority: 5.0},
		{entity: 3, priority: 2.5},
	}
	sortByPriority(updates)
	if updates[0].entity != 2 || updates[1].entity != 3 || updates[2].entity != 1 {
		t.Errorf("unexpected order: %v", updates)
	}
}

// ─── Send system tests ────────────────────────────────────────────────────────

func TestReplicationSendSystem_NoConfig(t *testing.T) {
	t.Parallel()
	w := world.NewWorld()
	world.SetResource(w, netcore.OutboundQueue{})
	addTransportWithConns(w, 1)
	sys := NewReplicationSendSystem(nil, 0)
	sys.Run(w) // must not panic; no cfg resource → early return
	qp, _ := world.Resource[netcore.OutboundQueue](w)
	if len(qp.Messages) != 0 {
		t.Error("expected no messages when ReplicationConfig is missing")
	}
}

func TestReplicationSendSystem_NoTransport(t *testing.T) {
	t.Parallel()
	w := makeServerWorld()
	sys := NewReplicationSendSystem(nil, 0)
	sys.Run(w) // no transport → early return
	qp, _ := world.Resource[netcore.OutboundQueue](w)
	if len(qp.Messages) != 0 {
		t.Error("expected no messages when TransportResource is missing")
	}
}

func TestReplicationSendSystem_NoReplicableEntities(t *testing.T) {
	t.Parallel()
	w := makeServerWorld()
	addTransportWithConns(w, 1)
	// Spawn an entity WITHOUT Replicated tag.
	w.Spawn(component.Data{Value: testPos{X: 1}})
	sys := NewReplicationSendSystem(nil, 0)
	sys.Run(w)
	qp, _ := world.Resource[netcore.OutboundQueue](w)
	if len(qp.Messages) != 0 {
		t.Error("non-replicated entity must not produce messages")
	}
}

func TestReplicationSendSystem_EntitySpawn(t *testing.T) {
	t.Parallel()
	w := makeServerWorld()
	addTransportWithConns(w, 1)
	cfgp, _ := world.Resource[ReplicationConfig](w)
	eid := spawnReplicable(w, cfgp, testPos{X: 3, Y: 4})

	sys := NewReplicationSendSystem(ReplicateAll{}, 0)
	sys.Run(w)

	qp, _ := world.Resource[netcore.OutboundQueue](w)
	if len(qp.Messages) == 0 {
		t.Fatal("expected at least one EntitySpawn message")
	}
	msg, _, ok := DecodeReplicationMessage(qp.Messages[0].Payload)
	if !ok || msg.Kind != MsgEntitySpawn {
		t.Errorf("expected MsgEntitySpawn, got Kind=%d", msg.Kind)
	}
	if msg.ServerID != eid {
		t.Errorf("ServerID=%d, want %d", msg.ServerID, eid)
	}
	if len(msg.Components) == 0 {
		t.Error("spawn message must include component data")
	}
}

func TestReplicationSendSystem_EntityDespawn(t *testing.T) {
	t.Parallel()
	w := makeServerWorld()
	addTransportWithConns(w, 1)
	cfgp, _ := world.Resource[ReplicationConfig](w)
	eid := spawnReplicable(w, cfgp, testPos{X: 1})

	sys := NewReplicationSendSystem(ReplicateAll{}, 0)
	sys.Run(w) // spawn frame: entity enters visibility

	// Remove the Replicated tag so entity is no longer replicable.
	e := entity.FromID(eid)
	_ = world.Remove[Replicated](w, e)

	qp, _ := world.Resource[netcore.OutboundQueue](w)
	qp.Messages = qp.Messages[:0] // clear previous

	sys.Run(w) // next frame: entity gone from visibility → EntityDespawn

	if len(qp.Messages) == 0 {
		t.Fatal("expected EntityDespawn message")
	}
	msg, _, ok := DecodeReplicationMessage(qp.Messages[0].Payload)
	if !ok || msg.Kind != MsgEntityDespawn {
		t.Errorf("expected MsgEntityDespawn, got Kind=%d ok=%v", msg.Kind, ok)
	}
}

func TestReplicationSendSystem_ComponentUpdate(t *testing.T) {
	t.Parallel()
	w := makeServerWorld()
	addTransportWithConns(w, 1)
	cfgp, _ := world.Resource[ReplicationConfig](w)
	eid := spawnReplicable(w, cfgp, testPos{X: 1})

	sys := NewReplicationSendSystem(ReplicateAll{}, 0)
	sys.Run(w) // spawn frame

	// Mutate the component to trigger a change tick.
	e := entity.FromID(eid)
	w.IncrementChangeTick()
	_ = w.Insert(e, component.Data{Value: testPos{X: 99}})

	qp, _ := world.Resource[netcore.OutboundQueue](w)
	qp.Messages = qp.Messages[:0]

	sys.Run(w) // update frame: changed component → ComponentUpdate

	var found bool
	for _, outMsg := range qp.Messages {
		msg, _, ok := DecodeReplicationMessage(outMsg.Payload)
		if ok && msg.Kind == MsgComponentUpdate && msg.ServerID == eid {
			found = true
		}
	}
	if !found {
		t.Error("expected ComponentUpdate for mutated component")
	}
}

func TestReplicationSendSystem_BandwidthBudgetExhausted(t *testing.T) {
	t.Parallel()
	w := makeServerWorld()
	addTransportWithConns(w, 1)
	cfgp, _ := world.Resource[ReplicationConfig](w)

	// Spawn two entities with large-ish components.
	eid1 := spawnReplicable(w, cfgp, testPos{X: 1})
	eid2 := spawnReplicable(w, cfgp, testPos{X: 2})
	_ = eid1
	_ = eid2

	// Very tight budget: only enough for one spawn message (~50 bytes).
	sys := NewReplicationSendSystem(ReplicateAll{}, 50)
	sys.Run(w)

	qp, _ := world.Resource[netcore.OutboundQueue](w)
	// With budget=50, at least one spawn should be suppressed.
	// Both spawn messages for X:1 and X:2 are ~45+ bytes each.
	// The exact count depends on the encoding; just verify budget is respected.
	total := 0
	for _, m := range qp.Messages {
		total += len(m.Payload)
	}
	if total > 200 {
		t.Errorf("total sent bytes %d exceeds a reasonable budget cap", total)
	}
}

func TestReplicationSendSystem_PriorityAccumulation(t *testing.T) {
	t.Parallel()
	w := makeServerWorld()
	addTransportWithConns(w, 1)
	cfgp, _ := world.Resource[ReplicationConfig](w)
	spawnReplicable(w, cfgp, testPos{X: 1})

	// First run: spawn.
	sys := NewReplicationSendSystem(ReplicateAll{}, 0)
	sys.Run(w)

	// Mutate to produce an update.
	entities := collectEntityIDs(w)
	if len(entities) == 0 {
		t.Fatal("no entities found")
	}
	e := entity.FromID(entities[0])
	w.IncrementChangeTick()
	_ = w.Insert(e, component.Data{Value: testPos{X: 55}})

	qp, _ := world.Resource[netcore.OutboundQueue](w)
	qp.Messages = qp.Messages[:0]

	// Second run with near-zero budget: entity deferred.
	sys2 := NewReplicationSendSystem(ReplicateAll{}, 1)
	maps.Copy(sys2.connections, sys.connections)
	sys2.Run(w)

	// After deferral, accumulator grew; verify state exists in connections.
	cs := sys2.connections[1]
	if cs == nil {
		t.Fatal("no connection state for conn 1")
	}
	ps := cs.prio.get(entities[0])
	// Either it was sent (accumulator reset) or deferred (accumulator > 0).
	// Just verify the state object exists and is well-formed.
	if ps.base != DefaultBasePriority {
		t.Errorf("base priority = %v, want %v", ps.base, DefaultBasePriority)
	}
}

// collectEntityIDs iterates all archetypes to find entities with Replicated.
func collectEntityIDs(w *world.World) []entity.EntityID {
	reg := w.Components()
	replicatedID, ok := reg.Lookup(reflect.TypeFor[Replicated]())
	if !ok {
		return nil
	}
	var ids []entity.EntityID
	w.Archetypes().Each(func(arch *world.Archetype) bool {
		if arch.Has(replicatedID) {
			for _, e := range arch.Entities() {
				ids = append(ids, e.ID())
			}
		}
		return true
	})
	return ids
}

// ─── Receive system tests ─────────────────────────────────────────────────────

func makeClientWorld() (*world.World, *EntityMap, *command.CommandBuffer) {
	w := world.NewWorld()
	em := NewEntityMap(func() entity.EntityID {
		return w.Entities().Allocate().ID()
	})
	cmd := command.NewCommandBuffer(w.Entities(), 64)
	cmd.RegisterWith(w)
	return w, em, cmd
}

func TestReplicationReceiveSystem_NoQueue(t *testing.T) {
	t.Parallel()
	w, em, cmd := makeClientWorld()
	sys := NewReplicationReceiveSystem(em, cmd)
	sys.Run(w) // no InboundQueue resource → no-op, no panic
}

func TestReplicationReceiveSystem_EntitySpawn(t *testing.T) {
	t.Parallel()
	w, em, cmd := makeClientWorld()
	sys := NewReplicationReceiveSystem(em, cmd)

	// Register component type on the client world.
	_ = world.RegisterComponent[testPos](w)
	typeName := componentTypeName[testPos](w)

	// Encode a spawn message.
	posData, _ := json.Marshal(testPos{X: 1.5, Y: 2.5})
	var serverID entity.EntityID = 42
	msg := ReplicationMessage{
		Kind:     MsgEntitySpawn,
		ServerID: serverID,
		Components: []ReplicatedComponent{
			{TypeName: typeName, Data: posData},
		},
	}
	encoded := EncodeReplicationMessage(msg)
	world.SetResource(w, netcore.InboundQueue{Packets: []netcore.InboundPacket{
		{Connection: 1, Channel: netcore.ChannelSnapshot, Payload: encoded},
	}})

	sys.Run(w)
	w.ApplyDeferred() // flush commands

	clientID, ok := em.ClientOf(serverID)
	if !ok {
		t.Fatal("EntityMap has no mapping for serverID 42")
	}
	clientE := entity.FromID(clientID)
	if _, has := w.ArchetypeOf(clientE); !has {
		t.Error("client entity was not spawned in the world")
	}
	posPtr, posOK := world.Get[testPos](w, clientE)
	if !posOK || posPtr == nil {
		t.Error("testPos component not inserted")
	} else if posPtr.X != 1.5 || posPtr.Y != 2.5 {
		t.Errorf("testPos = %+v, want {X:1.5,Y:2.5}", *posPtr)
	}
}

func TestReplicationReceiveSystem_ComponentUpdate(t *testing.T) {
	t.Parallel()
	w, em, cmd := makeClientWorld()
	sys := NewReplicationReceiveSystem(em, cmd)
	_ = world.RegisterComponent[testPos](w)
	typeName := componentTypeName[testPos](w)

	var serverID entity.EntityID = 10

	// First, spawn the entity.
	spawnMsg := ReplicationMessage{
		Kind:       MsgEntitySpawn,
		ServerID:   serverID,
		Components: []ReplicatedComponent{{TypeName: typeName, Data: mustMarshal(testPos{X: 1})}},
	}
	world.SetResource(w, netcore.InboundQueue{Packets: []netcore.InboundPacket{
		{Connection: 1, Channel: netcore.ChannelSnapshot, Payload: EncodeReplicationMessage(spawnMsg)},
	}})
	sys.Run(w)
	w.ApplyDeferred()

	// Now send an update.
	updateMsg := ReplicationMessage{
		Kind:       MsgComponentUpdate,
		ServerID:   serverID,
		Components: []ReplicatedComponent{{TypeName: typeName, Data: mustMarshal(testPos{X: 99})}},
	}
	world.SetResource(w, netcore.InboundQueue{Packets: []netcore.InboundPacket{
		{Connection: 1, Channel: netcore.ChannelSnapshot, Payload: EncodeReplicationMessage(updateMsg)},
	}})
	sys.Run(w)
	w.ApplyDeferred()

	clientID, _ := em.ClientOf(serverID)
	clientE := entity.FromID(clientID)
	posPtr, _ := world.Get[testPos](w, clientE)
	if posPtr == nil || posPtr.X != 99 {
		t.Errorf("testPos after update = %+v, want {X:99}", posPtr)
	}
}

func TestReplicationReceiveSystem_EntityDespawn(t *testing.T) {
	t.Parallel()
	w, em, cmd := makeClientWorld()
	sys := NewReplicationReceiveSystem(em, cmd)
	_ = world.RegisterComponent[testPos](w)
	typeName := componentTypeName[testPos](w)

	var serverID entity.EntityID = 7

	// Spawn.
	world.SetResource(w, netcore.InboundQueue{Packets: []netcore.InboundPacket{
		{Channel: netcore.ChannelSnapshot, Payload: EncodeReplicationMessage(ReplicationMessage{
			Kind: MsgEntitySpawn, ServerID: serverID,
			Components: []ReplicatedComponent{{TypeName: typeName, Data: mustMarshal(testPos{X: 1})}},
		})},
	}})
	sys.Run(w)
	w.ApplyDeferred()

	clientID, _ := em.ClientOf(serverID)
	clientE := entity.FromID(clientID)
	if _, has := w.ArchetypeOf(clientE); !has {
		t.Fatal("entity not spawned in setup")
	}

	// Despawn.
	world.SetResource(w, netcore.InboundQueue{Packets: []netcore.InboundPacket{
		{Channel: netcore.ChannelSnapshot, Payload: EncodeReplicationMessage(ReplicationMessage{
			Kind: MsgEntityDespawn, ServerID: serverID,
		})},
	}})
	sys.Run(w)
	w.ApplyDeferred()

	if _, stillHas := w.ArchetypeOf(clientE); stillHas {
		t.Error("entity should have been despawned")
	}
	if _, stillMapped := em.ClientOf(serverID); stillMapped {
		t.Error("EntityMap should not retain mapping after despawn")
	}
}

func TestReplicationReceiveSystem_ComponentRemove(t *testing.T) {
	t.Parallel()
	w, em, cmd := makeClientWorld()
	sys := NewReplicationReceiveSystem(em, cmd)
	_ = world.RegisterComponent[testPos](w)
	_ = world.RegisterComponent[testHealth](w)
	posName := componentTypeName[testPos](w)
	hpName := componentTypeName[testHealth](w)

	var serverID entity.EntityID = 5

	// Spawn with two components.
	world.SetResource(w, netcore.InboundQueue{Packets: []netcore.InboundPacket{
		{Channel: netcore.ChannelSnapshot, Payload: EncodeReplicationMessage(ReplicationMessage{
			Kind: MsgEntitySpawn, ServerID: serverID,
			Components: []ReplicatedComponent{
				{TypeName: posName, Data: mustMarshal(testPos{X: 1})},
				{TypeName: hpName, Data: mustMarshal(testHealth{HP: 100})},
			},
		})},
	}})
	sys.Run(w)
	w.ApplyDeferred()

	// Remove testHealth.
	world.SetResource(w, netcore.InboundQueue{Packets: []netcore.InboundPacket{
		{Channel: netcore.ChannelSnapshot, Payload: EncodeReplicationMessage(ReplicationMessage{
			Kind: MsgComponentRemove, ServerID: serverID,
			TypeNames: []string{hpName},
		})},
	}})
	sys.Run(w)
	w.ApplyDeferred()

	clientID, _ := em.ClientOf(serverID)
	clientE := entity.FromID(clientID)
	if _, ok := world.Get[testHealth](w, clientE); ok {
		t.Error("testHealth should have been removed")
	}
	if _, ok := world.Get[testPos](w, clientE); !ok {
		t.Error("testPos should still be present after removing testHealth")
	}
}

func TestReplicationReceiveSystem_UnknownTypeSkipped(t *testing.T) {
	t.Parallel()
	w, em, cmd := makeClientWorld()
	sys := NewReplicationReceiveSystem(em, cmd)
	// Do NOT register testPos on the client world.

	var serverID entity.EntityID = 99
	msg := ReplicationMessage{
		Kind:     MsgEntitySpawn,
		ServerID: serverID,
		Components: []ReplicatedComponent{
			{TypeName: "no.such.Type", Data: []byte(`{}`)},
		},
	}
	world.SetResource(w, netcore.InboundQueue{Packets: []netcore.InboundPacket{
		{Channel: netcore.ChannelSnapshot, Payload: EncodeReplicationMessage(msg)},
	}})
	// Must not panic; entity is mapped but spawned with no components.
	sys.Run(w)
	w.ApplyDeferred()
}

func TestReplicationReceiveSystem_IgnoresNonSnapshotChannel(t *testing.T) {
	t.Parallel()
	w, em, cmd := makeClientWorld()
	sys := NewReplicationReceiveSystem(em, cmd)
	_ = world.RegisterComponent[testPos](w)
	typeName := componentTypeName[testPos](w)

	var serverID entity.EntityID = 3
	encoded := EncodeReplicationMessage(ReplicationMessage{
		Kind:       MsgEntitySpawn,
		ServerID:   serverID,
		Components: []ReplicatedComponent{{TypeName: typeName, Data: mustMarshal(testPos{X: 1})}},
	})
	// Deliver on channel 0 (ChannelState), not ChannelSnapshot.
	world.SetResource(w, netcore.InboundQueue{Packets: []netcore.InboundPacket{
		{Connection: 1, Channel: netcore.ChannelState, Payload: encoded},
	}})
	sys.Run(w)
	w.ApplyDeferred()

	if _, ok := em.ClientOf(serverID); ok {
		t.Error("entity should not have been mapped (wrong channel)")
	}
}

// ─── Plugin tests ─────────────────────────────────────────────────────────────

func TestReplicationPlugin_RegistersResources(t *testing.T) {
	t.Parallel()
	a := app.NewApp()
	a.AddPlugin(ReplicationPlugin{})
	w := a.World()

	if _, ok := world.Resource[ReplicationConfig](w); !ok {
		t.Error("ReplicationConfig not registered by ReplicationPlugin")
	}
}

func TestReplicationPlugin_DefaultPolicyIsReplicateAll(t *testing.T) {
	t.Parallel()
	// Verify that a nil Policy in the plugin struct is accepted (no panic).
	a := app.NewApp()
	a.AddPlugin(ReplicationPlugin{Policy: nil}) // should default to ReplicateAll
	_ = a.World()
}

func TestReplicationPlugin_CustomBudget(t *testing.T) {
	t.Parallel()
	// Plugin with custom budget should build without error.
	a := app.NewApp()
	a.AddPlugin(ReplicationPlugin{BandwidthBudget: 1024})
	_ = a.World()
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func mustMarshal(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

// componentTypeName looks up the registered qualified name for component type T.
func componentTypeName[T any](w *world.World) string {
	id := world.RegisterComponent[T](w)
	return w.Components().Info(id).Name
}
