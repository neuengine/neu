package query

import (
	"github.com/neuengine/neu/internal/ecs/changedetect"
	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/entity"
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

// passesPerRow evaluates the per-row [Added]/[Changed] tick filters captured
// on a query against the entity occupying (arch, row). The comparison
// baseline is the world's last-cleared tick — the engine-level analogue of a
// system's last-run tick (per-system tick threading lands with the scheduler
// integration in T-2E03). A row passes only when every per-row record is
// satisfied (AND semantics, matching [With] composition).
//
// Returns true immediately when there are no per-row records, preserving the
// fast path for structural-only queries. The per-row signature replaces the
// Phase 1 accept-all stub; call sites pass (arch, row) so the predicate can
// reach the inline ComponentTicks added in T-2E01 without changing the
// iteration structure.
func passesPerRow(w *world.World, arch *world.Archetype, row int, perRow []tickFilterRecord) bool {
	if len(perRow) == 0 {
		return true
	}
	last := changedetect.Tick(w.LastChangeTick())
	var (
		ent      entity.Entity
		resolved bool
	)
	for _, rec := range perRow {
		ct, ok := rowTicks(w, arch, row, rec.id, &ent, &resolved)
		if !ok {
			return false // component absent on this row — cannot have been added/changed
		}
		switch rec.kind {
		case tickKindAdded:
			if !ct.IsAdded(last) {
				return false
			}
		case tickKindChanged:
			if !ct.IsChanged(last) {
				return false
			}
		}
	}
	return true
}

// rowTicks resolves the [changedetect.ComponentTicks] for component id on the
// entity at (arch, row), dispatching Table vs SparseSet exactly like
// [fetchComponent]. The entity is resolved at most once per row and only when
// a sparse-set lookup actually needs it (the table path is purely positional).
func rowTicks(
	w *world.World,
	arch *world.Archetype,
	row int,
	id component.ID,
	ent *entity.Entity,
	resolved *bool,
) (changedetect.ComponentTicks, bool) {
	info := w.Components().Info(id)
	if info == nil {
		return changedetect.ComponentTicks{}, false
	}
	if info.Storage == component.StorageSparseSet {
		if !*resolved {
			*ent = arch.Entities()[row]
			*resolved = true
		}
		ss, ok := w.SparseSet(id)
		if !ok {
			return changedetect.ComponentTicks{}, false
		}
		return ss.Ticks(*ent)
	}
	tbl := arch.Table()
	if tbl == nil {
		return changedetect.ComponentTicks{}, false
	}
	return tbl.TicksByID(id, row)
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
