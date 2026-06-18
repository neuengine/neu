package predict

import (
	"encoding/binary"

	"github.com/neuengine/neu/internal/ecs/world"
	netcore "github.com/neuengine/neu/internal/net"
	"github.com/neuengine/neu/pkg/scene"
)

// PredictionSystem runs in FixedUpdate. Each tick it:
//  1. Captures local input via CaptureInput (plugged in by game code).
//  2. Records it in InputBuffer and enqueues it for reliable send to the server.
//  3. Runs one deterministic tick via RollbackCoordinator (INV-2).
//  4. Records the snapshot checksum in PredictionHistory for later reconciliation.
//
// Requires NetworkPlugin resources: *RollbackCoordinator, *SnapshotManager,
// *InputBuffer, OutboundQueue.
type PredictionSystem struct {
	// CaptureInput is called each tick to obtain the player's serialized input.
	// Returning nil means no input this tick (zero-data input is recorded).
	// Game code must set this before the system runs.
	CaptureInput func() []byte

	localPlayer netcore.PlayerID
	serverConn  netcore.ConnectionID
	history     *PredictionHistory
}

// NewPredictionSystem creates a PredictionSystem for localPlayer that sends
// input to serverConn and records predictions in history.
func NewPredictionSystem(localPlayer netcore.PlayerID, serverConn netcore.ConnectionID, history *PredictionHistory) *PredictionSystem {
	return &PredictionSystem{
		localPlayer: localPlayer,
		serverConn:  serverConn,
		history:     history,
	}
}

// Run is the ECS system entry point; called once per FixedUpdate tick.
func (s *PredictionSystem) Run(w *world.World) {
	rcP, ok := world.Resource[*netcore.RollbackCoordinator](w)
	if !ok {
		return
	}
	rc := *rcP

	snapsP, ok := world.Resource[*netcore.SnapshotManager](w)
	if !ok {
		return
	}
	snaps := *snapsP

	bufP, ok := world.Resource[*netcore.InputBuffer](w)
	if !ok {
		return
	}
	buf := *bufP

	nextTick := rc.Current() + 1

	// Capture local input and record it for replay during rollback.
	var inputData []byte
	if s.CaptureInput != nil {
		inputData = s.CaptureInput()
	}
	buf.RecordInput(nextTick, s.localPlayer, inputData)

	// Enqueue the input for reliable transmission to the server.
	if oq, ok2 := world.Resource[netcore.OutboundQueue](w); ok2 {
		payload := encodeInputPacket(s.localPlayer, nextTick, inputData)
		oq.Messages = append(oq.Messages, netcore.OutboundMessage{
			Target:  s.serverConn,
			Channel: netcore.ChannelEvents,
			Payload: payload,
		})
	}

	// Advance the deterministic simulation one tick and take a snapshot.
	reader := scene.NewWorldAdapter(w)
	rc.AdvanceTick(w, reader, nextTick)

	// Store the snapshot checksum in the prediction history for reconciliation.
	if h, hok := snaps.Get(nextTick); hok {
		s.history.RecordTick(nextTick, h.Checksum)
	}
}

// History returns the prediction history backing this system (for use by the
// ReconciliationSystem in T-7F02).
func (s *PredictionSystem) History() *PredictionHistory { return s.history }

// encodeInputPacket packs player ID, tick, and raw input bytes into a
// compact little-endian payload: [4 bytes playerID][8 bytes tick][data...].
func encodeInputPacket(player netcore.PlayerID, tick uint64, data []byte) []byte {
	buf := make([]byte, 4+8+len(data))
	binary.LittleEndian.PutUint32(buf[0:], uint32(player))
	binary.LittleEndian.PutUint64(buf[4:], tick)
	copy(buf[12:], data)
	return buf
}

// DecodeInputPacket reverses encodeInputPacket.
// Returns false if the payload is shorter than the 12-byte header.
func DecodeInputPacket(payload []byte) (player netcore.PlayerID, tick uint64, data []byte, ok bool) {
	if len(payload) < 12 {
		return 0, 0, nil, false
	}
	player = netcore.PlayerID(binary.LittleEndian.Uint32(payload[0:]))
	tick = binary.LittleEndian.Uint64(payload[4:])
	data = payload[12:]
	return player, tick, data, true
}
