package changedetect

import (
	"iter"

	"github.com/neuengine/neu/internal/ecs/entity"
)

// removedEntry is a single record of one entity losing a component.
type removedEntry struct {
	entity      entity.Entity
	removalTick Tick
}

// RemovedComponents[T] tracks entities that had component T removed in recent
// frames. Entries persist for two update cycles so systems at different schedule
// points can observe removals (INV-5: entries with removalTick ≤ lastTick−2
// are pruned by DrainBefore during ClearTrackers).
//
// Access via [world.RegisterRemovedComponents]; it handles drain registration
// and the removal notification callback automatically. Do not share across
// goroutines — the schedule serialises all access.
type RemovedComponents[T any] struct {
	removals []removedEntry
}

// Append records the removal of entity e at the given tick. The callback
// registered by [world.RegisterRemovedComponents] calls this automatically;
// call it directly only in tests or custom integration code.
func (rc *RemovedComponents[T]) Append(e entity.Entity, removalTick Tick) {
	rc.removals = append(rc.removals, removedEntry{entity: e, removalTick: removalTick})
}

// Iter yields each entity that had component T removed after lastSystemTick.
// Entries are ordered by removal time (oldest first). Only entries strictly
// newer than lastSystemTick are yielded (strict > comparison, per INV-1).
func (rc *RemovedComponents[T]) Iter(lastSystemTick Tick) iter.Seq[entity.Entity] {
	return func(yield func(entity.Entity) bool) {
		for _, r := range rc.removals {
			if r.removalTick.IsNewerThan(lastSystemTick) {
				if !yield(r.entity) {
					return
				}
			}
		}
	}
}

// Len returns the total number of pending removal entries (including entries
// that may not be visible to any particular system tick).
func (rc *RemovedComponents[T]) Len() int { return len(rc.removals) }

// DrainBefore removes entries that have aged out of the two-frame retention
// window. Entries with removalTick ≤ lastTick−2 are pruned; entries newer
// than that cutoff are retained in-place (stable order preserved).
//
// lastTick is world.LastChangeTick after ClearTrackers advances it. This
// ensures entries added in frame N are visible through frame N+2 and removed
// in frame N+3. When lastTick < 2 the underflow guard keeps all entries.
//
// Called automatically by [RemovedRegistry.DrainAll] during ClearTrackers.
func (rc *RemovedComponents[T]) DrainBefore(lastTick Tick) {
	if lastTick < 2 {
		return
	}
	cutoff := lastTick - 2
	keep := 0
	for _, r := range rc.removals {
		if r.removalTick > cutoff {
			rc.removals[keep] = r
			keep++
		}
	}
	clear(rc.removals[keep:]) // release entity references to GC
	rc.removals = rc.removals[:keep]
}

// RemovedRegistry holds drain callbacks for every [RemovedComponents][T]
// registered in a World. Stored as a resource; [world.World.ClearTrackers]
// delegates to it for bulk cleanup each frame.
type RemovedRegistry struct {
	drainers []func(tick Tick)
}

// Register adds a drain callback. Called by [world.RegisterRemovedComponents]
// once per component type T at setup time.
func (r *RemovedRegistry) Register(drain func(tick Tick)) {
	r.drainers = append(r.drainers, drain)
}

// DrainAll calls every registered drain function with tick, pruning removal
// entries that have exited the two-frame window.
func (r *RemovedRegistry) DrainAll(tick Tick) {
	for _, d := range r.drainers {
		d(tick)
	}
}
