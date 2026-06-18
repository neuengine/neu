package predict

import (
	"testing"
	"time"

	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/typereg"
	"github.com/neuengine/neu/internal/ecs/world"
	netcore "github.com/neuengine/neu/internal/net"
)

// ─── PredictionHistory ───────────────────────────────────────────────────────

func TestPredictionHistoryDefaultCapacity(t *testing.T) {
	t.Parallel()
	h := NewPredictionHistory(0)
	if h.capacity != DefaultHistoryCapacity {
		t.Errorf("capacity = %d, want %d", h.capacity, DefaultHistoryCapacity)
	}
}

func TestPredictionHistoryRecordAndGet(t *testing.T) {
	t.Parallel()
	h := NewPredictionHistory(8)
	h.RecordTick(5, 0xDEAD)
	e, ok := h.Get(5)
	if !ok {
		t.Fatal("Get(5): want ok=true")
	}
	if e.Tick != 5 || e.Checksum != 0xDEAD {
		t.Errorf("entry = %+v, want Tick=5 Checksum=0xDEAD", e)
	}
}

func TestPredictionHistoryGetMissing(t *testing.T) {
	t.Parallel()
	h := NewPredictionHistory(8)
	if _, ok := h.Get(99); ok {
		t.Error("Get on empty history should return ok=false")
	}
}

func TestPredictionHistoryRecordUpdatesExisting(t *testing.T) {
	t.Parallel()
	h := NewPredictionHistory(8)
	h.RecordTick(3, 0x1111)
	h.RecordTick(3, 0x2222) // update same tick
	e, ok := h.Get(3)
	if !ok {
		t.Fatal("Get(3) not found")
	}
	if e.Checksum != 0x2222 {
		t.Errorf("checksum = 0x%X, want 0x2222", e.Checksum)
	}
	if h.Len() != 1 {
		t.Errorf("Len = %d, want 1 after updating same tick", h.Len())
	}
}

func TestPredictionHistoryEviction(t *testing.T) {
	t.Parallel()
	h := NewPredictionHistory(3)
	for i := uint64(1); i <= 4; i++ {
		h.RecordTick(i, uint32(i))
	}
	if h.Len() != 3 {
		t.Errorf("Len = %d, want 3 (capacity)", h.Len())
	}
	// Oldest entry (tick 1) should be evicted.
	if _, ok := h.Get(1); ok {
		t.Error("tick 1 should be evicted")
	}
	for _, tick := range []uint64{2, 3, 4} {
		if _, ok := h.Get(tick); !ok {
			t.Errorf("tick %d should be in ring", tick)
		}
	}
}

func TestPredictionHistoryDiscardThrough(t *testing.T) {
	t.Parallel()
	h := NewPredictionHistory(16)
	for i := uint64(1); i <= 5; i++ {
		h.RecordTick(i, 0)
	}
	h.DiscardThrough(3) // remove ticks 1,2,3
	if h.Len() != 2 {
		t.Errorf("Len = %d, want 2", h.Len())
	}
	for _, tick := range []uint64{1, 2, 3} {
		if _, ok := h.Get(tick); ok {
			t.Errorf("tick %d should be discarded", tick)
		}
	}
	for _, tick := range []uint64{4, 5} {
		if _, ok := h.Get(tick); !ok {
			t.Errorf("tick %d should remain", tick)
		}
	}
}

func TestPredictionHistoryDiscardBefore(t *testing.T) {
	t.Parallel()
	h := NewPredictionHistory(16)
	for i := uint64(1); i <= 5; i++ {
		h.RecordTick(i, 0)
	}
	h.DiscardBefore(3) // remove ticks 1,2; keep 3,4,5
	if h.Len() != 3 {
		t.Errorf("Len = %d, want 3", h.Len())
	}
	for _, tick := range []uint64{1, 2} {
		if _, ok := h.Get(tick); ok {
			t.Errorf("tick %d should be discarded", tick)
		}
	}
	for _, tick := range []uint64{3, 4, 5} {
		if _, ok := h.Get(tick); !ok {
			t.Errorf("tick %d should remain", tick)
		}
	}
}

func TestPredictionHistoryRecordFull(t *testing.T) {
	t.Parallel()
	h := NewPredictionHistory(8)
	ents := map[entity.EntityID]PredictedSnapshot{
		entity.EntityID(1): {Checksum: 0xABCD},
	}
	h.RecordFull(7, 0xBEEF, ents)
	e, ok := h.Get(7)
	if !ok {
		t.Fatal("Get(7) not found")
	}
	if e.Checksum != 0xBEEF {
		t.Errorf("checksum = 0x%X, want 0xBEEF", e.Checksum)
	}
	if len(e.Entities) != 1 {
		t.Errorf("entities len = %d, want 1", len(e.Entities))
	}
}

func TestPredictionHistoryRecordFullUpdates(t *testing.T) {
	t.Parallel()
	h := NewPredictionHistory(8)
	h.RecordTick(2, 0x1111)
	// Update with full snapshot.
	h.RecordFull(2, 0x2222, nil)
	e, ok := h.Get(2)
	if !ok {
		t.Fatal("Get(2) not found")
	}
	if e.Checksum != 0x2222 {
		t.Errorf("checksum = 0x%X, want 0x2222", e.Checksum)
	}
	if h.Len() != 1 {
		t.Errorf("Len = %d, want 1", h.Len())
	}
}

// ─── NetworkAuthority ────────────────────────────────────────────────────────

func TestNetworkAuthorityModes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		mode AuthorityMode
		name string
	}{
		{Predicted, "Predicted"},
		{Interpolated, "Interpolated"},
		{Authoritative, "Authoritative"},
	}
	for _, tc := range tests {
		na := NetworkAuthority{Mode: tc.mode}
		if na.Mode != tc.mode {
			t.Errorf("%s: Mode = %d, want %d", tc.name, na.Mode, tc.mode)
		}
	}
}

func TestAuthorityModeOrdering(t *testing.T) {
	t.Parallel()
	if Predicted != 0 {
		t.Errorf("Predicted = %d, want 0", Predicted)
	}
	if Interpolated != 1 {
		t.Errorf("Interpolated = %d, want 1", Interpolated)
	}
	if Authoritative != 2 {
		t.Errorf("Authoritative = %d, want 2", Authoritative)
	}
}

// ─── Input packet encode/decode ──────────────────────────────────────────────

func TestDecodeInputPacketRoundtrip(t *testing.T) {
	t.Parallel()
	payload := encodeInputPacket(7, 42, []byte("wasd"))
	p, tick, data, ok := DecodeInputPacket(payload)
	if !ok {
		t.Fatal("DecodeInputPacket: want ok=true")
	}
	if p != 7 {
		t.Errorf("player = %d, want 7", p)
	}
	if tick != 42 {
		t.Errorf("tick = %d, want 42", tick)
	}
	if string(data) != "wasd" {
		t.Errorf("data = %q, want %q", data, "wasd")
	}
}

func TestDecodeInputPacketShort(t *testing.T) {
	t.Parallel()
	_, _, _, ok := DecodeInputPacket([]byte{0, 1, 2})
	if ok {
		t.Error("DecodeInputPacket with short payload: want ok=false")
	}
}

func TestEncodeInputPacketNoData(t *testing.T) {
	t.Parallel()
	payload := encodeInputPacket(1, 100, nil)
	p, tick, data, ok := DecodeInputPacket(payload)
	if !ok {
		t.Fatal("DecodeInputPacket: want ok=true")
	}
	if p != 1 || tick != 100 {
		t.Errorf("player=%d tick=%d, want 1 100", p, tick)
	}
	if len(data) != 0 {
		t.Errorf("data len = %d, want 0", len(data))
	}
}

// ─── PredictionSystem ────────────────────────────────────────────────────────

func buildPredictWorld(t *testing.T) *world.World {
	t.Helper()
	w := world.NewWorld()
	reg := typereg.NewTypeRegistry()
	snaps := netcore.NewSnapshotManager(reg, 16)
	sched := netcore.NewDeterministicSchedule(0, time.Millisecond)
	rc := netcore.NewRollbackCoordinator(snaps, sched)
	buf := netcore.NewInputBuffer()
	world.SetResource(w, snaps)
	world.SetResource(w, sched)
	world.SetResource(w, rc)
	world.SetResource(w, buf)
	world.SetResource(w, netcore.OutboundQueue{})
	return w
}

func TestPredictionSystemRunNoResources(t *testing.T) {
	t.Parallel()
	w := world.NewWorld()
	hist := NewPredictionHistory(0)
	sys := NewPredictionSystem(1, 0, hist)
	// Should not panic with missing resources.
	sys.Run(w)
}

func TestPredictionSystemRunAdvancesTick(t *testing.T) {
	t.Parallel()
	w := buildPredictWorld(t)
	hist := NewPredictionHistory(0)
	sys := NewPredictionSystem(1, 0, hist)
	sys.Run(w)

	rcP, _ := world.Resource[*netcore.RollbackCoordinator](w)
	rc := *rcP
	if rc.Current() != 1 {
		t.Errorf("Current() = %d, want 1 after one Run", rc.Current())
	}
}

func TestPredictionSystemRunRecordsHistory(t *testing.T) {
	t.Parallel()
	w := buildPredictWorld(t)
	hist := NewPredictionHistory(0)
	sys := NewPredictionSystem(1, 0, hist)
	sys.Run(w)

	if hist.Len() != 1 {
		t.Errorf("history.Len() = %d, want 1", hist.Len())
	}
	if _, ok := hist.Get(1); !ok {
		t.Error("history should contain tick 1")
	}
}

func TestPredictionSystemRunCapturesInput(t *testing.T) {
	t.Parallel()
	w := buildPredictWorld(t)
	hist := NewPredictionHistory(0)
	sys := NewPredictionSystem(2, 0, hist)
	captured := false
	sys.CaptureInput = func() []byte {
		captured = true
		return []byte{1, 2, 3}
	}
	sys.Run(w)

	if !captured {
		t.Error("CaptureInput was not called")
	}
	bufP, _ := world.Resource[*netcore.InputBuffer](w)
	buf := *bufP
	in, ok := buf.GetInput(1, 2)
	if !ok {
		t.Fatal("input for player 2 tick 1 not recorded")
	}
	if string(in.Data) != string([]byte{1, 2, 3}) {
		t.Errorf("input data = %v, want [1 2 3]", in.Data)
	}
}

func TestPredictionSystemRunEnqueuesOutbound(t *testing.T) {
	t.Parallel()
	w := buildPredictWorld(t)
	hist := NewPredictionHistory(0)
	sys := NewPredictionSystem(1, 99, hist) // serverConn=99
	sys.Run(w)

	oqP, _ := world.Resource[netcore.OutboundQueue](w)
	if len(oqP.Messages) == 0 {
		t.Error("OutboundQueue should have at least one message after Run")
	}
	if oqP.Messages[0].Target != 99 {
		t.Errorf("message target = %d, want 99", oqP.Messages[0].Target)
	}
	if oqP.Messages[0].Channel != netcore.ChannelEvents {
		t.Errorf("channel = %d, want ChannelEvents", oqP.Messages[0].Channel)
	}
}

func TestPredictionSystemRunMultipleTicks(t *testing.T) {
	t.Parallel()
	w := buildPredictWorld(t)
	hist := NewPredictionHistory(0)
	sys := NewPredictionSystem(1, 0, hist)
	for range 5 {
		sys.Run(w)
	}
	if hist.Len() != 5 {
		t.Errorf("history.Len() = %d, want 5", hist.Len())
	}
	rcP, _ := world.Resource[*netcore.RollbackCoordinator](w)
	rc := *rcP
	if rc.Current() != 5 {
		t.Errorf("Current() = %d, want 5", rc.Current())
	}
}

func TestPredictionSystemRunMissingSnapshotManager(t *testing.T) {
	t.Parallel()
	w := world.NewWorld()
	sched := netcore.NewDeterministicSchedule(0, time.Millisecond)
	rc := netcore.NewRollbackCoordinator(nil, sched)
	buf := netcore.NewInputBuffer()
	world.SetResource(w, rc)
	world.SetResource(w, buf)
	// No SnapshotManager resource.
	hist := NewPredictionHistory(0)
	sys := NewPredictionSystem(1, 0, hist)
	sys.Run(w) // should not panic
}

func TestPredictionSystemHistory(t *testing.T) {
	t.Parallel()
	hist := NewPredictionHistory(4)
	sys := NewPredictionSystem(1, 0, hist)
	if sys.History() != hist {
		t.Error("History() should return the history passed to NewPredictionSystem")
	}
}

