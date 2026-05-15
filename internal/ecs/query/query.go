package query

import "github.com/teratron/boltengine/internal/ecs/component"

// QueryState is the cached, archetype-level matching primitive shared by
// every concrete query type (Query1, Query2, …). It encodes:
//
//   - required: components that must be present in the archetype.
//   - excluded: components that must NOT be present in the archetype.
//   - access: read/write/exclusive declarations for scheduler conflict
//     detection.
//
// Multi-arity wrappers (added in T-1D02) and per-row filters (T-1D03) extend
// this struct rather than replace it. The cache of matched archetypes lives
// on the wrapper because it depends on archetype layout, which the query
// package does not import.
type QueryState struct {
	required Mask
	excluded Mask
	access   Access
}

// NewQueryState builds a [QueryState] from required / excluded component IDs
// and an [Access] declaration. The access set is validated; an invalid set
// (e.g. exclusive overlapping read/write) returns an error.
//
// requiredIDs are also added to access.Read by default if they do not
// already appear in access.Write or access.Exclusive — a query that reads
// a component for matching must declare that read for the scheduler.
func NewQueryState(requiredIDs, excludedIDs []component.ID, access Access) (*QueryState, error) {
	required := MaskFromIDs(requiredIDs)
	excluded := MaskFromIDs(excludedIDs)

	for _, id := range requiredIDs {
		if access.Write.Has(id) || access.Exclusive.Has(id) {
			continue
		}
		access.AddRead(id)
	}

	if err := access.Validate(); err != nil {
		return nil, err
	}
	return &QueryState{required: required, excluded: excluded, access: access}, nil
}

// Required returns the mask of components that an archetype must contain.
func (q *QueryState) Required() Mask { return q.required }

// Excluded returns the mask of components that an archetype must NOT contain.
func (q *QueryState) Excluded() Mask { return q.excluded }

// Access returns the read/write/exclusive declaration for this query.
func (q *QueryState) Access() Access { return q.access }

// Matches reports whether an archetype identified by its component-mask
// satisfies this query: it contains every required component and none of
// the excluded ones.
func (q *QueryState) Matches(archetypeMask Mask) bool {
	if !archetypeMask.Contains(q.required) {
		return false
	}
	if !archetypeMask.IsDisjoint(q.excluded) {
		return false
	}
	return true
}

// MatchesIDs is a convenience for callers that already hold an archetype's
// component IDs as a slice and want to skip the intermediate Mask. The
// allocation cost is paid once per call — hot paths should cache a Mask on
// the archetype instead (added by T-1D02 / archetype-side cache).
func (q *QueryState) MatchesIDs(archetypeIDs []component.ID) bool {
	return q.Matches(MaskFromIDs(archetypeIDs))
}
