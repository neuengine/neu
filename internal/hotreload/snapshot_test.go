//go:build editor

package hotreload

import (
	"errors"
	"testing"

	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/typereg"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/scene"
)

type position struct{ X, Y float64 }
type label struct{ Text string }

const testEngineVersion = "9.9.9"

// names of the registered component types (computed once, shared by tests).
var (
	posName = typereg.RegisterType[position](typereg.NewTypeRegistry()).Name
	lblName = typereg.RegisterType[label](typereg.NewTypeRegistry()).Name
)

// newSourceWorld builds a source World with two registered components and a
// matching TypeRegistry, spawning two entities whose IDs are returned so the
// round-trip can assert they survive.
func newSourceWorld(t *testing.T) (*world.World, *typereg.TypeRegistry, []entity.Entity) {
	t.Helper()
	w := world.NewWorld()
	posID := world.RegisterComponent[position](w)
	lblID := world.RegisterComponent[label](w)
	reg := typereg.NewTypeRegistry()
	typereg.RegisterType[position](reg)
	typereg.RegisterType[label](reg)
	e1 := w.Spawn(
		component.Data{Value: position{X: 1, Y: 2}, ID: posID},
		component.Data{Value: label{Text: "hero"}, ID: lblID},
	)
	e2 := w.Spawn(component.Data{Value: position{X: 3, Y: 4}, ID: posID})
	return w, reg, []entity.Entity{e1, e2}
}

// freshRestoreWorld returns an empty World with the component types registered
// (the new process registers them at plugin Build).
func freshRestoreWorld(t *testing.T) *world.World {
	t.Helper()
	w := world.NewWorld()
	world.RegisterComponent[position](w)
	world.RegisterComponent[label](w)
	return w
}

func TestSnapshotRestorePreservesEntityIDs(t *testing.T) {
	t.Parallel()
	src, reg, ents := newSourceWorld(t)
	snap := BuildSnapshot(scene.NewWorldAdapter(src), reg, testEngineVersion, AppState{FlowState: "InGame"})
	if snap.Header.EntityCount != 2 {
		t.Fatalf("snapshot entity count = %d, want 2", snap.Header.EntityCount)
	}

	// Round-trip through bytes (exercises Encode/Decode).
	data, err := EncodeSnapshot(snap)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeSnapshot(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	dst := freshRestoreWorld(t)
	res, err := ApplySnapshot(dst, decoded, reg, testEngineVersion)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Restored != 2 {
		t.Fatalf("restored %d entities, want 2", res.Restored)
	}

	// INV-5: each original EntityID is alive in the restored world.
	for _, e := range ents {
		if !dst.Contains(e) {
			t.Errorf("entity id=%d (idx=%d gen=%d) not preserved after restore", e.ID(), e.Index(), e.Generation())
		}
	}
	// Component values survived the round-trip.
	if p, ok := world.Get[position](dst, ents[0]); !ok || *p != (position{X: 1, Y: 2}) {
		t.Errorf("entity0 position = %v (ok=%v), want {1 2}", p, ok)
	}
	if l, ok := world.Get[label](dst, ents[0]); !ok || l.Text != "hero" {
		t.Errorf("entity0 label = %v (ok=%v), want hero", l, ok)
	}
	if p, ok := world.Get[position](dst, ents[1]); !ok || *p != (position{X: 3, Y: 4}) {
		t.Errorf("entity1 position = %v (ok=%v), want {3 4}", p, ok)
	}
	if decoded.App.FlowState != "InGame" {
		t.Errorf("app flow state lost: %q", decoded.App.FlowState)
	}
}

func TestApplySnapshotRejectsVersionMismatch(t *testing.T) {
	t.Parallel()
	src, reg, _ := newSourceWorld(t)
	snap := BuildSnapshot(scene.NewWorldAdapter(src), reg, testEngineVersion, AppState{})

	dst := freshRestoreWorld(t)
	_, err := ApplySnapshot(dst, snap, reg, "different-engine-version")
	if !errors.Is(err, ErrEngineVersionMismatch) {
		t.Fatalf("version mismatch error = %v, want ErrEngineVersionMismatch", err)
	}
	// INV-1: nothing was applied — the world is untouched.
	if dst.Entities().Len() != 0 {
		t.Errorf("world mutated despite rejected snapshot: %d entities alive", dst.Entities().Len())
	}
}

func TestDecodeSnapshotRejectsCorrupt(t *testing.T) {
	t.Parallel()
	if _, err := DecodeSnapshot([]byte("{ this is not valid json")); err == nil {
		t.Error("corrupt snapshot bytes must fail at decode, before any world mutation (INV-1)")
	}
}

func TestApplySnapshotDropsUnregisteredType(t *testing.T) {
	t.Parallel()
	src, reg, ents := newSourceWorld(t)
	snap := BuildSnapshot(scene.NewWorldAdapter(src), reg, testEngineVersion, AppState{})

	// Restore against a registry that no longer knows 'label' (simulating a type
	// removed/renamed since capture). label is dropped; position still restores.
	reg2 := typereg.NewTypeRegistry()
	typereg.RegisterType[position](reg2)
	dst := freshRestoreWorld(t)
	res, err := ApplySnapshot(dst, snap, reg2, testEngineVersion)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	var droppedLabel bool
	for _, d := range res.Dropped {
		if d.TypeName == lblName {
			droppedLabel = true
		}
	}
	if !droppedLabel {
		t.Errorf("label should be reported in Dropped (INV-2), got %v", res.Dropped)
	}
	// position survived; the entity still exists with its position.
	if p, ok := world.Get[position](dst, ents[0]); !ok || *p != (position{X: 1, Y: 2}) {
		t.Errorf("position should still restore when label is dropped: %v (ok=%v)", p, ok)
	}
	// label was dropped, so it is absent.
	if _, ok := world.Get[label](dst, ents[0]); ok {
		t.Error("label should be absent after being dropped")
	}
}

func TestBuildSnapshotDropsTypeMissingFromRegistry(t *testing.T) {
	t.Parallel()
	// A world component whose type is NOT in the TypeRegistry cannot be restored,
	// so capture drops it (INV-2, capture side).
	w := world.NewWorld()
	posID := world.RegisterComponent[position](w)
	w.Spawn(component.Data{Value: position{X: 5, Y: 6}, ID: posID})
	emptyReg := typereg.NewTypeRegistry() // position not registered here

	snap := BuildSnapshot(scene.NewWorldAdapter(w), emptyReg, testEngineVersion, AppState{})
	var dropped bool
	for _, d := range snap.Dropped {
		if d.TypeName == posName && d.AffectedRows == 1 {
			dropped = true
		}
	}
	if !dropped {
		t.Errorf("unregistered position should be dropped at capture: %v", snap.Dropped)
	}
	// The entity is still captured (its ID is preserved even with no components).
	if len(snap.Entities) != 1 || len(snap.Entities[0].Components) != 0 {
		t.Errorf("entity should be captured component-less: %+v", snap.Entities)
	}
}
