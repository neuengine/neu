package world

import (
	"encoding/binary"
	"slices"

	"github.com/teratron/boltengine/internal/ecs/component"
	"github.com/teratron/boltengine/internal/ecs/entity"
)

// ArchetypeID uniquely identifies an archetype within an [ArchetypeStore].
// ID 0 is reserved for the empty archetype (no components).
type ArchetypeID uint32

// ArchetypeEdge caches the target archetypes reached by adding or removing a
// single component. Populated lazily on first traversal so subsequent moves
// of the same shape skip the hash lookup.
type ArchetypeEdge struct {
	Add    ArchetypeID
	Remove ArchetypeID
}

// Archetype represents a unique combination of component types. Every entity
// with the same set of components shares one Archetype instance and lives in
// its column-oriented [component.Table] (when at least one of the components
// uses [component.StorageTable] storage).
//
// componentIDs is sorted ascending and serves as the archetype's identity.
// entities tracks which entity occupies each row; len(entities) is always
// equal to the table's row count when table != nil.
type Archetype struct {
	edges        map[component.ID]ArchetypeEdge
	table        *component.Table
	componentIDs []component.ID
	entities     []entity.Entity
	id           ArchetypeID
}

// ID returns the archetype's identifier.
func (a *Archetype) ID() ArchetypeID { return a.id }

// ComponentIDs returns the sorted list of component IDs that define this
// archetype. The slice is owned by the archetype — callers must not mutate it.
func (a *Archetype) ComponentIDs() []component.ID { return a.componentIDs }

// Len returns the number of entities in the archetype.
func (a *Archetype) Len() int { return len(a.entities) }

// Entities returns the entity slice for this archetype. The slice is owned by
// the archetype — callers must not mutate it.
func (a *Archetype) Entities() []entity.Entity { return a.entities }

// Table returns the underlying column-oriented storage. May be nil for
// archetypes that contain only [component.StorageSparseSet] components or no
// components at all.
func (a *Archetype) Table() *component.Table { return a.table }

// Has reports whether the archetype includes the given component ID.
func (a *Archetype) Has(id component.ID) bool {
	return slices.Contains(a.componentIDs, id)
}

// entityRecord locates a live entity inside the World: which archetype it
// belongs to and which row in that archetype it occupies. The row index is
// also the column-table row index when a table exists.
type entityRecord struct {
	archetypeID ArchetypeID
	row         int
}

// ListenerID identifies a registered archetype-creation listener so the
// caller can later unregister it. Zero is reserved as the invalid sentinel.
type ListenerID uint32

// ArchetypeStore manages every archetype in a [World]. The empty archetype
// (no components, ID 0) is created at construction so SpawnEmpty has a
// well-defined home for its entities.
type ArchetypeStore struct {
	index          map[string]ArchetypeID
	listeners      map[ListenerID]func(*Archetype)
	archetypes     []Archetype
	nextListenerID ListenerID
	generation     uint32
}

// newArchetypeStore creates a store seeded with the empty archetype.
func newArchetypeStore() *ArchetypeStore {
	s := &ArchetypeStore{
		archetypes: make([]Archetype, 0, 16),
		index:      make(map[string]ArchetypeID, 16),
	}
	s.archetypes = append(s.archetypes, Archetype{
		id:    0,
		edges: make(map[component.ID]ArchetypeEdge),
	})
	s.index[""] = 0
	return s
}

// findOrCreate returns the archetype for the given (sorted) set of component
// IDs, creating a new archetype (and its backing Table when needed) if none
// exists. Each creation bumps the store's generation counter so query caches
// can detect when the archetype set has expanded.
func (s *ArchetypeStore) findOrCreate(sortedIDs []component.ID, registry *component.Registry) *Archetype {
	key := componentSetKey(sortedIDs)
	if id, ok := s.index[key]; ok {
		return &s.archetypes[id]
	}

	var tableSpecs []component.ColumnSpec
	for _, id := range sortedIDs {
		info := registry.Info(id)
		if info.Storage == component.StorageTable {
			tableSpecs = append(tableSpecs, component.ColumnSpecFromInfo(info))
		}
	}
	var tbl *component.Table
	if len(tableSpecs) > 0 {
		tbl = component.NewTable(tableSpecs, 0)
	}

	id := ArchetypeID(len(s.archetypes))
	s.archetypes = append(s.archetypes, Archetype{
		id:           id,
		componentIDs: append([]component.ID(nil), sortedIDs...),
		table:        tbl,
		edges:        make(map[component.ID]ArchetypeEdge),
	})
	s.index[key] = id
	s.generation++
	arch := &s.archetypes[id]
	for _, fn := range s.listeners {
		fn(arch)
	}
	return arch
}

// OnArchetypeCreated registers fn as a listener that is invoked once with
// every new [Archetype] produced by [ArchetypeStore.findOrCreate]. Returns a
// [ListenerID] that can be passed to [ArchetypeStore.UnregisterListener] to
// drop the subscription. nil callbacks are silently ignored and yield 0.
//
// Listeners are intended for caches that need to react to archetype-graph
// deltas (cached views, observers). Registration is single-threaded — the
// store's mutating paths are scheduler-coordinated.
func (s *ArchetypeStore) OnArchetypeCreated(fn func(*Archetype)) ListenerID {
	if fn == nil {
		return 0
	}
	s.nextListenerID++
	id := s.nextListenerID
	if s.listeners == nil {
		s.listeners = make(map[ListenerID]func(*Archetype))
	}
	s.listeners[id] = fn
	return id
}

// UnregisterListener drops the listener registered under id. No-op when id
// is 0 or unknown.
func (s *ArchetypeStore) UnregisterListener(id ListenerID) {
	if id == 0 {
		return
	}
	delete(s.listeners, id)
}

// get returns a pointer to the archetype with the given ID. Out-of-range IDs
// panic — only IDs returned by [ArchetypeStore.findOrCreate] are valid.
func (s *ArchetypeStore) get(id ArchetypeID) *Archetype {
	return &s.archetypes[id]
}

// Len returns the number of archetypes in the store (including the empty one).
func (s *ArchetypeStore) Len() int { return len(s.archetypes) }

// Generation returns the current archetype-generation counter. Query caches
// re-scan only when this value changes.
func (s *ArchetypeStore) Generation() uint32 { return s.generation }

// At returns the archetype with the given ID. Out-of-range IDs panic — only
// IDs returned by [ArchetypeStore.Each] or stored in an [entityRecord] are
// valid. Public mirror of the internal `get` helper.
func (s *ArchetypeStore) At(id ArchetypeID) *Archetype { return &s.archetypes[id] }

// Each invokes fn for every archetype in the store in creation order
// (starting with the empty archetype, ID 0). Iteration stops early when fn
// returns false. The pointer passed to fn is owned by the store — do not
// retain it past archetype-graph mutations.
func (s *ArchetypeStore) Each(fn func(*Archetype) bool) {
	for i := range s.archetypes {
		if !fn(&s.archetypes[i]) {
			return
		}
	}
}

// EachFrom is like [ArchetypeStore.Each] but starts at the given index.
// Used by query caches to re-scan only archetypes created since the last
// cache update.
func (s *ArchetypeStore) EachFrom(start int, fn func(*Archetype) bool) {
	for i := start; i < len(s.archetypes); i++ {
		if !fn(&s.archetypes[i]) {
			return
		}
	}
}

// componentSetKey encodes a sorted slice of component IDs into a compact
// binary string suitable as a map key. The empty slice maps to "".
func componentSetKey(sortedIDs []component.ID) string {
	if len(sortedIDs) == 0 {
		return ""
	}
	buf := make([]byte, len(sortedIDs)*4)
	for i, id := range sortedIDs {
		binary.LittleEndian.PutUint32(buf[i*4:], uint32(id))
	}
	return string(buf)
}

// sortIDsAscending returns a sorted copy of ids.
func sortIDsAscending(ids []component.ID) []component.ID {
	out := append([]component.ID(nil), ids...)
	slices.Sort(out)
	return out
}

// withID returns a sorted copy of `sorted` with `id` inserted (no-op if
// already present).
func withID(sorted []component.ID, id component.ID) []component.ID {
	if slices.Contains(sorted, id) {
		return append([]component.ID(nil), sorted...)
	}
	out := make([]component.ID, len(sorted)+1)
	copy(out, sorted)
	out[len(sorted)] = id
	slices.Sort(out)
	return out
}

// withoutID returns a sorted copy of `sorted` with `id` removed (no-op if
// absent).
func withoutID(sorted []component.ID, id component.ID) []component.ID {
	out := make([]component.ID, 0, len(sorted))
	for _, v := range sorted {
		if v != id {
			out = append(out, v)
		}
	}
	return out
}
