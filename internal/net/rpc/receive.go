package rpc

import (
	"encoding/json"
	"log"
	"reflect"
	"time"

	"github.com/neuengine/neu/internal/ecs/world"
	netcore "github.com/neuengine/neu/internal/net"
)

// RpcReceiveSystem is a PreUpdate ECS system. Each frame it reads packets from
// InboundQueue, decodes RPC messages embedded in the payload, and fires typed
// ECS events via the dispatchers registered in an RpcRegistry.
//
// Unknown type IDs are logged and dropped without panicking (INV-3).
type RpcReceiveSystem struct {
	reg *RpcRegistry
}

// NewRpcReceiveSystem creates a receive system backed by reg.
func NewRpcReceiveSystem(reg *RpcRegistry) *RpcReceiveSystem {
	return &RpcReceiveSystem{reg: reg}
}

// Run implements the ECS system interface; called once per frame.
// If an *RpcRateLimit resource is present on the world, inbound messages
// that exceed the limit are dropped and counted before dispatch.
func (sys *RpcReceiveSystem) Run(w *world.World) {
	q, ok := world.Resource[netcore.InboundQueue](w)
	if !ok {
		return
	}
	// Resolve optional rate limiter from world (may be nil if not installed).
	var rl *RpcRateLimit
	if rlpp, has := world.Resource[*RpcRateLimit](w); has && rlpp != nil {
		rl = *rlpp
	}
	now := time.Now()
	for _, pkt := range q.Packets {
		sys.processPacket(pkt, rl, now)
	}
}

func (sys *RpcReceiveSystem) processPacket(pkt netcore.InboundPacket, rl *RpcRateLimit, now time.Time) {
	data := pkt.Payload
	for len(data) > 0 {
		typeID, payload, rest, ok := DecodeRpcMessage(data)
		if !ok {
			return
		}
		data = rest

		if rl != nil && !rl.Allow(pkt.Connection, typeID, now) {
			rl.RecordDrop(pkt.Connection)
			continue
		}
		def, known := sys.reg.byID[typeID]
		if !known {
			log.Printf("rpc: unknown typeID %d from connection %d — dropped", typeID, pkt.Connection)
			continue
		}
		dispatcher, hasDispatcher := sys.reg.dispatchers[typeID]
		if !hasDispatcher {
			continue
		}
		// Allocate *T, unmarshal into it, then pass the T value to the dispatcher.
		ptr := reflect.New(def.GoType)
		if err := json.Unmarshal(payload, ptr.Interface()); err != nil {
			log.Printf("rpc: unmarshal %s from connection %d: %v", def.GoType.Name(), pkt.Connection, err)
			continue
		}
		dispatcher(ptr.Elem().Interface())
	}
}
