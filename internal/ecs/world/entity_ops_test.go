package world

import (
	"reflect"
	"testing"

	"github.com/teratron/boltengine/internal/ecs/component"
)

// Component fixtures for archetype tests.

type Position struct{ X, Y, Z float32 }
type Velocity struct{ DX, DY float32 }
type Damage struct{ Amount int }

// PlayerTag is a zero-size tag component (StorageTable + Size 0).
type PlayerTag struct{}

// Sparse-set stored component declared via a custom Info registration in the
// helper below.
type Loot struct{ Gold int }

// Required-graph fixtures.
type RootRequiresMid struct{ V int }

func (RootRequiresMid) Required() []component.Data {
	return []component.Data{{Value: MidRequiresLeaf{}}}
}

type MidRequiresLeaf struct{ V int }

func (MidRequiresLeaf) Required() []component.Data {
	return []component.Data{{Value: Leaf{}}}
}

type Leaf struct{ V int }

func newWorldWithSparseLoot(t *testing.T) (*World, component.ID) {
	t.Helper()
	w := NewWorld()
	id := w.Components().Register(component.Info{
		Type:    reflect.TypeFor[Loot](),
		Storage: component.StorageSparseSet,
	})
	return w, id
}

func TestSpawnSingleComponent(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	e := w.Spawn(component.NewData(w.Components(), Position{X: 1, Y: 2, Z: 3}))

	if !w.Contains(e) {
		t.Fatal("spawned entity must be alive")
	}
	got, ok := Get[Position](w, e)
	if !ok {
		t.Fatal("Get[Position] must succeed for spawned entity")
	}
	if got.X != 1 || got.Y != 2 || got.Z != 3 {
		t.Fatalf("Position = %+v, want {1,2,3}", *got)
	}
}

func TestSpawnMultipleComponents(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	e := w.Spawn(
		component.NewData(w.Components(), Position{X: 1}),
		component.NewData(w.Components(), Velocity{DX: 9, DY: 4}),
	)

	pos, _ := Get[Position](w, e)
	vel, _ := Get[Velocity](w, e)
	if pos.X != 1 {
		t.Fatalf("Position.X = %v, want 1", pos.X)
	}
	if vel.DX != 9 || vel.DY != 4 {
		t.Fatalf("Velocity = %+v, want {9,4}", *vel)
	}
}

func TestSpawnPlacesEntityInExpectedArchetype(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	a := w.Spawn(component.NewData(w.Components(), Position{}))
	b := w.Spawn(component.NewData(w.Components(), Position{}))
	c := w.Spawn(
		component.NewData(w.Components(), Position{}),
		component.NewData(w.Components(), Velocity{}),
	)

	recA := w.records[a.ID()]
	recB := w.records[b.ID()]
	recC := w.records[c.ID()]

	if recA.archetypeID != recB.archetypeID {
		t.Fatal("two entities with the same component set must share an archetype")
	}
	if recA.archetypeID == recC.archetypeID {
		t.Fatal("entities with different component sets must live in different archetypes")
	}
}

func TestSpawnEmptyLivesInEmptyArchetype(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	e := w.SpawnEmpty()
	rec := w.records[e.ID()]
	if rec.archetypeID != 0 {
		t.Fatalf("SpawnEmpty entity must be in archetype 0; got %d", rec.archetypeID)
	}
}

func TestDespawnRemovesFromArchetype(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	e := w.Spawn(component.NewData(w.Components(), Position{X: 7}))
	if err := w.Despawn(e); err != nil {
		t.Fatalf("Despawn: %v", err)
	}
	if w.Contains(e) {
		t.Fatal("entity must be dead after Despawn")
	}
	if _, ok := w.records[e.ID()]; ok {
		t.Fatal("entity record must be cleared on Despawn")
	}
}

func TestDespawnSwapAndPopUpdatesMovedRecord(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	a := w.Spawn(component.NewData(w.Components(), Position{X: 1}))
	b := w.Spawn(component.NewData(w.Components(), Position{X: 2}))
	c := w.Spawn(component.NewData(w.Components(), Position{X: 3}))

	if err := w.Despawn(a); err != nil {
		t.Fatalf("Despawn: %v", err)
	}

	// b and c must still resolve to their original X values after a was
	// swap-popped out of the archetype.
	pb, _ := Get[Position](w, b)
	pc, _ := Get[Position](w, c)
	if pb.X != 2 || pc.X != 3 {
		t.Fatalf("after swap-and-pop: B.X=%v, C.X=%v; want 2 and 3", pb.X, pc.X)
	}
}

func TestInsertNewComponentMigratesArchetype(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	e := w.Spawn(component.NewData(w.Components(), Position{X: 5}))
	startArch := w.records[e.ID()].archetypeID

	if err := w.Insert(e, component.NewData(w.Components(), Velocity{DX: 1})); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	endArch := w.records[e.ID()].archetypeID
	if startArch == endArch {
		t.Fatal("Insert with new component must migrate the entity to a new archetype")
	}

	pos, _ := Get[Position](w, e)
	vel, _ := Get[Velocity](w, e)
	if pos.X != 5 {
		t.Fatalf("Position.X must survive migration; got %v", pos.X)
	}
	if vel.DX != 1 {
		t.Fatalf("Velocity.DX must be set after migration; got %v", vel.DX)
	}
}

func TestInsertExistingComponentOverwritesInPlace(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	e := w.Spawn(component.NewData(w.Components(), Position{X: 1}))
	startArch := w.records[e.ID()].archetypeID

	if err := w.Insert(e, component.NewData(w.Components(), Position{X: 99})); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	endArch := w.records[e.ID()].archetypeID
	if startArch != endArch {
		t.Fatal("overwriting existing component must not migrate archetype")
	}
	pos, _ := Get[Position](w, e)
	if pos.X != 99 {
		t.Fatalf("overwrite failed; X=%v want 99", pos.X)
	}
}

func TestInsertOnDeadEntityReturnsError(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	e := w.SpawnEmpty()
	_ = w.Despawn(e)
	if err := w.Insert(e, component.NewData(w.Components(), Position{})); err != ErrEntityNotAlive {
		t.Fatalf("Insert on dead entity = %v, want ErrEntityNotAlive", err)
	}
}

func TestInsertEmptyVarargsIsNoOp(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	e := w.Spawn(component.NewData(w.Components(), Position{X: 1}))
	startArch := w.records[e.ID()].archetypeID

	if err := w.Insert(e); err != nil {
		t.Fatalf("Insert with no data: %v", err)
	}
	if w.records[e.ID()].archetypeID != startArch {
		t.Fatal("Insert() with no data must not migrate")
	}
}

func TestRemoveComponentMigratesArchetype(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	e := w.Spawn(
		component.NewData(w.Components(), Position{X: 5}),
		component.NewData(w.Components(), Velocity{DX: 9}),
	)
	startArch := w.records[e.ID()].archetypeID

	if err := Remove[Velocity](w, e); err != nil {
		t.Fatalf("Remove[Velocity]: %v", err)
	}
	endArch := w.records[e.ID()].archetypeID
	if startArch == endArch {
		t.Fatal("Remove must migrate the entity to a smaller archetype")
	}
	if _, ok := Get[Velocity](w, e); ok {
		t.Fatal("Velocity must be gone after Remove")
	}
	pos, ok := Get[Position](w, e)
	if !ok || pos.X != 5 {
		t.Fatalf("Position must survive Remove; got %+v ok=%v", pos, ok)
	}
}

func TestRemoveOnDeadEntityReturnsError(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	e := w.SpawnEmpty()
	_ = w.Despawn(e)
	if err := Remove[Position](w, e); err != ErrEntityNotAlive {
		t.Fatalf("Remove on dead entity = %v, want ErrEntityNotAlive", err)
	}
}

func TestRemoveUnregisteredTypeReturnsError(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	e := w.SpawnEmpty()
	if err := Remove[Damage](w, e); err != ErrComponentNotFound {
		t.Fatalf("Remove of unregistered type = %v, want ErrComponentNotFound", err)
	}
}

func TestRemoveAbsentComponentReturnsError(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	// Register Damage so Lookup succeeds, but the entity does not carry it.
	component.RegisterType[Damage](w.Components())
	e := w.Spawn(component.NewData(w.Components(), Position{}))

	if err := Remove[Damage](w, e); err != ErrComponentNotFound {
		t.Fatalf("Remove of absent component = %v, want ErrComponentNotFound", err)
	}
}

func TestGetReturnsFalseForUnregisteredType(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	e := w.SpawnEmpty()
	if _, ok := Get[Damage](w, e); ok {
		t.Fatal("Get for unregistered type must return ok=false")
	}
}

func TestGetReturnsFalseForDeadEntity(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	e := w.Spawn(component.NewData(w.Components(), Position{}))
	_ = w.Despawn(e)
	if _, ok := Get[Position](w, e); ok {
		t.Fatal("Get on dead entity must return ok=false")
	}
}

func TestGetReturnsFalseWhenEntityLacksComponent(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	e := w.Spawn(component.NewData(w.Components(), Position{}))
	component.RegisterType[Velocity](w.Components()) // registered but not on e
	if _, ok := Get[Velocity](w, e); ok {
		t.Fatal("Get for component not on entity must return ok=false")
	}
}

func TestGetMutationVisible(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	e := w.Spawn(component.NewData(w.Components(), Position{X: 0}))
	ptr, _ := Get[Position](w, e)
	ptr.X = 42

	got, _ := Get[Position](w, e)
	if got.X != 42 {
		t.Fatalf("mutation through Get pointer not visible; X=%v", got.X)
	}
}

func TestZeroSizeTagComponent(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	e := w.Spawn(component.NewData(w.Components(), PlayerTag{}))

	ptr, ok := Get[PlayerTag](w, e)
	if !ok {
		t.Fatal("zero-size tag component must report ok=true")
	}
	if ptr != nil {
		t.Fatalf("zero-size tag must yield nil pointer; got %p", ptr)
	}
}

func TestSparseSetComponentRoundTrip(t *testing.T) {
	t.Parallel()

	w, _ := newWorldWithSparseLoot(t)
	e := w.Spawn(
		component.NewData(w.Components(), Position{}),
		component.Data{ID: w.lookup(t, Loot{}), Value: Loot{Gold: 42}},
	)

	got, ok := Get[Loot](w, e)
	if !ok || got.Gold != 42 {
		t.Fatalf("sparse-set Loot = %+v ok=%v, want Gold=42", got, ok)
	}
}

func TestSparseSetSurvivesArchetypeMigration(t *testing.T) {
	t.Parallel()

	w, _ := newWorldWithSparseLoot(t)
	e := w.Spawn(
		component.NewData(w.Components(), Position{}),
		component.Data{ID: w.lookup(t, Loot{}), Value: Loot{Gold: 7}},
	)
	// Migrate by inserting a new table-stored component.
	if err := w.Insert(e, component.NewData(w.Components(), Velocity{DX: 1})); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	got, ok := Get[Loot](w, e)
	if !ok || got.Gold != 7 {
		t.Fatalf("sparse-set Loot must survive migration; got %+v ok=%v", got, ok)
	}
}

func TestRemoveSparseSetComponent(t *testing.T) {
	t.Parallel()

	w, _ := newWorldWithSparseLoot(t)
	e := w.Spawn(
		component.NewData(w.Components(), Position{}),
		component.Data{ID: w.lookup(t, Loot{}), Value: Loot{Gold: 99}},
	)

	if err := Remove[Loot](w, e); err != nil {
		t.Fatalf("Remove[Loot]: %v", err)
	}
	if _, ok := Get[Loot](w, e); ok {
		t.Fatal("Loot must be gone after Remove")
	}
}

func TestDespawnEvictsSparseSetComponent(t *testing.T) {
	t.Parallel()

	w, lootID := newWorldWithSparseLoot(t)
	e := w.Spawn(component.Data{ID: lootID, Value: Loot{Gold: 1}})
	_ = w.Despawn(e)

	ss := w.sparseSets[lootID]
	if ss != nil && ss.Len() != 0 {
		t.Fatalf("sparse set must be empty after Despawn; len=%d", ss.Len())
	}
}

func TestRequiredComponentsAutoInjected(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	e := w.Spawn(component.NewData(w.Components(), RootRequiresMid{V: 1}))

	if _, ok := Get[MidRequiresLeaf](w, e); !ok {
		t.Fatal("MidRequiresLeaf must be auto-injected")
	}
	if _, ok := Get[Leaf](w, e); !ok {
		t.Fatal("Leaf must be transitively auto-injected")
	}
}

func TestArchetypeStoreGenerationBumpsOnNewArchetype(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	gen0 := w.archetypes.Generation()
	w.Spawn(component.NewData(w.Components(), Position{}))
	gen1 := w.archetypes.Generation()
	if gen1 == gen0 {
		t.Fatal("generation must bump when a new archetype is created")
	}
	// Same shape — no new archetype.
	w.Spawn(component.NewData(w.Components(), Position{}))
	if w.archetypes.Generation() != gen1 {
		t.Fatal("generation must NOT bump for an existing archetype")
	}
}

func TestArchetypeAccessors(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	e := w.Spawn(component.NewData(w.Components(), Position{}))
	rec := w.records[e.ID()]
	arch := w.archetypes.get(rec.archetypeID)

	if arch.ID() != rec.archetypeID {
		t.Fatalf("Archetype.ID() = %d, want %d", arch.ID(), rec.archetypeID)
	}
	if arch.Len() != 1 {
		t.Fatalf("Archetype.Len() = %d, want 1", arch.Len())
	}
	if len(arch.ComponentIDs()) != 1 {
		t.Fatalf("ComponentIDs len = %d, want 1", len(arch.ComponentIDs()))
	}
	if len(arch.Entities()) != 1 || arch.Entities()[0] != e {
		t.Fatalf("Entities = %v, want [%v]", arch.Entities(), e)
	}
	if arch.Table() == nil {
		t.Fatal("Table must be non-nil for table-stored archetype")
	}
	if !arch.Has(arch.ComponentIDs()[0]) {
		t.Fatal("Has must return true for own component")
	}
}

// lookup is a helper that resolves the component ID for a value's reflect
// type, failing the test if the type is not registered.
func (w *World) lookup(t *testing.T, sample any) component.ID {
	t.Helper()
	id, ok := w.Components().Lookup(reflect.TypeOf(sample))
	if !ok {
		t.Fatalf("component for %T must be registered", sample)
	}
	return id
}
