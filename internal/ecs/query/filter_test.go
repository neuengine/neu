package query_test

import (
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/query"
	"github.com/neuengine/neu/internal/ecs/world"
)

// fSparse is registered as sparse-set storage to exercise the SparseSet
// branch of the per-row tick filter (rowTicks).
type fSparse struct{ N int }

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

// TestAddedChangedFilter is the T-2E02 Verify target. It supersedes the
// Phase 1 accept-all scaffold test and asserts real tick selectivity: with
// the comparison baseline at the world's last-cleared tick, [Added] yields
// only components inserted after that tick and [Changed] only components
// mutated after it — not every row in a structurally matching archetype.
func TestAddedChangedFilter(t *testing.T) {
	t.Parallel()

	t.Run("Added selects only components inserted since the baseline", func(t *testing.T) {
		w := world.NewWorld()
		// fVel on `stale` is added at tick 0 (== baseline LastChangeTick),
		// so it must NOT match: change detection uses a strict >, and
		// "added at the baseline" is not "added since the baseline".
		stale := w.Spawn(component.Data{Value: fPos{X: 1}}, component.Data{Value: fVel{}})
		w.IncrementChangeTick() // advance: a logical system/frame boundary
		fresh := w.Spawn(component.Data{Value: fPos{X: 2}}, component.Data{Value: fVel{}})

		q, err := query.NewQuery1[fPos](w, query.Added[fVel]{})
		if err != nil {
			t.Fatal(err)
		}
		if got := q.Count(w); got != 1 {
			t.Fatalf("Added[fVel] Count = %d, want 1 (only the post-baseline entity)", got)
		}
		seen := 0
		for e, p := range q.All(w) {
			seen++
			if e != fresh {
				t.Fatalf("Added[fVel] yielded entity %v, want fresh %v", e, fresh)
			}
			if p.X != 2 {
				t.Fatalf("Added[fVel] yielded fPos.X = %v, want 2", p.X)
			}
		}
		if seen != 1 {
			t.Fatalf("Added[fVel] iteration visited %d rows, want 1", seen)
		}
		_ = stale
	})

	t.Run("Changed selects only mutated rows", func(t *testing.T) {
		w := world.NewWorld()
		mutated := w.Spawn(component.Data{Value: fPos{X: 1}}, component.Data{Value: fVel{DX: 1}})
		untouched := w.Spawn(component.Data{Value: fPos{X: 2}}, component.Data{Value: fVel{DX: 2}})
		w.IncrementChangeTick()
		// In-place overwrite (same archetype) → StampChanged at the new tick.
		if err := w.Insert(mutated, component.Data{Value: fVel{DX: 99}}); err != nil {
			t.Fatal(err)
		}

		q, err := query.NewQuery1[fPos](w, query.Changed[fVel]{})
		if err != nil {
			t.Fatal(err)
		}
		if got := q.Count(w); got != 1 {
			t.Fatalf("Changed[fVel] Count = %d, want 1 (only the mutated entity)", got)
		}
		for e := range q.All(w) {
			if e != mutated {
				t.Fatalf("Changed[fVel] yielded %v, want mutated %v", e, mutated)
			}
		}
		_ = untouched
	})

	t.Run("nothing matches once the baseline catches up", func(t *testing.T) {
		w := world.NewWorld()
		w.Spawn(component.Data{Value: fPos{X: 1}}, component.Data{Value: fVel{}})
		w.IncrementChangeTick()
		w.Spawn(component.Data{Value: fPos{X: 2}}, component.Data{Value: fVel{}})
		w.ClearTrackers() // LastChangeTick advances to the current tick

		q, err := query.NewQuery1[fPos](w, query.Added[fVel]{})
		if err != nil {
			t.Fatal(err)
		}
		if got := q.Count(w); got != 0 {
			t.Fatalf("Added[fVel] after ClearTrackers Count = %d, want 0", got)
		}
	})

	t.Run("Changed works for sparse-set stored components", func(t *testing.T) {
		w := world.NewWorld()
		// Register fSparse as sparse-set storage *before* any spawn so the
		// world routes it through the global SparseSet (rowTicks' sparse
		// branch + ss.Ticks path).
		w.Components().Register(component.Info{
			Type:    reflect.TypeFor[fSparse](),
			Storage: component.StorageSparseSet,
		})
		mutated := w.Spawn(component.Data{Value: fPos{X: 1}}, component.Data{Value: fSparse{N: 1}})
		w.Spawn(component.Data{Value: fPos{X: 2}}, component.Data{Value: fSparse{N: 2}})
		w.IncrementChangeTick()
		if err := w.Insert(mutated, component.Data{Value: fSparse{N: 9}}); err != nil {
			t.Fatal(err)
		}

		q, err := query.NewQuery1[fPos](w, query.Changed[fSparse]{})
		if err != nil {
			t.Fatal(err)
		}
		if got := q.Count(w); got != 1 {
			t.Fatalf("Changed[fSparse] Count = %d, want 1 (only the mutated sparse entity)", got)
		}
		for e := range q.All(w) {
			if e != mutated {
				t.Fatalf("Changed[fSparse] yielded %v, want %v", e, mutated)
			}
		}
	})
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

	// Advance the tick, then spawn fHP-bearing entities: their fHP is added
	// strictly after the baseline (LastChangeTick == 0), so real change
	// detection matches all four.
	w.IncrementChangeTick()
	for i := range 4 {
		w.Spawn(
			component.Data{Value: fPos{}},
			component.Data{Value: fVel{}},
			component.Data{Value: fHP{V: i}},
		)
	}
	if got := q.Count(w); got != 4 {
		t.Fatalf("Query2 + Added[fHP] with post-baseline fHP entities: Count = %d, want 4", got)
	}
}

func TestQuery3CountWithPerRowFilter(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	// Entity without fLabel spawned at the baseline tick.
	w.Spawn(
		component.Data{Value: fPos{}},
		component.Data{Value: fVel{}},
		component.Data{Value: fHP{V: 2}},
	)
	// Advance, then spawn the fLabel-bearing entity: its fLabel is added
	// after the baseline, and Changed includes additions, so exactly one
	// entity satisfies Changed[fLabel].
	w.IncrementChangeTick()
	w.Spawn(
		component.Data{Value: fPos{}},
		component.Data{Value: fVel{}},
		component.Data{Value: fHP{V: 1}},
		component.Data{Value: fLabel{}},
	)

	q, err := query.NewQuery3[fPos, fVel, fHP](w, query.Changed[fLabel]{})
	if err != nil {
		t.Fatal(err)
	}
	if got := q.Count(w); got != 1 {
		t.Fatalf("Query3 + Changed[fLabel] Count = %d, want 1 (only the post-baseline fLabel entity)", got)
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
