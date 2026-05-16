package world

import (
	"errors"
	"reflect"
	"testing"

	"github.com/neuengine/neu/internal/ecs/component"
)

func TestSpawnWithEntity_ParksReservedID(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	e := w.Entities().Allocate()
	w.SpawnWithEntity(e)

	if !w.Contains(e) {
		t.Fatal("entity must be alive after SpawnWithEntity")
	}
	rec, ok := w.records[e.ID()]
	if !ok {
		t.Fatal("entityRecord missing after SpawnWithEntity")
	}
	if rec.archetypeID != 0 {
		t.Fatalf("archetypeID = %d, want 0 (empty archetype)", rec.archetypeID)
	}
}

func TestSpawnWithEntityAndData_NoData(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	e := w.Entities().Allocate()
	w.SpawnWithEntityAndData(e)

	if !w.Contains(e) {
		t.Fatal("entity must be alive when called with no data")
	}
	rec := w.records[e.ID()]
	if rec.archetypeID != 0 {
		t.Fatalf("no-data spawn must land in empty archetype, got %d", rec.archetypeID)
	}
}

func TestSpawnWithEntityAndData_WithComponents(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	e := w.Entities().Allocate()
	w.SpawnWithEntityAndData(e, component.Data{Value: Position{X: 1, Y: 2, Z: 3}})

	if !w.Contains(e) {
		t.Fatal("entity must be alive after SpawnWithEntityAndData")
	}
	ptr, ok := Get[Position](w, e)
	if !ok || ptr == nil {
		t.Fatal("Position must be present after SpawnWithEntityAndData")
	}
	if ptr.X != 1 || ptr.Y != 2 || ptr.Z != 3 {
		t.Fatalf("unexpected Position: %+v", ptr)
	}
}

func TestRemoveByID_Success(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	e := w.Spawn(
		component.Data{Value: Position{X: 4}},
		component.Data{Value: Velocity{DX: 5}},
	)
	posID, _ := w.Components().Lookup(typeOf(Position{}))

	if err := RemoveByID(w, e, posID); err != nil {
		t.Fatalf("RemoveByID err = %v", err)
	}
	if _, ok := Get[Position](w, e); ok {
		t.Fatal("Position must be absent after RemoveByID")
	}
	// Velocity must still be present.
	if vel, ok := Get[Velocity](w, e); !ok || vel.DX != 5 {
		t.Fatal("Velocity must survive RemoveByID")
	}
}

func TestRemoveByID_DeadEntity(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	e := w.Spawn(component.Data{Value: Position{}})
	posID, _ := w.Components().Lookup(typeOf(Position{}))
	_ = w.Despawn(e)

	if err := RemoveByID(w, e, posID); !errors.Is(err, ErrEntityNotAlive) {
		t.Fatalf("err = %v, want ErrEntityNotAlive", err)
	}
}

func TestRemoveByID_ComponentNotPresent(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	e := w.Spawn(component.Data{Value: Position{}})
	// Register Velocity but do not attach it to e.
	velID := w.Components().RegisterByType(typeOf(Velocity{}))

	if err := RemoveByID(w, e, velID); !errors.Is(err, ErrComponentNotFound) {
		t.Fatalf("err = %v, want ErrComponentNotFound", err)
	}
}

func TestRemoveByID_SparseSetComponent(t *testing.T) {
	t.Parallel()

	w, lootID := newWorldWithSparseLoot(t)
	e := w.Spawn(component.Data{Value: Loot{Gold: 42}})

	if err := RemoveByID(w, e, lootID); err != nil {
		t.Fatalf("RemoveByID err = %v", err)
	}
	if _, ok := Get[Loot](w, e); ok {
		t.Fatal("sparse-set Loot must be evicted after RemoveByID")
	}
}

// typeOf is a local helper that avoids re-importing reflect at every call site.
func typeOf(v any) reflect.Type { return reflect.TypeOf(v) }
