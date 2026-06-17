package replication

import (
	"sort"

	"github.com/neuengine/neu/internal/ecs/entity"
)

// DefaultBandwidthBudget is the per-frame replication byte budget (64 KiB).
const DefaultBandwidthBudget = 64 * 1024

// DefaultBasePriority is the base scheduling priority assigned to new entities.
const DefaultBasePriority float32 = 1.0

// priorityState holds the bandwidth-scheduling accumulator for one entity.
// Deferred entities grow their effective priority each frame, preventing starvation
// when the bandwidth budget is exhausted before all entities are serialized.
type priorityState struct {
	base        float32
	accumulator float32
}

// effective returns the current priority (base + accumulated deferred).
func (p *priorityState) effective() float32 { return p.base + p.accumulator }

// onSent resets the accumulator after successful serialization this frame.
func (p *priorityState) onSent() { p.accumulator = 0 }

// onDeferred increments the accumulator when this entity was skipped.
func (p *priorityState) onDeferred() { p.accumulator += p.base }

// PriorityStore tracks per-entity scheduling state for one connection.
// Each connection owns its own PriorityStore so priorities are independent.
type PriorityStore struct {
	states map[entity.EntityID]*priorityState
}

// NewPriorityStore returns an empty PriorityStore.
func NewPriorityStore() *PriorityStore {
	return &PriorityStore{states: make(map[entity.EntityID]*priorityState)}
}

// get returns or lazily creates the priorityState for e.
func (ps *PriorityStore) get(e entity.EntityID) *priorityState {
	if s, ok := ps.states[e]; ok {
		return s
	}
	s := &priorityState{base: DefaultBasePriority}
	ps.states[e] = s
	return s
}

// forget removes the priority record for e (called on despawn or visibility-leave).
func (ps *PriorityStore) forget(e entity.EntityID) {
	delete(ps.states, e)
}

// pendingUpdate bundles one entity's encoded replication data with its scheduling priority.
type pendingUpdate struct {
	entity   entity.EntityID
	priority float32
	msgs     []byte
}

// sortByPriority orders updates in descending priority (highest-priority sent first).
func sortByPriority(updates []pendingUpdate) {
	sort.Slice(updates, func(i, j int) bool {
		return updates[i].priority > updates[j].priority
	})
}
