package lockstep

import (
	"encoding/binary"

	"github.com/neuengine/neu/internal/ecs/world"
	netcore "github.com/neuengine/neu/internal/net"
)

// LocalInputSystem runs in FixedUpdate. It:
//  1. Calls CaptureInput to obtain the local player's serialized input.
//  2. Tags it with currentTick+InputDelay (the target execution tick).
//  3. Records it in InputBuffer and broadcasts it to all peers on ChannelEvents (INV-3).
//
// Requires NetworkPlugin resources: *InputBuffer, *RollbackCoordinator, OutboundQueue.
type LocalInputSystem struct {
	// CaptureInput is called each tick; return nil for no-input ticks.
	CaptureInput func() []byte

	localPlayer netcore.PlayerID
	inputDelay  uint8
}

// NewLocalInputSystem creates a system that tags local player input with
// inputDelay and records/broadcasts it.
func NewLocalInputSystem(localPlayer netcore.PlayerID, inputDelay uint8) *LocalInputSystem {
	if inputDelay == 0 {
		inputDelay = DefaultInputDelay
	}
	return &LocalInputSystem{localPlayer: localPlayer, inputDelay: inputDelay}
}

// Run is the ECS system entry point.
func (s *LocalInputSystem) Run(w *world.World) {
	rcP, ok := world.Resource[*netcore.RollbackCoordinator](w)
	if !ok {
		return
	}
	rc := *rcP

	bufP, ok := world.Resource[*netcore.InputBuffer](w)
	if !ok {
		return
	}
	buf := *bufP

	targetTick := rc.Current() + uint64(s.inputDelay)

	var inputData []byte
	if s.CaptureInput != nil {
		inputData = s.CaptureInput()
	}

	buf.RecordInput(targetTick, s.localPlayer, inputData)

	// Broadcast to all peers (INV-3 — only inputs cross the wire in steady state).
	if oq, ok2 := world.Resource[netcore.OutboundQueue](w); ok2 {
		payload := encodeInputPacket(s.localPlayer, targetTick, inputData)
		oq.Messages = append(oq.Messages, netcore.OutboundMessage{
			Channel: netcore.ChannelEvents,
			Payload: payload, // Target==0 → broadcast
		})
	}
}

// RemoteInputSystem runs in PostUpdate. It reads all ChannelEvents packets from
// InboundQueue and records any serialized input into InputBuffer so the lockstep
// gate (LockstepScheduler.Step) can see remote peers' inputs.
type RemoteInputSystem struct{}

// Run is the ECS system entry point.
func (s *RemoteInputSystem) Run(w *world.World) {
	iq, ok := world.Resource[netcore.InboundQueue](w)
	if !ok {
		return
	}
	bufP, ok := world.Resource[*netcore.InputBuffer](w)
	if !ok {
		return
	}
	buf := *bufP

	for _, pkt := range iq.Packets {
		if pkt.Channel != netcore.ChannelEvents {
			continue
		}
		player, tick, data, valid := decodeInputPacket(pkt.Payload)
		if !valid {
			continue
		}
		buf.RecordInput(tick, player, data)
	}
}

// encodeInputPacket packs player, tick, and input bytes into:
// [4 bytes playerID | 8 bytes tick | data...]
func encodeInputPacket(player netcore.PlayerID, tick uint64, data []byte) []byte {
	buf := make([]byte, 4+8+len(data))
	binary.LittleEndian.PutUint32(buf[0:], uint32(player))
	binary.LittleEndian.PutUint64(buf[4:], tick)
	copy(buf[12:], data)
	return buf
}

// decodeInputPacket reverses encodeInputPacket.
func decodeInputPacket(payload []byte) (player netcore.PlayerID, tick uint64, data []byte, ok bool) {
	if len(payload) < 12 {
		return 0, 0, nil, false
	}
	player = netcore.PlayerID(binary.LittleEndian.Uint32(payload[0:]))
	tick = binary.LittleEndian.Uint64(payload[4:])
	data = payload[12:]
	return player, tick, data, true
}

// encodeChecksumPacket packs tick and crc32 into:
// [8 bytes tick | 4 bytes checksum]
func encodeChecksumPacket(tick uint64, checksum uint32) []byte {
	buf := make([]byte, 8+4)
	binary.LittleEndian.PutUint64(buf[0:], tick)
	binary.LittleEndian.PutUint32(buf[8:], checksum)
	return buf
}

// DecodeChecksumPacket reverses encodeChecksumPacket. Returns false if the
// payload is shorter than 12 bytes.
func DecodeChecksumPacket(payload []byte) (tick uint64, checksum uint32, ok bool) {
	if len(payload) < 12 {
		return 0, 0, false
	}
	tick = binary.LittleEndian.Uint64(payload[0:])
	checksum = binary.LittleEndian.Uint32(payload[8:])
	return tick, checksum, true
}
