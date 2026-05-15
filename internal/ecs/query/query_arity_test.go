package query_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/teratron/boltengine/internal/ecs/component"
	"github.com/teratron/boltengine/internal/ecs/query"
	"github.com/teratron/boltengine/internal/ecs/world"
)

// Test fixtures — kept in this _test package so they don't leak into the
// production query package.

type Position struct{ X, Y, Z float64 }
type Velocity struct{ DX, DY, DZ float64 }
type Health struct{ HP int }

// LootSparse is registered with SparseSet storage to exercise the alternate
// fetch path through queries.
type LootSparse struct{ Gold int }

// newWorldWithSparseLoot installs SparseSet storage for LootSparse before
// any spawn registers it as a default Table component.
func newWorldWithSparseLoot(t *testing.T) *world.World {
	t.Helper()
	w := world.NewWorld()
	w.Components().Register(component.Info{
		Type:    reflect.TypeFor[LootSparse](),
		Storage: component.StorageSparseSet,
	})
	return w
}

func TestQuery1Basic(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	e1 := w.Spawn(component.Data{Value: Position{X: 1}})
	e2 := w.Spawn(component.Data{Value: Position{X: 2}}, component.Data{Value: Velocity{DX: 5}})
	_ = w.Spawn(component.Data{Value: Velocity{DX: 99}}) // no Position — must NOT match

	q, err := query.NewQuery1[Position](w)
	if err != nil {
		t.Fatalf("NewQuery1 failed: %v", err)
	}

	seen := map[uint32]float64{}
	for e, p := range q.All(w) {
		if p == nil {
			t.Fatalf("Query1 yielded nil pointer for entity %v", e)
		}
		seen[e.ID().Index()] = p.X
	}

	if len(seen) != 2 {
		t.Fatalf("Query1 yielded %d entities, want 2 (got %v)", len(seen), seen)
	}
	if seen[e1.ID().Index()] != 1 || seen[e2.ID().Index()] != 2 {
		t.Fatalf("Query1 contents = %v, want {%d:1, %d:2}", seen,
			e1.ID().Index(), e2.ID().Index())
	}
}

func TestQuery1Mutation(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	e := w.Spawn(component.Data{Value: Position{X: 10}})

	q, err := query.NewQuery1[Position](w)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range q.All(w) {
		p.X = 42
	}

	got, ok := world.Get[Position](w, e)
	if !ok {
		t.Fatal("entity must still carry Position")
	}
	if got.X != 42 {
		t.Fatalf("after mutation Position.X = %v, want 42", got.X)
	}
}

func TestQuery1CountAndSingle(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	q, err := query.NewQuery1[Position](w)
	if err != nil {
		t.Fatal(err)
	}

	if got := q.Count(w); got != 0 {
		t.Fatalf("empty Count = %d, want 0", got)
	}
	if _, _, err := q.Single(w); !errors.Is(err, query.ErrQueryNoMatch) {
		t.Fatalf("empty Single err = %v, want ErrQueryNoMatch", err)
	}

	e := w.Spawn(component.Data{Value: Position{X: 7}})
	if got := q.Count(w); got != 1 {
		t.Fatalf("single Count = %d, want 1", got)
	}
	gotE, gotPtr, err := q.Single(w)
	if err != nil {
		t.Fatalf("Single err = %v", err)
	}
	if gotE != e {
		t.Fatalf("Single entity = %v, want %v", gotE, e)
	}
	if gotPtr.X != 7 {
		t.Fatalf("Single Position.X = %v, want 7", gotPtr.X)
	}

	w.Spawn(component.Data{Value: Position{X: 8}})
	if got := q.Count(w); got != 2 {
		t.Fatalf("Count after second spawn = %d, want 2", got)
	}
	if _, _, err := q.Single(w); !errors.Is(err, query.ErrQueryMultipleMatches) {
		t.Fatalf("multi Single err = %v, want ErrQueryMultipleMatches", err)
	}
}

func TestQuery1CacheGrowsWithArchetypes(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	q, err := query.NewQuery1[Position](w)
	if err != nil {
		t.Fatal(err)
	}

	if got := q.Count(w); got != 0 {
		t.Fatalf("initial Count = %d, want 0", got)
	}

	w.Spawn(component.Data{Value: Position{}})
	if got := q.Count(w); got != 1 {
		t.Fatalf("after first spawn Count = %d, want 1", got)
	}

	// New archetype with Position+Velocity must also be picked up.
	w.Spawn(component.Data{Value: Position{}}, component.Data{Value: Velocity{}})
	if got := q.Count(w); got != 2 {
		t.Fatalf("after second-archetype spawn Count = %d, want 2", got)
	}
}

func TestQuery1EarlyStop(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	for i := range 5 {
		w.Spawn(component.Data{Value: Position{X: float64(i)}})
	}
	q, err := query.NewQuery1[Position](w)
	if err != nil {
		t.Fatal(err)
	}

	count := 0
	for _, p := range q.All(w) {
		count++
		if p.X >= 2 {
			break
		}
	}
	if count == 0 || count >= 5 {
		t.Fatalf("early-stop count = %d, want 1..4", count)
	}
}

func TestQuery1OnSparseSetStorage(t *testing.T) {
	t.Parallel()

	w := newWorldWithSparseLoot(t)
	e := w.Spawn(component.Data{Value: LootSparse{Gold: 99}})

	q, err := query.NewQuery1[LootSparse](w)
	if err != nil {
		t.Fatal(err)
	}

	matched := false
	for ge, l := range q.All(w) {
		matched = true
		if ge != e {
			t.Fatalf("Query1 entity = %v, want %v", ge, e)
		}
		if l == nil || l.Gold != 99 {
			t.Fatalf("Query1 LootSparse = %+v, want {99}", l)
		}
	}
	if !matched {
		t.Fatal("Query1 over SparseSet storage yielded no entity")
	}
}

func TestQuery2Basic(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	e1 := w.Spawn(component.Data{Value: Position{X: 1}}, component.Data{Value: Velocity{DX: 10}})
	e2 := w.Spawn(component.Data{Value: Position{X: 2}}, component.Data{Value: Velocity{DX: 20}})
	w.Spawn(component.Data{Value: Position{X: 3}}) // missing Velocity — must NOT match

	q, err := query.NewQuery2[Position, Velocity](w)
	if err != nil {
		t.Fatal(err)
	}

	if got := q.Count(w); got != 2 {
		t.Fatalf("Query2 Count = %d, want 2", got)
	}

	seen := map[uint32]float64{}
	for e, t2 := range q.All(w) {
		if t2.A == nil || t2.B == nil {
			t.Fatalf("Query2 yielded nil pointers for %v", e)
		}
		seen[e.ID().Index()] = t2.A.X + t2.B.DX
	}
	if seen[e1.ID().Index()] != 11 || seen[e2.ID().Index()] != 22 {
		t.Fatalf("Query2 sums = %v, want {%d:11, %d:22}", seen,
			e1.ID().Index(), e2.ID().Index())
	}
}

func TestQuery2SameTypeRejected(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	if _, err := query.NewQuery2[Position, Position](w); err == nil {
		t.Fatal("NewQuery2 with identical type parameters must return error")
	}
}

func TestQuery2Mutation(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	e := w.Spawn(component.Data{Value: Position{X: 0}}, component.Data{Value: Velocity{DX: 5}})

	q, err := query.NewQuery2[Position, Velocity](w)
	if err != nil {
		t.Fatal(err)
	}
	for _, t2 := range q.All(w) {
		t2.A.X += t2.B.DX
	}

	pos, _ := world.Get[Position](w, e)
	if pos.X != 5 {
		t.Fatalf("after Query2 mutation Position.X = %v, want 5", pos.X)
	}
}

func TestQuery2EarlyStop(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	for i := range 5 {
		w.Spawn(component.Data{Value: Position{X: float64(i)}}, component.Data{Value: Velocity{}})
	}
	q, err := query.NewQuery2[Position, Velocity](w)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, t2 := range q.All(w) {
		count++
		if t2.A.X >= 1 {
			break
		}
	}
	if count == 0 || count >= 5 {
		t.Fatalf("early-stop count = %d, want 1..4", count)
	}
}

func TestQuery3Basic(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	e := w.Spawn(
		component.Data{Value: Position{X: 1}},
		component.Data{Value: Velocity{DX: 2}},
		component.Data{Value: Health{HP: 100}},
	)
	w.Spawn(component.Data{Value: Position{}}, component.Data{Value: Velocity{}}) // no Health
	w.Spawn(component.Data{Value: Health{HP: 50}})                                // only Health

	q, err := query.NewQuery3[Position, Velocity, Health](w)
	if err != nil {
		t.Fatal(err)
	}

	if got := q.Count(w); got != 1 {
		t.Fatalf("Query3 Count = %d, want 1", got)
	}

	matched := false
	for ge, t3 := range q.All(w) {
		matched = true
		if ge != e {
			t.Fatalf("Query3 yielded entity %v, want %v", ge, e)
		}
		if t3.A.X != 1 || t3.B.DX != 2 || t3.C.HP != 100 {
			t.Fatalf("Query3 tuple = %+v %+v %+v, want {1} {2} {100}",
				*t3.A, *t3.B, *t3.C)
		}
	}
	if !matched {
		t.Fatal("Query3 yielded no entity")
	}
}

func TestQuery3EarlyStop(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	for i := range 5 {
		w.Spawn(
			component.Data{Value: Position{X: float64(i)}},
			component.Data{Value: Velocity{}},
			component.Data{Value: Health{HP: i}},
		)
	}
	q, err := query.NewQuery3[Position, Velocity, Health](w)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, t3 := range q.All(w) {
		count++
		if t3.C.HP >= 1 {
			break
		}
	}
	if count == 0 || count >= 5 {
		t.Fatalf("Query3 early-stop count = %d, want 1..4", count)
	}
}

func TestQuery3SameTypeRejected(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	if _, err := query.NewQuery3[Position, Velocity, Position](w); err == nil {
		t.Fatal("NewQuery3 must reject duplicate type parameters (A==C)")
	}
	if _, err := query.NewQuery3[Position, Position, Velocity](w); err == nil {
		t.Fatal("NewQuery3 must reject A==B")
	}
	if _, err := query.NewQuery3[Position, Velocity, Velocity](w); err == nil {
		t.Fatal("NewQuery3 must reject B==C")
	}
}

func TestQueryStateAccessExposed(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	q1, _ := query.NewQuery1[Position](w)
	if q1.State() == nil {
		t.Fatal("Query1.State() must be non-nil")
	}
	if q1.State().Required().Count() != 1 {
		t.Fatalf("Query1 required count = %d, want 1", q1.State().Required().Count())
	}

	q2, _ := query.NewQuery2[Position, Velocity](w)
	if q2.State().Required().Count() != 2 {
		t.Fatalf("Query2 required count = %d, want 2", q2.State().Required().Count())
	}

	q3, _ := query.NewQuery3[Position, Velocity, Health](w)
	if q3.State().Required().Count() != 3 {
		t.Fatalf("Query3 required count = %d, want 3", q3.State().Required().Count())
	}

	// Required IDs must auto-promote to Read access.
	if q3.State().Access().Read.Count() != 3 {
		t.Fatalf("Query3 read count = %d, want 3", q3.State().Access().Read.Count())
	}
}
