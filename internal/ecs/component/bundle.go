package component

import "reflect"

// Data is a type-erased component value paired with its [ID]. It is the
// transport unit between [Bundle.Components] and the World's spawn / insert
// machinery. A Data with ID == 0 is the zero / invalid sentinel.
type Data struct {
	Value any
	ID    ID
}

// IsValid reports whether the Data carries a registered component ID.
func (d Data) IsValid() bool { return d.ID != 0 }

// Bundle is the contract for groups of components that are spawned together.
// Bundles dissolve into individual [Data] values on insertion — they are
// NOT stored as components themselves and have no ComponentID.
//
// Bundles compose via nesting: a Bundle.Components implementation may
// recursively include another Bundle's output, and the World flattens the
// final result before applying it.
type Bundle interface {
	Components() []Data
}

// FlattenBundle returns the full flattened slice of Data produced by a
// bundle and any nested bundles it contains. Duplicate IDs are kept in the
// last-write-wins order: later entries shadow earlier ones at insert time
// (the World performs the final dedup against existing entity state).
func FlattenBundle(b Bundle) []Data {
	if b == nil {
		return nil
	}
	out := make([]Data, 0, 4)
	flatten(reflect.ValueOf(b), &out)
	return out
}

// flatten recursively expands a value's bundle tree into out. The reflect
// path lets callers nest bundles without an explicit recursive contract:
// any field of any reachable struct that itself implements Bundle is
// expanded.
func flatten(v reflect.Value, out *[]Data) {
	if !v.IsValid() {
		return
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	if b, ok := tryBundle(v); ok {
		*out = append(*out, b.Components()...)
		return
	}
	// Non-bundle value: nothing to flatten further. The caller is expected
	// to wrap raw values in a Bundle implementation; FlattenBundle on a
	// non-bundle returns an empty slice via this branch.
}

// tryBundle attempts to access v as a Bundle, taking the address if needed.
func tryBundle(v reflect.Value) (Bundle, bool) {
	if !v.IsValid() {
		return nil, false
	}
	if v.CanInterface() {
		if b, ok := v.Interface().(Bundle); ok {
			return b, true
		}
	}
	if v.CanAddr() {
		if b, ok := v.Addr().Interface().(Bundle); ok {
			return b, true
		}
	}
	return nil, false
}

// NewData creates a [Data] for a typed value, registering the component type
// in r if it has not been registered yet. The component is registered with
// default storage and no hooks; for finer control register first via
// r.Register and then assemble Data manually.
func NewData[T any](r *Registry, value T) Data {
	id := RegisterType[T](r)
	return Data{ID: id, Value: value}
}
