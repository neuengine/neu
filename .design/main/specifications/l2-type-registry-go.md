# Type Registry — Go Implementation

**Version:** 0.1.0
**Status:** Stable
**Layer:** go
**Implements:** [type-registry.md](l1-type-registry.md)

## Overview

Go-level design for the type registry. Provides runtime introspection of registered types using Go's `reflect` package, with aggressive caching to minimize reflection overhead on hot paths. Covers type registration, field metadata extraction, struct tag parsing, dynamic object proxies, serialization hooks, and integration with the component registry.

## Related Specifications

- [type-registry.md](l1-type-registry.md) — L1 concept specification (parent)

## 1. Motivation

The Go implementation of the Type Registry provides the metadata foundation for runtime introspection, dynamic UI generation, and serialization. It ensures:

- Type safety through `reflect.Type` validation.
- Efficient metadata retrieval via densly indexed `TypeID` lookups.
- Extensible type-level and field-level attributes using Go struct tags.
- Support for type-erased manipulation via `DynamicObject` proxies.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: Relies heavily on the `reflect` package and generics.
- **Initialization-Only Registration**: Types must be registered during application setup; registration is not thread-safe and is locked after init.
- **Struct Alignment**: Relies on `reflect` to correctly compute field offsets and struct padding for memory-safe access.

## 3. Core Invariants

> [!NOTE]
> See [type-registry.md §3](l1-type-registry.md) for technology-agnostic invariants.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: Unique TypeID | `TypeRegistry` assigns monotonically increasing `TypeID` in a central registry. |
| **INV-2**: Bijective Map | `byType` and `byName` maps ensure 1:1 mapping between Go types and registry entries. |
| **INV-3**: Metadata Persistence | `TypeRegistration` caches all field metadata, tags, and size/alignment on first registration. |
| **INV-4**: Type-Safe Proxy | `DynamicObject.Set` verifies types against `FieldInfo.Type` before mutation. |
| **INV-5**: Hook Priority | `SerializeValue` and `DeserializeValue` prioritize custom `TypeHooks` over reflection. |

## Go Package

```
internal/ecs
```

No external dependencies. Uses only `reflect`, `fmt`, `sync`, and `strings` from the standard library.

## Type Definitions

### TypeID

```go
// TypeID is a unique numeric identifier assigned sequentially at registration time.
type TypeID uint32
```

### TypeRegistry

```go
// TypeRegistry is the central metadata store for all registered types.
// Thread-safe for reads after initialization. Registration is not concurrent.
type TypeRegistry struct {
    byType    map[reflect.Type]*TypeRegistration // primary index
    byName    map[string]reflect.Type            // name -> type reverse lookup
    byID      []*TypeRegistration                // indexed by TypeID (dense)
    nextID    TypeID                             // next available ID
}
```

### TypeRegistration

```go
// TypeRegistration holds all metadata for a single registered type.
type TypeRegistration struct {
    ID         TypeID
    Name       string             // fully qualified name: "package/TypeName"
    Type       reflect.Type       // the Go reflect type
    Fields     []FieldInfo        // struct field metadata (empty for non-structs)
    Hooks      TypeHooks          // optional lifecycle hooks
    Tags       TypeTags           // type-level attributes parsed from struct tags
    Size       uintptr            // cached size in bytes
    Align      uintptr            // cached alignment in bytes
}
```

### FieldInfo

```go
// FieldInfo stores pre-computed metadata for a single struct field.
type FieldInfo struct {
    Name       string             // Go field name
    Type       reflect.Type       // field type
    TypeID     TypeID             // TypeID of the field type (0 if not registered)
    Offset     uintptr            // byte offset within the struct (from reflect)
    Index      int                // field index in the struct
    Tags       FieldTags          // parsed struct tag attributes
    Exported   bool               // whether the field is exported
}
```

### Struct Tag Attributes

```go
// FieldTags holds parsed attributes from Go struct tags.
type FieldTags struct {
    // ECS-specific tags (from `ecs:"..."`)
    Storage    StorageStrategy    // "table" or "sparse" (only meaningful on component types)
    Ignore     bool               // "ignore" — skip this field in serialization

    // Editor tags (from `editor:"..."`)
    Hidden     bool               // "hidden" — not visible in editor
    ReadOnly   bool               // "readonly" — visible but not editable
    Label      string             // custom display label

    // Range tags (from `range:"min,max"`)
    HasRange   bool
    RangeMin   float64
    RangeMax   float64

    // Raw contains unparsed tag string for extension by user code.
    Raw        reflect.StructTag
}

// StorageStrategy determines how a component is stored in archetypes.
type StorageStrategy uint8

const (
    StorageTable     StorageStrategy = iota // dense, column-oriented (default)
    StorageSparseSet                        // sparse set, fast add/remove
)

// TypeTags holds type-level (not field-level) attributes.
type TypeTags struct {
    Storage StorageStrategy // default storage strategy for this component type
}
```

### Type Hooks

```go
// TypeHooks contains optional lifecycle functions for a registered type.
type TypeHooks struct {
    Clone       CloneFunc
    Default     DefaultFunc
    Serialize   SerializeFunc
    Deserialize DeserializeFunc
}

// CloneFunc creates a deep copy of a value.
type CloneFunc func(src any) any

// DefaultFunc creates a zero/default instance of the type.
type DefaultFunc func() any

// SerializeFunc converts a value to bytes.
type SerializeFunc func(value any) ([]byte, error)

// DeserializeFunc restores a value from bytes.
type DeserializeFunc func(data []byte) (any, error)
```

### DynamicObject

```go
// DynamicObject is a type-erased proxy that provides field-level access
// to an arbitrary registered struct via reflection.
// Internally wraps a reflect.Value (must be addressable).
type DynamicObject struct {
    reg   *TypeRegistration
    value reflect.Value       // must be a pointer's Elem (addressable)
}
```

## Key Methods

### Registration

```
// RegisterType[T] registers type T in the registry.
// Extracts field info, parses struct tags, computes size/alignment.
// Panics if T is already registered (programming error, caught at init time).
func RegisterType[T any](registry *TypeRegistry)

// RegisterTypeWithHooks[T] registers type T with custom lifecycle hooks.
func RegisterTypeWithHooks[T any](registry *TypeRegistry, hooks TypeHooks)

// MustResolve[T] returns the TypeRegistration for T.
// Panics if T is not registered.
func MustResolve[T any](registry *TypeRegistry) *TypeRegistration

// Resolve returns the TypeRegistration for the given reflect.Type, or nil.
func (r *TypeRegistry) Resolve(t reflect.Type) *TypeRegistration

// ResolveByName returns the TypeRegistration for the given string name, or nil.
func (r *TypeRegistry) ResolveByName(name string) *TypeRegistration

// ResolveByID returns the TypeRegistration for the given TypeID, or nil.
func (r *TypeRegistry) ResolveByID(id TypeID) *TypeRegistration
```

### Field Info Extraction

```
// extractFields uses reflect to iterate struct fields and build FieldInfo entries.
// Parses struct tags for `ecs`, `editor`, and `range` keys.
// Skips unexported fields for serialization but still records them for completeness.
//
// Called once at registration time — results are cached on TypeRegistration.
func extractFields(t reflect.Type) []FieldInfo

// parseFieldTags parses the struct tag string into a FieldTags struct.
//   ecs:"storage:sparse,ignore"
//   editor:"hidden,label:Hit Points"
//   range:"0,100"
func parseFieldTags(tag reflect.StructTag) FieldTags
```

### DynamicObject Operations

```
// NewDynamicObject wraps an existing value in a DynamicObject proxy.
// The value must be a pointer to a registered struct.
func NewDynamicObject(registry *TypeRegistry, ptr any) (*DynamicObject, error)

// NewDynamicObjectByID creates a new zero-valued instance of the type
// identified by TypeID and wraps it in a DynamicObject.
func NewDynamicObjectByID(registry *TypeRegistry, id TypeID) (*DynamicObject, error)

// Get returns the value of a field by name.
// Returns error if field does not exist.
func (d *DynamicObject) Get(fieldName string) (any, error)

// Set sets the value of a field by name.
// Returns error if field does not exist or value type mismatches.
func (d *DynamicObject) Set(fieldName string, value any) error

// Fields returns an iterator over all fields and their current values.
func (d *DynamicObject) Fields() DynamicFieldIterator

// TypeID returns the TypeID of the wrapped type.
func (d *DynamicObject) TypeID() TypeID

// Value returns the underlying Go value as any.
func (d *DynamicObject) Value() any
```

### DynamicFieldIterator

```
// DynamicFieldIterator walks the fields of a DynamicObject.
type DynamicFieldIterator struct { ... }

// Next returns the next field info and its current value.
// Returns false when exhausted.
func (it *DynamicFieldIterator) Next() (FieldInfo, any, bool)
```

### Integration with ComponentRegistry

```
// The TypeRegistry is a superset of component type metadata.
// When a component is registered via RegisterComponent[T], it also calls
// RegisterType[T] if not already registered.
//
// ComponentRegistry adds ECS-specific metadata on top:
//   - Storage strategy (from ecs struct tag or explicit override)
//   - Component hooks (OnAdd, OnInsert, OnRemove)
//   - Required components list
//
// The relationship:
//   TypeRegistry  — knows about ALL registered types (components, resources, events, etc.)
//   ComponentRegistry — subset, only components, adds ECS-specific fields

// EnsureTypeRegistered is called by component/resource/event registration
// to guarantee the type exists in the TypeRegistry.
func EnsureTypeRegistered[T any](world *World)
```

### Serialization

```
// SerializeValue uses the registered SerializeFunc hook if present,
// otherwise falls back to a default reflection-based serializer.
func (r *TypeRegistry) SerializeValue(value any) ([]byte, error)

// DeserializeValue uses the registered DeserializeFunc hook if present,
// otherwise falls back to a default reflection-based deserializer.
func (r *TypeRegistry) DeserializeValue(typeID TypeID, data []byte) (any, error)
```

## Performance Strategy

- **All reflect calls cached at registration time**: `reflect.Type`, field offsets, sizes, alignments, and parsed tags are stored in `TypeRegistration` and `FieldInfo`. No `reflect.TypeOf` calls on the hot path.
- **DynamicObject field access by offset**: `Get`/`Set` use pre-computed `FieldInfo.Offset` with `reflect.NewAt` for direct memory access — avoids `reflect.Value.FieldByName` string lookup.
- **Field name to index map**: Each `TypeRegistration` can optionally cache a `map[string]int` for O(1) field name resolution (built lazily on first `DynamicObject.Get` call).
- **Dense TypeID indexing**: `byID` is a slice, not a map. TypeID lookup is O(1) array index.
- **Registration is init-time only**: No locking needed for reads after initialization. `TypeRegistry` is effectively immutable after the app builds.
- **Serialization hooks bypass reflect**: When custom `SerializeFunc` / `DeserializeFunc` are provided, reflection is not used at all.

## Error Handling

| Condition | Behavior |
| :--- | :--- |
| Register duplicate type | Panic (programming error, caught at init) |
| `MustResolve` on unregistered type | Panic with descriptive message |
| `Resolve` on unregistered type | Returns `nil` (caller decides) |
| `DynamicObject.Get` unknown field | Returns `ErrFieldNotFound` |
| `DynamicObject.Set` type mismatch | Returns `ErrFieldTypeMismatch` |
| `NewDynamicObject` with non-pointer | Returns `ErrNotPointer` |
| `NewDynamicObject` with unregistered type | Returns `ErrTypeNotRegistered` |
| Malformed struct tag | Logged via `slog.Warn` at registration, field tags default to zero values |

```go
var (
    ErrFieldNotFound     = errors.New("ecs: field not found")
    ErrFieldTypeMismatch = errors.New("ecs: field value type mismatch")
    ErrNotPointer        = errors.New("ecs: DynamicObject requires a pointer value")
    ErrTypeNotRegistered = errors.New("ecs: type not registered in TypeRegistry")
)
```

## Testing Strategy

- **Registration tests**: Register structs with various field types (primitives, nested structs, slices, maps). Verify `FieldInfo` correctness.
- **Struct tag parsing**: Test all tag formats: `ecs:"storage:sparse"`, `editor:"hidden,label:Foo"`, `range:"0,100"`, combined tags, malformed tags.
- **DynamicObject tests**: Get/Set all field types, verify error cases (unknown field, type mismatch, unexported field).
- **NewDynamicObjectByID**: Create instance, verify zero values, modify via Set, read back.
- **Serialization roundtrip**: Register type with hooks, serialize, deserialize, compare. Also test default (reflection-based) path.
- **Integration tests**: Register as component, verify TypeRegistry and ComponentRegistry both contain the type.
- **Benchmarks**: `BenchmarkResolveByID`, `BenchmarkDynamicObjectGet`, `BenchmarkDynamicObjectSet`, `BenchmarkFieldIteration`, `BenchmarkSerializeRoundtrip`.
- **Go limitation test**: Verify that DynamicObject correctly handles types that cannot be constructed at runtime (interfaces, function types) with appropriate errors.

## 7. Drawbacks & Alternatives

- **Drawback**: Reflection is slower than direct code, adding overhead to dynamic operations (e.g., editor UI).
- **Alternative**: Code generation for all introspectable types.
- **Decision**: Reflection is used with aggressive caching of offsets to balance performance and developer flexibility.

## Canonical References

<!-- MANDATORY for Stable status. List authoritative source files that downstream agents
     MUST read before implementing this spec. Use relative paths from project root.
     Stub state — fill with concrete files when implementation begins (Phase 1+). -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

<!-- Empty table = no canonical sources yet. Populate one row per authoritative file
     when implementation lands (Phase 1+). Stable promotion requires ≥1 row. -->

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-03-26 | Initial L2 draft |
| 0.2.0 | 2026-05-14 | Phase 1 implementation complete; validated by `examples/ecs/poc` |
