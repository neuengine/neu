package query_test

import (
	"testing"

	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/query"
	"github.com/neuengine/neu/internal/ecs/world"
)

// Extra Table components for the generated higher-arity queries (Position,
// Velocity, Health are defined in query_arity_test.go).
type Mana struct{ MP int }
type Armor struct{ AC int }

func TestQuery4Generated(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	e := w.Spawn(
		component.Data{Value: Position{X: 1}},
		component.Data{Value: Velocity{DX: 2}},
		component.Data{Value: Health{HP: 3}},
		component.Data{Value: Mana{MP: 4}},
	)
	w.Spawn(component.Data{Value: Position{}}, component.Data{Value: Velocity{}}, component.Data{Value: Health{}}) // no Mana

	q, err := query.NewQuery4[Position, Velocity, Health, Mana](w)
	if err != nil {
		t.Fatalf("NewQuery4: %v", err)
	}
	if got := q.Count(w); got != 1 {
		t.Fatalf("Query4 Count = %d, want 1", got)
	}
	matched := false
	for ge, t4 := range q.All(w) {
		matched = true
		if ge != e {
			t.Fatalf("Query4 entity = %v, want %v", ge, e)
		}
		if t4.A.X != 1 || t4.B.DX != 2 || t4.C.HP != 3 || t4.D.MP != 4 {
			t.Fatalf("Query4 tuple = {%v %v %v %v}, want {1 2 3 4}", *t4.A, *t4.B, *t4.C, *t4.D)
		}
		t4.D.MP = 40 // mutate through the pointer
	}
	if !matched {
		t.Fatal("Query4 yielded no entity")
	}
	if m, _ := world.Get[Mana](w, e); m.MP != 40 {
		t.Fatalf("Query4 mutation not persisted: Mana.MP = %d, want 40", m.MP)
	}
}

func TestQuery5Generated(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	e := w.Spawn(
		component.Data{Value: Position{X: 1}},
		component.Data{Value: Velocity{DX: 2}},
		component.Data{Value: Health{HP: 3}},
		component.Data{Value: Mana{MP: 4}},
		component.Data{Value: Armor{AC: 5}},
	)
	q, err := query.NewQuery5[Position, Velocity, Health, Mana, Armor](w)
	if err != nil {
		t.Fatalf("NewQuery5: %v", err)
	}
	if got := q.Count(w); got != 1 {
		t.Fatalf("Query5 Count = %d, want 1", got)
	}
	matched := false
	for ge, t5 := range q.All(w) {
		matched = true
		if ge != e {
			t.Fatalf("Query5 entity = %v, want %v", ge, e)
		}
		if t5.A.X != 1 || t5.B.DX != 2 || t5.C.HP != 3 || t5.D.MP != 4 || t5.E.AC != 5 {
			t.Fatalf("Query5 tuple = {%v %v %v %v %v}, want {1 2 3 4 5}", *t5.A, *t5.B, *t5.C, *t5.D, *t5.E)
		}
	}
	if !matched {
		t.Fatal("Query5 yielded no entity")
	}
}

func TestQuery4SameTypeRejected(t *testing.T) {
	t.Parallel()
	w := world.NewWorld()
	if _, err := query.NewQuery4[Position, Velocity, Health, Position](w); err == nil {
		t.Fatal("NewQuery4 must reject duplicate type parameters (A==D)")
	}
}
