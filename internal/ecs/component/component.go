// Package component defines the component registry and metadata used by the
// ECS world. A component is a pure-data Go struct attached to entities; the
// registry assigns each registered Go type a unique [ID] used downstream by
// archetype storage, queries, and the scheduler.
package component

import (
	"reflect"
)

// ID is a unique identifier assigned to each registered component type.
// IDs are sequential, start at 1, and never change for the lifetime of the
// registry. The zero value (0) is reserved as the invalid sentinel.
type ID uint32

// IsValid reports whether the ID refers to a registered component (i.e. is
// non-zero). It does not perform a range check against any registry.
func (id ID) IsValid() bool { return id != 0 }

// StorageType determines how a component's data is physically stored.
// The actual storage backends are implemented in T-1B02; this enum is
// recorded on [Info] so that downstream layers can dispatch correctly.
type StorageType uint8

const (
	// StorageTable is column-oriented archetype-table storage (default).
	StorageTable StorageType = iota
	// StorageSparseSet is entity-indexed sparse-set storage.
	StorageSparseSet
)

// CloneBehavior describes how a component is duplicated when an entity is
// cloned. The cloning machinery itself lands later; the registry only stores
// the policy.
type CloneBehavior uint8

const (
	// CloneDeep performs a deep copy of the component data (default).
	CloneDeep CloneBehavior = iota
	// CloneIgnore skips this component during entity cloning.
	CloneIgnore
	// CloneCustom defers to a user-provided clone function.
	CloneCustom
)

// Info holds metadata for a single registered component type. It is owned
// by the [Registry] and indexed by [ID].
type Info struct {
	// CloneBehavior controls duplication during entity cloning.
	CloneBehavior CloneBehavior
	// Hooks holds optional lifecycle callbacks (T-1B03).
	Hooks Hooks
	// ID is the registry-assigned identifier. Set by Registry.Register; the
	// caller-supplied value is ignored.
	ID ID
	// Storage is the storage backend selected for this component (T-1B02).
	Storage StorageType
	// Type is the runtime type of the component value.
	Type reflect.Type
	// Name is the fully qualified Go type name (PkgPath.TypeName) used for
	// serialization and debug lookup. Anonymous types fall back to
	// reflect.Type.String().
	Name string
	// RequiredBy lists components that must be present whenever this one is.
	// Resolved transitively at registration in T-1B03.
	RequiredBy []ID
	// Alignment is the alignment requirement in bytes.
	Alignment uintptr
	// Size is the size in bytes of a single component instance.
	Size uintptr
	// Immutable forbids mutable query access to this component when true.
	Immutable bool
}

// IsZeroSized reports whether the component carries no payload (a tag).
func (i *Info) IsZeroSized() bool { return i.Size == 0 }
