# Type Registry

**Version:** 0.2.0
**Status:** Stable
**Layer:** concept

## Related Specifications

- [component-system.md](l1-component-system.md) — Components register in the type registry
- [scene-system.md](l1-scene-system.md) — Scene serialization uses registry for dynamic types
- [definition-system.md](l1-definition-system.md) — Definition System resolves JSON type names through the registry

## Overview

The Type Registry provides dynamic introspection capabilities for the engine, allowing the engine to manipulate types at runtime without compile-time knowledge of their specific structures.

## 1. Motivation

A static language like Go requires a centralized dynamic metadata store for systems like GUI editors, scene serialization, and scripting to work. Without it, every new component would need manual boilerplate for serialization and UI mapping.

## 2. Constraints & Assumptions

- **Go standard lib**: Use `reflect` but minimize overhead for performance.
- **Registration**: All types wishing to be serializable or visible in the editor MUST register.
- **Safety**: Runtime type manipulation must still respect Go's type system at the boundaries.

## 3. Core Invariants

- **INV-1**: Every registered type must have a globally unique String identifier (e.g., `namespace/TypeName`).
- **INV-2**: The registry MUST support complex types, including nested structs, slices, and maps.
- **INV-3**: Registration must be possible from anywhere (ideally via an `init()` hook or similar engine entry point).

## 4. Detailed Design

### 4.1 Registry Store

The central `TypeRegistry` stores the following for each type:

1. **Type ID**: The human-readable unique string.
2. **Reflect.Type**: The Go reflect type handle.
3. **Field Metadata**: Names, types, offsets, and custom Go tags (e.g., `editor:"hidden"`, `range:"0,10"`).
4. **Hooks**: Per-type functions for cloning, default initialization, and serialization.

### 4.2 Attribute System

The registry supports attaching metadata (attributes) to fields or types:

- **Min/Max Range**: For numeric fields.
- **Visibility**: Whether it should be shown in the editor.
- **Tooltips**: Documentation strings.

### 4.3 Proxy Operations

The registry provides a generic `DynamicObject` wrapper (proxy) to:

- Get/Set fields by name.
- Iterate fields of an arbitrary struct.
- Construct new instances of a type by its string ID.

### 4.4 Default Processor Discovery

The type registry stores optional processor metadata per component type, enabling automatic system instantiation:

```plaintext
TypeRegistration
  type_id:             string
  reflect_type:        reflect.Type
  fields:              []FieldInfo
  hooks:               TypeHooks
  default_processor:   TypeID          // optional — system type to auto-create
  execution_mode:      ExecutionMode   // Runtime | EditorOnly | Both
```

When the World encounters a component type for the first time:

1. Query the registry for `default_processor`.
2. If set and `execution_mode` matches the current build, instantiate the system.
3. Recursively check the new system's `RequiredTypes` for their own default processors.

**Execution mode filter**: Systems marked `EditorOnly` are only instantiated in editor builds (behind `//go:build editor`). This prevents editor-only inspectors and debug visualizers from polluting production builds.

**Override**: Explicit system registration via `AddSystems()` takes precedence. If a user registers a custom system for a component type, the registry's `default_processor` is skipped for that type.

### 4.5 Interface-Based Type Matching

The registry supports querying types by interface satisfaction, not just concrete type:

```plaintext
TypeRegistry
  GetTypesByInterface(iface: reflect.Type) -> []TypeRegistration

// Example: find all types implementing Renderable
renderables := registry.GetTypesByInterface(reflect.TypeOf((*Renderable)(nil)).Elem())
```

This enables system dependency declarations like "I require any component implementing `Spatial`" rather than naming a specific concrete type. The registry builds an interface-to-types index at registration time, so lookups are O(1) per interface.

**Use cases**:

- A transform system processes any component implementing `Transformable`.
- An inspector plugin displays properties for any component implementing `Editable`.
- A serialization system handles any component implementing `Serializable`.

## 5. Open Questions

- Performance penalty for using `reflect` in hot paths (e.g., per-frame UI updates).
- Handling of type versioning (renaming fields in different versions of the engine).

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
| 0.1.0 | 2026-03-25 | Initial draft |
| 0.2.0 | 2026-03-26 | Added default processor discovery, interface-based type matching |
| 0.3.0 | 2026-05-14 | Phase 1 implementation complete; validated by `examples/ecs/poc` |
