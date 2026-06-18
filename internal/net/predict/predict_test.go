package predict

import (
	"testing"
	"time"

	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/typereg"
	"github.com/neuengine/neu/internal/ecs/world"
	netcore "github.com/neuengine/neu/internal/net"
	neumath "github.com/neuengine/neu/pkg/math"
	"github.com/neuengine/neu/pkg/app/appface"
	"github.com/neuengine/neu/pkg/scene"
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

// ─── CorrectionState / CorrectionSmoothingSystem ─────────────────────────────

func TestCorrectionStateDefaults(t *testing.T) {
	t.Parallel()
	cs := CorrectionState{BlendDuration: DefaultBlendDuration}
	if cs.BlendDuration != DefaultBlendDuration {
		t.Errorf("BlendDuration = %v, want %v", cs.BlendDuration, DefaultBlendDuration)
	}
}

func TestNewCorrectionSmoothingSystemDefaultDt(t *testing.T) {
	t.Parallel()
	sys := NewCorrectionSmoothingSystem(0)
	if sys.dt != time.Second/60 {
		t.Errorf("dt = %v, want %v", sys.dt, time.Second/60)
	}
}

func spawnEntityWithCorrectionState(w *world.World, cs CorrectionState) entity.Entity {
	data := component.NewData[CorrectionState](w.Components(), cs)
	return w.Spawn(data)
}

func TestCorrectionSmoothingSystemNoRegisteredComponent(t *testing.T) {
	t.Parallel()
	w := world.NewWorld()
	sys := NewCorrectionSmoothingSystem(0)
	// Should not panic when CorrectionState is unregistered.
	sys.Run(w)
}

func TestCorrectionSmoothingSystemDecaysOffset(t *testing.T) {
	t.Parallel()
	w := world.NewWorld()
	dt := 10 * time.Millisecond
	sys := NewCorrectionSmoothingSystem(dt)

	cs := CorrectionState{
		VisualOffset:   neumath.Vec3{X: 10, Y: 0, Z: 0},
		RotationOffset: neumath.QuatIdentity(),
		BlendRemaining: 100 * time.Millisecond,
		BlendDuration:  100 * time.Millisecond,
	}
	ent := spawnEntityWithCorrectionState(w, cs)
	sys.Run(w)

	got, ok := world.Get[CorrectionState](w, ent)
	if !ok {
		t.Fatal("CorrectionState removed too early")
	}
	if got.BlendRemaining != 90*time.Millisecond {
		t.Errorf("BlendRemaining = %v, want 90ms", got.BlendRemaining)
	}
	// VisualOffset should have moved toward zero.
	if got.VisualOffset.X >= 10 {
		t.Errorf("VisualOffset.X = %v, expected decay", got.VisualOffset.X)
	}
}

func TestCorrectionSmoothingSystemRemovesWhenExpired(t *testing.T) {
	t.Parallel()
	w := world.NewWorld()
	dt := 50 * time.Millisecond
	sys := NewCorrectionSmoothingSystem(dt)

	cs := CorrectionState{
		VisualOffset:   neumath.Vec3{X: 1},
		RotationOffset: neumath.QuatIdentity(),
		BlendRemaining: 30 * time.Millisecond, // less than dt
		BlendDuration:  100 * time.Millisecond,
	}
	ent := spawnEntityWithCorrectionState(w, cs)
	sys.Run(w)

	if _, ok := world.Get[CorrectionState](w, ent); ok {
		t.Error("CorrectionState should be removed when BlendRemaining expires")
	}
}

func TestCorrectionSmoothingSystemMultipleEntities(t *testing.T) {
	t.Parallel()
	w := world.NewWorld()
	dt := 20 * time.Millisecond
	sys := NewCorrectionSmoothingSystem(dt)

	cs1 := CorrectionState{
		VisualOffset:   neumath.Vec3{X: 5},
		RotationOffset: neumath.QuatIdentity(),
		BlendRemaining: 10 * time.Millisecond, // expires
		BlendDuration:  100 * time.Millisecond,
	}
	cs2 := CorrectionState{
		VisualOffset:   neumath.Vec3{X: 3},
		RotationOffset: neumath.QuatIdentity(),
		BlendRemaining: 100 * time.Millisecond,
		BlendDuration:  100 * time.Millisecond,
	}
	ent1 := spawnEntityWithCorrectionState(w, cs1)
	ent2 := spawnEntityWithCorrectionState(w, cs2)
	sys.Run(w)

	if _, ok := world.Get[CorrectionState](w, ent1); ok {
		t.Error("ent1 CorrectionState should be removed (expired)")
	}
	if _, ok := world.Get[CorrectionState](w, ent2); !ok {
		t.Error("ent2 CorrectionState should remain")
	}
}

// ─── ServerState / ReconciliationSystem ──────────────────────────────────────

func buildReconcileWorld(t *testing.T) *world.World {
	t.Helper()
	w := buildPredictWorld(t)
	world.SetResource(w, ServerState{})
	return w
}

func TestReconciliationSystemNoServerStateResource(t *testing.T) {
	t.Parallel()
	w := buildPredictWorld(t)
	hist := NewPredictionHistory(0)
	sys := NewReconciliationSystem(hist)
	sys.Run(w) // should not panic
}

func TestReconciliationSystemNotValid(t *testing.T) {
	t.Parallel()
	w := buildReconcileWorld(t)
	hist := NewPredictionHistory(8)
	hist.RecordTick(5, 0xABCD)
	sys := NewReconciliationSystem(hist)
	sys.Run(w) // Valid=false: no action
	if hist.Len() != 1 {
		t.Error("history should be unchanged when Valid=false")
	}
}

func TestReconciliationSystemMatchDiscardsHistory(t *testing.T) {
	t.Parallel()
	w := buildReconcileWorld(t)
	// Set up prediction history with ticks 1..5.
	hist := NewPredictionHistory(8)
	for i := uint64(1); i <= 5; i++ {
		hist.RecordTick(i, uint32(i)*0x100)
	}
	// Mark tick 3 as confirmed by server.
	ssP, _ := world.Resource[ServerState](w)
	ssP.Tick = 3
	ssP.Checksum = 3 * 0x100
	ssP.Valid = true

	sys := NewReconciliationSystem(hist)
	sys.Run(w)

	// Ticks 1,2,3 should be discarded; 4,5 should remain.
	if hist.Len() != 2 {
		t.Errorf("Len = %d, want 2 after DiscardThrough(3)", hist.Len())
	}
	for _, tick := range []uint64{1, 2, 3} {
		if _, ok := hist.Get(tick); ok {
			t.Errorf("tick %d should be discarded", tick)
		}
	}
}

func TestReconciliationSystemMismatchRollsBack(t *testing.T) {
	t.Parallel()
	w := buildReconcileWorld(t)

	// Advance the coordinator a few ticks so there are snapshots to rollback to.
	rcP, _ := world.Resource[*netcore.RollbackCoordinator](w)
	rc := *rcP
	reader := scene.NewWorldAdapter(w)
	for i := uint64(1); i <= 3; i++ {
		rc.AdvanceTick(w, reader, i)
	}

	hist := NewPredictionHistory(8)
	hist.RecordTick(1, 0xBAAD) // stored checksum for tick 1 (will mismatch)

	ssP, _ := world.Resource[ServerState](w)
	ssP.Tick = 1
	ssP.Checksum = 0xCAFE // server says 0xCAFE; history says 0xBAAD
	ssP.Valid = true

	sys := NewReconciliationSystem(hist)
	sys.Run(w)

	// Valid should be cleared.
	if ssP.Valid {
		t.Error("ServerState.Valid should be cleared after consumption")
	}
	// History for tick 1 should be discarded.
	if _, ok := hist.Get(1); ok {
		t.Error("tick 1 should be discarded after rollback")
	}
}

func TestReconciliationSystemMissingHistoryEntry(t *testing.T) {
	t.Parallel()
	w := buildReconcileWorld(t)

	hist := NewPredictionHistory(8)
	// No history for tick 5.

	ssP, _ := world.Resource[ServerState](w)
	ssP.Tick = 5
	ssP.Checksum = 0x1234
	ssP.Valid = true

	sys := NewReconciliationSystem(hist)
	sys.Run(w) // should not panic; just discards before tick 5
}

// ─── PredictionPlugin ────────────────────────────────────────────────────────

// fakeBuilder records calls to AddSystem and world changes for plugin testing.
type fakePredictBuilder struct {
	w       *world.World
	systems []string
}

func newFakePredictBuilder() *fakePredictBuilder {
	return &fakePredictBuilder{w: world.NewWorld()}
}

func (f *fakePredictBuilder) World() *world.World { return f.w }
func (f *fakePredictBuilder) AddSystem(schedule string, sys scheduler.System) appface.Builder {
	f.systems = append(f.systems, schedule+":"+sys.Name())
	return f
}
func (f *fakePredictBuilder) AddSystems(schedule string, systems ...scheduler.System) appface.Builder {
	for _, s := range systems {
		f.systems = append(f.systems, schedule+":"+s.Name())
	}
	return f
}
func (f *fakePredictBuilder) SetResource(v any) appface.Builder         { return f }
func (f *fakePredictBuilder) InitResource(v any) appface.Builder        { return f }
func (f *fakePredictBuilder) AddPlugin(p appface.Plugin) appface.Builder { p.Build(f); return f }
func (f *fakePredictBuilder) AddPlugins(g appface.PluginGroup) appface.Builder {
	for _, p := range g.Plugins() {
		p.Build(f)
	}
	return f
}

func TestPredictionPluginBuild(t *testing.T) {
	t.Parallel()
	fb := newFakePredictBuilder()

	// NetworkPlugin resources must exist before PredictionPlugin.
	reg := typereg.NewTypeRegistry()
	snaps := netcore.NewSnapshotManager(reg, 16)
	sched := netcore.NewDeterministicSchedule(0, time.Millisecond)
	rc := netcore.NewRollbackCoordinator(snaps, sched)
	buf := netcore.NewInputBuffer()
	w := fb.w
	world.SetResource(w, snaps)
	world.SetResource(w, sched)
	world.SetResource(w, rc)
	world.SetResource(w, buf)
	world.SetResource(w, netcore.OutboundQueue{})

	plugin := PredictionPlugin{LocalPlayer: 1, ServerConn: 0}
	plugin.Build(fb)

	// Three systems should be registered.
	if len(fb.systems) != 3 {
		t.Errorf("systems registered = %d, want 3: got %v", len(fb.systems), fb.systems)
	}
	// ServerState resource should be set.
	if _, ok := world.Resource[ServerState](w); !ok {
		t.Error("ServerState resource should be set by PredictionPlugin")
	}
}

