package entity

// EntitySet is an unordered set of entities with O(1) Insert / Remove /
// Contains and cache-friendly iteration via a dense backing slice.
//
// Implementation: dense slice + sparse map[EntityID]int. Removal uses
// swap-and-pop on the dense slice to keep iteration contiguous.
type EntitySet struct {
	sparse map[EntityID]int
	dense  []Entity
}

// NewEntitySet creates an empty EntitySet.
func NewEntitySet() *EntitySet {
	return &EntitySet{
		sparse: make(map[EntityID]int),
	}
}

// NewEntitySetWithCapacity creates an EntitySet with pre-allocated capacity.
func NewEntitySetWithCapacity(capacity int) *EntitySet {
	if capacity < 0 {
		capacity = 0
	}
	return &EntitySet{
		dense:  make([]Entity, 0, capacity),
		sparse: make(map[EntityID]int, capacity),
	}
}

// Insert adds an entity to the set. Returns false if the entity is invalid
// (null sentinel) or already present.
func (s *EntitySet) Insert(entity Entity) bool {
	if !entity.IsValid() {
		return false
	}
	if _, exists := s.sparse[entity.ID()]; exists {
		return false
	}
	s.sparse[entity.ID()] = len(s.dense)
	s.dense = append(s.dense, entity)
	return true
}

// Remove deletes an entity from the set in O(1) using swap-and-pop. Returns
// false if the entity was not present.
func (s *EntitySet) Remove(entity Entity) bool {
	idx, ok := s.sparse[entity.ID()]
	if !ok {
		return false
	}
	last := len(s.dense) - 1
	if idx != last {
		moved := s.dense[last]
		s.dense[idx] = moved
		s.sparse[moved.ID()] = idx
	}
	s.dense = s.dense[:last]
	delete(s.sparse, entity.ID())
	return true
}

// Contains reports whether the entity is present in the set.
func (s *EntitySet) Contains(entity Entity) bool {
	_, ok := s.sparse[entity.ID()]
	return ok
}

// Len returns the number of entities in the set.
func (s *EntitySet) Len() int {
	return len(s.dense)
}

// Iter invokes fn for each entity in insertion order (modulo swap-and-pop
// reordering caused by prior removals). The callback must not mutate the set
// during iteration.
func (s *EntitySet) Iter(fn func(Entity)) {
	for _, e := range s.dense {
		fn(e)
	}
}

// Clear removes all entities from the set, retaining underlying capacity.
func (s *EntitySet) Clear() {
	s.dense = s.dense[:0]
	for k := range s.sparse {
		delete(s.sparse, k)
	}
}

// EntityMap is a generic entity-keyed map with O(1) operations. It wraps a
// native Go map but enforces null-entity rejection on writes.
type EntityMap[V any] struct {
	entries map[EntityID]V
}

// NewEntityMap creates an empty EntityMap.
func NewEntityMap[V any]() *EntityMap[V] {
	return &EntityMap[V]{entries: make(map[EntityID]V)}
}

// NewEntityMapWithCapacity creates an EntityMap with pre-allocated capacity.
func NewEntityMapWithCapacity[V any](capacity int) *EntityMap[V] {
	if capacity < 0 {
		capacity = 0
	}
	return &EntityMap[V]{entries: make(map[EntityID]V, capacity)}
}

// Set associates a value with an entity. No-op for the null entity.
func (m *EntityMap[V]) Set(entity Entity, value V) {
	if !entity.IsValid() {
		return
	}
	m.entries[entity.ID()] = value
}

// Get returns the value for the given entity and whether it was present.
func (m *EntityMap[V]) Get(entity Entity) (V, bool) {
	v, ok := m.entries[entity.ID()]
	return v, ok
}

// Remove deletes the entry for the given entity. Returns false if no entry
// existed.
func (m *EntityMap[V]) Remove(entity Entity) bool {
	if _, ok := m.entries[entity.ID()]; !ok {
		return false
	}
	delete(m.entries, entity.ID())
	return true
}

// Contains reports whether the entity has an entry in the map.
func (m *EntityMap[V]) Contains(entity Entity) bool {
	_, ok := m.entries[entity.ID()]
	return ok
}

// Len returns the number of entries in the map.
func (m *EntityMap[V]) Len() int {
	return len(m.entries)
}

// Iter invokes fn for each (entity, value) pair. Iteration order is the
// underlying map's order (intentionally non-deterministic).
func (m *EntityMap[V]) Iter(fn func(Entity, V)) {
	for id, v := range m.entries {
		fn(FromID(id), v)
	}
}

// Clear removes all entries, retaining capacity.
func (m *EntityMap[V]) Clear() {
	for k := range m.entries {
		delete(m.entries, k)
	}
}
