package world

import (
	"reflect"
	"testing"

	"github.com/neuengine/neu/internal/ecs/component"
)

type accPos struct{ X int }
type accSparse struct{ V int }

func TestArchetypesAccessor(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	if w.Archetypes() == nil {
		t.Fatal("Archetypes() must be non-nil")
	}
	if w.Archetypes() != w.archetypes {
		t.Fatal("Archetypes() must expose the same store")
	}
}

func TestArchetypeStoreAt(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	empty := w.Archetypes().At(0)
	if empty == nil {
		t.Fatal("At(0) must return the empty archetype")
	}
	if empty.ID() != 0 {
		t.Fatalf("At(0).ID() = %d, want 0", empty.ID())
	}
	if len(empty.ComponentIDs()) != 0 {
		t.Fatalf("empty archetype must have 0 components, got %d", len(empty.ComponentIDs()))
	}
}

func TestArchetypeStoreEach(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	w.Spawn(component.Data{Value: accPos{X: 1}})

	count := 0
	w.Archetypes().Each(func(a *Archetype) bool {
		count++
		return true
	})
	if count < 2 {
		t.Fatalf("Each must visit at least 2 archetypes (empty + Position), got %d", count)
	}

	// Early stop after first visit.
	visited := 0
	w.Archetypes().Each(func(a *Archetype) bool {
		visited++
		return false
	})
	if visited != 1 {
		t.Fatalf("Each early-stop visited %d archetypes, want 1", visited)
	}
}

func TestArchetypeStoreEachFrom(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	w.Spawn(component.Data{Value: accPos{X: 1}})
	total := w.Archetypes().Len()

	visited := 0
	w.Archetypes().EachFrom(1, func(a *Archetype) bool {
		visited++
		return true
	})
	if visited != total-1 {
		t.Fatalf("EachFrom(1) visited %d, want %d", visited, total-1)
	}

	// Early stop on a non-empty range.
	stopAfter := 0
	w.Archetypes().EachFrom(1, func(a *Archetype) bool {
		stopAfter++
		return false
	})
	if stopAfter != 1 {
		t.Fatalf("EachFrom early-stop visited %d, want 1", stopAfter)
	}

	// Out-of-range start is a no-op.
	w.Archetypes().EachFrom(total, func(a *Archetype) bool {
		t.Fatal("EachFrom past end must not visit anything")
		return false
	})
}

func TestSparseSetAccessor(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	w.Components().Register(component.Info{
		Type:    reflect.TypeFor[accSparse](),
		Storage: component.StorageSparseSet,
	})

	id, ok := w.Components().Lookup(reflect.TypeFor[accSparse]())
	if !ok {
		t.Fatal("accSparse must be registered")
	}

	// Before any sparse-set entity is spawned, accessor reports false.
	if _, ok := w.SparseSet(id); ok {
		t.Fatal("SparseSet() must return false before any spawn")
	}

	w.Spawn(component.Data{Value: accSparse{V: 7}})

	ss, ok := w.SparseSet(id)
	if !ok {
		t.Fatal("SparseSet() must report true after a sparse-stored spawn")
	}
	if ss == nil {
		t.Fatal("SparseSet() must return a non-nil set")
	}
	if ss.Len() != 1 {
		t.Fatalf("SparseSet().Len() = %d, want 1", ss.Len())
	}
}
