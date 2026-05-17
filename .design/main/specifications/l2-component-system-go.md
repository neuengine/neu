# Component System — Go Implementation

**Version:** 0.1.0
**Status:** Stable
**Layer:** go
**Implements:** [component-system.md](l1-component-system.md)

## Overview

This specification defines the Go implementation of the component system. Components are pure data structs attached to entities. The component system handles registration (mapping Go types to unique IDs), storage strategy selection, lifecycle hooks, required component resolution, bundles, and the component registry that serves as the single source of truth for component metadata.

## Related Specifications

- [component-system.md](l1-component-system.md) — L1 concept specification (parent)

## 1. Motivation

The Go implementation of the Component system provides the data-oriented foundation for the engine. It ensures:

- Type-safe registration of Go structs as ECS components.
- Strategic storage selection (Table vs SparseSet) based on usage patterns.
- Automated lifecycle management via hooks (OnAdd, OnRemove, etc.).
- Resource-efficient component bundles for complex entity initialization.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: Relies on `reflect` for type metadata and `unique` for component identification.
- **Data Purity**: Components MUST be plain data structs; they should not contain logic or pointers to unmanaged memory.
- **Alignment**: Component storage must respect Go's alignment requirements for the target architecture.

## 3. Core Invariants

> [!NOTE]
> See [component-system.md §3](l1-component-system.md) for technology-agnostic invariants.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: Unique Type ID | `ComponentRegistry` assigns a unique `uint32` to each Go type version. |
| **INV-2**: Storage Selection | Struct tags in Go are used to override default Table storage to SparseSet. |
| **INV-3**: Lifecycle Hooks | `ComponentHooks` are invoked by the World during entity modification. |
| **INV-4**: Dependency Resolution | Required components are transitively resolved at registration time. |
| **INV-5**: Bundle dissolution | `Bundle.Components()` recursively flattens nested groups into a flat slice. |

## Go Package

```
internal/ecs/
```

All types in this spec belong to package `ecs`.

## Type Definitions

### ComponentID

```go
// ComponentID is a unique identifier assigned to each registered component type.
// Assigned sequentially starting from 1. Zero is reserved as invalid.
type ComponentID uint32
```

### StorageType

```go
// StorageType determines how a component's data is physically stored.
type StorageType uint8

const (
    // StorageTable is column-oriented archetype table storage (default).
    // Optimal for cache-friendly iteration.
    StorageTable StorageType = iota

    // StorageSparseSet is entity-indexed sparse set storage.
    // Optimal for components frequently added/removed.
    StorageSparseSet
)
```

### ComponentInfo

```go
// ComponentInfo holds metadata for a registered component type.
type ComponentInfo struct {
    ID                ComponentID
    Name              string          // fully qualified Go type name
    Type              reflect.Type    // runtime type information
    Size              uintptr         // size in bytes
    Alignment         uintptr         // alignment in bytes
    Storage           StorageType     // table or sparse set
    RequiredBy        []ComponentID   // transitive required component IDs
    Hooks             ComponentHooks  // lifecycle hook functions
    Immutable         bool            // if true, mutation after insertion is forbidden
    CloneBehavior     CloneBehavior   // how to clone this component
}
```

### ComponentHooks

```go
// OnAddHook fires the first time a component type appears on an entity.
type OnAddHook func(world *DeferredWorld, entity Entity)

// OnInsertHook fires on every insertion (including overwrite).
type OnInsertHook func(world *DeferredWorld, entity Entity)

// OnReplaceHook fires when a component value is overwritten. Old value accessible.
type OnReplaceHook func(world *DeferredWorld, entity Entity)

// OnRemoveHook fires just before a component is removed from an entity.
type OnRemoveHook func(world *DeferredWorld, entity Entity)

// ComponentHooks groups all lifecycle hooks for a component type.
type ComponentHooks struct {
    OnAdd     OnAddHook
    OnInsert  OnInsertHook
    OnReplace OnReplaceHook
    OnRemove  OnRemoveHook
}
```

### CloneBehavior

```go
// CloneBehavior defines how a component is duplicated when cloning an entity.
type CloneBehavior uint8

const (
    // CloneDeep performs a deep copy of the component data (default).
    CloneDeep CloneBehavior = iota

    // CloneIgnore skips this component during entity cloning.
    CloneIgnore

    // CloneCustom uses a user-provided clone function.
    CloneCustom
)
```

### ComponentRegistry

```go
// ComponentRegistry maps Go types to ComponentIDs and stores ComponentInfo
// for all registered component types. It is owned by the World.
type ComponentRegistry struct {
    infosByID   []ComponentInfo            // indexed by ComponentID (dense)
    typeToID    map[reflect.Type]ComponentID
    nameToID    map[string]ComponentID     // for serialization/debug lookup
    nextID      ComponentID                // next ID to assign
}

// NewComponentRegistry creates an empty registry.
func NewComponentRegistry() *ComponentRegistry

// Register registers a component type and returns its ComponentID.
// Panics if the type is already registered. Thread-unsafe — call during setup only.
func (r *ComponentRegistry) Register(info ComponentInfo) ComponentID

// Lookup returns the ComponentID for a Go type, or (0, false) if not registered.
func (r *ComponentRegistry) Lookup(t reflect.Type) (ComponentID, bool)

// Info returns the ComponentInfo for a given ComponentID.
// Panics if the ID is out of range.
func (r *ComponentRegistry) Info(id ComponentID) *ComponentInfo

// LookupByName returns the ComponentID for a type name, or (0, false).
func (r *ComponentRegistry) LookupByName(name string) (ComponentID, bool)

// Len returns the number of registered component types.
func (r *ComponentRegistry) Len() int
```

### Generic Registration Helper

```go
// RegisterComponent registers a component type T with the World's registry
// using reflection to derive metadata. Storage strategy is determined by
// the struct tag `ecs:"sparse"` if present, otherwise defaults to StorageTable.
//
// Pseudo-code:
//   t := reflect.TypeOf((*T)(nil)).Elem()
//   if already registered: return existing ID
//   determine storage from struct tag
//   build ComponentInfo from reflect data
//   resolve required components transitively
//   register and return ID
func RegisterComponent[T any](world *World) ComponentID
```

Storage selection via struct tags:

```go
// Table storage (default — no tag needed):
type Position struct {
    X, Y, Z float32
}

// Sparse set storage (via struct tag):
type Poisoned struct {
    _       struct{} `ecs:"sparse"`
    Damage  float32
    Elapsed float32
}
```

### Required Components

```go
// RequiredComponents is an optional interface that component types can
// implement to declare transitive dependencies.
type RequiredComponents interface {
    // Required returns ComponentData for all required components with their
    // default values. These are inserted automatically if not already present.
    Required() []ComponentData
}
```

Resolution algorithm (pseudo-code):

```
func resolveRequired(registry, componentID) -> []ComponentID:
    visited = set{}
    stack = [componentID]
    result = []

    while stack not empty:
        current = stack.pop()
        if current in visited: continue
        visited.add(current)
        info = registry.Info(current)
        for each req in info.RequiredBy:
            if req in visited: error("circular dependency")
            result.append(req)
            stack.push(req)

    return result  // in dependency order (leaves first)
```

Circular dependencies are detected at registration time and cause a panic.

### Bundle

```go
// Bundle is an interface for groups of components that are spawned together.
// A bundle dissolves into individual ComponentData on spawn — bundles are NOT
// stored as components.
type Bundle interface {
    // Components returns all component data in this bundle, including
    // any nested bundles (flattened recursively).
    Components() []ComponentData
}

// ComponentData is a type-erased component value paired with its ID.
type ComponentData struct {
    ID    ComponentID
    Value any
}
```

### Helper for Building ComponentData

```go
// NewComponentData creates a ComponentData from a typed value.
// The component type must already be registered in the World.
//
// Pseudo-code:
//   id = RegisterComponent[T](world)
//   return ComponentData{ID: id, Value: value}
func NewComponentData[T any](world *World, value T) ComponentData
```

## Key Methods

### Registration Flow

1. User defines a component as a plain Go struct.
2. On first use (e.g., `Spawn`, `Insert`, `RegisterComponent[T]`), the type is registered via reflection.
3. `reflect.Type` is used to derive name, size, and alignment.
4. Struct tags are inspected for `ecs:"sparse"` to determine storage type.
5. If the type implements `RequiredComponents`, dependencies are transitively resolved.
6. `ComponentInfo` is stored in the registry; `ComponentID` is returned.

### Hook Invocation Order

When inserting component A that requires B and C (where B requires C):

1. Resolve full dependency set: `[C, B, A]` (leaves first).
2. Insert missing components with defaults: C, then B.
3. Fire `OnAdd` + `OnInsert` for C.
4. Fire `OnAdd` + `OnInsert` for B.
5. Fire `OnAdd` + `OnInsert` for A.

On overwrite (A already exists):

1. Fire `OnReplace` for A.
2. Fire `OnInsert` for A.

### Immutable Component Enforcement

```
// At query construction time:
func validateQueryAccess(registry, access Access) error:
    for each componentID in access.WriteSet:
        if registry.Info(componentID).Immutable:
            return error("cannot mutably access immutable component")
    return nil
```

## Performance Strategy

- **ComponentInfo stored in dense slice**: Indexed by `ComponentID` (uint32), O(1) lookup.
- **Registration is one-time cost**: Happens during setup, not on hot path.
- **ComponentData uses `any`**: Type-erased for flexibility. On the hot path (iteration), components are accessed through typed column pointers, not through `any` boxing.
- **Struct tag parsing**: Only at registration time. Cached in `ComponentInfo.Storage`.
- **Required component resolution**: Pre-computed at registration. No runtime dependency walking.

## Error Handling

- Double registration of same type: return existing ID (idempotent), do not panic.
- Circular required components: panic at registration time with descriptive message.
- Unregistered component in `ComponentData`: panic with message indicating the type needs registration.
- Immutable component mutation attempt: return error at query validation time.
- Invalid `ComponentID` (0 or out of range): panic in `Info()` — indicates a programming error.

## Testing Strategy

- **Registration**: Register multiple types, verify unique IDs, verify idempotent re-registration.
- **Storage selection**: Verify struct tag parsing selects correct `StorageType`.
- **Required components**: Test transitive resolution, circular dependency detection, default value insertion.
- **Hooks**: Verify each hook fires exactly once per event, in correct order.
- **Bundles**: Test nested bundle flattening, verify all components appear in `Components()`.
- **Immutable enforcement**: Verify mutable query access is rejected for immutable components.
- **Benchmarks**: `BenchmarkRegisterComponent`, `BenchmarkComponentInfoLookup`.
- **Reflection edge cases**: Zero-size components (tag types), components with unexported fields.

## 7. Drawbacks & Alternatives

- **Drawback**: Using `any` in `ComponentData` causes interface boxing overhead during initialization.
- **Alternative**: Code generation for every component type.
- **Decision**: Reflection-based registration with `any` for initialization is flexible; hot paths use raw memory pointers to bypass this.

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
