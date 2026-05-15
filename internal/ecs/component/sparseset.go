package component

import (
	"reflect"
	"unsafe"

	"github.com/teratron/boltengine/internal/ecs/entity"
)

// SparseSet is the storage backend used for components tagged with
// [StorageSparseSet]. It maps Entity → component value with O(1) Add /
// Remove / Get and dense iteration. Each SparseSet stores exactly one
// component type.
//
// Layout:
//   - sparse: indexed by entity slot index → dense+1 (0 = absent).
//   - dense:  parallel arrays of entities and raw component bytes.
//
// Removal uses swap-and-pop on the dense arrays so iteration stays
// contiguous. The set is NOT safe for concurrent mutation.
type SparseSet struct {
	sparse   []uint32        // entity index -> dense+1 (0 means absent)
	entities []entity.Entity // dense, length == n
	data     []byte          // dense, length == n*size (nil if zero-size)
	spec     ColumnSpec
}

// NewSparseSet creates an empty SparseSet for the given column spec.
func NewSparseSet(spec ColumnSpec) *SparseSet {
	return &SparseSet{spec: spec}
}

// Spec returns the column descriptor of this set.
func (s *SparseSet) Spec() ColumnSpec { return s.spec }

// Len returns the number of entities currently stored.
func (s *SparseSet) Len() int { return len(s.entities) }

// Has reports whether the entity has a component in this set.
func (s *SparseSet) Has(e entity.Entity) bool {
	if !e.IsValid() {
		return false
	}
	idx := int(e.Index())
	return idx < len(s.sparse) && s.sparse[idx] != 0 && s.entities[s.sparse[idx]-1] == e
}

// Add inserts a component for the given entity. value is copied via
// reflection from the supplied any-typed value. Re-adding for an entity that
// already has a component overwrites the previous value (no OnReplace hooks
// here — those land in T-1B03 wired through the World).
//
// Panics if the value's reflect.Type does not match the set's column type
// (excluding the zero-size case where any value is accepted).
func (s *SparseSet) Add(e entity.Entity, value any) {
	if !e.IsValid() {
		return
	}
	if !s.spec.IsZeroSized() {
		if reflect.TypeOf(value) != s.spec.Type {
			panic("component.SparseSet.Add: value type does not match column type")
		}
	}
	idx := int(e.Index())
	s.ensureSparseLen(idx + 1)

	if s.sparse[idx] != 0 && s.entities[s.sparse[idx]-1] == e {
		// Overwrite existing slot.
		row := int(s.sparse[idx] - 1)
		s.writeRow(row, value)
		return
	}

	row := len(s.entities)
	s.entities = append(s.entities, e)
	if s.spec.Size > 0 {
		s.data = append(s.data, make([]byte, s.spec.Size)...)
	}
	s.sparse[idx] = uint32(row + 1)
	s.writeRow(row, value)
}

// Remove deletes the component for the given entity using swap-and-pop.
// Returns false if the entity was not present.
func (s *SparseSet) Remove(e entity.Entity) bool {
	if !s.Has(e) {
		return false
	}
	idx := int(e.Index())
	row := int(s.sparse[idx] - 1)
	last := len(s.entities) - 1

	if row != last {
		moved := s.entities[last]
		s.entities[row] = moved
		s.sparse[int(moved.Index())] = uint32(row + 1)
		if s.spec.Size > 0 {
			copy(s.rowBytes(row), s.rowBytes(last))
		}
	}

	s.entities = s.entities[:last]
	if s.spec.Size > 0 {
		s.data = s.data[:uintptr(last)*s.spec.Size]
	}
	s.sparse[idx] = 0
	return true
}

// Get returns a raw pointer to the component data for the given entity,
// along with a boolean indicating presence. The pointer is valid until the
// next structural change. Returns (nil, true) for zero-size components.
func (s *SparseSet) Get(e entity.Entity) (unsafe.Pointer, bool) {
	if !s.Has(e) {
		return nil, false
	}
	row := int(s.sparse[int(e.Index())] - 1)
	if s.spec.Size == 0 {
		return nil, true
	}
	return unsafe.Pointer(&s.data[uintptr(row)*s.spec.Size]), true
}

// Iter invokes fn for each entity stored in the set, in dense (insertion-
// modulo-removal) order. Passing a zero-size component yields a nil pointer.
func (s *SparseSet) Iter(fn func(entity.Entity, unsafe.Pointer)) {
	for row, e := range s.entities {
		var ptr unsafe.Pointer
		if s.spec.Size > 0 {
			ptr = unsafe.Pointer(&s.data[uintptr(row)*s.spec.Size])
		}
		fn(e, ptr)
	}
}

// Clear removes all entries while retaining capacity for the dense arrays.
func (s *SparseSet) Clear() {
	for i := range s.sparse {
		s.sparse[i] = 0
	}
	s.entities = s.entities[:0]
	if s.spec.Size > 0 {
		s.data = s.data[:0]
	}
}

// rowBytes returns the dense-data slice for a given row. Caller must ensure
// the column is non-zero-size.
func (s *SparseSet) rowBytes(row int) []byte {
	off := uintptr(row) * s.spec.Size
	return s.data[off : off+s.spec.Size]
}

// writeRow copies the reflect-derived bytes of value into the row's slot.
// Zero-size columns short-circuit.
func (s *SparseSet) writeRow(row int, value any) {
	if s.spec.Size == 0 {
		return
	}
	dst := s.rowBytes(row)
	v := reflect.ValueOf(value)
	// For comparable / value-typed components we can copy via reflect
	// pointer indirection: take the addressable storage from a temporary.
	tmp := reflect.New(s.spec.Type).Elem()
	tmp.Set(v)
	src := unsafe.Slice((*byte)(unsafe.Pointer(tmp.UnsafeAddr())), s.spec.Size)
	copy(dst, src)
}

// ensureSparseLen grows the sparse slice to at least n entries.
func (s *SparseSet) ensureSparseLen(n int) {
	if len(s.sparse) >= n {
		return
	}
	if cap(s.sparse) >= n {
		s.sparse = s.sparse[:n]
		return
	}
	grown := make([]uint32, n, growCap(cap(s.sparse), n))
	copy(grown, s.sparse)
	s.sparse = grown
}

// growCap is a doubling growth strategy clamped to required capacity.
func growCap(have, want int) int {
	c := max(have, 8)
	for c < want {
		c *= 2
	}
	return c
}
