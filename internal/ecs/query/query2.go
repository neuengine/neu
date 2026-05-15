package query

import (
	"iter"

	"github.com/teratron/boltengine/internal/ecs/component"
	"github.com/teratron/boltengine/internal/ecs/entity"
	"github.com/teratron/boltengine/internal/ecs/world"
)

// Query2 is a two-component query. It iterates every live entity whose
// archetype contains both A and B and yields (Entity, [Tuple2]) pairs via
// [Query2.All].
type Query2[A, B any] struct {
	state    *QueryState
	ids      [2]component.ID
	perRow   []tickFilterRecord
	matched  []world.ArchetypeID
	nextScan int
}

// NewQuery2 builds a two-component query, auto-registering A and B as
// [component.StorageTable] components when first seen. Returns an error if
// A and B are the same Go type. Optional [QueryFilter]s narrow the result
// set the same way they do for [NewQuery1].
func NewQuery2[A, B any](w *world.World, filters ...QueryFilter) (*Query2[A, B], error) {
	idA := componentIDFor[A](w)
	idB := componentIDFor[B](w)
	if idA == idB {
		return nil, errSameTypeInQuery
	}
	b := applyFilters(w, []component.ID{idA, idB}, filters)
	state, err := NewQueryState(b.required, b.excluded, Access{})
	if err != nil {
		return nil, err
	}
	return &Query2[A, B]{
		state:  state,
		ids:    [2]component.ID{idA, idB},
		perRow: b.perRow,
	}, nil
}

// State returns the underlying [QueryState].
func (q *Query2[A, B]) State() *QueryState { return q.state }

// All returns an iterator over every (entity, Tuple2[*A, *B]) pair matching
// the query.
//
// Usage:
//
//	for e, t := range q.All(world) {
//	    _ = e
//	    t.A.X += t.B.DX
//	}
func (q *Query2[A, B]) All(w *world.World) iter.Seq2[entity.Entity, Tuple2[A, B]] {
	q.refresh(w)
	return func(yield func(entity.Entity, Tuple2[A, B]) bool) {
		for _, archID := range q.matched {
			arch := w.Archetypes().At(archID)
			entities := arch.Entities()
			for row, e := range entities {
				if !passesPerRow(w, q.perRow) {
					continue
				}
				tup := Tuple2[A, B]{
					A: (*A)(fetchComponent(w, arch, e, row, q.ids[0])),
					B: (*B)(fetchComponent(w, arch, e, row, q.ids[1])),
				}
				if !yield(e, tup) {
					return
				}
			}
		}
	}
}

// Count returns the number of entities currently matching the query.
func (q *Query2[A, B]) Count(w *world.World) int {
	q.refresh(w)
	if len(q.perRow) == 0 {
		n := 0
		for _, archID := range q.matched {
			n += w.Archetypes().At(archID).Len()
		}
		return n
	}
	n := 0
	for _, archID := range q.matched {
		arch := w.Archetypes().At(archID)
		for row := 0; row < arch.Len(); row++ {
			if passesPerRow(w, q.perRow) {
				n++
			}
		}
	}
	return n
}

func (q *Query2[A, B]) refresh(w *world.World) {
	store := w.Archetypes()
	if q.nextScan == store.Len() {
		return
	}
	store.EachFrom(q.nextScan, func(a *world.Archetype) bool {
		if q.state.MatchesIDs(a.ComponentIDs()) {
			q.matched = append(q.matched, a.ID())
		}
		return true
	})
	q.nextScan = store.Len()
}
