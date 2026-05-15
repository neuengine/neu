// Package view implements cached entity views (T-1I01): a stable list of
// archetypes that match a fixed [query.QueryState], maintained reactively as
// the archetype graph grows. Views give systems an O(N_matches) iteration
// path and an O(K_archetypes) membership check, bypassing per-frame query
// re-scans.
//
// # Reactive vs polling
//
// View subscribes to [world.ArchetypeStore.OnArchetypeCreated] at construction.
// New matching archetypes are appended automatically — callers do not need to
// poll or refresh. Existing archetypes are scanned once at construction.
//
// # Lifecycle
//
// Views typically live as long as the World. For tests or short-lived caches
// call [View.Close] to drop the listener subscription. Forgetting to close is
// not a leak in steady state but will keep the view's match list growing as
// archetypes are created.
//
// # Phase 1 scope (T-1I01)
//
// Type-erased entity iteration only. Component-bound caches (e.g. cached
// Query1/Query2 backed by views) are deferred to Phase 2 work where systems
// gain SystemParam fetching.
package view

import (
	"iter"
	"slices"

	"github.com/teratron/boltengine/internal/ecs/component"
	"github.com/teratron/boltengine/internal/ecs/entity"
	"github.com/teratron/boltengine/internal/ecs/query"
	"github.com/teratron/boltengine/internal/ecs/world"
)

// View is a cached set of [world.ArchetypeID]s matching a fixed
// [query.QueryState]. Construction performs an initial scan of every existing
// archetype and registers an [world.ArchetypeStore.OnArchetypeCreated]
// listener so future matches are appended automatically.
type View struct {
	state      *query.QueryState
	matched    []world.ArchetypeID
	listenerID world.ListenerID
}

// New creates a view bound to state. Existing archetypes are scanned and
// matched; future archetypes are tracked via a push-based listener.
func New(w *world.World, state *query.QueryState) *View {
	v := &View{state: state}
	store := w.Archetypes()
	store.Each(func(arch *world.Archetype) bool {
		v.consider(arch)
		return true
	})
	v.listenerID = store.OnArchetypeCreated(v.consider)
	return v
}

// Requiring is a shorthand constructor for views that only need a "must have"
// component set. Excluded components, [query.Access] tracking, and tick
// filters are not configured. Returns the constructed view and any error
// produced by [query.NewQueryState] (currently only [query.ErrInvalidAccess]).
func Requiring(w *world.World, ids ...component.ID) (*View, error) {
	state, err := query.NewQueryState(ids, nil, query.Access{})
	if err != nil {
		return nil, err
	}
	return New(w, state), nil
}

// Close unsubscribes the view from archetype-creation notifications. After
// Close the view's matched list is frozen — new archetypes are no longer
// considered. Calling Close more than once is a no-op.
func (v *View) Close(w *world.World) {
	if v.listenerID == 0 {
		return
	}
	w.Archetypes().UnregisterListener(v.listenerID)
	v.listenerID = 0
}

// consider tests arch against the view's QueryState and appends its ID when
// it matches. Used both for the initial scan and as the create-listener.
func (v *View) consider(arch *world.Archetype) {
	if v.state.Matches(query.MaskFromIDs(arch.ComponentIDs())) {
		v.matched = append(v.matched, arch.ID())
	}
}

// MatchedArchetypes returns a copy of the cached matching archetype IDs in
// creation order. Useful for diagnostics; iterate entities via [View.Entities]
// for the common path.
func (v *View) MatchedArchetypes() []world.ArchetypeID {
	out := make([]world.ArchetypeID, len(v.matched))
	copy(out, v.matched)
	return out
}

// MatchedCount returns the number of matched archetypes.
func (v *View) MatchedCount() int { return len(v.matched) }

// Count sums the entity counts of every matched archetype. O(K_archetypes).
func (v *View) Count(w *world.World) int {
	store := w.Archetypes()
	n := 0
	for _, id := range v.matched {
		n += store.At(id).Len()
	}
	return n
}

// Entities returns a Go 1.23 range-over-func iterator that yields every entity
// in every matched archetype, in archetype-creation order. Yielding is
// allocation-free in steady state.
func (v *View) Entities(w *world.World) iter.Seq[entity.Entity] {
	store := w.Archetypes()
	return func(yield func(entity.Entity) bool) {
		for _, id := range v.matched {
			arch := store.At(id)
			for _, e := range arch.Entities() {
				if !yield(e) {
					return
				}
			}
		}
	}
}

// Contains reports whether e is currently in a matched archetype. Returns
// false when e is dead, has not been spawned yet, or its archetype is not in
// the view's match set. O(K_archetypes) over the matched list.
func (v *View) Contains(w *world.World, e entity.Entity) bool {
	if !w.Contains(e) {
		return false
	}
	archID, ok := w.ArchetypeOf(e)
	if !ok {
		return false
	}
	return slices.Contains(v.matched, archID)
}
