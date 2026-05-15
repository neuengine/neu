// Package entity defines the lightweight, generationally-versioned entity
// identifiers used throughout the ECS runtime.
//
// Entities are 64-bit values packed as (generation << 32) | index. The zero
// value is reserved as the null sentinel and is never assigned to a live
// entity by the allocator (see EntityAllocator, T-1A02).
package entity

const (
	indexBits       = 32
	indexMask       = (1 << indexBits) - 1
	generationShift = indexBits
)

// EntityID is a packed 64-bit identifier: lower 32 bits are the slot index,
// upper 32 bits are the generation counter. The zero value (0) represents the
// invalid/null entity and is reserved by the allocator.
type EntityID uint64

// NewEntityID constructs an EntityID from a slot index and generation counter.
func NewEntityID(index, generation uint32) EntityID {
	return EntityID(uint64(generation))<<generationShift | EntityID(index)
}

// Index returns the lower 32 bits (slot index).
func (id EntityID) Index() uint32 {
	return uint32(id & indexMask)
}

// Generation returns the upper 32 bits (generation counter).
func (id EntityID) Generation() uint32 {
	return uint32(id >> generationShift)
}

// IsNull reports whether the EntityID is the null sentinel (zero value).
func (id EntityID) IsNull() bool {
	return id == 0
}

// Entity wraps an EntityID and is the value type passed through public APIs.
// The zero value Entity{} is the null sentinel.
type Entity struct {
	id EntityID
}

// NewEntity creates an Entity from a slot index and generation counter.
func NewEntity(index, generation uint32) Entity {
	return Entity{id: NewEntityID(index, generation)}
}

// FromID wraps a raw EntityID into an Entity value.
func FromID(id EntityID) Entity {
	return Entity{id: id}
}

// ID returns the packed EntityID.
func (e Entity) ID() EntityID {
	return e.id
}

// Index returns the entity's slot index.
func (e Entity) Index() uint32 {
	return e.id.Index()
}

// Generation returns the entity's generation counter.
func (e Entity) Generation() uint32 {
	return e.id.Generation()
}

// IsValid reports whether the entity is not the zero/null sentinel. It does
// not consult an allocator: liveness against generational reuse is checked by
// EntityAllocator.IsAlive.
func (e Entity) IsValid() bool {
	return e.id != 0
}
