package net

import (
	"bytes"
	"testing"
)

func TestInputBufferRecordGet(t *testing.T) {
	t.Parallel()
	b := NewInputBuffer()
	b.RecordInput(10, 1, []byte("up"))
	b.RecordInput(10, 2, []byte("down"))

	in, ok := b.GetInput(10, 1)
	if !ok || string(in.Data) != "up" || in.Tick != 10 || in.Player != 1 {
		t.Errorf("GetInput(10,1) = %+v ok=%v", in, ok)
	}
	if in.Checksum == 0 {
		t.Error("checksum should be set")
	}
	// Distinct player on same tick.
	if in2, ok := b.GetInput(10, 2); !ok || string(in2.Data) != "down" {
		t.Errorf("GetInput(10,2) = %+v ok=%v", in2, ok)
	}
	// Missing player / tick.
	if _, ok := b.GetInput(10, 3); ok {
		t.Error("GetInput for unrecorded player should be false")
	}
	if _, ok := b.GetInput(11, 1); ok {
		t.Error("GetInput for unrecorded tick should be false")
	}
}

func TestInputBufferRecordCopiesData(t *testing.T) {
	t.Parallel()
	b := NewInputBuffer()
	data := []byte("x")
	b.RecordInput(5, 1, data)
	data[0] = 'y' // mutate caller's slice
	if in, _ := b.GetInput(5, 1); string(in.Data) != "x" {
		t.Errorf("buffer should hold a copy, got %q", in.Data)
	}
}

func TestInputBufferRingAliasingCleared(t *testing.T) {
	t.Parallel()
	b := NewInputBuffer()
	// tick 1 and tick 1+inputRing share a slot; the old entry must not leak.
	b.RecordInput(1, 1, []byte("old"))
	b.RecordInput(1+inputRing, 1, []byte("new"))
	if _, ok := b.GetInput(1, 1); ok {
		t.Error("aliased old tick should no longer resolve")
	}
	if in, ok := b.GetInput(1+inputRing, 1); !ok || string(in.Data) != "new" {
		t.Errorf("new tick in shared slot = %+v ok=%v", in, ok)
	}
}

func TestInputBufferPredictRepeatLast(t *testing.T) {
	t.Parallel()
	b := NewInputBuffer()
	b.RecordInput(20, 1, []byte("hold"))
	// No input for tick 23 → predict the last known (tick 20), retagged.
	p := b.PredictInput(23, 1)
	if string(p.Data) != "hold" || p.Tick != 23 || p.Player != 1 {
		t.Errorf("PredictInput(23,1) = %+v, want last input retagged to tick 23", p)
	}
	// Exact match returns the real input.
	if got := b.PredictInput(20, 1); string(got.Data) != "hold" || got.Tick != 20 {
		t.Errorf("PredictInput on a known tick = %+v", got)
	}
	// Nothing known → zero-Data input tagged for the tick.
	empty := b.PredictInput(50, 9)
	if len(empty.Data) != 0 || empty.Tick != 50 {
		t.Errorf("PredictInput with no history = %+v, want zero-data tick 50", empty)
	}
}

func TestInputBufferHasAllPeers(t *testing.T) {
	t.Parallel()
	b := NewInputBuffer()
	peers := []PlayerID{1, 2, 3}
	b.RecordInput(7, 1, []byte("a"))
	b.RecordInput(7, 2, []byte("b"))
	if b.HasAllPeers(7, peers) {
		t.Error("not all peers submitted yet")
	}
	b.RecordInput(7, 3, []byte("c"))
	if !b.HasAllPeers(7, peers) {
		t.Error("all peers submitted → gate should open (lockstep INV-1)")
	}
}

func TestEncodeDecodeInputRoundTrip(t *testing.T) {
	t.Parallel()
	s := InputState{Buttons: 0b1011, Axes: [4]int16{256, -128, 0, 32767}}
	enc := EncodeInput(s)
	got, ok := DecodeInput(enc)
	if !ok || got != s {
		t.Errorf("round-trip = %+v ok=%v, want %+v", got, ok, s)
	}
	// Determinism: identical input encodes to identical bytes (INV-4).
	if !bytes.Equal(EncodeInput(s), enc) {
		t.Error("EncodeInput is not deterministic")
	}
	// Too-short buffer is rejected, not a panic.
	if _, ok := DecodeInput([]byte{1, 2}); ok {
		t.Error("short buffer should fail to decode")
	}
}
