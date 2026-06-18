package lockstep

import (
	"github.com/neuengine/neu/internal/ecs/world"
	netcore "github.com/neuengine/neu/internal/net"
)

// DesyncReceiveSystem runs in PostUpdate. It reads every ChannelEvents packet
// from InboundQueue, identifies checksum packets via DecodeChecksumPacket, and
// forwards them to LockstepScheduler.ReceiveChecksum for desync detection
// (INV-4). On a mismatch the scheduler halts and fires its OnDesync callback.
type DesyncReceiveSystem struct {
	ls *LockstepScheduler
}

// NewDesyncReceiveSystem creates a system that routes incoming checksum packets
// to ls.
func NewDesyncReceiveSystem(ls *LockstepScheduler) *DesyncReceiveSystem {
	return &DesyncReceiveSystem{ls: ls}
}

// Run is the ECS system entry point; called once per PostUpdate frame.
func (s *DesyncReceiveSystem) Run(w *world.World) {
	iqP, ok := world.Resource[netcore.InboundQueue](w)
	if !ok {
		return
	}
	for _, pkt := range iqP.Packets {
		if pkt.Channel != netcore.ChannelEvents {
			continue
		}
		tick, checksum, ok := DecodeChecksumPacket(pkt.Payload)
		if !ok {
			continue
		}
		s.ls.ReceiveChecksum(w, netcore.PlayerID(pkt.Connection), tick, checksum)
	}
}
