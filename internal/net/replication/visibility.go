package replication

import "github.com/neuengine/neu/internal/ecs/entity"

// VisibilitySet describes which entities are visible to one peer for a single
// tick and the delta (Added, Removed) since the previous tick.
// Added entities trigger EntitySpawn messages; Removed trigger EntityDespawn.
type VisibilitySet struct {
	// Entities is the full set of currently-visible entity IDs.
	Entities map[entity.EntityID]struct{}
	// Added contains entities that entered visibility this tick.
	Added []entity.EntityID
	// Removed contains entities that left visibility this tick.
	Removed []entity.EntityID
}

// NewVisibilitySet returns an empty VisibilitySet with pre-allocated storage.
func NewVisibilitySet() VisibilitySet {
	return VisibilitySet{Entities: make(map[entity.EntityID]struct{})}
}

// VisibilityPolicy computes per-tick visibility for one connection.
// Implementations receive the full list of replicable entity IDs and the
// previous VisibilitySet, and return a new VisibilitySet with Added/Removed
// deltas pre-computed.
type VisibilityPolicy interface {
	Compute(entities []entity.EntityID, prev VisibilitySet) VisibilitySet
}

// computeDelta builds a VisibilitySet from a raw set of next-tick entity IDs
// and the previous set, filling Added and Removed delta slices.
func computeDelta(next map[entity.EntityID]struct{}, prev VisibilitySet) VisibilitySet {
	vs := VisibilitySet{Entities: next}
	for id := range next {
		if _, was := prev.Entities[id]; !was {
			vs.Added = append(vs.Added, id)
		}
	}
	for id := range prev.Entities {
		if _, still := next[id]; !still {
			vs.Removed = append(vs.Removed, id)
		}
	}
	return vs
}

// ReplicateAll admits every entity in the provided list to all connections.
// Suitable for small games or automated testing. All entities are visible.
type ReplicateAll struct{}

// Compute implements VisibilityPolicy.
func (ReplicateAll) Compute(entities []entity.EntityID, prev VisibilitySet) VisibilitySet {
	next := make(map[entity.EntityID]struct{}, len(entities))
	for _, id := range entities {
		next[id] = struct{}{}
	}
	return computeDelta(next, prev)
}

// GridVisibility uses 2-D cell-based spatial partitioning to limit what each
// peer sees to entities within Radius world units of their current position.
// PositionOf resolves an entity to its (x, y) position; returning ok=false
// includes the entity in all visibility sets (position unknown → conservative).
type GridVisibility struct {
	CellSize   float32
	Radius     float32
	PositionOf func(id entity.EntityID) (x, y float32, ok bool)
	// ViewerOf returns the viewer position for this connection.
	ViewerOf func() (x, y float32, ok bool)
}

// Compute implements VisibilityPolicy.
func (g GridVisibility) Compute(entities []entity.EntityID, prev VisibilitySet) VisibilitySet {
	next := make(map[entity.EntityID]struct{}, len(entities))
	vx, vy, vok := g.ViewerOf()
	r2 := g.Radius * g.Radius
	for _, id := range entities {
		if !vok {
			next[id] = struct{}{}
			continue
		}
		ex, ey, pok := g.PositionOf(id)
		if !pok {
			next[id] = struct{}{}
			continue
		}
		dx, dy := ex-vx, ey-vy
		if dx*dx+dy*dy <= r2 {
			next[id] = struct{}{}
		}
	}
	return computeDelta(next, prev)
}

// CustomVisibility delegates per-entity decisions to a user-supplied predicate.
// The predicate returns true if the entity should be visible this tick.
type CustomVisibility struct {
	Visible func(id entity.EntityID) bool
}

// Compute implements VisibilityPolicy.
func (c CustomVisibility) Compute(entities []entity.EntityID, prev VisibilitySet) VisibilitySet {
	next := make(map[entity.EntityID]struct{}, len(entities))
	for _, id := range entities {
		if c.Visible(id) {
			next[id] = struct{}{}
		}
	}
	return computeDelta(next, prev)
}
