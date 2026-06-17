package rpc

import (
	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
)

// RpcPlugin wires the RPC subsystem into an App: it installs the RpcRegistry,
// RpcRateLimit, and RpcReceiveSystem. Add it after NetworkPlugin.
//
//	app.AddPlugin(rpc.RpcPlugin{})
type RpcPlugin struct {
	// Registry is the RpcRegistry to use. If nil, an empty registry is created.
	Registry *RpcRegistry
	// GlobalRateLimit is the token-bucket rate (messages/s) per connection.
	// Zero or negative → default (100 RPC/s per connection).
	GlobalRateLimit float64
}

// Build implements appface.Plugin.
func (p RpcPlugin) Build(app appface.Builder) {
	reg := p.Registry
	if reg == nil {
		reg = NewRpcRegistry()
	}
	rl := NewRpcRateLimit(p.GlobalRateLimit)

	w := app.World()
	world.SetResource(w, reg)
	world.SetResource(w, rl)

	sys := NewRpcReceiveSystem(reg)
	app.AddSystem(appface.PreUpdate, scheduler.NewFuncSystem("rpc.RpcReceive", sys.Run))
}
