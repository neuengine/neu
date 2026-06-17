package rpc

import (
	"encoding/json"
	"fmt"
	"reflect"

	netcore "github.com/neuengine/neu/internal/net"
)

// RpcTarget specifies the destination of an RPC call.
// Implement by selecting one of the concrete target types below.
type RpcTarget interface{ rpcTarget() }

// TargetServer directs an RPC from a client to the server.
// Implemented as a broadcast because the client typically has one connection.
type TargetServer struct{}

// TargetClient directs an RPC from the server to a specific client connection.
type TargetClient netcore.ConnectionID

// TargetAllClients broadcasts an RPC from the server to all clients.
type TargetAllClients struct{}

// TargetGroup directs an RPC from the server to a named set of connections.
type TargetGroup struct{ IDs []netcore.ConnectionID }

// TargetExcept broadcasts an RPC from the server to all clients except one.
type TargetExcept netcore.ConnectionID

func (TargetServer) rpcTarget()     {}
func (TargetClient) rpcTarget()     {}
func (TargetAllClients) rpcTarget() {}
func (TargetGroup) rpcTarget()      {}
func (TargetExcept) rpcTarget()     {}

// RpcSender dispatches outgoing RPC calls over a NetworkTransport.
type RpcSender struct {
	reg *RpcRegistry
	tr  netcore.NetworkTransport
}

// NewRpcSender creates a sender backed by the given registry and transport.
func NewRpcSender(reg *RpcRegistry, tr netcore.NetworkTransport) *RpcSender {
	return &RpcSender{reg: reg, tr: tr}
}

// Send serializes payload as JSON, wraps it in the RPC wire format, and
// dispatches it to the given target. Returns an error if the type is not
// registered, serialization fails, or the transport reports an error.
func Send[T any](s *RpcSender, target RpcTarget, payload T) error {
	t := reflect.TypeFor[T]()
	def, ok := s.reg.byType[t]
	if !ok {
		return fmt.Errorf("rpc: type %s not registered", t.Name())
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("rpc: marshal %s: %w", t.Name(), err)
	}
	msg := EncodeRpcMessage(def.TypeID, data)
	return dispatchTarget(s.tr, def.Channel, msg, target)
}

func dispatchTarget(tr netcore.NetworkTransport, ch netcore.ChannelID, msg []byte, target RpcTarget) error {
	switch t := target.(type) {
	case TargetServer:
		tr.Broadcast(ch, msg)
	case TargetClient:
		return tr.Send(netcore.ConnectionID(t), ch, msg)
	case TargetAllClients:
		tr.Broadcast(ch, msg)
	case TargetGroup:
		for _, id := range t.IDs {
			if err := tr.Send(id, ch, msg); err != nil {
				return fmt.Errorf("rpc: send to connection %d: %w", id, err)
			}
		}
	case TargetExcept:
		for _, id := range tr.Connections() {
			if id != netcore.ConnectionID(t) {
				_ = tr.Send(id, ch, msg)
			}
		}
	default:
		return fmt.Errorf("rpc: unknown RpcTarget type %T", target)
	}
	return nil
}
