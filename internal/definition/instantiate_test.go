package definition

import (
	"reflect"
	"testing"

	"github.com/neuengine/neu/internal/ecs/hierarchy"
	"github.com/neuengine/neu/internal/ecs/world"
	def "github.com/neuengine/neu/pkg/definition"
)

// Test component types. encoding/json matches "x"/"text" to these fields
// case-insensitively, so no struct tags are needed.
type Position struct{ X, Y float64 }
type Label struct{ Text string }

// validatorResolver answers the definition's validation-time question ("does
// this type name exist?"). Distinct from the realization TypeResolver below.
type validatorResolver struct{ known map[string]bool }

func (r validatorResolver) ResolveType(name string) bool { return r.known[name] }

func realizeResolver(m map[string]reflect.Type) TypeResolver {
	return func(name string) (reflect.Type, bool) { t, ok := m[name]; return t, ok }
}

func decodeScene(t *testing.T, validNames []string, data string) def.Definition {
	t.Helper()
	known := make(map[string]bool, len(validNames))
	for _, n := range validNames {
		known[n] = true
	}
	d, err := def.Decode([]byte(data), validatorResolver{known: known}, def.NewActionRegistry())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return d
}

const sceneJSON = `{"definition":"scene","version":"1","content":{"entities":[
  {"name":"player","components":{"Position":{"x":1,"y":2},"Label":{"text":"hero"}},
   "children":[{"name":"weapon","components":{"Position":{"x":3,"y":4}}}]}
]}}`

func TestInstantiateSpawnsEntitiesComponentsAndParent(t *testing.T) {
	d := decodeScene(t, []string{"Position", "Label"}, sceneJSON)
	resolve := realizeResolver(map[string]reflect.Type{
		"Position": reflect.TypeFor[Position](),
		"Label":    reflect.TypeFor[Label](),
	})
	w := world.NewWorld()

	in, errs := Instantiate(w, &d, resolve)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(in.Entities) != 2 {
		t.Fatalf("spawned %d entities, want 2", len(in.Entities))
	}
	player, weapon := in.Entities[0], in.Entities[1]

	if pos, ok := world.Get[Position](w, player); !ok || *pos != (Position{X: 1, Y: 2}) {
		t.Errorf("player Position = %v (ok=%v), want {1 2}", pos, ok)
	}
	if lbl, ok := world.Get[Label](w, player); !ok || lbl.Text != "hero" {
		t.Errorf("player Label = %v (ok=%v), want hero", lbl, ok)
	}
	if pos, ok := world.Get[Position](w, weapon); !ok || *pos != (Position{X: 3, Y: 4}) {
		t.Errorf("weapon Position = %v (ok=%v), want {3 4}", pos, ok)
	}
	if co, ok := world.Get[hierarchy.ChildOf](w, weapon); !ok || co.Parent != player {
		t.Errorf("weapon ChildOf.Parent = %v (ok=%v), want player %v", co, ok, player)
	}
}

func TestInstantiateUnknownRealizationTypeCollectsError(t *testing.T) {
	// Validation knows "Ghost" (passes Decode) but the realization resolver does
	// not — the adapter must collect a boundary error, not panic or drop silently.
	data := `{"definition":"scene","version":"1","content":{"entities":[
	  {"name":"e","components":{"Ghost":{"a":1}}}]}}`
	d := decodeScene(t, []string{"Ghost"}, data)
	resolve := realizeResolver(map[string]reflect.Type{}) // empty

	w := world.NewWorld()
	in, errs := Instantiate(w, &d, resolve)
	if len(in.Entities) != 1 {
		t.Errorf("entity should still spawn; got %d", len(in.Entities))
	}
	if len(errs) != 1 {
		t.Fatalf("got %d errors, want 1 (unknown realization type)", len(errs))
	}
}

func TestInstantiateCollectsFlowActions(t *testing.T) {
	data := `{"definition":"flow","version":"1","content":{
	  "initial_state":"menu","states":{"menu":{"on_enter":[{"action":"log","message":"hi"}]}}}}`
	d := decodeScene(t, nil, data) // flow has no component types to validate
	w := world.NewWorld()

	in, errs := Instantiate(w, &d, realizeResolver(nil))
	if len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	if len(in.Entities) != 0 {
		t.Errorf("flow should spawn no entities, got %d", len(in.Entities))
	}
	if len(in.Actions) != 1 || in.Actions[0].Action != "log" {
		t.Fatalf("collected actions = %+v, want one log action", in.Actions)
	}
}

func TestInstanceDespawn(t *testing.T) {
	d := decodeScene(t, []string{"Position", "Label"}, sceneJSON)
	resolve := realizeResolver(map[string]reflect.Type{
		"Position": reflect.TypeFor[Position](),
		"Label":    reflect.TypeFor[Label](),
	})
	w := world.NewWorld()
	in, _ := Instantiate(w, &d, resolve)

	in.Despawn(w)
	for _, e := range in.Entities {
		if w.Contains(e) {
			t.Errorf("entity %v still alive after Despawn", e)
		}
	}
	in.Despawn(w) // idempotent — must not panic
}

func TestInstantiateCoercionErrorCollected(t *testing.T) {
	// Validation passes ("x" is a valid JSON string), but the string won't decode
	// into Position.X (float64) — the construct round-trip must surface the error.
	data := `{"definition":"scene","version":"1","content":{"entities":[
	  {"name":"e","components":{"Position":{"x":"oops","y":2}}}]}}`
	d := decodeScene(t, []string{"Position"}, data)
	resolve := realizeResolver(map[string]reflect.Type{"Position": reflect.TypeFor[Position]()})

	w := world.NewWorld()
	in, errs := Instantiate(w, &d, resolve)
	if len(in.Entities) != 1 {
		t.Errorf("entity should spawn even if a component fails; got %d", len(in.Entities))
	}
	if len(errs) != 1 {
		t.Fatalf("got %d errors, want 1 (coercion failure)", len(errs))
	}
}

// TestWorldSinkBadRefDefensive exercises the defensive ref-bounds guards that
// definition.Instantiate never triggers (it always spawns before referencing).
func TestWorldSinkBadRefDefensive(t *testing.T) {
	s := &worldSink{w: world.NewWorld(), resolve: realizeResolver(nil)}
	s.SetParent(0, 1)              // no entities spawned → both refs invalid
	s.InsertComponent(3, "X", nil) // ref out of range
	if len(s.errs) == 0 {
		t.Error("expected boundary errors for out-of-range entity refs")
	}
}

func TestInstanceStoreReloadReplacesEntities(t *testing.T) {
	d := decodeScene(t, []string{"Position", "Label"}, sceneJSON)
	resolve := realizeResolver(map[string]reflect.Type{
		"Position": reflect.TypeFor[Position](),
		"Label":    reflect.TypeFor[Label](),
	})
	w := world.NewWorld()
	store := NewInstanceStore()

	in1, _ := store.Instantiate(w, "/scene.json", &d, resolve)
	if _, ok := store.Get("/scene.json"); !ok {
		t.Fatal("store did not record the instance")
	}

	in2, _ := store.Reload(w, "/scene.json", &d, resolve)

	for _, e := range in1.Entities {
		if w.Contains(e) {
			t.Errorf("reload did not despawn old entity %v", e)
		}
	}
	for _, e := range in2.Entities {
		if !w.Contains(e) {
			t.Errorf("reload did not spawn new entity %v", e)
		}
	}
}
