package lockstep

import (
	"encoding/binary"

	"github.com/neuengine/neu/internal/ecs/world"
	netcore "github.com/neuengine/neu/internal/net"
)

// latejoinMagic is the 4-byte tag that prefixes a latejoin snapshot packet on
// ChannelSnapshot, distinguishing it from desync checksum packets on ChannelEvents.
var latejoinMagic = [4]byte{'L', 'J', 'S', 'N'}

// LatejoinSystem handles full-state snapshot transfer for peers that connect
// mid-game. Run in PreUpdate.
//
//   - Server path (isServer=true): detects new connections and sends the most
//     recent SnapshotHandle serialised as a latejoin packet on ChannelSnapshot.
//   - Client path (isServer=false): watches InboundQueue for a latejoin packet
//     and, on receipt, restores the World to the snapshot tick.
//
// Late join is the only time full state crosses the wire in a lockstep session;
// all subsequent ticks use only inputs (INV-3).
type LatejoinSystem struct {
	ls         *LockstepScheduler
	isServer   bool
	knownConns map[netcore.ConnectionID]struct{}
}

// NewLatejoinSystem creates a LatejoinSystem. Set isServer=true on the host
// that sends snapshots to joining peers.
func NewLatejoinSystem(ls *LockstepScheduler, isServer bool) *LatejoinSystem {
	return &LatejoinSystem{
		ls:         ls,
		isServer:   isServer,
		knownConns: make(map[netcore.ConnectionID]struct{}),
	}
}

// Run is the ECS system entry point; called once per PreUpdate frame.
func (s *LatejoinSystem) Run(w *world.World) {
	if s.isServer {
		s.runServer(w)
	} else {
		s.runClient(w)
	}
}

// runServer detects connections that appeared since the last frame and sends
// each one the latest snapshot.
func (s *LatejoinSystem) runServer(w *world.World) {
	trP, ok := world.Resource[netcore.TransportResource](w)
	if !ok {
		return
	}
	tr := trP.T

	snapsP, ok := world.Resource[*netcore.SnapshotManager](w)
	if !ok {
		return
	}
	snaps := *snapsP

	conns := tr.Connections()

	for _, conn := range conns {
		if _, known := s.knownConns[conn]; known {
			continue
		}
		s.knownConns[conn] = struct{}{}
		h, ok := snaps.Get(s.ls.currentTick)
		if !ok {
			continue // no snapshot yet; peer will have to wait
		}
		payload := encodeLateJoinPacket(h.Tick, h.Data)
		_ = tr.Send(conn, netcore.ChannelSnapshot, payload)
	}

	// Remove gone connections so new reconnects are treated as new peers.
	live := make(map[netcore.ConnectionID]struct{}, len(conns))
	for _, c := range conns {
		live[c] = struct{}{}
	}
	for c := range s.knownConns {
		if _, alive := live[c]; !alive {
			delete(s.knownConns, c)
		}
	}
}

// runClient watches InboundQueue for a latejoin snapshot and applies it.
func (s *LatejoinSystem) runClient(w *world.World) {
	iqP, ok := world.Resource[netcore.InboundQueue](w)
	if !ok {
		return
	}
	for _, pkt := range iqP.Packets {
		if pkt.Channel != netcore.ChannelSnapshot {
			continue
		}
		tick, data, ok := decodeLateJoinPacket(pkt.Payload)
		if !ok {
			continue
		}
		snapsP, ok2 := world.Resource[*netcore.SnapshotManager](w)
		if !ok2 {
			return
		}
		snaps := *snapsP

		h := netcore.SnapshotHandle{Tick: tick, Data: data}
		_, _ = snaps.RestoreSnapshot(w, h)

		if rcP, ok3 := world.Resource[*netcore.RollbackCoordinator](w); ok3 {
			(*rcP).SetTick(tick)
		}
		s.ls.currentTick = tick
		return // consume first latejoin packet only
	}
}

// encodeLateJoinPacket packs [magic(4)][tick(8 LE)][snapshotData...].
func encodeLateJoinPacket(tick uint64, data []byte) []byte {
	out := make([]byte, 4+8+len(data))
	copy(out[0:4], latejoinMagic[:])
	binary.LittleEndian.PutUint64(out[4:12], tick)
	copy(out[12:], data)
	return out
}

// decodeLateJoinPacket reverses encodeLateJoinPacket.
// Returns false if the payload is too short or carries the wrong magic.
func decodeLateJoinPacket(payload []byte) (tick uint64, data []byte, ok bool) {
	if len(payload) < 12 {
		return 0, nil, false
	}
	var magic [4]byte
	copy(magic[:], payload[0:4])
	if magic != latejoinMagic {
		return 0, nil, false
	}
	tick = binary.LittleEndian.Uint64(payload[4:12])
	return tick, payload[12:], true
}
