package world

import (
	"testing"

	"github.com/teratron/boltengine/internal/ecs/component"
)

func TestArchetypeStore_OnArchetypeCreated_FiresOnNewArch(t *testing.T) {
	t.Parallel()
	w := NewWorld()
	calls := 0
	id := w.Archetypes().OnArchetypeCreated(func(*Archetype) { calls++ })
	if id == 0 {
		t.Fatal("OnArchetypeCreated must return a non-zero ListenerID")
	}

	w.Spawn(component.Data{Value: Position{}})
	w.Spawn(component.Data{Value: Position{}}, component.Data{Value: Velocity{}})
	if calls != 2 {
		t.Fatalf("listener fired %d times, want 2", calls)
	}

	// Reusing an existing archetype must NOT fire.
	w.Spawn(component.Data{Value: Position{}})
	if calls != 2 {
		t.Fatalf("listener fired on archetype reuse: %d", calls)
	}
}

func TestArchetypeStore_OnArchetypeCreated_NilFnReturnsZero(t *testing.T) {
	t.Parallel()
	w := NewWorld()
	if id := w.Archetypes().OnArchetypeCreated(nil); id != 0 {
		t.Fatalf("nil listener must yield 0; got %d", id)
	}
}

func TestArchetypeStore_UnregisterListener(t *testing.T) {
	t.Parallel()
	w := NewWorld()
	calls := 0
	id := w.Archetypes().OnArchetypeCreated(func(*Archetype) { calls++ })
	w.Spawn(component.Data{Value: Position{}})
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
	w.Archetypes().UnregisterListener(id)
	w.Spawn(component.Data{Value: Position{}}, component.Data{Value: Velocity{}})
	if calls != 1 {
		t.Fatalf("listener fired after unregister: %d", calls)
	}
}

func TestArchetypeStore_UnregisterListener_SentinelNoOp(t *testing.T) {
	t.Parallel()
	w := NewWorld()
	w.Archetypes().UnregisterListener(0)        // sentinel
	w.Archetypes().UnregisterListener(99999999) // unknown — no-op
}

func TestWorld_ArchetypeOf_AfterSpawn(t *testing.T) {
	t.Parallel()
	w := NewWorld()
	e := w.Spawn(component.Data{Value: Position{}})
	id, ok := w.ArchetypeOf(e)
	if !ok {
		t.Fatal("ArchetypeOf must succeed for live spawned entity")
	}
	if id == 0 {
		t.Fatal("non-empty entity must NOT live in archetype 0")
	}
}

func TestWorld_ArchetypeOf_Reserved(t *testing.T) {
	t.Parallel()
	w := NewWorld()
	e := w.entities.Allocate()
	if _, ok := w.ArchetypeOf(e); ok {
		t.Fatal("ArchetypeOf must be false for reserved-but-not-spawned entity")
	}
}

func TestWorld_ArchetypeOf_SpawnEmptyIsArchetype0(t *testing.T) {
	t.Parallel()
	w := NewWorld()
	e := w.SpawnEmpty()
	id, ok := w.ArchetypeOf(e)
	if !ok {
		t.Fatal("ArchetypeOf must succeed for SpawnEmpty entity")
	}
	if id != 0 {
		t.Fatalf("SpawnEmpty must place entity in archetype 0, got %d", id)
	}
}
