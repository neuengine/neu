package typereg

import (
	"fmt"
	"reflect"
)

// TypeRegistration is the cached metadata block for a single registered Go
// type. Field offsets, tags, and type-level attributes are computed once at
// registration so subsequent lookups never call into reflect.
type TypeRegistration struct {
	Hooks       TypeHooks
	Type        reflect.Type
	fieldByName map[string]int
	Name        string
	Fields      []FieldInfo
	Align       uintptr
	Size        uintptr
	ID          TypeID
	Tags        TypeTags
}

// FieldByName returns the [FieldInfo] for the field named name, or nil when
// the field is absent. The lookup map is built lazily on first call.
func (r *TypeRegistration) FieldByName(name string) *FieldInfo {
	if len(r.Fields) == 0 {
		return nil
	}
	if r.fieldByName == nil {
		r.fieldByName = make(map[string]int, len(r.Fields))
		for i := range r.Fields {
			r.fieldByName[r.Fields[i].Name] = i
		}
	}
	idx, ok := r.fieldByName[name]
	if !ok {
		return nil
	}
	return &r.Fields[idx]
}

// TypeRegistry is the central metadata store. Registration is single-threaded
// (typically at app setup); reads after registration are concurrent-safe
// because the underlying maps and dense slice are not mutated post-init.
type TypeRegistry struct {
	byType map[reflect.Type]*TypeRegistration
	byName map[string]*TypeRegistration
	byID   []*TypeRegistration // dense; index == TypeID; index 0 reserved as nil sentinel
}

// NewTypeRegistry returns an empty registry with the invalid-sentinel slot
// pre-populated so [TypeRegistry.ResolveByID](0) returns nil.
func NewTypeRegistry() *TypeRegistry {
	return &TypeRegistry{
		byType: make(map[reflect.Type]*TypeRegistration, 64),
		byName: make(map[string]*TypeRegistration, 64),
		byID:   []*TypeRegistration{nil},
	}
}

// Len reports the number of registered types (excluding the sentinel slot).
func (r *TypeRegistry) Len() int { return len(r.byID) - 1 }

// RegisterType registers T and returns its [TypeRegistration]. Idempotent —
// re-registering the same type yields the existing registration unchanged.
// Panics if T's fully-qualified name collides with a different registered
// type, since that's a programming error caught at init time.
func RegisterType[T any](r *TypeRegistry) *TypeRegistration {
	t := reflect.TypeFor[T]()
	return r.RegisterByType(t)
}

// RegisterByType is the non-generic registration entry point used internally
// (and by callers that already hold a reflect.Type, e.g. when integrating
// with the component registry). Idempotent.
func (r *TypeRegistry) RegisterByType(t reflect.Type) *TypeRegistration {
	if existing, ok := r.byType[t]; ok {
		return existing
	}
	name := typeName(t)
	if other, dup := r.byName[name]; dup && other.Type != t {
		panic(fmt.Sprintf("%s: %q already maps to %v", ErrDuplicateTypeName, name, other.Type))
	}

	reg := &TypeRegistration{
		ID:     TypeID(len(r.byID)),
		Name:   name,
		Type:   t,
		Fields: extractFields(t),
		Tags:   extractTypeTags(t),
		Size:   t.Size(),
		Align:  uintptr(t.Align()),
	}

	r.byType[t] = reg
	r.byName[name] = reg
	r.byID = append(r.byID, reg)

	// Late-bind field TypeIDs for fields whose type is already registered.
	// Fields registered later won't back-fill — callers that need the linked
	// IDs should register inner types first or call BindFieldTypeIDs.
	r.bindFieldTypeIDs(reg)
	return reg
}

// BindFieldTypeIDs (re)scans every registration and fills in [FieldInfo.TypeID]
// for fields whose type was registered after the parent. Cheap: O(N×F) over
// already-cached metadata, no reflect calls. Idempotent.
func (r *TypeRegistry) BindFieldTypeIDs() {
	for _, reg := range r.byID[1:] {
		r.bindFieldTypeIDs(reg)
	}
}

func (r *TypeRegistry) bindFieldTypeIDs(reg *TypeRegistration) {
	for i := range reg.Fields {
		if reg.Fields[i].TypeID != 0 {
			continue
		}
		if other, ok := r.byType[reg.Fields[i].Type]; ok {
			reg.Fields[i].TypeID = other.ID
		}
	}
}

// Resolve returns the registration for t, or nil if t is not registered.
func (r *TypeRegistry) Resolve(t reflect.Type) *TypeRegistration {
	return r.byType[t]
}

// ResolveByName returns the registration whose fully-qualified name matches,
// or nil if none does.
func (r *TypeRegistry) ResolveByName(name string) *TypeRegistration {
	return r.byName[name]
}

// RegisterAlias maps an additional (legacy) name to an already-registered type.
// It is the migration hook the hot-reload system uses to resolve a component
// serialized under a former type name after a rename — e.g. after renaming
// PlayerHealth to Health, RegisterAlias("…PlayerHealth", reflect.TypeFor[Health]())
// lets a pre-rename snapshot still deserialize into Health.
//
// The target type t MUST already be registered ([ErrTypeNotRegistered]
// otherwise). An alias that collides with a different registered canonical or
// alias name is rejected ([ErrDuplicateTypeName]); re-aliasing the same name to
// the same type is idempotent. Aliases are looked up by [ResolveByName] exactly
// like canonical names.
func (r *TypeRegistry) RegisterAlias(oldName string, t reflect.Type) error {
	reg, ok := r.byType[t]
	if !ok {
		return fmt.Errorf("%w: %v", ErrTypeNotRegistered, t)
	}
	if existing, dup := r.byName[oldName]; dup && existing.Type != t {
		return fmt.Errorf("%w: alias %q already maps to %v", ErrDuplicateTypeName, oldName, existing.Type)
	}
	r.byName[oldName] = reg
	return nil
}

// ResolveByID returns the registration with the given [TypeID]. The sentinel
// (id == 0) and out-of-range ids both yield nil.
func (r *TypeRegistry) ResolveByID(id TypeID) *TypeRegistration {
	if id == 0 || int(id) >= len(r.byID) {
		return nil
	}
	return r.byID[id]
}

// MustResolve returns the registration for T. Panics with [ErrTypeNotRegistered]
// when T has not been registered — convenient for setup-time code that knows
// the type was registered at init.
func MustResolve[T any](r *TypeRegistry) *TypeRegistration {
	t := reflect.TypeFor[T]()
	reg := r.Resolve(t)
	if reg == nil {
		panic(fmt.Sprintf("%s: %v", ErrTypeNotRegistered, t))
	}
	return reg
}

// typeName returns the fully-qualified name used by the byName index.
// Anonymous and built-in types fall back to [reflect.Type.String]; named
// types use "PkgPath.Name" so distinct packages with the same simple name
// don't collide.
func typeName(t reflect.Type) string {
	if t.Name() == "" || t.PkgPath() == "" {
		return t.String()
	}
	return t.PkgPath() + "." + t.Name()
}
