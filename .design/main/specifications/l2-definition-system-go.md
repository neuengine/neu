# Definition System — Go Implementation

**Version:** 0.1.0
**Status:** Draft
**Layer:** go
**Implements:** [l1-definition-system.md](l1-definition-system.md)

## Overview

Go-level design for the JSON declarative layer. A definition file decodes (via stdlib
`encoding/json`) into a typed envelope whose `definition` discriminator selects one of
four interpreters: `ui`, `scene`, `flow`, `template`. Each file is loaded as an asset
through a `DefinitionLoader`, validated up front (structure + every type name resolvable
in the `TypeRegistry` + include-DAG acyclic), and only then stored — so a file that
passes validation always instantiates (INV-1). Interpreters never touch the world
directly; they emit `Command`s (INV-2). Because definitions are assets, editing a file
hot-reloads the affected content (INV-3). Definitions compose by asset-path reference,
forming a DAG with load-time cycle rejection (INV-5).

## Related Specifications

- [l1-definition-system.md](l1-definition-system.md) — L1 concept specification (parent)
- [l2-type-registry-go.md](l2-type-registry-go.md) — resolves JSON type names → Go types + field metadata (INV-4)
- [l2-scene-system-go.md](l2-scene-system-go.md) — `scene` definitions reuse the `DynamicScene` reflection codec + two-pass remap
- [l2-state-system-go.md](l2-state-system-go.md) — `flow` transitions emit `NextState` writes; `DespawnOnExit` cleanup
- [l2-ui-system-go.md](l2-ui-system-go.md) — `ui` definitions build `Node`/`Style`/widget trees
- [l2-asset-system-go.md](l2-asset-system-go.md) — `DefinitionLoader` registration, hot-reload `AssetEvent[T]`, path→Handle resolution
- [l2-command-system-go.md](l2-command-system-go.md) — interpreters emit deferred `Command`s (INV-2)
- [l2-component-system-go.md](l2-component-system-go.md) — component values deserialized via registry field metadata

## 1. Motivation

The gap between code and content is closed by one JSON format the engine interprets at
runtime. The Go binding's role is to (1) decode the envelope and dispatch on its
discriminator, (2) front-load all failure into a validation pass so instantiation is
infallible (INV-1), and (3) route every effect through the existing command, asset,
type-registry, scene, state, and UI layers rather than a parallel construction path. The
same format serves hand-authored files, editor output, and AI-generated content.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: `encoding/json` for decode; reflection deserialization reuses the Phase 3 scene codec; generics for `Handle[T]` typed definition assets.
- **C-003**: JSON is stdlib; no external schema-validation library — validation is hand-rolled against `TypeRegistry` metadata.
- **INV-1**: validation is total — structure, type-reference resolution, and DAG integrity are all checked before the definition asset is stored; a stored definition cannot fail instantiation.
- **INV-2**: interpreters emit `Command`s only; they never call `*World` mutators directly.
- **INV-4**: every JSON value maps to a registered type or a built-in primitive — an unregistered type name fails validation, never produces an opaque blob.
- **INV-5**: includes form a DAG; a cycle is detected during validation (DFS colouring) and rejected.
- A file has exactly one root `definition` kind; mixed roots are a validation error.

## 3. Core Invariants

> [!NOTE]
> See [l1-definition-system.md §3](l1-definition-system.md) for the technology-agnostic
> invariants (INV-1…INV-5). Go-specific compliance is tabulated in §4.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: schema-valid ⇒ always instantiates (fail-fast at validation) | `DefinitionLoader.Load` runs `validate()` (structure + type refs + DAG) and returns an error there; only a fully validated definition is stored as an asset. The interpreter's `instantiate()` therefore has no validation branches — every reference is pre-resolved, so it cannot fail at runtime. |
| **INV-2**: definitions never bypass the ECS | Interpreters take a `*ecs.Commands` and emit `Spawn`/`Insert`/`SetResource`/`NextState` commands; there is no `*World` argument. Entities, components, and hierarchy are created through the standard deferred pipeline. |
| **INV-3**: hot-reload re-instantiates without restart | `DefinitionLoader` registers for `AssetEvent[Definition]::Modified`; a `hotReloadSystem` despawns the entities tagged with the definition's `DefinitionInstance` ID and re-runs the interpreter from the updated asset. Flow definitions update the graph in place, preserving the current state if it still exists. |
| **INV-4**: every value maps to a registered type / primitive | Type names (`"Transform"`, `"Health"`) resolve through `typereg.Lookup`; component values deserialize via the registry's field metadata. An unknown type name ⇒ `ErrUnknownType` at validation. Primitives (numbers, strings, bools, arrays) decode directly. |
| **INV-5**: composable DAG; cycles rejected at load | The validator builds an include graph from `template`/`ui`/`scene` path references and runs a three-colour DFS; a back-edge ⇒ `ErrDefinitionCycle{Path}` before the asset is stored. |

## Go Package

```
pkg/definition/
  envelope.go      // Envelope{Definition, Version, Metadata, Content}, Kind enum
  ui.go            // UIDefinition content model (node tree + style maps)
  scene.go         // SceneDefinition content model (entities + components + children)
  flow.go          // FlowDefinition content model (states, transitions, actions)
  template.go      // TemplateDefinition (components + overridable fields)
  action.go        // Action, ActionRegistry (transition/quit/spawn/play_audio/...)
  errors.go        // ErrUnknownType, ErrDefinitionCycle, ErrSchemaInvalid, ErrUnknownAction
  plugin.go        // DefinitionPlugin: registers loader, interpreters, hot-reload system
internal/definition/
  loader.go        // DefinitionLoader (asset.AssetLoader): decode → validate → typed asset
  validate.go      // structure + type-ref (TypeRegistry) + include-DAG (3-colour DFS) checks
  interp_ui.go     // UIDefinition → Node/Style/widget Commands (reuses l2-ui-system-go)
  interp_scene.go  // SceneDefinition → entities via the DynamicScene codec + ChildOf
  interp_flow.go   // FlowDefinition → FlowState enum + NextState transitions + on_enter/exit
  interp_template.go // template instantiation + field overrides
  hotreload.go     // AssetEvent[Definition]::Modified → despawn tagged + re-instantiate (INV-3)
```

## Type Definitions

```go
type Kind uint8
const (KindUI Kind = iota; KindScene; KindFlow; KindTemplate)

type Metadata struct { Name, Description string; Tags []string }

// Envelope is the common file header; Content is decoded per Kind.
type Envelope struct {
    Definition Kind            // "ui" | "scene" | "flow" | "template"
    Version    string          // schema version
    Metadata   Metadata
    Content    json.RawMessage // interpreted by the Kind-specific decoder
}

// Definition is the stored, validated asset (one of the four content types).
type Definition struct {
    Kind     Kind
    Metadata Metadata
    Includes []asset.Path // resolved references (INV-5 DAG nodes)
    content  any          // *UIDefinition | *SceneDefinition | *FlowDefinition | *TemplateDefinition
}

// DefinitionInstance tags every entity spawned by a definition (for hot-reload cleanup).
type DefinitionInstance struct { Source asset.Path }

type Action struct { Type string; Params map[string]any }
type ActionRegistry struct{ handlers map[string]ActionHandler }
type ActionHandler func(c *ecs.Commands, p map[string]any) error

// Flow state model.
type FlowState struct {
    UI          asset.Path
    Scene       asset.Path
    Systems     []string
    Overlay     bool
    OnEnter     []Action
    OnExit      []Action
    Transitions []FlowTransition
}
type FlowTransition struct { Event string; Target string }
```

## Key Methods

```go
func (p DefinitionPlugin) Build(app *app.App) // registers loader + interpreters + hotReloadSystem

// Loader (INV-1): decode → validate → typed asset.
func (l *DefinitionLoader) Load(ctx asset.LoadContext, r io.Reader, _ LoadSettings) (Definition, error)

// Validation (INV-4, INV-5).
func validate(d *Definition, reg *typereg.TypeRegistry, srv *asset.AssetServer) error

// Instantiation (INV-2): emit Commands. Infallible post-validation.
func Instantiate(c *ecs.Commands, d *Definition) DefinitionInstance

// Actions (extensible).
func (r *ActionRegistry) Register(actionType string, h ActionHandler) error

// Hot-reload (INV-3).
func hotReloadSystem(/* EventReader[AssetEvent[Definition]], Commands, Query[DefinitionInstance] */)
```

## Performance Strategy

- **Validate once, instantiate many**: parsing + validation happen at asset load; instantiation walks a pre-resolved tree emitting commands — no re-parsing, no reflection-driven type lookups in the spawn path beyond cached registry `ComponentID`s.
- **Reuse the scene codec**: `scene` definitions delegate to the Phase 3 `DynamicScene` interned binary/reflection codec rather than a second deserializer.
- **DAG cache**: the include graph is built during validation and stored on the `Definition`; hot-reload re-validates only the changed file and its dependents, not the whole graph.
- **Command batching**: a definition's spawns flow through one command buffer flush, so a large UI/scene tree applies at a single sync point.

## Error Handling

| Condition | Behavior |
| :--- | :--- |
| Malformed JSON | `ErrSchemaInvalid` (wraps the `json` error); asset slot ⇒ `Failed`, nothing stored (INV-1) |
| Unknown type name in a component/node | `ErrUnknownType{Name}` at validation (INV-4) |
| Circular include | `ErrDefinitionCycle{Path}` from the DFS (INV-5) |
| Unknown action type | `ErrUnknownAction{Type}` at validation (actions resolved against `ActionRegistry`) |
| Mixed/absent root `definition` kind | `ErrSchemaInvalid` ("expected exactly one root kind") |
| Hot-reload of a now-invalid file | keep the previous valid instance live; `slog.Warn` with the validation error (no crash) |

```go
type ErrUnknownType     struct{ Name string }
type ErrDefinitionCycle struct{ Path asset.Path }
type ErrSchemaInvalid   struct{ Reason string; Err error }
type ErrUnknownAction   struct{ Type string }
```

## Testing Strategy

- **INV-1 round-trip**: a corpus of valid `ui`/`scene`/`flow`/`template` files validates then instantiates without error; instantiation of a validated definition never returns an error (property test).
- **INV-2**: instantiation emits only `Command`s — a test interpreter with a recording `Commands` asserts no direct world mutation occurs.
- **INV-4**: a definition referencing an unregistered type fails validation with `ErrUnknownType`; all built-in primitives decode.
- **INV-5**: a → b → a include chain ⇒ `ErrDefinitionCycle`; a valid diamond (a→b, a→c, b→d, c→d) validates.
- **INV-3 hot-reload**: modifying a `ui` file despawns the old `DefinitionInstance` entities and re-instantiates; a `flow` edit preserves the current state if it still exists.
- **Scene parity**: a `scene` definition round-trips through the `DynamicScene` codec byte-stably (reuses the Phase 3 test pattern).
- **No-panic fuzz**: `go test -fuzz` on truncated/garbage definition bytes ⇒ error, never panic.

## 7. Drawbacks & Alternatives

- **Drawback**: runtime interpretation re-walks the tree on every instantiation.
  **Alternative**: compile definitions to Go at build time.
  **Decision**: L1 §2 mandates runtime interpretation (editor/hot-reload workflow); validation-once amortizes the cost and a binary format is a deferred open question. Kept.
- **Drawback**: hand-rolled validation duplicates some `TypeRegistry` knowledge.
  **Alternative**: a third-party JSON-schema validator.
  **Decision**: C-003 forbids an external validator in core; validating against the live `TypeRegistry` is both dep-free and more precise (it checks real registered types, not a static schema). Kept.
- **Drawback**: expressions/formulas (`"width": "parent.width * 0.5"`) are unsupported.
  **Alternative**: an embedded expression language.
  **Decision**: L1 §5 open question — definitions stay static for now; an expression layer is a future, opt-in extension. Kept.

## Canonical References

<!-- MANDATORY for Stable status. Stub — populate when implementation lands
     (Phase 6, Track A). Stable promotion blocked until: (1) l1-definition-system is
     Stable (layer constraint); (2) loader + validator + interpreters implemented with a
     validating example (examples/config/). -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-30 | Initial L2 draft — Go translation of l1-definition-system v0.5.0. `Envelope` discriminated on a `Kind` enum (ui/scene/flow/template), `DefinitionLoader` with a total validation pass (structure + `TypeRegistry` type refs + include-DAG 3-colour DFS) so instantiation is infallible (INV-1), `Command`-only interpreters (INV-2), `DefinitionInstance`-tagged hot-reload via `AssetEvent` (INV-3), registry-resolved values (INV-4), DAG cycle rejection (INV-5), extensible `ActionRegistry`. Reuses the Phase 3 `DynamicScene` codec, state-system `NextState`, and the UI node model. Authored ahead of Phase 6 Track A (`/magic.spec`). Draft — L1 parent Draft + no implementation yet. |
