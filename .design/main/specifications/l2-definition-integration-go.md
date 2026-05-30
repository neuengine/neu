# Definition Integration — Go Implementation

**Version:** 0.1.0
**Status:** Draft
**Layer:** go
**Implements:** [l1-definition-integration.md](l1-definition-integration.md)

## Overview

Go-level design for the definition system's integration boundary: the stable
`pkg/editor` `DefinitionEditorInterface` an external GUI editor consumes, and the
deterministic, order-based field serialization contract with custom entity
hooks. The editor's only coupling is the interface in `pkg/editor` (interface +
data only per multi-repo INV-3) — interpreter internals stay in
`internal/definition`. Serialization is driven by explicit `order` (not struct
declaration order) so files stay forward/backward compatible across engine
versions; missing ordered fields deserialize to their default.

## Related Specifications

- [l1-definition-integration.md](l1-definition-integration.md) — L1 concept specification (parent)
- [l2-definition-system-go.md](l2-definition-system-go.md) — the content model + interpreter this surface wraps (now Stable)
- [l2-multi-repo-architecture-go.md](l2-multi-repo-architecture-go.md) — `pkg/editor` interface-only boundary the editor depends on
- [l2-type-registry-go.md](l2-type-registry-go.md) — source of schema, defaults, and `SerializationField` metadata
- [l2-scene-system-go.md](l2-scene-system-go.md) — `DynamicScene` + entity serialization consumer of the hooks

## 1. Motivation

The L1 separates the definition *integration surface* from the *content model*.
The Go binding places the editor contract in `pkg/editor` (so the separate
editor repo links only a stable, internal-free interface — multi-repo INV-3) and
formalizes order-based serialization + entity hooks in the type-registry/scene
layer so file compatibility and serialization depth are explicit, not incidental
to struct layout.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: `pkg/editor` is interface + data types only (no logic, no `internal/` import — enforced by the multi-repo guard test).
- Definitions are process-local; never serialized across a network boundary at runtime (INV-2) — the package exposes no network path.
- Serialization order is explicit via a `SerializationField.Order int`; declaration order is never load-bearing (INV-3).
- Entity hooks suppress default-component auto-insertion during deserialize and clear stale components on re-deserialize (INV-4).
- **C-003**: JSON via stdlib; ordering + hooks are reflection over the existing type registry, no external dep.

## 3. Core Invariants

> [!NOTE]
> See [l1-definition-integration.md §3](l1-definition-integration.md) for
> INV-1…INV-4. Go-specific compliance is tabulated in §4.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: editor couples only to the stable `DefinitionEditorInterface` | The interface + its data types (`PropertyInfo`, `ValidationError`, `DefinitionType`) live in `pkg/editor/definition.go` — interface-and-type only; the implementation in `internal/definition` is never exposed. The multi-repo `TestEditorPkgIsContractOnly` guard enforces this structurally. |
| **INV-2**: definitions never cross a network boundary at runtime | The package exposes only local-file/asset load + in-process interpret; there is no serialize-to-wire API. Backend comms use typed protocol messages elsewhere. |
| **INV-3**: explicit, version-stable field order; missing ordered fields → default | A `SerializationField{Name, Order, Mode, TypeHint}` table per type (from the type registry) drives a `serializeOrdered`/`deserializeOrdered` pair; fields are written/read by ascending `Order`; an absent ordered field deserializes to the registry default. |
| **INV-4**: entity hooks suppress default-component insertion + clear stale on reload | `EntitySerializer.PreSerialize(e, Deserialize)` constructs the entity with a suppress-defaults flag; `PostDeserialize` validates the component set + fires deferred hooks; re-deserialize into an existing entity clears its components first. |

## Go Package

```
pkg/editor/                  // interface + data only (multi-repo INV-3)
  definition.go              // DefinitionEditorInterface, DefinitionEditorPlugin, DefinitionType
  property.go                // PropertyInfo, EditorProperty, ValidationError (shared with inspector)
internal/definition/
  editorapi.go               // concrete DefinitionEditorInterface impl over the interpreter
  serialize.go               // SerializationField, serializeOrdered/deserializeOrdered (INV-3)
  entityhooks.go             // EntitySerializer Pre/Post hooks (INV-4)
```

## Type Definitions

```go
// --- pkg/editor (stable surface; no logic) ---

type DefinitionType uint8 // UI | Scene | Flow | Template

// DefinitionEditorInterface is the editor's sole coupling to definitions (INV-1).
type DefinitionEditorInterface interface {
    GetDefinitionType(asset AssetHandle) DefinitionType
    GetPropertyList(node DefinitionNode) []PropertyInfo
    GetPropertyValue(node DefinitionNode, name string) any
    SetPropertyValue(node DefinitionNode, name string, value any) error
    ValidateDefinition(asset AssetHandle) []ValidationError
}

// DefinitionEditorPlugin lets a plugin claim definition types (handles() dispatch).
type DefinitionEditorPlugin interface {
    Handles(t DefinitionType) bool
    Edit(node DefinitionNode)
    GetInspectorProperties(node DefinitionNode) []EditorProperty
}

// DefinitionNode exposes per-property revert + dynamic-list signalling.
type DefinitionNode interface {
    CanRevert(property string) bool
    GetRevertValue(property string) any
    NotifyPropertyListChanged()
}

// --- internal/definition (serialization contract) ---

type SerializationMode uint8
const (SerDefault SerializationMode = iota; SerContent; SerIgnore; SerAlways)

type SerializationField struct {
    Name     string
    Order    int               // explicit; e.g. -10, 0, 100 (INV-3)
    Mode     SerializationMode
    TypeHint string            // polymorphic discriminator
}

type EntitySerializer interface {
    PreSerialize(e *ecs.Entity, mode SerializeMode) // Deserialize ⇒ suppress default components
    PostDeserialize(e *ecs.Entity)                  // resolve refs, validate, fire hooks
}
```

## Performance Strategy

- **Order table built once per type**: `[]SerializationField` is cached in the type registry; serialize/deserialize iterate the cached slice, not reflect-walk per call.
- **`Content` vs `Default` bounds depth**: `SerContent` inlines a sub-graph; `SerDefault` writes a reference — preventing accidental whole-world serialization when a component holds a shared-resource handle (L1 §4.4).
- **Editor surface is cold**: `DefinitionEditorInterface` calls happen on editor interaction, never per frame; clarity over speed.

## Error Handling

| Condition | Behavior |
| :--- | :--- |
| `SetPropertyValue` with a wrong-typed value | `ValidationError` (typed); the node is not mutated |
| Missing ordered field on deserialize | apply the registry default (INV-3) — not an error |
| Unknown `TypeHint` for a polymorphic field | `ValidationError`; field skipped |
| Re-deserialize into an existing entity | `PreSerialize` clears components first (no stale survivors, INV-4) |
| Editor plugin claims an unknown `DefinitionType` | `Handles` returns false; first-responder dispatch moves on |

## Testing Strategy

- **Interface stability** (INV-1): `pkg/editor` definition surface passes the multi-repo `TestEditorPkgIsContractOnly` (interface/type only, no `internal/` import).
- **Order round-trip** (INV-3): serialize → reorder struct fields → deserialize yields identical values; a v2 file with an extra `Order:100` field loads into a v1 type using defaults for the missing field.
- **Content vs Default** (INV-3): a `SerContent` field inlines its sub-graph; a `SerDefault` field writes a reference (no deep expansion).
- **Entity hooks** (INV-4): deserialize suppresses default `Transform` insertion; re-deserialize clears stale components; `PostDeserialize` fires once.
- **No-network** (INV-2): the package exposes no serialize-to-wire path (compile-time: no such exported method).

## 7. Drawbacks & Alternatives

- **Drawback**: explicit `Order` is more authoring overhead than struct order.
  **Alternative**: rely on declaration order.
  **Decision**: L1 INV-3 — declaration order is never load-bearing; explicit order is what guarantees cross-version file compatibility. Kept.
- **Drawback**: a separate `pkg/editor` surface duplicates some property types with the inspector plugin.
  **Alternative**: one merged editor package.
  **Decision**: they already share `property.go`; the definition surface is a thin addition to the existing `pkg/editor` boundary (multi-repo). Kept.

## Canonical References

<!-- MANDATORY for Stable status. Stub — populate when implementation lands
     (Phase 6 Track A follow-on). Blocked until: (1) l1-definition-integration
     Stable; (2) pkg/editor definition surface + order-based serialization + entity
     hooks implemented with tests (round-trip + cross-version + multi-repo guard). -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-30 | Initial L2 draft — Go translation of l1-definition-integration v0.1.0. Stable `pkg/editor` `DefinitionEditorInterface` + `DefinitionEditorPlugin` + `DefinitionNode` (interface/type only, multi-repo INV-3); `internal/definition` order-based `SerializationField` serialize/deserialize (explicit `Order`, missing→default, Content/Default depth control, INV-3) + `EntitySerializer` Pre/Post hooks suppressing default components + clearing stale on reload (INV-4); no runtime network path (INV-2). Authored ahead of Phase 6 (`/magic.spec`). Draft — L1 parent Draft + no implementation yet. |
