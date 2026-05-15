# Code Generation Tools Specification

**Version:** 0.2.0
**Status:** Draft
**Layer:** tool
**Implements:** l1-build-tooling.md

## Overview

Go's static nature and lack of variadic generics often lead to boilerplate or reflection overhead in ECS architectures. This specification defines the `ecs-gen` utility and the use of `go:generate` to automatically produce type-safe query wrappers, component registration code, and serialization logic.

## 1. Motivation

- **Type Safety**: Eliminate `interface{}` and type assertions in system logic.
- **Performance**: Bypass `reflect` at runtime by generating code that uses concrete types.
- **Ergonomics**: Reduce manual boilerplate for registering components and defining queries.
- **Maintenance**: Ensure metadata (stored in struct tags) is in sync with generated code.

## 2. Key Tools

### 2.1 `ecs-gen` Utility

A command-line tool that scans Go source files for ECS-specific annotations and generates corresponding code.

**Command Usage:**

```bash
go run cmd/ecs-gen/main.go --path ./internal/game --output ./internal/game/ecs_gen.go
```

### 2.2 `go:generate` Integration

Users add a generate directive to their package or component files:

```go
//go:generate ecs-gen
package components

// Position is a component.
//ecs:component
type Position struct {
    X, Y float64
}
```

## 3. Supported Annotations

| Annotation | Description | Location |
| :--- | :--- | :--- |
| `//ecs:component` | Marks a struct as an ECS component. Generates registration and storage code. | Struct |
| `//ecs:resource` | Marks a struct as a global resource. | Struct |
| `//ecs:query` | Generates a type-safe query wrapper (e.g., `PositionVelocityQuery`). | Variable/System |
| `//ecs:bundle` | Defines a group of components to be spawned together. | Struct |

## 4. Generated Artifacts

### 4.1 Type-Safe Queries

Instead of calling `world.Query1[Position]()`, `ecs-gen` can generate:

```go
type PositionQuery struct {
    q *ecs.QueryState1[Position]
}

func (pq *PositionQuery) Iter(world *ecs.World, fn func(e ecs.Entity, p *Position)) {
    pq.q.Iter(world, fn)
}
```

### 4.2 Registration Boilerplate

Automatically generate a `RegisterAll(world *ecs.World)` function that calls `RegisterComponent[T]` for every annotated type in the package.

### 4.3 Storage Optimizations

Generate specialized storage accessors that bypass the generic `[]byte` path for specific hot-path components.

## 5. Implementation Strategy

1. **Parser**: Use `go/ast`, `go/parser`, and `go/token` to scan source files.
2. **Template**: Use `text/template` for generating Go code.
3. **Validation**: Check for duplicate component names or invalid struct tags.
4. **Formatting**: Always pipe output through `gofmt`.

## 6. Open Questions

- Should we support custom template overrides for code generation?
- Integration with external build tools like `buf` or `cmake`?

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
| 0.1.0 | 2026-03-27 | Initial draft |
| 0.2.0 | 2026-05-15 | Added `Implements: l1-build-tooling.md` parent link (resolves MISSING_IMPLEMENTS layer-integrity gap flagged by `/magic-analyze`) |
