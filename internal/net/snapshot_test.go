package net

import (
	"testing"
	"time"

	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/typereg"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/scene"
)

type pos struct{ X, Y float64 }

// netWorld builds a World + matching TypeRegistry with one component type and
// spawns n entities; returns the world, registry, and the spawned entities.
func netWorld(t *testing.T, n int) (*world.World, *typereg.TypeRegistry, []entity.Entity) {
	t.Helper()
	w := world.NewWorld()
	pid := world.RegisterComponent[pos](w)
	reg := typereg.NewTypeRegistry()
	typereg.RegisterType[pos](reg)
	ents := make([]entity.Entity, n)
	for i := range ents {
		ents[i] = w.Spawn(component.Data{Value: pos{X: float64(i), Y: float64(i * 2)}, ID: pid})
	}
	return w, reg, ents
}

// ─── DeterministicSchedule ───────────────────────────────────────────────────

func TestDeterministicScheduleRunsInOrder(t *testing.T) {
	t.Parallel()
	d := NewDeterministicSchedule(42, 16*time.Millisecond)
	var order []string
	d.AddFunc("a", func(*world.World) { order = append(order, "a") })
	d.AddFunc("b", func(*world.World) { order = append(order, "b") })
	d.AddFunc("c", func(*world.World) { order = append(order, "c") })
	if d.SystemCount() != 3 {
		t.Fatalf("SystemCount = %d, want 3", d.SystemCount())
	}
	w := world.NewWorld()
	d.RunTick(w, 1)
	if len(order) != 3 || order[0] != "a" || order[1] != "b" || order[2] != "c" {
		t.Errorf("systems ran out of order: %v", order)
	}
	if d.Tick != 1 {
		t.Errorf("Tick = %d, want 1", d.Tick)
	}
}

func TestDeterministicRNGReproducible(t *testing.T) {
	t.Parallel()
	// Two schedules with the same seed produce identical RNG sequences per tick.
	a := NewDeterministicSchedule(7, time.Millisecond)
	b := NewDeterministicSchedule(7, time.Millisecond)
	w := world.NewWorld()
	for tick := uint64(1); tick <= 5; tick++ {
		a.RunTick(w, tick)
		b.RunTick(w, tick)
		if a.Rand().Uint64() != b.Rand().Uint64() {
			t.Fatalf("RNG diverged at tick %d for identical seeds", tick)
		}
	}
	// A different seed diverges.
	c := NewDeterministicSchedule(8, time.Millisecond)
	a.RunTick(w, 1)
	c.RunTick(w, 1)
	if a.Rand().Uint64() == c.Rand().Uint64() {
		t.Error("different seeds should produce different RNG (extremely unlikely match)")
	}
}

// ─── SnapshotManager ─────────────────────────────────────────────────────────

func TestSnapshotChecksumStableAcrossWorlds(t *testing.T) {
	t.Parallel()
	// Two independently-built identical worlds must yield the same checksum
	// (cross-machine desync detection, INV-4).
	w1, reg1, _ := netWorld(t, 3)
	w2, reg2, _ := netWorld(t, 3)
	m1 := NewSnapshotManager(reg1, 16)
	m2 := NewSnapshotManager(reg2, 16)
	c1 := m1.TakeChecksum(scene.NewWorldAdapter(w1))
	c2 := m2.TakeChecksum(scene.NewWorldAdapter(w2))
	if c1 != c2 {
		t.Errorf("identical worlds gave different checksums: %d vs %d", c1, c2)
	}
	// A divergent world differs.
	w3, reg3, _ := netWorld(t, 4) // one more entity
	c3 := NewSnapshotManager(reg3, 16).TakeChecksum(scene.NewWorldAdapter(w3))
	if c1 == c3 {
		t.Error("divergent worlds should produce different checksums")
	}
}

func TestSnapshotTakeRestoreRoundTrip(t *testing.T) {
	t.Parallel()
	src, reg, ents := netWorld(t, 3)
	m := NewSnapshotManager(reg, 16)
	h := m.TakeSnapshot(scene.NewWorldAdapter(src), 100)
	if h.Tick != 100 || h.EntityCount != 3 || h.Checksum == 0 || len(h.Data) == 0 {
		t.Fatalf("handle malformed: %+v", h)
	}
	if m.Len() != 1 {
		t.Errorf("ring Len = %d, want 1", m.Len())
	}

	// Restore into a fresh world: IDs pinned, values intact.
	dst := world.NewWorld()
	world.RegisterComponent[pos](dst)
	res, err := m.RestoreSnapshot(dst, h)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if res.Restored != 3 {
		t.Fatalf("restored %d, want 3", res.Restored)
	}
	for i, e := range ents {
		if !dst.Contains(e) {
			t.Errorf("entity %d id=%d not preserved", i, e.ID())
		}
		if p, ok := world.Get[pos](dst, e); !ok || *p != (pos{X: float64(i), Y: float64(i * 2)}) {
			t.Errorf("entity %d pos = %v ok=%v", i, p, ok)
		}
	}
}

func TestSnapshotRestoreRejectsCorrupt(t *testing.T) {
	t.Parallel()
	_, reg, _ := netWorld(t, 1)
	m := NewSnapshotManager(reg, 16)
	dst := world.NewWorld()
	world.RegisterComponent[pos](dst)
	if _, err := m.RestoreSnapshot(dst, SnapshotHandle{Data: []byte("{not json")}); err == nil {
		t.Error("corrupt snapshot data should fail before mutation")
	}
}

func TestSnapshotRingEviction(t *testing.T) {
	t.Parallel()
	src, reg, _ := netWorld(t, 1)
	m := NewSnapshotManager(reg, 3) // small ring
	reader := scene.NewWorldAdapter(src)
	for tick := uint64(1); tick <= 5; tick++ {
		m.TakeSnapshot(reader, tick)
	}
	if m.Len() != 3 {
		t.Fatalf("ring Len = %d, want 3 (capped)", m.Len())
	}
	// Oldest (ticks 1,2) evicted; 3,4,5 retained.
	if _, ok := m.Get(2); ok {
		t.Error("tick 2 should have been evicted")
	}
	for _, tick := range []uint64{3, 4, 5} {
		if _, ok := m.Get(tick); !ok {
			t.Errorf("tick %d should be retained", tick)
		}
	}
}

func TestNewSnapshotManagerDefaultCapacity(t *testing.T) {
	t.Parallel()
	m := NewSnapshotManager(nil, 0) // <=0 → default 16
	if m.cap != 16 {
		t.Errorf("default cap = %d, want 16", m.cap)
	}
}
