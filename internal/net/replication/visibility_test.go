package replication

import (
	"testing"

	"github.com/neuengine/neu/internal/ecs/entity"
)

func e(idx uint32) entity.EntityID { return entity.NewEntityID(idx, 1) }

// ─── VisibilitySet helpers ────────────────────────────────────────────────────

func TestNewVisibilitySetEmpty(t *testing.T) {
	t.Parallel()
	vs := NewVisibilitySet()
	if vs.Entities == nil {
		t.Error("Entities map should be initialized")
	}
	if len(vs.Added) != 0 || len(vs.Removed) != 0 {
		t.Error("new set should have no deltas")
	}
}

// ─── computeDelta ─────────────────────────────────────────────────────────────

func TestComputeDeltaAllNew(t *testing.T) {
	t.Parallel()
	ids := []entity.EntityID{e(1), e(2), e(3)}
	next := make(map[entity.EntityID]struct{})
	for _, id := range ids {
		next[id] = struct{}{}
	}
	vs := computeDelta(next, NewVisibilitySet())
	if len(vs.Entities) != 3 {
		t.Errorf("Entities = %d, want 3", len(vs.Entities))
	}
	if len(vs.Added) != 3 {
		t.Errorf("Added = %d, want 3 (all new)", len(vs.Added))
	}
	if len(vs.Removed) != 0 {
		t.Errorf("Removed = %d, want 0", len(vs.Removed))
	}
}

func TestComputeDeltaNoChange(t *testing.T) {
	t.Parallel()
	prev := VisibilitySet{Entities: map[entity.EntityID]struct{}{e(1): {}, e(2): {}}}
	next := map[entity.EntityID]struct{}{e(1): {}, e(2): {}}
	vs := computeDelta(next, prev)
	if len(vs.Added) != 0 {
		t.Errorf("Added = %d, want 0 (no change)", len(vs.Added))
	}
	if len(vs.Removed) != 0 {
		t.Errorf("Removed = %d, want 0 (no change)", len(vs.Removed))
	}
}

func TestComputeDeltaEnteredAndLeft(t *testing.T) {
	t.Parallel()
	prev := VisibilitySet{Entities: map[entity.EntityID]struct{}{e(1): {}, e(2): {}}}
	next := map[entity.EntityID]struct{}{e(2): {}, e(3): {}}
	vs := computeDelta(next, prev)
	if len(vs.Added) != 1 || vs.Added[0] != e(3) {
		t.Errorf("Added = %v, want [%v]", vs.Added, e(3))
	}
	if len(vs.Removed) != 1 || vs.Removed[0] != e(1) {
		t.Errorf("Removed = %v, want [%v]", vs.Removed, e(1))
	}
}

// ─── ReplicateAll ─────────────────────────────────────────────────────────────

func TestReplicateAllIncludesAll(t *testing.T) {
	t.Parallel()
	entities := []entity.EntityID{e(1), e(2), e(3)}
	vs := ReplicateAll{}.Compute(entities, NewVisibilitySet())
	if len(vs.Entities) != 3 {
		t.Errorf("Entities = %d, want 3", len(vs.Entities))
	}
	if len(vs.Added) != 3 {
		t.Errorf("Added = %d, want 3 (all new)", len(vs.Added))
	}
}

func TestReplicateAllDelta(t *testing.T) {
	t.Parallel()
	entities := []entity.EntityID{e(1), e(2)}
	prev := VisibilitySet{Entities: map[entity.EntityID]struct{}{e(1): {}, e(3): {}}}
	vs := ReplicateAll{}.Compute(entities, prev)
	// e(2) added, e(3) removed
	if len(vs.Added) != 1 || vs.Added[0] != e(2) {
		t.Errorf("Added = %v, want [%v]", vs.Added, e(2))
	}
	if len(vs.Removed) != 1 || vs.Removed[0] != e(3) {
		t.Errorf("Removed = %v, want [%v]", vs.Removed, e(3))
	}
}

func TestReplicateAllEmpty(t *testing.T) {
	t.Parallel()
	vs := ReplicateAll{}.Compute(nil, NewVisibilitySet())
	if len(vs.Entities) != 0 || len(vs.Added) != 0 {
		t.Errorf("empty entity list should yield empty set: %+v", vs)
	}
}

// ─── GridVisibility ───────────────────────────────────────────────────────────

func TestGridVisibilityWithinRadius(t *testing.T) {
	t.Parallel()
	positions := map[entity.EntityID][2]float32{
		e(1): {0, 0},
		e(2): {5, 0},   // within radius 10
		e(3): {100, 0}, // outside
	}
	g := GridVisibility{
		CellSize: 10,
		Radius:   10,
		PositionOf: func(id entity.EntityID) (x, y float32, ok bool) {
			pos, found := positions[id]
			return pos[0], pos[1], found
		},
		ViewerOf: func() (float32, float32, bool) { return 0, 0, true },
	}
	entities := []entity.EntityID{e(1), e(2), e(3)}
	vs := g.Compute(entities, NewVisibilitySet())
	if _, ok := vs.Entities[e(1)]; !ok {
		t.Error("e(1) at origin should be visible")
	}
	if _, ok := vs.Entities[e(2)]; !ok {
		t.Error("e(2) at dist=5 should be visible (radius=10)")
	}
	if _, ok := vs.Entities[e(3)]; ok {
		t.Error("e(3) at dist=100 should NOT be visible")
	}
}

func TestGridVisibilityUnknownViewerIncludesAll(t *testing.T) {
	t.Parallel()
	g := GridVisibility{
		CellSize:   10,
		Radius:     5,
		PositionOf: func(_ entity.EntityID) (float32, float32, bool) { return 0, 0, true },
		ViewerOf:   func() (float32, float32, bool) { return 0, 0, false },
	}
	entities := []entity.EntityID{e(1), e(2)}
	vs := g.Compute(entities, NewVisibilitySet())
	if len(vs.Entities) != 2 {
		t.Errorf("unknown viewer should include all: got %d", len(vs.Entities))
	}
}

func TestGridVisibilityUnknownEntityPositionIncluded(t *testing.T) {
	t.Parallel()
	g := GridVisibility{
		CellSize: 10,
		Radius:   1,
		PositionOf: func(id entity.EntityID) (float32, float32, bool) {
			if id == e(99) {
				return 0, 0, false // no position known
			}
			return 999, 999, true
		},
		ViewerOf: func() (float32, float32, bool) { return 0, 0, true },
	}
	vs := g.Compute([]entity.EntityID{e(99)}, NewVisibilitySet())
	if _, ok := vs.Entities[e(99)]; !ok {
		t.Error("entity with unknown position should be conservatively included")
	}
}

// ─── CustomVisibility ─────────────────────────────────────────────────────────

func TestCustomVisibilityFilters(t *testing.T) {
	t.Parallel()
	allowed := map[entity.EntityID]bool{e(1): true, e(3): true}
	cv := CustomVisibility{
		Visible: func(id entity.EntityID) bool { return allowed[id] },
	}
	entities := []entity.EntityID{e(1), e(2), e(3)}
	vs := cv.Compute(entities, NewVisibilitySet())
	if len(vs.Entities) != 2 {
		t.Errorf("Entities = %d, want 2", len(vs.Entities))
	}
	if _, ok := vs.Entities[e(2)]; ok {
		t.Error("e(2) should be filtered out")
	}
}
