package query_test

import (
	"sync/atomic"
	"testing"

	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/query"
	"github.com/neuengine/neu/internal/ecs/world"
)

// fixture component types unique to this test file (kept distinct from
// query_arity_test.go fixtures to make failures easier to attribute).
type fPos struct{ X float64 }
type fVel struct{ DX float64 }
type fHP struct{ V int }
type fLabel struct{}

func TestWithFilterNarrowsArchetypes(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	w.Spawn(component.Data{Value: fPos{X: 1}})
	w.Spawn(component.Data{Value: fPos{X: 2}}, component.Data{Value: fVel{DX: 5}})

	q, err := query.NewQuery1[fPos](w, query.With[fVel]{})
	if err != nil {
		t.Fatal(err)
	}
	if got := q.Count(w); got != 1 {
		t.Fatalf("Query1[fPos] With[fVel] Count = %d, want 1 (only the Pos+Vel entity)", got)
	}

	matched := 0
	for _, p := range q.All(w) {
		matched++
		if p.X != 2 {
			t.Fatalf("With[fVel] yielded fPos.X = %v, want 2", p.X)
		}
	}
	if matched != 1 {
		t.Fatalf("With filter visit count = %d, want 1", matched)
	}
}

func TestWithoutFilterExcludesArchetypes(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	w.Spawn(component.Data{Value: fPos{X: 1}})
	w.Spawn(component.Data{Value: fPos{X: 2}}, component.Data{Value: fVel{DX: 5}})

	q, err := query.NewQuery1[fPos](w, query.Without[fVel]{})
	if err != nil {
		t.Fatal(err)
	}
	if got := q.Count(w); got != 1 {
		t.Fatalf("Without[fVel] Count = %d, want 1 (Pos-only entity)", got)
	}
	for _, p := range q.All(w) {
		if p.X != 1 {
			t.Fatalf("Without[fVel] yielded fPos.X = %v, want 1", p.X)
		}
	}
}

func TestWithAndWithoutCombined(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	w.Spawn(component.Data{Value: fPos{X: 1}}) // Pos only
	w.Spawn(component.Data{Value: fPos{X: 2}}, component.Data{Value: fVel{DX: 1}})
	w.Spawn(component.Data{Value: fPos{X: 3}}, component.Data{Value: fVel{DX: 1}}, component.Data{Value: fHP{V: 9}})

	// Want: Pos+Vel but NOT HP → only entity 2 (X=2).
	q, err := query.NewQuery1[fPos](w, query.With[fVel]{}, query.Without[fHP]{})
	if err != nil {
		t.Fatal(err)
	}
	if got := q.Count(w); got != 1 {
		t.Fatalf("With[fVel]+Without[fHP] Count = %d, want 1", got)
	}
	for _, p := range q.All(w) {
		if p.X != 2 {
			t.Fatalf("composed filters yielded X = %v, want 2", p.X)
		}
	}
}

func TestQuery2WithFilter(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	w.Spawn(component.Data{Value: fPos{X: 1}}, component.Data{Value: fVel{DX: 1}})
	w.Spawn(component.Data{Value: fPos{X: 2}}, component.Data{Value: fVel{DX: 1}}, component.Data{Value: fHP{V: 9}})

	q, err := query.NewQuery2[fPos, fVel](w, query.With[fHP]{})
	if err != nil {
		t.Fatal(err)
	}
	if got := q.Count(w); got != 1 {
		t.Fatalf("Query2 With[fHP] Count = %d, want 1", got)
	}
}

func TestQuery3WithoutFilter(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	w.Spawn(
		component.Data{Value: fPos{}},
		component.Data{Value: fVel{}},
		component.Data{Value: fHP{V: 1}},
	)
	w.Spawn(
		component.Data{Value: fPos{}},
		component.Data{Value: fVel{}},
		component.Data{Value: fHP{V: 2}},
		component.Data{Value: fLabel{}},
	)

	q, err := query.NewQuery3[fPos, fVel, fHP](w, query.Without[fLabel]{})
	if err != nil {
		t.Fatal(err)
	}
	if got := q.Count(w); got != 1 {
		t.Fatalf("Query3 Without[fLabel] Count = %d, want 1", got)
	}
}

// TestAddedAndChangedScaffoldBehavior locks the Phase 1 scaffold contract
// for [Added] and [Changed]: archetype-level matching is enforced, the
// per-row tick check is a no-op (passes everything). When change-detection
// lands in Phase 2, this test must be updated to assert tick semantics.
func TestAddedAndChangedScaffoldBehavior(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	w.Spawn(component.Data{Value: fPos{X: 1}})
	w.Spawn(component.Data{Value: fPos{X: 2}}, component.Data{Value: fVel{DX: 5}})

	added, err := query.NewQuery1[fPos](w, query.Added[fVel]{})
	if err != nil {
		t.Fatal(err)
	}
	if got := added.Count(w); got != 1 {
		t.Fatalf("Added[fVel] archetype-match count = %d, want 1 (Phase 1 scaffold)", got)
	}

	changed, err := query.NewQuery1[fPos](w, query.Changed[fVel]{})
	if err != nil {
		t.Fatal(err)
	}
	if got := changed.Count(w); got != 1 {
		t.Fatalf("Changed[fVel] archetype-match count = %d, want 1 (Phase 1 scaffold)", got)
	}
}

func TestNilFilterIgnored(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	w.Spawn(component.Data{Value: fPos{X: 1}})
	q, err := query.NewQuery1[fPos](w, nil, query.With[fPos]{})
	if err != nil {
		t.Fatal(err)
	}
	if got := q.Count(w); got != 1 {
		t.Fatalf("nil-filter-tolerant Count = %d, want 1", got)
	}
}

func TestFilterImpossibleQueryReturnsNoMatches(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	w.Spawn(component.Data{Value: fPos{X: 1}}, component.Data{Value: fVel{DX: 1}})

	// require fPos AND not fPos — impossible.
	q, err := query.NewQuery1[fPos](w, query.Without[fPos]{})
	if err != nil {
		t.Fatal(err)
	}
	if got := q.Count(w); got != 0 {
		t.Fatalf("impossible filter set Count = %d, want 0", got)
	}
	for range q.All(w) {
		t.Fatal("impossible filter set must yield no entities")
	}
}

func TestFilterCountMatchesIteration(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	for i := range 10 {
		w.Spawn(component.Data{Value: fPos{X: float64(i)}}, component.Data{Value: fVel{}})
	}
	for i := range 7 {
		w.Spawn(component.Data{Value: fPos{X: float64(100 + i)}})
	}

	q, err := query.NewQuery1[fPos](w, query.With[fVel]{})
	if err != nil {
		t.Fatal(err)
	}
	count := q.Count(w)
	visited := 0
	for range q.All(w) {
		visited++
	}
	if count != visited {
		t.Fatalf("Count (%d) and iteration visit count (%d) disagree", count, visited)
	}
	if count != 10 {
		t.Fatalf("Count = %d, want 10", count)
	}
}

// TestParIterVisitsEveryEntityOnce uses an atomic counter to confirm that
// every matched entity is visited exactly once across all goroutines.
func TestParIterVisitsEveryEntityOnce(t *testing.T) {
	t.Parallel()

	const total = 5_000
	w := world.NewWorld()
	for i := range total {
		w.Spawn(component.Data{Value: fPos{X: float64(i)}})
	}

	q, err := query.NewQuery1[fPos](w)
	if err != nil {
		t.Fatal(err)
	}

	var visited int64
	q.ParIter(w, func(_ entity.Entity, p *fPos) {
		atomic.AddInt64(&visited, 1)
		_ = p.X
	})

	if visited != total {
		t.Fatalf("ParIter visited %d entities, want %d", visited, total)
	}
}

func TestParIterEmptyQueryNoOp(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	q, err := query.NewQuery1[fPos](w)
	if err != nil {
		t.Fatal(err)
	}

	called := false
	q.ParIter(w, func(_ entity.Entity, _ *fPos) {
		called = true
	})
	if called {
		t.Fatal("ParIter on an empty world must not invoke fn")
	}
}

func TestParIterTinyArchetypeRunsInline(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	const small = 5
	for i := range small {
		w.Spawn(component.Data{Value: fPos{X: float64(i)}})
	}

	q, err := query.NewQuery1[fPos](w)
	if err != nil {
		t.Fatal(err)
	}

	var visited int64
	q.ParIter(w, func(_ entity.Entity, _ *fPos) {
		atomic.AddInt64(&visited, 1)
	})
	if visited != small {
		t.Fatalf("ParIter visited %d, want %d", visited, small)
	}
}

func TestQuery2CountWithPerRowFilter(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	for i := range 3 {
		w.Spawn(component.Data{Value: fPos{X: float64(i)}}, component.Data{Value: fVel{}})
	}
	q, err := query.NewQuery2[fPos, fVel](w, query.Added[fHP]{})
	if err != nil {
		t.Fatal(err)
	}
	// No fHP on any entity → no archetype matches → count is 0.
	if got := q.Count(w); got != 0 {
		t.Fatalf("Query2 + Added[fHP] without fHP entities: Count = %d, want 0", got)
	}

	// Now spawn entities with fHP — Phase 1 scaffold: all rows pass.
	for i := range 4 {
		w.Spawn(
			component.Data{Value: fPos{}},
			component.Data{Value: fVel{}},
			component.Data{Value: fHP{V: i}},
		)
	}
	if got := q.Count(w); got != 4 {
		t.Fatalf("Query2 + Added[fHP] with fHP entities: Count = %d, want 4", got)
	}
}

func TestQuery3CountWithPerRowFilter(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	w.Spawn(
		component.Data{Value: fPos{}},
		component.Data{Value: fVel{}},
		component.Data{Value: fHP{V: 1}},
		component.Data{Value: fLabel{}},
	)
	w.Spawn(
		component.Data{Value: fPos{}},
		component.Data{Value: fVel{}},
		component.Data{Value: fHP{V: 2}},
	)

	q, err := query.NewQuery3[fPos, fVel, fHP](w, query.Changed[fLabel]{})
	if err != nil {
		t.Fatal(err)
	}
	if got := q.Count(w); got != 1 {
		t.Fatalf("Query3 + Changed[fLabel] Count = %d, want 1", got)
	}
}

func TestParIterAcrossArchetypes(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	for i := range 1000 {
		w.Spawn(component.Data{Value: fPos{X: float64(i)}})
	}
	for i := range 1000 {
		w.Spawn(component.Data{Value: fPos{X: float64(i)}}, component.Data{Value: fVel{}})
	}

	q, err := query.NewQuery1[fPos](w)
	if err != nil {
		t.Fatal(err)
	}

	var visited int64
	q.ParIter(w, func(_ entity.Entity, _ *fPos) {
		atomic.AddInt64(&visited, 1)
	})
	if visited != 2000 {
		t.Fatalf("ParIter across archetypes visited %d, want 2000", visited)
	}
}
