package interp

import (
	"encoding/json"
	"time"

	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	netcore "github.com/neuengine/neu/internal/net"
	"github.com/neuengine/neu/internal/net/replication"
	"github.com/neuengine/neu/pkg/app/appface"
)

// InterpolationPlugin wires the snapshot interpolation subsystem into an App.
// It registers DisplayState, creates a SnapshotBuffer, and adds two systems:
//   - interp.Ingest (PostUpdate): reads InboundQueue ChannelSnapshot packets,
//     builds SnapshotEntry objects, and adjusts the adaptive render delay.
//   - interp.Interpolate (Last): reads the buffer and writes DisplayState.
//
// Add after NetworkPlugin and ReplicationPlugin so InboundQueue is available.
type InterpolationPlugin struct {
	// Config holds static parameters; zero values are replaced with defaults.
	Config InterpolationConfig
	// LookupClient translates server EntityIDs to client EntityIDs.
	// If nil, server IDs are used directly as client IDs.
	LookupClient func(entity.EntityID) (entity.EntityID, bool)
}

// Build implements appface.Plugin.
func (p InterpolationPlugin) Build(app appface.Builder) {
	cfg := p.Config
	if cfg.RenderDelay <= 0 {
		cfg.RenderDelay = 100 * time.Millisecond
	}
	if cfg.BufferCapacity <= 0 {
		cfg.BufferCapacity = DefaultBufferCapacity
	}
	if cfg.TargetFill <= 0 {
		cfg.TargetFill = 3
	}
	if cfg.MaxExtrapolation <= 0 {
		cfg.MaxExtrapolation = DefaultMaxExtrapolation
	}

	w := app.World()
	world.RegisterComponent[DisplayState](w)

	buf := NewSnapshotBuffer(cfg.BufferCapacity)
	adaptive := NewAdaptiveDelay(cfg)
	interpSys := NewInterpolateSystem(buf, cfg.RenderDelay, cfg.Extrapolate, cfg.MaxExtrapolation, p.LookupClient)
	ingestSys := &snapshotIngestSystem{buf: buf, adaptive: adaptive, interp: interpSys, start: time.Now()}

	world.SetResource(w, cfg)

	app.
		AddSystem(appface.PostUpdate, scheduler.NewFuncSystem("interp.Ingest", ingestSys.Run)).
		AddSystem(appface.Last, scheduler.NewFuncSystem("interp.Interpolate", interpSys.Run))
}

// snapshotIngestSystem reads ChannelSnapshot packets from InboundQueue, builds
// SnapshotEntry objects containing per-entity component JSON, and inserts them
// into the SnapshotBuffer. It also drives AdaptiveDelay each frame.
type snapshotIngestSystem struct {
	buf      *SnapshotBuffer
	adaptive *AdaptiveDelay
	interp   *InterpolateSystem
	start    time.Time
	nextTick uint64
}

// Run is the ECS system entry point; called once per frame in PostUpdate.
func (s *snapshotIngestSystem) Run(w *world.World) {
	qp, ok := world.Resource[netcore.InboundQueue](w)
	if !ok {
		return
	}
	now := time.Since(s.start)

	for _, pkt := range qp.Packets {
		if pkt.Channel != netcore.ChannelSnapshot {
			continue
		}
		s.ingestPacket(pkt.Payload, now)
	}

	s.adaptive.Adjust(s.buf)
	s.interp.SetRenderDelay(s.adaptive.RenderDelay)
}

// ingestPacket parses one raw ChannelSnapshot payload and, if it contains any
// entity state, inserts a new SnapshotEntry into the buffer.
func (s *snapshotIngestSystem) ingestPacket(payload []byte, now time.Duration) {
	entry := SnapshotEntry{
		Timestamp:  now,
		ReceivedAt: now,
		Entities:   make(map[entity.EntityID]EntityState),
	}
	data := payload
	for len(data) > 0 {
		msg, rest, ok := replication.DecodeReplicationMessage(data)
		if !ok {
			break
		}
		data = rest
		if msg.Kind != replication.MsgEntitySpawn && msg.Kind != replication.MsgComponentUpdate {
			continue
		}
		state := entry.Entities[msg.ServerID]
		if state == nil {
			state = make(EntityState, len(msg.Components))
			entry.Entities[msg.ServerID] = state
		}
		for _, rc := range msg.Components {
			state[rc.TypeName] = json.RawMessage(rc.Data)
		}
	}
	if len(entry.Entities) == 0 {
		return
	}
	s.nextTick++
	entry.Tick = s.nextTick
	s.buf.Insert(entry)
}
