package typereg

import (
	"errors"
	"fmt"
	"iter"
	"reflect"
)

// Sentinel errors for DynamicObject operations.
var (
	ErrFieldNotFound     = errors.New("typereg: field not found")
	ErrFieldTypeMismatch = errors.New("typereg: field value type mismatch")
	ErrNotPointer        = errors.New("typereg: DynamicObject requires a pointer value")
)

// CloneFunc creates a deep copy of a value.
type CloneFunc func(src any) any

// DefaultFunc creates a zero/default instance of the type.
type DefaultFunc func() any

// SerializeFunc converts a value to bytes.
type SerializeFunc func(value any) ([]byte, error)

// DeserializeFunc restores a value from bytes.
type DeserializeFunc func(data []byte) (any, error)

// TypeHooks contains optional lifecycle callbacks for a registered type.
// Hook fields that are nil fall back to the default reflection-based behaviour
// in [TypeRegistry.SerializeValue] / [TypeRegistry.DeserializeValue].
type TypeHooks struct {
	Clone       CloneFunc
	Default     DefaultFunc
	Serialize   SerializeFunc
	Deserialize DeserializeFunc
}

// RegisterTypeWithHooks registers T together with custom lifecycle hooks.
// Idempotent: re-registering the same type returns the existing registration
// unchanged. Panics when T's fully-qualified name collides with a different
// already-registered type.
func RegisterTypeWithHooks[T any](r *TypeRegistry, hooks TypeHooks) *TypeRegistration {
	reg := RegisterType[T](r)
	reg.Hooks = hooks
	return reg
}

// SerializeValue converts value to bytes using the custom SerializeFunc hook
// if the type was registered with one; otherwise it returns an error indicating
// that no serializer is available (actual reflection-based codecs land in
// Phase 2).
func (r *TypeRegistry) SerializeValue(value any) ([]byte, error) {
	if value == nil {
		return nil, fmt.Errorf("%w: nil value", ErrTypeNotRegistered)
	}
	t := reflect.TypeOf(value)
	reg := r.Resolve(t)
	if reg == nil {
		return nil, fmt.Errorf("%w: %v", ErrTypeNotRegistered, t)
	}
	if reg.Hooks.Serialize != nil {
		return reg.Hooks.Serialize(value)
	}
	return nil, fmt.Errorf("typereg: no Serialize hook registered for %s (reflection-based codec lands in Phase 2)", reg.Name)
}

// DeserializeValue restores a value of the type identified by id from data,
// using the custom DeserializeFunc hook if one was registered; otherwise it
// returns an error (reflection-based codec lands in Phase 2).
func (r *TypeRegistry) DeserializeValue(id TypeID, data []byte) (any, error) {
	reg := r.ResolveByID(id)
	if reg == nil {
		return nil, fmt.Errorf("%w: TypeID %d", ErrTypeNotRegistered, id)
	}
	if reg.Hooks.Deserialize != nil {
		return reg.Hooks.Deserialize(data)
	}
	return nil, fmt.Errorf("typereg: no Deserialize hook registered for %s (reflection-based codec lands in Phase 2)", reg.Name)
}

// DynamicObject is a type-erased proxy that provides named field-level access
// to an arbitrary registered struct via pre-computed field offsets.
// The wrapped value must remain addressable (i.e. come from a pointer) for the
// lifetime of the DynamicObject.
type DynamicObject struct {
	reg   *TypeRegistration
	value reflect.Value // Elem() of the pointer — addressable
}

// NewDynamicObject wraps ptr (which must be a non-nil pointer to a registered
// struct) in a DynamicObject. Returns an error when ptr is not a pointer, the
// pointed-to type is not registered, or ptr is nil.
func NewDynamicObject(r *TypeRegistry, ptr any) (*DynamicObject, error) {
	if ptr == nil {
		return nil, ErrNotPointer
	}
	rv := reflect.ValueOf(ptr)
	if rv.Kind() != reflect.Ptr {
		return nil, ErrNotPointer
	}
	reg := r.Resolve(rv.Type().Elem())
	if reg == nil {
		return nil, fmt.Errorf("%w: %v", ErrTypeNotRegistered, rv.Type().Elem())
	}
	return &DynamicObject{reg: reg, value: rv.Elem()}, nil
}

// NewDynamicObjectByID creates a new zero-valued instance of the type
// identified by id and wraps it in a DynamicObject.
func NewDynamicObjectByID(r *TypeRegistry, id TypeID) (*DynamicObject, error) {
	reg := r.ResolveByID(id)
	if reg == nil {
		return nil, fmt.Errorf("%w: TypeID %d", ErrTypeNotRegistered, id)
	}
	ptr := reflect.New(reg.Type)
	return &DynamicObject{reg: reg, value: ptr.Elem()}, nil
}

// TypeID returns the TypeID of the wrapped type.
func (d *DynamicObject) TypeID() TypeID { return d.reg.ID }

// Value returns the underlying Go value as any.
func (d *DynamicObject) Value() any { return d.value.Addr().Interface() }

// Registration returns the TypeRegistration of the wrapped type.
func (d *DynamicObject) Registration() *TypeRegistration { return d.reg }

// Get returns the value of a named field.
// Returns ErrFieldNotFound when the field name is absent.
func (d *DynamicObject) Get(fieldName string) (any, error) {
	fi := d.reg.FieldByName(fieldName)
	if fi == nil {
		return nil, fmt.Errorf("%w: %q", ErrFieldNotFound, fieldName)
	}
	return d.fieldValue(fi).Interface(), nil
}

// Set sets the value of a named field.
// Returns ErrFieldNotFound when the field name is absent and
// ErrFieldTypeMismatch when value's type does not match the field's type.
func (d *DynamicObject) Set(fieldName string, value any) error {
	fi := d.reg.FieldByName(fieldName)
	if fi == nil {
		return fmt.Errorf("%w: %q", ErrFieldNotFound, fieldName)
	}
	rv := reflect.ValueOf(value)
	if rv.Type() != fi.Type {
		return fmt.Errorf("%w: field %q expects %v, got %v", ErrFieldTypeMismatch, fieldName, fi.Type, rv.Type())
	}
	d.fieldValue(fi).Set(rv)
	return nil
}

// Fields returns an iterator over all fields and their current values.
func (d *DynamicObject) Fields() iter.Seq2[FieldInfo, any] {
	return func(yield func(FieldInfo, any) bool) {
		for i := range d.reg.Fields {
			fi := d.reg.Fields[i]
			val := d.fieldValue(&fi).Interface()
			if !yield(fi, val) {
				return
			}
		}
	}
}

// fieldValue returns the reflect.Value for the field described by fi using the
// cached field Index — avoids reflect.Value.FieldByName string lookup on the
// hot path.
func (d *DynamicObject) fieldValue(fi *FieldInfo) reflect.Value {
	return d.value.Field(fi.Index)
}
