package net

import (
	"encoding/binary"
	"hash/crc32"
)

// PlayerID identifies a player whose input is buffered and replayed. It is
// distinct from ConnectionID (a player may map to a connection, but the
// simulation reasons about players).
type PlayerID uint32

// inputRing is the fixed depth of the per-tick input ring (~2s at 60 Hz). A tick
// maps to a slot via tick % inputRing.
const inputRing = 128

// SerializedInput is one player's input for one tick, in a platform-independent
// binary form so replaying it reproduces identical simulation results
// (l1-networking-system INV-4). Data is opaque to the buffer; encoders pack
// buttons as bitfields and axes as fixed-point (see EncodeInput).
type SerializedInput struct {
	Data     []byte
	Tick     uint64
	Player   PlayerID
	Checksum uint16 // crc32-derived integrity check over Data
}

// InputBuffer stores recent per-player inputs in a ring for rollback replay and
// lockstep. It is a plain resource (single-threaded ECS access); the slot for a
// tick is reused once the ring wraps, bounding memory.
type InputBuffer struct {
	ring [inputRing]map[PlayerID]SerializedInput
}

// NewInputBuffer returns an empty buffer with its ring slots initialised.
func NewInputBuffer() *InputBuffer {
	b := &InputBuffer{}
	for i := range b.ring {
		b.ring[i] = make(map[PlayerID]SerializedInput)
	}
	return b
}

// slot returns the ring index for a tick.
func (b *InputBuffer) slot(tick uint64) int { return int(tick % inputRing) }

// RecordInput stores a player's input for a tick. A stale slot (from a previous
// ring wrap at a different tick) is cleared before the first write for the new
// tick, so a lookup never returns input from an aliased older tick.
func (b *InputBuffer) RecordInput(tick uint64, player PlayerID, data []byte) {
	m := b.ring[b.slot(tick)]
	// Drop entries belonging to a different (aliased) tick.
	for p, in := range m {
		if in.Tick != tick {
			delete(m, p)
		}
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	m[player] = SerializedInput{
		Tick:     tick,
		Player:   player,
		Data:     cp,
		Checksum: uint16(crc32.ChecksumIEEE(cp)),
	}
}

// GetInput returns the recorded input for (tick, player) and whether it exists
// (and belongs to that exact tick, not an aliased ring slot).
func (b *InputBuffer) GetInput(tick uint64, player PlayerID) (SerializedInput, bool) {
	in, ok := b.ring[b.slot(tick)][player]
	if !ok || in.Tick != tick {
		return SerializedInput{}, false
	}
	return in, true
}

// PredictInput returns the input to use when the real input for (tick, player)
// has not arrived: the player's most recent earlier input (repeat-last), tagged
// for the requested tick. Returns a zero-Data input if nothing is known. Netcode
// plugins may override with a smarter predictor.
func (b *InputBuffer) PredictInput(tick uint64, player PlayerID) SerializedInput {
	if in, ok := b.GetInput(tick, player); ok {
		return in
	}
	// Scan backwards up to inputRing ticks for the latest known input.
	for back := uint64(1); back <= inputRing && back <= tick; back++ {
		if in, ok := b.GetInput(tick-back, player); ok {
			in.Tick = tick
			return in
		}
	}
	return SerializedInput{Tick: tick, Player: player}
}

// HasAllPeers reports whether every peer has submitted input for the tick — the
// lockstep advance gate (l1-lockstep INV-1).
func (b *InputBuffer) HasAllPeers(tick uint64, peers []PlayerID) bool {
	m := b.ring[b.slot(tick)]
	for _, p := range peers {
		if in, ok := m[p]; !ok || in.Tick != tick {
			return false
		}
	}
	return true
}

// InputState is a representative deterministic input payload: a button bitfield
// plus fixed-point axes. Games define their own; this one demonstrates the
// platform-stable encoding and serves the input-buffer tests.
type InputState struct {
	Buttons uint32   // bitfield
	Axes    [4]int16 // fixed-point (e.g. value * 256), platform-stable
}

// EncodeInput packs an InputState into a platform-independent little-endian
// byte slice (INV-4 determinism: no float, fixed layout).
func EncodeInput(s InputState) []byte {
	buf := make([]byte, 4+4*2)
	binary.LittleEndian.PutUint32(buf[0:], s.Buttons)
	for i, a := range s.Axes {
		binary.LittleEndian.PutUint16(buf[4+i*2:], uint16(a))
	}
	return buf
}

// DecodeInput reverses EncodeInput. Returns false if the buffer is too short.
func DecodeInput(data []byte) (InputState, bool) {
	if len(data) < 4+4*2 {
		return InputState{}, false
	}
	var s InputState
	s.Buttons = binary.LittleEndian.Uint32(data[0:])
	for i := range s.Axes {
		s.Axes[i] = int16(binary.LittleEndian.Uint16(data[4+i*2:]))
	}
	return s, true
}
