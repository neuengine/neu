package interp

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	netcore "github.com/neuengine/neu/internal/net"
	"github.com/neuengine/neu/internal/net/replication"
	"github.com/neuengine/neu/pkg/app/appface"
	neumath "github.com/neuengine/neu/pkg/math"
)

// mockBuilder is a minimal appface.Builder for plugin tests.
type mockBuilder struct{ w *world.World }

func (m *mockBuilder) World() *world.World                                        { return m.w }
func (m *mockBuilder) AddSystem(_ string, _ scheduler.System) appface.Builder     { return m }
func (m *mockBuilder) AddSystems(_ string, _ ...scheduler.System) appface.Builder { return m }
func (m *mockBuilder) SetResource(_ any) appface.Builder                          { return m }
func (m *mockBuilder) InitResource(_ any) appface.Builder                         { return m }
func (m *mockBuilder) AddPlugin(_ appface.Plugin) appface.Builder                 { return m }
func (m *mockBuilder) AddPlugins(_ appface.PluginGroup) appface.Builder           { return m }

// --- SnapshotBuffer tests ---

func TestSnapshotBufferInsertOrderedArrival(t *testing.T) {
	buf := NewSnapshotBuffer(8)

	buf.Insert(emptyEntry(1, 100*time.Millisecond))
	buf.Insert(emptyEntry(2, 200*time.Millisecond))
	buf.Insert(emptyEntry(3, 300*time.Millisecond))

	if buf.Len() != 3 {
		t.Fatalf("want 3 entries, got %d", buf.Len())
	}
	for i, e := range buf.ring {
		want := uint64(i + 1)
		if e.Tick != want {
			t.Errorf("ring[%d].Tick = %d, want %d", i, e.Tick, want)
		}
	}
}

func TestSnapshotBufferINV3DiscardsOutOfOrder(t *testing.T) {
	// After inserting tick=5, earlier ticks must be discarded (INV-3).
	buf := NewSnapshotBuffer(8)

	buf.Insert(emptyEntry(5, 500*time.Millisecond))
	buf.Insert(emptyEntry(3, 300*time.Millisecond)) // out-of-order: tick 3 <= latestTick 5
	buf.Insert(emptyEntry(5, 500*time.Millisecond)) // duplicate

	if buf.Len() != 1 {
		t.Errorf("want 1 entry, got %d", buf.Len())
	}
	if buf.ring[0].Tick != 5 {
		t.Errorf("expected tick 5, got %d", buf.ring[0].Tick)
	}
}

func TestSnapshotBufferEvictsOldestWhenFull(t *testing.T) {
	buf := NewSnapshotBuffer(3)

	buf.Insert(emptyEntry(1, 100*time.Millisecond))
	buf.Insert(emptyEntry(2, 200*time.Millisecond))
	buf.Insert(emptyEntry(3, 300*time.Millisecond))
	buf.Insert(emptyEntry(4, 400*time.Millisecond)) // evicts tick=1

	if buf.Len() != 3 {
		t.Fatalf("want 3, got %d", buf.Len())
	}
	if buf.ring[0].Tick != 2 {
		t.Errorf("expected oldest tick=2 after eviction, got %d", buf.ring[0].Tick)
	}
	if buf.ring[2].Tick != 4 {
		t.Errorf("expected newest tick=4, got %d", buf.ring[2].Tick)
	}
}

func TestSnapshotBufferBracket(t *testing.T) {
	buf := NewSnapshotBuffer(8)
	buf.Insert(emptyEntry(1, 100*time.Millisecond))
	buf.Insert(emptyEntry(2, 200*time.Millisecond))
	buf.Insert(emptyEntry(3, 300*time.Millisecond))

	prev, next, ok := buf.Bracket(150 * time.Millisecond)
	if !ok {
		t.Fatal("Bracket should find a pair for renderTime=150ms")
	}
	if prev.Tick != 1 || next.Tick != 2 {
		t.Errorf("expected bracket (1,2), got (%d,%d)", prev.Tick, next.Tick)
	}
}

func TestSnapshotBufferBracketMisses(t *testing.T) {
	buf := NewSnapshotBuffer(8)
	buf.Insert(emptyEntry(1, 100*time.Millisecond))
	buf.Insert(emptyEntry(2, 200*time.Millisecond))

	if _, _, ok := buf.Bracket(50 * time.Millisecond); ok {
		t.Error("Bracket should return !ok before first entry")
	}
	if _, _, ok := buf.Bracket(250 * time.Millisecond); ok {
		t.Error("Bracket should return !ok after last entry")
	}
}

func TestSnapshotBufferLatest(t *testing.T) {
	buf := NewSnapshotBuffer(4)
	if _, ok := buf.Latest(); ok {
		t.Error("Latest on empty buffer should return !ok")
	}
	buf.Insert(emptyEntry(1, 100*time.Millisecond))
	buf.Insert(emptyEntry(3, 300*time.Millisecond))

	e, ok := buf.Latest()
	if !ok || e.Tick != 3 {
		t.Errorf("Latest should be tick=3, got ok=%v tick=%d", ok, e.Tick)
	}
}

// --- computeT tests ---

func TestComputeT(t *testing.T) {
	cases := []struct {
		render, prev, next time.Duration
		wantT              float32
	}{
		{150 * time.Millisecond, 100 * time.Millisecond, 200 * time.Millisecond, 0.5},
		{100 * time.Millisecond, 100 * time.Millisecond, 200 * time.Millisecond, 0.0},
		{200 * time.Millisecond, 100 * time.Millisecond, 200 * time.Millisecond, 1.0},
		{50 * time.Millisecond, 100 * time.Millisecond, 200 * time.Millisecond, 0.0},  // clamp < 0
		{250 * time.Millisecond, 100 * time.Millisecond, 200 * time.Millisecond, 1.0}, // clamp > 1
		{150 * time.Millisecond, 100 * time.Millisecond, 100 * time.Millisecond, 0.0}, // zero dt
	}
	for _, tc := range cases {
		got := computeT(tc.render, tc.prev, tc.next)
		if got != tc.wantT {
			t.Errorf("computeT(%v, %v, %v)=%v, want %v", tc.render, tc.prev, tc.next, got, tc.wantT)
		}
	}
}

// --- lerpComponent tests ---

func TestLerpComponentVec3(t *testing.T) {
	w := world.NewWorld()
	id := world.RegisterComponent[neumath.Vec3](w)
	typeName := w.Components().Info(id).Name

	aJSON := mustMarshal(t, neumath.Vec3{X: 0, Y: 0, Z: 0})
	bJSON := mustMarshal(t, neumath.Vec3{X: 10, Y: 20, Z: 30})
	out := lerpComponent(typeName, aJSON, bJSON, 0.5, w.Components())

	var result neumath.Vec3
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.X != 5 || result.Y != 10 || result.Z != 15 {
		t.Errorf("lerp Vec3 got %+v, want {5,10,15}", result)
	}
}

func TestLerpComponentQuat(t *testing.T) {
	w := world.NewWorld()
	id := world.RegisterComponent[neumath.Quat](w)
	typeName := w.Components().Info(id).Name

	a := neumath.QuatIdentity()
	b := neumath.QuatFromAxisAngle(neumath.Vec3{Y: 1}, 1.0)
	out := lerpComponent(typeName, mustMarshal(t, a), mustMarshal(t, b), 0.5, w.Components())

	var result neumath.Quat
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	length := result.X*result.X + result.Y*result.Y + result.Z*result.Z + result.W*result.W
	if length < 0.99 || length > 1.01 {
		t.Errorf("Slerp result not normalized: |q|²=%v", length)
	}
}

func TestLerpComponentFloat32(t *testing.T) {
	w := world.NewWorld()
	id := world.RegisterComponent[float32](w)
	typeName := w.Components().Info(id).Name

	out := lerpComponent(typeName, mustMarshal(t, float32(0)), mustMarshal(t, float32(100)), 0.25, w.Components())

	var result float32
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result != 25 {
		t.Errorf("lerp float32 got %v, want 25", result)
	}
}

func TestLerpComponentUnknownSnaps(t *testing.T) {
	w := world.NewWorld()
	aJSON := json.RawMessage(`"hello"`)
	bJSON := json.RawMessage(`"world"`)
	out := lerpComponent("unregistered.Type", aJSON, bJSON, 0.5, w.Components())
	if string(out) != `"hello"` {
		t.Errorf("unknown type should snap to a, got %s", out)
	}
}

// --- InterpolateSystem tests ---

func setupInterpWorld(t *testing.T) (*world.World, string) {
	t.Helper()
	w := world.NewWorld()
	world.RegisterComponent[DisplayState](w)
	id := world.RegisterComponent[float32](w)
	return w, w.Components().Info(id).Name
}

func TestInterpolateSystemWarmupINV2(t *testing.T) {
	w, typeName := setupInterpWorld(t)
	e := w.Spawn()
	eid := e.ID()

	buf := NewSnapshotBuffer(4)
	buf.Insert(entryWithF32(1, 100*time.Millisecond, eid, typeName, 42.0))

	sys := NewInterpolateSystem(buf, 50*time.Millisecond, false, 0, nil)
	sys.Run(w)

	// Single entry → warm-up path: DisplayState written directly (INV-2).
	ds, ok := getDisplayState(w, e)
	if !ok {
		t.Fatal("DisplayState not written during warm-up")
	}
	if got := decodeF32(t, ds, typeName); got != 42.0 {
		t.Errorf("warm-up: want 42.0, got %v", got)
	}
}

func TestInterpolateSystemINV1AuthoritativeUnchanged(t *testing.T) {
	w, typeName := setupInterpWorld(t)
	e := w.Spawn()
	eid := e.ID()
	_ = w.Insert(e, component.Data{Value: float32(99.0)})

	buf := NewSnapshotBuffer(4)
	buf.Insert(entryWithF32(1, 100*time.Millisecond, eid, typeName, 1.0))
	buf.Insert(entryWithF32(2, 200*time.Millisecond, eid, typeName, 2.0))

	sys := NewInterpolateSystem(buf, 50*time.Millisecond, false, 0, nil)
	sys.Run(w)

	// Authoritative float32 must NOT change (INV-1).
	authPtr, authOK := world.Get[float32](w, e)
	if !authOK {
		t.Fatal("authoritative float32 component missing after interpolation")
	}
	if *authPtr != 99.0 {
		t.Errorf("authoritative component mutated: got %v, want 99.0", *authPtr)
	}
}

func TestInterpolateSystemBlendMidpoint(t *testing.T) {
	w, typeName := setupInterpWorld(t)
	e := w.Spawn()
	eid := e.ID()

	buf := NewSnapshotBuffer(4)
	buf.Insert(entryWithF32(1, 0*time.Millisecond, eid, typeName, 0.0))
	buf.Insert(entryWithF32(2, 100*time.Millisecond, eid, typeName, 100.0))

	// renderDelay=50ms → renderTime = latest.Timestamp(100ms) - 50ms = 50ms
	// bracket: (tick1=0ms, tick2=100ms) → t = (50-0)/100 = 0.5 → value = 50
	sys := NewInterpolateSystem(buf, 50*time.Millisecond, false, 0, nil)
	sys.Run(w)

	ds, ok := getDisplayState(w, e)
	if !ok {
		t.Fatal("DisplayState not written")
	}
	if got := decodeF32(t, ds, typeName); got != 50.0 {
		t.Errorf("blend midpoint: want 50.0, got %v", got)
	}
}

func TestInterpolateSystemINV4SnapToLatestWhenNoBracket(t *testing.T) {
	w, typeName := setupInterpWorld(t)
	e := w.Spawn()
	eid := e.ID()

	buf := NewSnapshotBuffer(4)
	buf.Insert(entryWithF32(1, 0*time.Millisecond, eid, typeName, 0.0))
	buf.Insert(entryWithF32(2, 100*time.Millisecond, eid, typeName, 100.0))

	// renderDelay=200ms → renderTime = 100ms - 200ms = -100ms → before bracket → snap to latest
	sys := NewInterpolateSystem(buf, 200*time.Millisecond, false, 0, nil)
	sys.Run(w)

	ds, ok := getDisplayState(w, e)
	if !ok {
		t.Fatal("DisplayState not written")
	}
	if got := decodeF32(t, ds, typeName); got != 100.0 {
		t.Errorf("snap to latest: want 100.0, got %v", got)
	}
}

func TestInterpolateSystemExtrapolationCapped(t *testing.T) {
	w, typeName := setupInterpWorld(t)
	e := w.Spawn()
	eid := e.ID()

	buf := NewSnapshotBuffer(4)
	buf.Insert(entryWithF32(1, 0*time.Millisecond, eid, typeName, 0.0))
	buf.Insert(entryWithF32(2, 100*time.Millisecond, eid, typeName, 100.0))

	// renderDelay=-50ms → renderTime = 100ms - (-50ms) = 150ms (past latest)
	// extraFactor = (150ms - 100ms) / (100ms - 0ms) = 0.5 = maxExtrapolation
	// t = 1 + 0.5 = 1.5 → lerp(0, 100, 1.5) = 150
	sys := NewInterpolateSystem(buf, -50*time.Millisecond, true, 0.5, nil)
	sys.Run(w)

	ds, ok := getDisplayState(w, e)
	if !ok {
		t.Fatal("DisplayState not written during extrapolation")
	}
	if got := decodeF32(t, ds, typeName); got != 150.0 {
		t.Errorf("extrapolation at cap: want 150.0, got %v", got)
	}
}

func TestInterpolateSystemEntityNotInWorldSkipped(t *testing.T) {
	w, typeName := setupInterpWorld(t)

	// eid not spawned in world.
	eid := entity.EntityID(999)

	buf := NewSnapshotBuffer(4)
	buf.Insert(entryWithF32(1, 0*time.Millisecond, eid, typeName, 10.0))
	buf.Insert(entryWithF32(2, 100*time.Millisecond, eid, typeName, 20.0))

	sys := NewInterpolateSystem(buf, 50*time.Millisecond, false, 0, nil)
	// Must not panic.
	sys.Run(w)
}

func TestInterpolateSystemLookupClient(t *testing.T) {
	w, typeName := setupInterpWorld(t)
	clientE := w.Spawn()
	clientID := clientE.ID()
	serverID := entity.EntityID(999)

	lookup := func(srv entity.EntityID) (entity.EntityID, bool) {
		if srv == serverID {
			return clientID, true
		}
		return 0, false
	}

	buf := NewSnapshotBuffer(4)
	buf.Insert(entryWithF32(1, 0*time.Millisecond, serverID, typeName, 0.0))
	buf.Insert(entryWithF32(2, 100*time.Millisecond, serverID, typeName, 100.0))

	sys := NewInterpolateSystem(buf, 50*time.Millisecond, false, 0, lookup)
	sys.Run(w)

	ds, ok := getDisplayState(w, clientE)
	if !ok {
		t.Fatal("DisplayState not written for mapped client entity")
	}
	if got := decodeF32(t, ds, typeName); got != 50.0 {
		t.Errorf("with lookup: want 50.0, got %v", got)
	}
}

// --- helpers ---

func TestSnapshotBufferDefaultCapacity(t *testing.T) {
	buf := NewSnapshotBuffer(0)
	if buf.capacity != DefaultBufferCapacity {
		t.Errorf("capacity want %d, got %d", DefaultBufferCapacity, buf.capacity)
	}
}

func TestRegisterLerper(t *testing.T) {
	w := world.NewWorld()
	id := world.RegisterComponent[float32](w)
	typeName := w.Components().Info(id).Name

	RegisterLerper(func(a, b float32, t float32) float32 {
		return a*1000 + b*1000 // obviously non-standard, just detects the custom lerp was called
	})
	t.Cleanup(func() {
		// Restore the original float32 lerper so other tests aren't affected.
		RegisterLerper(func(a, b float32, t float32) float32 { return a + (b-a)*t })
	})

	out := lerpComponent(typeName, mustMarshal(t, float32(1)), mustMarshal(t, float32(2)), 0.5, w.Components())
	var result float32
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result != 3000 {
		t.Errorf("custom lerper: want 3000, got %v", result)
	}
}

func TestInterpolateSystemBlendNewEntityNotInPrev(t *testing.T) {
	// Entity appears in next but not in prev (new spawn during interpolation window).
	w, typeName := setupInterpWorld(t)
	e := w.Spawn()
	eid := e.ID()

	data, _ := json.Marshal(float32(77.0))
	prev := SnapshotEntry{
		Tick:      1,
		Timestamp: 0,
		Entities:  map[entity.EntityID]EntityState{}, // eid absent
	}
	next := SnapshotEntry{
		Tick:      2,
		Timestamp: 100 * time.Millisecond,
		Entities:  map[entity.EntityID]EntityState{eid: {typeName: data}},
	}

	buf := NewSnapshotBuffer(4)
	buf.Insert(prev)
	buf.Insert(next)

	// renderTime = 100ms - 50ms = 50ms → inside bracket
	sys := NewInterpolateSystem(buf, 50*time.Millisecond, false, 0, nil)
	sys.Run(w)

	ds, ok := getDisplayState(w, e)
	if !ok {
		t.Fatal("DisplayState not written for entity absent from prev")
	}
	// When entity is absent from prev, component value should snap to next (77.0).
	if got := decodeF32(t, ds, typeName); got != 77.0 {
		t.Errorf("new entity in blend: want 77.0, got %v", got)
	}
}

func TestInterpolateSystemBlendComponentNotInPrev(t *testing.T) {
	// Entity exists in both snapshots but a specific component is only in next.
	w, typeName := setupInterpWorld(t)
	e := w.Spawn()
	eid := e.ID()

	prevData, _ := json.Marshal(float32(0.0))
	nextData, _ := json.Marshal(float32(100.0))

	// Use a fake type name for the "new" component to simulate it appearing mid-stream.
	const newTypeName = "example.NewComponent"

	prev := SnapshotEntry{
		Tick:      1,
		Timestamp: 0,
		Entities: map[entity.EntityID]EntityState{
			eid: {typeName: prevData}, // float32 present; newTypeName absent
		},
	}
	next := SnapshotEntry{
		Tick:      2,
		Timestamp: 100 * time.Millisecond,
		Entities: map[entity.EntityID]EntityState{
			eid: {typeName: nextData, newTypeName: nextData}, // both present in next
		},
	}

	buf := NewSnapshotBuffer(4)
	buf.Insert(prev)
	buf.Insert(next)

	sys := NewInterpolateSystem(buf, 50*time.Millisecond, false, 0, nil)
	sys.Run(w)

	ds, ok := getDisplayState(w, e)
	if !ok {
		t.Fatal("DisplayState not written")
	}
	// newTypeName was absent from prev → snaps to next value (100.0).
	if got := decodeF32(t, ds, newTypeName); got != 100.0 {
		t.Errorf("new component in blend: want 100.0, got %v", got)
	}
}

// --- InterpolationConfig tests ---

func TestDefaultInterpolationConfig(t *testing.T) {
	cfg := DefaultInterpolationConfig()
	if cfg.RenderDelay <= 0 {
		t.Error("RenderDelay must be > 0")
	}
	if cfg.BufferCapacity <= 0 {
		t.Error("BufferCapacity must be > 0")
	}
	if cfg.TargetFill <= 0 {
		t.Error("TargetFill must be > 0")
	}
	if cfg.MaxExtrapolation <= 0 {
		t.Error("MaxExtrapolation must be > 0")
	}
}

// --- AdaptiveDelay tests ---

func TestAdaptiveDelayEmptyBuffer(t *testing.T) {
	cfg := DefaultInterpolationConfig()
	a := NewAdaptiveDelay(cfg)
	initial := a.RenderDelay
	buf := NewSnapshotBuffer(4) // empty

	a.Adjust(buf)

	if a.RenderDelay <= initial {
		t.Errorf("empty buffer: want delay increase, got %v → %v", initial, a.RenderDelay)
	}
}

func TestAdaptiveDelayUnderfillIncreasesDelay(t *testing.T) {
	cfg := DefaultInterpolationConfig()
	cfg.TargetFill = 5
	a := NewAdaptiveDelay(cfg)
	initial := a.RenderDelay

	// 2 entries total, both with Timestamp > renderTime → ahead=2 < targetFill=5
	buf := NewSnapshotBuffer(8)
	buf.Insert(emptyEntry(1, 200*time.Millisecond))
	buf.Insert(emptyEntry(2, 400*time.Millisecond))

	a.Adjust(buf)

	if a.RenderDelay <= initial {
		t.Errorf("underfill: want delay increase, got %v → %v", initial, a.RenderDelay)
	}
}

func TestAdaptiveDelayOverfillDecreasesDelay(t *testing.T) {
	cfg := DefaultInterpolationConfig()
	cfg.TargetFill = 1
	cfg.RenderDelay = 300 * time.Millisecond // large so many entries are ahead
	a := NewAdaptiveDelay(cfg)
	initial := a.RenderDelay

	buf := NewSnapshotBuffer(8)
	buf.Insert(emptyEntry(1, 100*time.Millisecond))
	buf.Insert(emptyEntry(2, 200*time.Millisecond))
	buf.Insert(emptyEntry(3, 300*time.Millisecond))
	buf.Insert(emptyEntry(4, 400*time.Millisecond))
	buf.Insert(emptyEntry(5, 500*time.Millisecond))

	a.Adjust(buf)

	if a.RenderDelay >= initial {
		t.Errorf("overfill: want delay decrease, got %v → %v", initial, a.RenderDelay)
	}
}

func TestAdaptiveDelayAtTargetNoChange(t *testing.T) {
	cfg := DefaultInterpolationConfig()
	cfg.TargetFill = 2
	cfg.RenderDelay = 250 * time.Millisecond
	a := NewAdaptiveDelay(cfg)
	initial := a.RenderDelay

	// latest.Timestamp = 500ms, renderTime = 500ms - 250ms = 250ms
	// entries at 100ms, 200ms, 300ms, 400ms, 500ms
	// entries with Timestamp > 250ms: 300ms, 400ms, 500ms = 3... hmm that's > 2
	// Let me use fewer entries so ahead == targetFill == 2:
	// latest=400ms, renderTime=400ms-250ms=150ms → entries 200ms, 300ms, 400ms: ahead=3 not 2
	// Let's use latest=300ms, renderTime=300ms-250ms=50ms → entries 100ms,200ms,300ms: ahead=3
	// Let's use renderDelay=100ms, latest=300ms, renderTime=200ms → entries >200ms: 300ms: ahead=1
	// targetFill=1, renderDelay=100ms, latest=300ms: renderTime=200ms, entries > 200ms: 300ms=1
	// That gives ahead == targetFill == 1. Let me restructure.

	a.RenderDelay = 100 * time.Millisecond
	initial = a.RenderDelay
	a.TargetFill = 1

	buf := NewSnapshotBuffer(8)
	buf.Insert(emptyEntry(1, 100*time.Millisecond))
	buf.Insert(emptyEntry(2, 200*time.Millisecond))
	buf.Insert(emptyEntry(3, 300*time.Millisecond))
	// latest=300ms, renderTime=300ms-100ms=200ms → entries >200ms: 300ms only → ahead=1 == targetFill=1

	a.Adjust(buf)

	if a.RenderDelay != initial {
		t.Errorf("at target: want no change, got %v → %v", initial, a.RenderDelay)
	}
}

func TestAdaptiveDelayClampMax(t *testing.T) {
	cfg := DefaultInterpolationConfig()
	a := NewAdaptiveDelay(cfg)
	a.RenderDelay = a.MaxDelay // already at max
	a.TargetFill = 999         // ensure underfill

	buf := NewSnapshotBuffer(4) // empty → aggressive increase

	a.Adjust(buf)

	if a.RenderDelay != a.MaxDelay {
		t.Errorf("clamp max: want %v, got %v", a.MaxDelay, a.RenderDelay)
	}
}

func TestAdaptiveDelaySmallRenderDelayAdjustsMin(t *testing.T) {
	// When RenderDelay < 2*DefaultMinDelay, MinDelay = RenderDelay/2.
	cfg := InterpolationConfig{RenderDelay: 10 * time.Millisecond, TargetFill: 3}
	a := NewAdaptiveDelay(cfg)
	if a.MinDelay != 5*time.Millisecond {
		t.Errorf("want MinDelay=5ms for renderDelay=10ms, got %v", a.MinDelay)
	}
}

func TestAdaptiveDelayClampMin(t *testing.T) {
	cfg := DefaultInterpolationConfig()
	a := NewAdaptiveDelay(cfg)
	a.RenderDelay = a.MinDelay // already at min
	a.TargetFill = 0           // ensure overfill (any ahead > 0 is overfill)

	buf := NewSnapshotBuffer(8)
	buf.Insert(emptyEntry(1, 100*time.Millisecond))
	buf.Insert(emptyEntry(2, 200*time.Millisecond))

	a.Adjust(buf)

	if a.RenderDelay != a.MinDelay {
		t.Errorf("clamp min: want %v, got %v", a.MinDelay, a.RenderDelay)
	}
}

func TestInterpolateSystemSetRenderDelay(t *testing.T) {
	buf := NewSnapshotBuffer(4)
	sys := NewInterpolateSystem(buf, 100*time.Millisecond, false, 0, nil)
	sys.SetRenderDelay(200 * time.Millisecond)
	if sys.renderDelay != 200*time.Millisecond {
		t.Errorf("SetRenderDelay: want 200ms, got %v", sys.renderDelay)
	}
}

// --- snapshotIngestSystem tests ---

func TestInterpolationPluginBuildSetsConfig(t *testing.T) {
	w := world.NewWorld()
	// Zero config — all default branches trigger.
	p := InterpolationPlugin{Config: InterpolationConfig{}}
	p.Build(&mockBuilder{w: w})

	cfg, ok := world.Resource[InterpolationConfig](w)
	if !ok {
		t.Fatal("InterpolationConfig not set in world")
	}
	if cfg.RenderDelay <= 0 {
		t.Error("RenderDelay should be positive after defaults")
	}
	if cfg.BufferCapacity <= 0 {
		t.Error("BufferCapacity should be positive after defaults")
	}
}

func TestInterpolationPluginBuildWithExplicitConfig(t *testing.T) {
	w := world.NewWorld()
	p := InterpolationPlugin{Config: DefaultInterpolationConfig()}
	p.Build(&mockBuilder{w: w})

	cfg, ok := world.Resource[InterpolationConfig](w)
	if !ok {
		t.Fatal("InterpolationConfig not set in world")
	}
	if cfg.RenderDelay != 100*time.Millisecond {
		t.Errorf("want 100ms, got %v", cfg.RenderDelay)
	}
}

func TestSnapshotIngestSystemNoQueue(t *testing.T) {
	w := world.NewWorld()
	buf := NewSnapshotBuffer(4)
	adaptive := NewAdaptiveDelay(DefaultInterpolationConfig())
	interpSys := NewInterpolateSystem(buf, 100*time.Millisecond, false, 0, nil)
	ingest := &snapshotIngestSystem{buf: buf, adaptive: adaptive, interp: interpSys, start: time.Now()}

	ingest.Run(w) // must not panic — no InboundQueue resource

	if buf.Len() != 0 {
		t.Error("buffer should remain empty when no InboundQueue is set")
	}
}

func TestSnapshotIngestSystemBuildsEntry(t *testing.T) {
	w := world.NewWorld()
	world.RegisterComponent[DisplayState](w)
	f32id := world.RegisterComponent[float32](w)
	typeName := w.Components().Info(f32id).Name

	data, _ := json.Marshal(float32(42.0))
	msg := replication.ReplicationMessage{
		Kind:       replication.MsgEntitySpawn,
		ServerID:   entity.EntityID(1),
		Components: []replication.ReplicatedComponent{{TypeName: typeName, Data: data}},
	}
	payload := replication.EncodeReplicationMessage(msg)

	world.SetResource(w, netcore.InboundQueue{
		Packets: []netcore.InboundPacket{{Payload: payload, Channel: netcore.ChannelSnapshot}},
	})

	buf := NewSnapshotBuffer(4)
	adaptive := NewAdaptiveDelay(DefaultInterpolationConfig())
	interpSys := NewInterpolateSystem(buf, 100*time.Millisecond, false, 0, nil)
	ingest := &snapshotIngestSystem{buf: buf, adaptive: adaptive, interp: interpSys, start: time.Now()}

	ingest.Run(w)

	if buf.Len() != 1 {
		t.Fatalf("expected 1 entry, got %d", buf.Len())
	}
	entry := buf.ring[0]
	state, ok := entry.Entities[entity.EntityID(1)]
	if !ok {
		t.Fatal("entity 1 not in snapshot entry")
	}
	if _, hasTN := state[typeName]; !hasTN {
		t.Errorf("component %q missing from entity state", typeName)
	}
}

func TestSnapshotIngestSystemSkipsNonSnapshotChannel(t *testing.T) {
	w := world.NewWorld()
	world.SetResource(w, netcore.InboundQueue{
		Packets: []netcore.InboundPacket{{
			Payload: []byte{0xFF}, // garbage, but on wrong channel
			Channel: netcore.ChannelEvents,
		}},
	})

	buf := NewSnapshotBuffer(4)
	adaptive := NewAdaptiveDelay(DefaultInterpolationConfig())
	interpSys := NewInterpolateSystem(buf, 100*time.Millisecond, false, 0, nil)
	ingest := &snapshotIngestSystem{buf: buf, adaptive: adaptive, interp: interpSys, start: time.Now()}

	ingest.Run(w)

	if buf.Len() != 0 {
		t.Error("non-snapshot packets should be ignored")
	}
}

func TestSnapshotIngestSystemIgnoresDespawnMessages(t *testing.T) {
	w := world.NewWorld()
	msg := replication.ReplicationMessage{
		Kind:     replication.MsgEntityDespawn,
		ServerID: entity.EntityID(5),
	}
	payload := replication.EncodeReplicationMessage(msg)
	world.SetResource(w, netcore.InboundQueue{
		Packets: []netcore.InboundPacket{{Payload: payload, Channel: netcore.ChannelSnapshot}},
	})

	buf := NewSnapshotBuffer(4)
	adaptive := NewAdaptiveDelay(DefaultInterpolationConfig())
	interpSys := NewInterpolateSystem(buf, 100*time.Millisecond, false, 0, nil)
	ingest := &snapshotIngestSystem{buf: buf, adaptive: adaptive, interp: interpSys, start: time.Now()}

	ingest.Run(w)

	if buf.Len() != 0 {
		t.Error("despawn-only packets should not create snapshot entries")
	}
}

func emptyEntry(tick uint64, ts time.Duration) SnapshotEntry {
	return SnapshotEntry{Tick: tick, Timestamp: ts, Entities: map[entity.EntityID]EntityState{}}
}

func entryWithF32(tick uint64, ts time.Duration, eid entity.EntityID, typeName string, v float32) SnapshotEntry {
	data, _ := json.Marshal(v)
	return SnapshotEntry{
		Tick:      tick,
		Timestamp: ts,
		Entities:  map[entity.EntityID]EntityState{eid: {typeName: data}},
	}
}

func getDisplayState(w *world.World, e entity.Entity) (DisplayState, bool) {
	ds, ok := world.Get[DisplayState](w, e)
	if !ok {
		return DisplayState{}, false
	}
	return *ds, true
}

func decodeF32(t *testing.T, ds DisplayState, typeName string) float32 {
	t.Helper()
	data, ok := ds.Components[typeName]
	if !ok {
		var keys []string
		for k := range ds.Components {
			keys = append(keys, k)
		}
		t.Fatalf("DisplayState missing component %q; present: %v", typeName, keys)
		return 0
	}
	var v float32
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("unmarshal float32: %v", err)
	}
	return v
}

func mustMarshal(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}
