package component

import (
	"fmt"
	"reflect"
)

// Registry maps Go types to component [ID]s and stores [Info] for every
// registered type. The registry is the single source of truth for component
// metadata used by the World.
//
// Concurrency: the registry is NOT safe for concurrent registration. All
// registrations must complete during World setup before systems begin to
// run. Read-only methods (Lookup, Info, Len) are safe to call from any
// goroutine once registration is finished.
//
// Determinism: IDs are allocated sequentially starting at 1 in registration
// order. Re-registering the same Go type returns the previously assigned ID
// (idempotent), so two worlds that perform identical registration sequences
// produce identical IDs — a property the archetype-hashing layer (T-1C03)
// relies on.
type Registry struct {
	typeToID  map[reflect.Type]ID
	nameToID  map[string]ID
	infosByID []Info // index 0 reserved as the invalid sentinel
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		// Reserve index 0 so that ID(0) -> Info{} never points at a real
		// component. infosByID[id] is then a direct lookup.
		infosByID: []Info{{}},
		typeToID:  make(map[reflect.Type]ID),
		nameToID:  make(map[string]ID),
	}
}

// Register adds a component type to the registry and returns its assigned
// [ID]. Re-registering the same reflect.Type is idempotent and returns the
// previously assigned ID; the caller-supplied [Info] is ignored on the
// duplicate path.
//
// Register panics if:
//   - info.Type is nil;
//   - a different reflect.Type is registered under a name that already
//     belongs to another registered type (name collision across packages);
//   - the type's [RequiredComponents] graph contains a cycle.
func (r *Registry) Register(info Info) ID {
	if info.Type == nil {
		panic("component.Registry.Register: Info.Type is nil")
	}
	if existing, ok := r.typeToID[info.Type]; ok {
		return existing
	}
	id := r.insert(info)
	r.infosByID[id].RequiredBy = r.resolveRequired(
		&r.infosByID[id], make(map[reflect.Type]bool),
	)
	return id
}

// insert performs the low-level slot allocation for a new component type.
// It assigns the next sequential ID, fills derived metadata (size,
// alignment, qualified name), and updates the lookup maps. It does NOT
// resolve required-component dependencies — that step is wrapped by
// [Register] / [registerType] so cycle detection is centralised.
func (r *Registry) insert(info Info) ID {
	name := info.Name
	if name == "" {
		name = qualifiedTypeName(info.Type)
	}
	if other, ok := r.nameToID[name]; ok {
		panic(fmt.Sprintf(
			"component.Registry: name %q is already bound to %s (id=%d); cannot bind to %s",
			name, r.infosByID[other].Type, other, info.Type,
		))
	}

	id := ID(len(r.infosByID))
	info.ID = id
	info.Name = name
	if info.Size == 0 {
		info.Size = info.Type.Size()
	}
	if info.Alignment == 0 {
		info.Alignment = uintptr(info.Type.Align())
	}
	r.infosByID = append(r.infosByID, info)
	r.typeToID[info.Type] = id
	r.nameToID[name] = id
	return id
}

// Lookup returns the [ID] previously assigned to a Go type, or (0, false) if
// the type was never registered.
func (r *Registry) Lookup(t reflect.Type) (ID, bool) {
	id, ok := r.typeToID[t]
	return id, ok
}

// LookupByName returns the [ID] for a fully-qualified component type name,
// or (0, false) if no matching type was registered.
func (r *Registry) LookupByName(name string) (ID, bool) {
	id, ok := r.nameToID[name]
	return id, ok
}

// Info returns a pointer to the metadata for the given component ID.
// Panics on the invalid sentinel (id == 0) or an out-of-range id.
func (r *Registry) Info(id ID) *Info {
	if id == 0 || int(id) >= len(r.infosByID) {
		panic(fmt.Sprintf("component.Registry.Info: invalid id %d (registered=%d)", id, r.Len()))
	}
	return &r.infosByID[id]
}

// Len returns the number of registered component types.
func (r *Registry) Len() int {
	return len(r.infosByID) - 1 // subtract the reserved sentinel slot
}

// Each invokes fn for every registered component in ID order. Iteration
// stops if fn returns false. Useful for diagnostics and serialization.
func (r *Registry) Each(fn func(*Info) bool) {
	for i := 1; i < len(r.infosByID); i++ {
		if !fn(&r.infosByID[i]) {
			return
		}
	}
}

// RegisterType registers component type T using reflection to derive the
// metadata. It is the canonical entry point for code that does not need to
// override storage, hooks, or clone behavior.
//
// Re-registration of the same T is idempotent: the previously assigned ID
// is returned without modifying the existing [Info].
func RegisterType[T any](r *Registry) ID {
	t := reflect.TypeFor[T]()
	return r.Register(Info{
		Type:    t,
		Storage: StorageTable,
	})
}

// RegisterByType registers a component identified by reflect.Type using the
// default [Info] (StorageTable). Idempotent — returns the existing ID if the
// type is already registered. Used by the World layer to auto-register
// component types passed via [Data] without resorting to generics.
func (r *Registry) RegisterByType(t reflect.Type) ID {
	if t == nil {
		panic("component.Registry.RegisterByType: nil reflect.Type")
	}
	if existing, ok := r.typeToID[t]; ok {
		return existing
	}
	return r.Register(Info{Type: t, Storage: StorageTable})
}

// qualifiedTypeName returns "<pkg-path>.<type-name>" for named types and
// reflect.Type.String() for anonymous types.
func qualifiedTypeName(t reflect.Type) string {
	if name := t.Name(); name != "" {
		if pkg := t.PkgPath(); pkg != "" {
			return pkg + "." + name
		}
		return name
	}
	return t.String()
}
