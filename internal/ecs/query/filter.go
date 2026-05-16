package query

import (
	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/world"
)

// QueryFilter narrows which archetypes (and, for tick-based filters, which
// individual entities) a query matches. The four concrete filters are
// [With], [Without], [Added], and [Changed]; the interface is sealed via an
// unexported method so external packages cannot implement new filters in
// Phase 1. Custom filter extension points open in Phase 2 once the
// change-detection contract stabilizes.
type QueryFilter interface {
	apply(w *world.World, b *filterBuilder)
}

// filterBuilder accumulates filter contributions during query construction.
// Each [QueryFilter.apply] writes into the builder's slices; the resulting
// required / excluded sets feed [NewQueryState], and the perRow records are
// retained on the concrete query type for iteration-time evaluation.
type filterBuilder struct {
	required []component.ID
	excluded []component.ID
	perRow   []tickFilterRecord
}

// tickFilterRecord captures the per-entity portion of an [Added] or
// [Changed] filter. The component ID is resolved during construction so the
// iteration hot path never touches reflect.
type tickFilterRecord struct {
	kind tickKind
	id   component.ID
}

type tickKind uint8

const (
	tickKindAdded tickKind = iota
	tickKindChanged
)

// passesPerRow evaluates the per-row tick filters captured on a query. In
// Phase 1 the column-level change ticks are not yet tracked — that wiring
// belongs to Phase 2 (change-detection track) — so the scaffold accepts
// every row that already passed the structural archetype test. The shape
// of this function is what the Phase 2 implementation will replace; until
// then [Added] and [Changed] are equivalent to a [With] of the same type.
func passesPerRow(_ *world.World, perRow []tickFilterRecord) bool {
	if len(perRow) == 0 {
		return true
	}
	// Phase 1 scaffold: archetype-level match is sufficient. Phase 2 will
	// compare arch.Table().ChangeTick(id, row) against w.LastChangeTick().
	return true
}

// With[T] requires T to be present on matched archetypes. T is not fetched
// — pass it as a phantom filter when a system needs an entity to *also*
// carry T without binding it to a query type parameter.
//
// Usage: query.NewQuery1[Position](w, query.With[Velocity]{}).
type With[T any] struct{}

func (With[T]) apply(w *world.World, b *filterBuilder) {
	b.required = append(b.required, componentIDFor[T](w))
}

// Without[T] excludes archetypes that contain T. Combined with required
// types from the query's parameters or [With] filters, this is the
// canonical way to express "has A but not B".
type Without[T any] struct{}

func (Without[T]) apply(w *world.World, b *filterBuilder) {
	b.excluded = append(b.excluded, componentIDFor[T](w))
}

// Added[T] matches archetypes containing T and — in Phase 2+ — entities
// whose T was added since the system's last run. The Phase 1 scaffold
// treats the per-row check as a no-op, so [Added] currently behaves like
// [With]; the structural intent is preserved on the query for the
// scheduler's benefit and the row-level filter activates without source
// changes once change-detection lands.
type Added[T any] struct{}

func (Added[T]) apply(w *world.World, b *filterBuilder) {
	id := componentIDFor[T](w)
	b.required = append(b.required, id)
	b.perRow = append(b.perRow, tickFilterRecord{kind: tickKindAdded, id: id})
}

// Changed[T] matches archetypes containing T and — in Phase 2+ — entities
// whose T was mutated since the system's last run. See [Added] for the
// Phase 1 scaffold semantics.
type Changed[T any] struct{}

func (Changed[T]) apply(w *world.World, b *filterBuilder) {
	id := componentIDFor[T](w)
	b.required = append(b.required, id)
	b.perRow = append(b.perRow, tickFilterRecord{kind: tickKindChanged, id: id})
}

// applyFilters runs every supplied filter against a fresh builder seeded
// with the query's primary required IDs (the type parameters of Query1/2/3).
// Returns the combined required / excluded slices and the per-row records.
func applyFilters(w *world.World, primary []component.ID, filters []QueryFilter) *filterBuilder {
	b := &filterBuilder{required: append([]component.ID(nil), primary...)}
	for _, f := range filters {
		if f == nil {
			continue
		}
		f.apply(w, b)
	}
	return b
}
