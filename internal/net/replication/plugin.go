package replication

import (
	"github.com/neuengine/neu/internal/ecs/command"
	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
)

// ReplicationPlugin wires the replication subsystem into an App: it installs
// the ReplicationConfig resource, registers the EntityMap, and adds the send
// (PostUpdate) and receive (PreUpdate) systems.
//
// Add this plugin after NetworkPlugin so the OutboundQueue and InboundQueue
// resources are already present.
//
//	app.AddPlugin(replication.ReplicationPlugin{})
type ReplicationPlugin struct {
	// Policy is the default VisibilityPolicy applied to all connections.
	// Defaults to ReplicateAll{} if nil.
	Policy VisibilityPolicy
	// BandwidthBudget is the per-frame replication byte budget per connection.
	// Defaults to DefaultBandwidthBudget when <= 0.
	BandwidthBudget int
}

// Build implements appface.Plugin.
func (p ReplicationPlugin) Build(app appface.Builder) {
	w := app.World()

	// Register the replication whitelist config resource.
	world.SetResource(w, ReplicationConfig{})

	// EntityMap backed by the world's own entity allocator.
	em := NewEntityMap(func() entity.EntityID {
		return w.Entities().Allocate().ID()
	})

	// CommandBuffer for receive-side deferred mutations; registered so
	// w.ApplyDeferred() flushes it at the standard apply point.
	cmd := command.NewCommandBuffer(w.Entities(), 64)
	cmd.RegisterWith(w)

	policy := p.Policy
	if policy == nil {
		policy = ReplicateAll{}
	}
	budget := p.BandwidthBudget
	if budget <= 0 {
		budget = DefaultBandwidthBudget
	}

	send := NewReplicationSendSystem(policy, budget)
	recv := NewReplicationReceiveSystem(em, cmd)

	app.AddSystem(appface.PostUpdate, scheduler.NewFuncSystem("replication.Send", send.Run))
	app.AddSystem(appface.PreUpdate, scheduler.NewFuncSystem("replication.Receive", recv.Run))
}
