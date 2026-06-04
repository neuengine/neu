package scene

import (
	"testing"

	"github.com/neuengine/neu/internal/ecs/query"
	"github.com/neuengine/neu/internal/ecs/typereg"
	"github.com/neuengine/neu/internal/ecs/world"
)

// Test component types for the hydrate path.
type hPosition struct {
	X, Y float64
}

type hLabel struct {
	Text string
}

// hydrateFixture registers the two component types in a TypeRegistry and builds a
// SerializedScene referencing them by their registered names, so the test is
// self-consistent regardless of the registry's naming convention.
func hydrateFixture(t *testing.T) (*typereg.TypeRegistry, SerializedScene) {
	t.Helper()
	reg := typereg.NewTypeRegistry()
	posName := typereg.RegisterType[hPosition](reg).Name
	lblName := typereg.RegisterType[hLabel](reg).Name

	// Names: [posType, lblType, "X", "Y", "Text"]; Variants: [1.0, 2.0, "hero"].
	sc := SerializedScene{
		Names:    []string{posName, lblName, "X", "Y", "Text"},
		Variants: []any{float64(1), float64(2), "hero"},
		Entities: []SerializedEntity{
			{Components: []SerializedComponent{
				{TypeIdx: 0, Props: [][2]int{{2, 0}, {3, 1}}}, // hPosition X=1 Y=2
				{TypeIdx: 1, Props: [][2]int{{4, 2}}},         // hLabel Text="hero"
			}},
			{Components: []SerializedComponent{
				{TypeIdx: 0, Props: [][2]int{{2, 1}, {3, 0}}}, // hPosition X=2 Y=1
			}},
		},
	}
	return reg, sc
}

func TestHydrateReconstructsComponents(t *testing.T) {
	t.Parallel()
	reg, sc := hydrateFixture(t)

	ds, errs := Hydrate(&sc, reg)
	if len(errs) != 0 {
		t.Fatalf("Hydrate errors: %v", errs)
	}
	ents := ds.Entities()
	if len(ents) != 2 {
		t.Fatalf("hydrated %d entities, want 2", len(ents))
	}
	// Entity 0: hPosition{1,2} + hLabel{"hero"}.
	if len(ents[0].Components) != 2 {
		t.Fatalf("entity 0 has %d components, want 2", len(ents[0].Components))
	}
	if pos, ok := ents[0].Components[0].Value.Interface().(hPosition); !ok || pos != (hPosition{X: 1, Y: 2}) {
		t.Errorf("entity 0 component 0 = %v (ok=%v), want hPosition{1 2}", ents[0].Components[0].Value, ok)
	}
	if lbl, ok := ents[0].Components[1].Value.Interface().(hLabel); !ok || lbl.Text != "hero" {
		t.Errorf("entity 0 component 1 = %v (ok=%v), want hLabel{hero}", ents[0].Components[1].Value, ok)
	}
	// Positional identity: entity i → EntityID(i+1).
	if ents[0].ID != 1 || ents[1].ID != 2 {
		t.Errorf("entity IDs = %d,%d, want 1,2 (positional)", ents[0].ID, ents[1].ID)
	}
}

func TestHydrateUnknownTypeIsCollected(t *testing.T) {
	t.Parallel()
	reg, sc := hydrateFixture(t)
	// Point entity 1's component at an unregistered type name.
	sc.Names = append(sc.Names, "game.Ghost")
	ghostIdx := len(sc.Names) - 1
	sc.Entities[1].Components[0].TypeIdx = ghostIdx

	ds, errs := Hydrate(&sc, reg)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for the unknown type, got %v", errs)
	}
	// The known entity still hydrates; the unknown component is skipped (partial).
	if len(ds.Entities()) != 2 || len(ds.Entities()[1].Components) != 0 {
		t.Errorf("unknown-type entity should hydrate with 0 components, got %d", len(ds.Entities()[1].Components))
	}
}

func TestHydrateUnconstructibleValueIsCollected(t *testing.T) {
	t.Parallel()
	reg, sc := hydrateFixture(t)
	// Give hPosition.X a string value — JSON cannot decode it into a float64,
	// so constructComponent must surface a collected error (not panic).
	sc.Variants = append(sc.Variants, "not-a-number")
	badIdx := len(sc.Variants) - 1
	sc.Entities[0].Components[0].Props[0][1] = badIdx // X = "not-a-number"

	ds, errs := Hydrate(&sc, reg)
	if len(errs) == 0 {
		t.Fatal("expected a collected error for the type-mismatched value")
	}
	// Entity 0's hPosition is skipped; its hLabel still hydrates.
	if got := len(ds.Entities()[0].Components); got != 1 {
		t.Errorf("entity 0 hydrated %d components, want 1 (bad hPosition skipped)", got)
	}
}

func TestHydrateNilInputs(t *testing.T) {
	t.Parallel()
	if ds, errs := Hydrate(nil, typereg.NewTypeRegistry()); len(ds.Entities()) != 0 || errs != nil {
		t.Error("nil scene should hydrate to an empty DynamicScene")
	}
	_, sc := hydrateFixture(t)
	if ds, errs := Hydrate(&sc, nil); len(ds.Entities()) != 0 || errs != nil {
		t.Error("nil registry should hydrate to an empty DynamicScene")
	}
}

func TestSpawnSerializedIntoWorld(t *testing.T) {
	t.Parallel()
	reg, sc := hydrateFixture(t)

	w := world.NewWorld()
	world.RegisterComponent[hPosition](w)
	world.RegisterComponent[hLabel](w)

	spawner := NewSceneSpawner()
	id, errs := spawner.SpawnSerialized(NewWorldAdapter(w), &sc, reg)
	if len(errs) != 0 {
		t.Fatalf("SpawnSerialized errors: %v", errs)
	}
	if id == 0 || len(spawner.InstanceEntities(id)) != 2 {
		t.Fatalf("instance %d has %d entities, want 2", id, len(spawner.InstanceEntities(id)))
	}
	// The world now holds 2 hPosition components and 1 hLabel.
	if n := countQuery1[hPosition](t, w); n != 2 {
		t.Errorf("world has %d hPosition, want 2", n)
	}
	if n := countQuery1[hLabel](t, w); n != 1 {
		t.Errorf("world has %d hLabel, want 1", n)
	}
}

func countQuery1[T any](t *testing.T, w *world.World) int {
	t.Helper()
	q, err := query.NewQuery1[T](w)
	if err != nil {
		t.Fatalf("NewQuery1: %v", err)
	}
	return q.Count(w)
}
