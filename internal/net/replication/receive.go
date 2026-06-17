package replication

import (
	"encoding/json"
	"log"
	"reflect"

	"github.com/neuengine/neu/internal/ecs/command"
	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/world"
	netcore "github.com/neuengine/neu/internal/net"
)

// ReplicationReceiveSystem runs in PreUpdate and applies inbound replication
// messages to the client World via a deferred CommandBuffer (INV-5).
// It never writes directly to the World during system execution.
type ReplicationReceiveSystem struct {
	em  *EntityMap
	cmd *command.CommandBuffer
}

// NewReplicationReceiveSystem creates a receive system backed by em and cmd.
// cmd must be registered with the World via CommandBuffer.RegisterWith so that
// buffered commands are flushed at the standard apply point.
func NewReplicationReceiveSystem(em *EntityMap, cmd *command.CommandBuffer) *ReplicationReceiveSystem {
	return &ReplicationReceiveSystem{em: em, cmd: cmd}
}

// Run is the ECS system entry point; called once per frame in PreUpdate.
func (sys *ReplicationReceiveSystem) Run(w *world.World) {
	qp, ok := world.Resource[netcore.InboundQueue](w)
	if !ok {
		return
	}
	for _, pkt := range qp.Packets {
		if pkt.Channel != netcore.ChannelSnapshot {
			continue
		}
		sys.processPacket(pkt, w)
	}
}

func (sys *ReplicationReceiveSystem) processPacket(pkt netcore.InboundPacket, w *world.World) {
	data := pkt.Payload
	for len(data) > 0 {
		msg, rest, ok := DecodeReplicationMessage(data)
		if !ok {
			return
		}
		data = rest
		switch msg.Kind {
		case MsgEntitySpawn:
			sys.applyEntitySpawn(msg, w)
		case MsgComponentUpdate:
			sys.applyComponentUpdate(msg, w)
		case MsgComponentRemove:
			sys.applyComponentRemove(msg, w)
		case MsgEntityDespawn:
			sys.applyEntityDespawn(msg, w)
		}
	}
}

func (sys *ReplicationReceiveSystem) applyEntitySpawn(msg ReplicationMessage, w *world.World) {
	clientID := sys.em.Map(msg.ServerID)
	clientE := entity.FromID(clientID)
	vals := sys.resolveComponents(msg.Components, w)
	sys.cmd.Push(command.NewCustomCommand(func(w *world.World) {
		if _, has := w.ArchetypeOf(clientE); !has {
			w.SpawnWithEntity(clientE)
		}
		for _, v := range vals {
			_ = w.Insert(clientE, component.Data{Value: v})
		}
	}))
}

func (sys *ReplicationReceiveSystem) applyComponentUpdate(msg ReplicationMessage, w *world.World) {
	clientID, ok := sys.em.ClientOf(msg.ServerID)
	if !ok {
		return
	}
	clientE := entity.FromID(clientID)
	vals := sys.resolveComponents(msg.Components, w)
	sys.cmd.Push(command.NewCustomCommand(func(w *world.World) {
		for _, v := range vals {
			_ = w.Insert(clientE, component.Data{Value: v})
		}
	}))
}

func (sys *ReplicationReceiveSystem) applyComponentRemove(msg ReplicationMessage, w *world.World) {
	clientID, ok := sys.em.ClientOf(msg.ServerID)
	if !ok {
		return
	}
	clientE := entity.FromID(clientID)
	for _, typeName := range msg.TypeNames {
		id, ok := w.Components().LookupByName(typeName)
		if !ok {
			log.Printf("replication: remove unknown component type %q", typeName)
			continue
		}
		cid := id
		sys.cmd.Push(command.NewCustomCommand(func(w *world.World) {
			_ = world.RemoveByID(w, clientE, cid)
		}))
	}
}

func (sys *ReplicationReceiveSystem) applyEntityDespawn(msg ReplicationMessage, w *world.World) {
	clientID, ok := sys.em.Unmap(msg.ServerID)
	if !ok {
		return
	}
	clientE := entity.FromID(clientID)
	sys.cmd.Push(command.NewCustomCommand(func(w *world.World) {
		if _, has := w.ArchetypeOf(clientE); has {
			_ = w.Despawn(clientE)
		}
	}))
}

// resolveComponents decodes each ReplicatedComponent into a typed Go value.
// Unknown type names and malformed JSON are logged and skipped.
func (sys *ReplicationReceiveSystem) resolveComponents(comps []ReplicatedComponent, w *world.World) []any {
	result := make([]any, 0, len(comps))
	for _, rc := range comps {
		id, ok := w.Components().LookupByName(rc.TypeName)
		if !ok {
			log.Printf("replication: unknown component type %q — skipped", rc.TypeName)
			continue
		}
		info := w.Components().Info(id)
		ptr := reflect.New(info.Type)
		if err := json.Unmarshal(rc.Data, ptr.Interface()); err != nil {
			log.Printf("replication: unmarshal %q: %v", rc.TypeName, err)
			continue
		}
		result = append(result, ptr.Elem().Interface())
	}
	return result
}
