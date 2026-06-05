package worldsnap

import (
	"testing"

	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/typereg"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/scene"
)

type position struct{ X, Y float64 }
type label struct{ Text string }

// sourceWorld builds a World with two registered components + a matching
// TypeRegistry, spawning entities whose IDs the round-trip must preserve.
func sourceWorld(t *testing.T) (*world.World, *typereg.TypeRegistry, []entity.Entity) {
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

func freshWorld(t *testing.T) *world.World {
	t.Helper()
	w := world.NewWorld()
	world.RegisterComponent[position](w)
	world.RegisterComponent[label](w)
	return w
}

func TestCaptureRestorePreservesIDs(t *testing.T) {
	t.Parallel()
	src, reg, ents := sourceWorld(t)
	captured, types, dropped := Capture(scene.NewWorldAdapter(src), reg)
	if len(captured) != 2 {
		t.Fatalf("captured %d entities, want 2", len(captured))
	}
	if len(dropped) != 0 {
		t.Errorf("unexpected drops: %v", dropped)
	}
	if len(types) == 0 {
		t.Error("componentTypes should list the captured types")
	}

	dst := freshWorld(t)
	res := Restore(dst, captured, reg)
	if res.Restored != 2 {
		t.Fatalf("restored %d, want 2", res.Restored)
	}
	// INV: each original EntityID is alive in the restored world (pinned).
	for _, e := range ents {
		if !dst.Contains(e) {
			t.Errorf("entity id=%d not preserved", e.ID())
		}
	}
	if p, ok := world.Get[position](dst, ents[0]); !ok || *p != (position{X: 1, Y: 2}) {
		t.Errorf("entity0 position = %v (ok=%v)", p, ok)
	}
	if l, ok := world.Get[label](dst, ents[0]); !ok || l.Text != "hero" {
		t.Errorf("entity0 label = %v (ok=%v)", l, ok)
	}
	if p, ok := world.Get[position](dst, ents[1]); !ok || *p != (position{X: 3, Y: 4}) {
		t.Errorf("entity1 position = %v (ok=%v)", p, ok)
	}
}

func TestCaptureDropsUnregisteredType(t *testing.T) {
	t.Parallel()
	w := world.NewWorld()
	posID := world.RegisterComponent[position](w)
	w.Spawn(component.Data{Value: position{X: 5, Y: 6}, ID: posID})
	emptyReg := typereg.NewTypeRegistry() // position not registered

	captured, _, dropped := Capture(scene.NewWorldAdapter(w), emptyReg)
	if len(dropped) != 1 || dropped[0].AffectedRows != 1 {
		t.Errorf("expected one dropped type, got %v", dropped)
	}
	// The entity is still captured (its ID survives, component-less).
	if len(captured) != 1 || len(captured[0].Components) != 0 {
		t.Errorf("entity should be captured component-less: %+v", captured)
	}
}

func TestRestoreDropsTypeMissingOnRestore(t *testing.T) {
	t.Parallel()
	src, reg, ents := sourceWorld(t)
	captured, _, _ := Capture(scene.NewWorldAdapter(src), reg)

	// Restore against a registry missing 'label' → label dropped, position kept.
	reg2 := typereg.NewTypeRegistry()
	typereg.RegisterType[position](reg2)
	dst := freshWorld(t)
	res := Restore(dst, captured, reg2)
	if len(res.Dropped) != 1 {
		t.Fatalf("expected one dropped type on restore, got %v", res.Dropped)
	}
	if p, ok := world.Get[position](dst, ents[0]); !ok || *p != (position{X: 1, Y: 2}) {
		t.Errorf("position should still restore: %v ok=%v", p, ok)
	}
	if _, ok := world.Get[label](dst, ents[0]); ok {
		t.Error("dropped label should be absent")
	}
}

func TestRestoreEmptyAndNullSafe(t *testing.T) {
	t.Parallel()
	reg := typereg.NewTypeRegistry()
	dst := freshWorld(t)
	// Empty snapshot restores nothing.
	if res := Restore(dst, nil, reg); res.Restored != 0 {
		t.Errorf("empty restore = %d, want 0", res.Restored)
	}
	// A null-ID entity snapshot is skipped, not restored.
	if res := Restore(dst, []EntitySnapshot{{ID: 0}}, reg); res.Restored != 0 {
		t.Errorf("null-ID entity should be skipped, restored %d", res.Restored)
	}
}
