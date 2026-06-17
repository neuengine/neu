package replication

import (
	"encoding/json"
	"reflect"

	"github.com/neuengine/neu/internal/ecs/changedetect"
	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/world"
	netcore "github.com/neuengine/neu/internal/net"
)

// connSendState is the per-connection mutable state maintained between frames.
type connSendState struct {
	vis    VisibilitySet
	ack    *ClientAckState
	prio   *PriorityStore
	policy VisibilityPolicy // overrides the system-level default when non-nil
}

// ReplicationSendSystem runs in PostUpdate and serializes ECS state changes into
// replication messages queued on OutboundQueue. It is strictly read-only with
// respect to the World (INV-5).
type ReplicationSendSystem struct {
	connections     map[netcore.ConnectionID]*connSendState
	defaultPolicy   VisibilityPolicy
	bandwidthBudget int
}

// NewReplicationSendSystem creates a send system with the given default visibility
// policy and per-frame byte budget. Nil policy defaults to ReplicateAll.
// Budget <= 0 defaults to DefaultBandwidthBudget.
func NewReplicationSendSystem(policy VisibilityPolicy, budget int) *ReplicationSendSystem {
	if policy == nil {
		policy = ReplicateAll{}
	}
	if budget <= 0 {
		budget = DefaultBandwidthBudget
	}
	return &ReplicationSendSystem{
		connections:     make(map[netcore.ConnectionID]*connSendState),
		defaultPolicy:   policy,
		bandwidthBudget: budget,
	}
}

// RegisterPolicy installs a custom VisibilityPolicy for one connection,
// overriding the system-level default for that connection only.
func (sys *ReplicationSendSystem) RegisterPolicy(id netcore.ConnectionID, p VisibilityPolicy) {
	sys.ensureConn(id).policy = p
}

// Run is the ECS system entry point; called once per frame in PostUpdate.
func (sys *ReplicationSendSystem) Run(w *world.World) {
	cfgp, ok := world.Resource[ReplicationConfig](w)
	if !ok {
		return
	}

	qp, ok := world.Resource[netcore.OutboundQueue](w)
	if !ok {
		return
	}

	trp, hasTR := world.Resource[netcore.TransportResource](w)
	var connIDs []netcore.ConnectionID
	if hasTR && trp != nil && trp.T != nil {
		connIDs = trp.T.Connections()
	}
	if len(connIDs) == 0 {
		return
	}

	reg := w.Components()
	replicatedID, hasReplicated := reg.Lookup(reflect.TypeFor[Replicated]())
	if !hasReplicated {
		return
	}

	// Collect all replicable entities and their archetype+row locations.
	type entityLoc struct {
		arch *world.Archetype
		row  int
	}
	entityLocs := make(map[entity.EntityID]entityLoc)
	w.Archetypes().Each(func(arch *world.Archetype) bool {
		if !arch.Has(replicatedID) {
			return true
		}
		for row, e := range arch.Entities() {
			entityLocs[e.ID()] = entityLoc{arch, row}
		}
		return true
	})

	allEntityIDs := make([]entity.EntityID, 0, len(entityLocs))
	for id := range entityLocs {
		allEntityIDs = append(allEntityIDs, id)
	}

	tick := changedetect.Tick(w.ChangeTick())
	activeSet := make(map[netcore.ConnectionID]struct{}, len(connIDs))

	for _, connID := range connIDs {
		activeSet[connID] = struct{}{}
		cs := sys.ensureConn(connID)
		policy := cs.policy
		if policy == nil {
			policy = sys.defaultPolicy
		}

		newVis := policy.Compute(allEntityIDs, cs.vis)

		// EntitySpawn for newly visible entities (full snapshot).
		addedSet := make(map[entity.EntityID]struct{}, len(newVis.Added))
		for _, eid := range newVis.Added {
			addedSet[eid] = struct{}{}
			loc, lok := entityLocs[eid]
			if !lok {
				continue
			}
			comps := collectComponents(reg, loc.arch, loc.row, cfgp)
			if len(comps) == 0 {
				continue
			}
			msg := ReplicationMessage{Kind: MsgEntitySpawn, ServerID: eid, Components: comps}
			qp.Messages = append(qp.Messages, netcore.OutboundMessage{
				Target:  connID,
				Channel: netcore.ChannelSnapshot,
				Payload: EncodeReplicationMessage(msg),
			})
			cs.ack.UpdateAck(eid, tick)
		}

		// EntityDespawn for entities that left visibility.
		for _, eid := range newVis.Removed {
			msg := ReplicationMessage{Kind: MsgEntityDespawn, ServerID: eid}
			qp.Messages = append(qp.Messages, netcore.OutboundMessage{
				Target:  connID,
				Channel: netcore.ChannelSnapshot,
				Payload: EncodeReplicationMessage(msg),
			})
			cs.ack.Forget(eid)
			cs.prio.forget(eid)
		}

		// Collect changed component updates for entities still in visibility.
		var pending []pendingUpdate
		for eid := range newVis.Entities {
			if _, justSpawned := addedSet[eid]; justSpawned {
				continue // already sent a full spawn; no delta needed this frame
			}
			loc, lok := entityLocs[eid]
			if !lok {
				continue
			}
			comps := collectChangedComponents(reg, loc.arch, loc.row, cfgp, cs.ack, eid)
			if len(comps) == 0 {
				continue
			}
			msg := ReplicationMessage{Kind: MsgComponentUpdate, ServerID: eid, Components: comps}
			ps := cs.prio.get(eid)
			pending = append(pending, pendingUpdate{
				entity:   eid,
				priority: ps.effective(),
				msgs:     EncodeReplicationMessage(msg),
			})
		}

		// Sort by priority and serialize within the bandwidth budget.
		sortByPriority(pending)
		remaining := sys.bandwidthBudget
		for _, upd := range pending {
			ps := cs.prio.get(upd.entity)
			if remaining >= len(upd.msgs) {
				qp.Messages = append(qp.Messages, netcore.OutboundMessage{
					Target:  connID,
					Channel: netcore.ChannelSnapshot,
					Payload: upd.msgs,
				})
				remaining -= len(upd.msgs)
				ps.onSent()
				cs.ack.UpdateAck(upd.entity, tick)
			} else {
				ps.onDeferred()
			}
		}

		cs.vis = newVis
	}

	// Remove state for connections that are no longer active.
	for id := range sys.connections {
		if _, alive := activeSet[id]; !alive {
			delete(sys.connections, id)
		}
	}
}

func (sys *ReplicationSendSystem) ensureConn(id netcore.ConnectionID) *connSendState {
	if cs, ok := sys.connections[id]; ok {
		return cs
	}
	cs := &connSendState{
		vis:  NewVisibilitySet(),
		ack:  NewClientAckState(),
		prio: NewPriorityStore(),
	}
	sys.connections[id] = cs
	return cs
}

// collectComponents returns all whitelisted, non-zero-size components for the entity at (arch, row).
// Components are included only if they appear in cfg with a replicable rule.
func collectComponents(reg *component.Registry, arch *world.Archetype, row int, cfg *ReplicationConfig) []ReplicatedComponent {
	tbl := arch.Table()
	if tbl == nil {
		return nil
	}
	values := tbl.RowValues(row)
	comps := make([]ReplicatedComponent, 0, len(values))
	for id, val := range values {
		info := reg.Info(id)
		if info.Size == 0 {
			continue // tag components carry no serializable payload
		}
		rule, inConfig := cfg.Get(id)
		if !inConfig || rule == RuleServerOnly || rule == RuleClientOnly {
			continue
		}
		data, err := json.Marshal(val)
		if err != nil {
			continue
		}
		comps = append(comps, ReplicatedComponent{TypeName: info.Name, Data: data})
	}
	return comps
}

// collectChangedComponents returns whitelisted components whose change tick is
// newer than the client's last-acked tick for that entity.
func collectChangedComponents(reg *component.Registry, arch *world.Archetype, row int, cfg *ReplicationConfig, ack *ClientAckState, eid entity.EntityID) []ReplicatedComponent {
	tbl := arch.Table()
	if tbl == nil {
		return nil
	}
	values := tbl.RowValues(row)
	comps := make([]ReplicatedComponent, 0)
	for id, val := range values {
		info := reg.Info(id)
		if info.Size == 0 {
			continue
		}
		rule, inConfig := cfg.Get(id)
		if !inConfig || rule == RuleServerOnly || rule == RuleClientOnly {
			continue
		}
		ct, hasct := tbl.TicksByID(id, row)
		if !hasct || !ack.NeedsSend(eid, ct.Changed) {
			continue
		}
		data, err := json.Marshal(val)
		if err != nil {
			continue
		}
		comps = append(comps, ReplicatedComponent{TypeName: info.Name, Data: data})
	}
	return comps
}
