package rpc

import (
	"fmt"
	"reflect"

	"github.com/neuengine/neu/internal/ecs/event"
	"github.com/neuengine/neu/internal/ecs/world"
	netcore "github.com/neuengine/neu/internal/net"
)

// RpcTypeID is the 16-bit wire identifier for a registered RPC message type.
type RpcTypeID = uint16

// RpcDirection constrains which peers may send a particular RPC type.
type RpcDirection uint8

const (
	// DirClientToServer: only clients may invoke this RPC.
	DirClientToServer RpcDirection = iota
	// DirServerToClient: only the server may invoke this RPC.
	DirServerToClient
	// DirBidirectional: either peer may invoke this RPC.
	DirBidirectional
)

// RpcDefinition holds the metadata stored for a registered RPC type.
type RpcDefinition struct {
	TypeID    RpcTypeID
	Direction RpcDirection
	Channel   netcore.ChannelID
	GoType    reflect.Type
}

// RpcRegistry maps wire TypeIDs to RPC definitions and typed event dispatchers.
// Populate via RegisterRpc[T]; never construct directly.
type RpcRegistry struct {
	byID        map[RpcTypeID]*RpcDefinition
	byType      map[reflect.Type]*RpcDefinition
	dispatchers map[RpcTypeID]func(any)
	nextID      RpcTypeID
}

// NewRpcRegistry returns an empty registry.
func NewRpcRegistry() *RpcRegistry {
	return &RpcRegistry{
		byID:        make(map[RpcTypeID]*RpcDefinition),
		byType:      make(map[reflect.Type]*RpcDefinition),
		dispatchers: make(map[RpcTypeID]func(any)),
	}
}

// RegisterRpc registers T as an RPC message type on reg and w. It:
//   - assigns the next sequential TypeID;
//   - installs an EventBus[T] on w via event.RegisterEvent[T];
//   - stores a dispatch closure that calls bus.Send(payload.(T)) on receipt.
//
// Returns an error if T has already been registered.
func RegisterRpc[T any](reg *RpcRegistry, w *world.World, dir RpcDirection, ch netcore.ChannelID) (*RpcDefinition, error) {
	t := reflect.TypeFor[T]()
	if _, exists := reg.byType[t]; exists {
		return nil, fmt.Errorf("rpc: type %s already registered", t.Name())
	}
	reg.nextID++
	def := &RpcDefinition{
		TypeID:    reg.nextID,
		Direction: dir,
		Channel:   ch,
		GoType:    t,
	}
	reg.byID[def.TypeID] = def
	reg.byType[t] = def

	bus := event.RegisterEvent[T](w)
	reg.dispatchers[def.TypeID] = func(payload any) {
		bus.Send(payload.(T))
	}
	return def, nil
}

// ByID looks up a definition by its wire TypeID.
func (r *RpcRegistry) ByID(id RpcTypeID) (*RpcDefinition, bool) {
	def, ok := r.byID[id]
	return def, ok
}

// ByType looks up a definition by Go reflection type.
func (r *RpcRegistry) ByType(t reflect.Type) (*RpcDefinition, bool) {
	def, ok := r.byType[t]
	return def, ok
}

// Len returns the number of registered RPC types.
func (r *RpcRegistry) Len() int { return len(r.byID) }
