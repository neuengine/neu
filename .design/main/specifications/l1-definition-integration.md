# Definition System — Editor, Network & Serialization Contract

**Version:** 0.1.0
**Status:** Draft
**Layer:** concept

## Overview

This specification covers the **integration boundary** of the
[Definition System](l1-definition-system.md): how an external GUI editor reads
and writes definitions, the network-transmission boundary, and the
deterministic serialization contract (field ordering and custom entity hooks).

It was extracted from [l1-definition-system.md](l1-definition-system.md)
§4.9–§4.13 to keep the parent spec focused on the definition *content model*
(file structure, UI/Scene/Flow/Template definitions, binding pipeline,
hot-reload) while this spec owns the *integration surface*.

## Related Specifications

- [l1-definition-system.md](l1-definition-system.md) — parent: definition content model & file format
- [l1-app-framework.md](l1-app-framework.md) — monolithic process / network boundary rationale
- [l1-type-registry.md](l1-type-registry.md) — source of schema, defaults, and serialization metadata
- [l1-scene-system.md](l1-scene-system.md) — DynamicScene + entity serialization consumer

## 1. Motivation

The definition format is the data contract a future GUI editor consumes. The
editor lives outside the engine and must interact through a stable surface that
does not couple it to interpreter internals. Separately, the serialization
contract must guarantee cross-version file compatibility and controlled
serialization depth. These integration concerns are orthogonal to the content
model and evolve on a different cadence.

## 2. Constraints & Assumptions

- Definitions are **process-local**: never transmitted over the network as part
  of the game loop.
- The editor is a separate, future-specified application; this spec defines only
  the engine-side data contract it depends on.
- Serialization ordering is explicit and version-stable; declaration order is
  never load-bearing.

## 3. Core Invariants

1. The editor's only coupling to the engine is the stable
   `DefinitionEditorInterface` — interpreter internals are never exposed.
2. Definition files are never serialized or deserialized across a network
   boundary at runtime.
3. Field serialization order is explicit and stable across engine versions;
   missing ordered fields deserialize to their default value.
4. Custom entity serialization hooks suppress default-component auto-insertion
   during deserialization and clear stale components on re-deserialize.

## 4. Detailed Design

### 4.1 Editor Integration Points

The Definition System is designed as the data format a GUI editor reads and writes:

- **Editor → Engine**: Editor saves JSON files. Engine hot-reloads them.
- **Engine → Editor**: Engine can export current World state as definition files (via DynamicScene + UI tree serialization).
- **Schema as contract**: JSON Schema files generated from TypeRegistry define what the editor can offer as autocomplete and validation.
- **Live preview**: Editor connects to a running engine instance and pushes definition updates for immediate preview.

The editor itself is a future specification. The Definition System provides the data contract the editor will consume.

### 4.2 Editor Plugin Integration

The definition system provides a stable interface for editor plugins to interact with definitions without coupling to internal interpreter complexity.

**Stable editor API**: A `DefinitionEditorInterface` exposes only the operations editor plugins need:

```plaintext
DefinitionEditorInterface
  GetDefinitionType(asset: Handle) -> DefinitionType
  GetPropertyList(node: DefinitionNode) -> []PropertyInfo
  GetPropertyValue(node: DefinitionNode, name: string) -> any
  SetPropertyValue(node: DefinitionNode, name: string, value: any)
  ValidateDefinition(asset: Handle) -> []ValidationError
```

Internal interpreter complexity is hidden behind this interface. Even as the interpreter evolves, editor plugins only depend on the stable surface.

**handles() dispatch**: Editor plugins declare which definition types they can edit:

```plaintext
DefinitionEditorPlugin interface:
  Handles(defType DefinitionType) -> bool
  Edit(definition: DefinitionNode)
  GetInspectorProperties(node: DefinitionNode) -> []EditorProperty
```

The editor iterates registered plugins and asks each `Handles()` — first responder wins. This allows custom editors per definition type (UI editor, flow graph editor, scene editor) without central knowledge of all types.

**Property revert/default system**: Any property in a definition can report its default value for the editor's "reset to default" button:

```plaintext
DefinitionNode
  CanRevert(property: string) -> bool
  GetRevertValue(property: string) -> any
```

When `CanRevert` returns true, the editor displays a reset indicator next to the property. Clicking it restores the value from `GetRevertValue`. Defaults are derived from the TypeRegistry's default initialization hooks.

**Dynamic property list notification**: A definition node can signal that its available properties have changed:

```plaintext
DefinitionNode
  NotifyPropertyListChanged()
```

This fires when a property value change causes other properties to appear or disappear. For example, changing a node's `type` from `"Button"` to `"ScrollView"` reveals scroll-related properties and hides button-specific ones. The editor refreshes its inspector panel in response.

### 4.3 Network Boundary

Definitions operate strictly within a single engine process. They are **never transmitted over the network** as part of the game loop. A definition file is loaded from local storage (or an asset CDN during development), interpreted into ECS commands, and the resulting entities live in the local World.

Backend services (matchmaking, persistence, analytics) communicate with the game via typed network messages — not via definition files. If a server needs to describe a scene to a client, it sends a compact binary protocol; the client may then load a locally cached definition file referenced by that protocol. The definition system does not serialize or deserialize across network boundaries at runtime.

This constraint preserves the engine's monolithic performance model (see [app-framework.md, Section 4.10](l1-app-framework.md)).

### 4.4 Order-Based Field Serialization

Component and definition fields are serialized in a deterministic, explicit order rather than relying on struct field declaration order:

```plaintext
SerializationField
  name:       string
  order:      int              // explicit serialization order (e.g., -10, 0, 100)
  mode:       SerializationMode
  type_hint:  string           // optional type discriminator for polymorphic fields

SerializationMode:
  Default    // serialize normally (value types by value, reference types by reference)
  Content    // serialize the contents, not the reference (inline expansion)
  Ignore     // skip this field entirely
  Always     // serialize even if value equals default
```

**Why explicit order matters**: When fields are added, removed, or reordered across engine versions, order-based serialization ensures backward compatibility. A field with `order: 100` added in v2 doesn't break v1 files that lack it — the deserializer simply applies the default value for missing ordered fields.

**Mode: Content vs Default**: `Content` mode inlines the full object graph (e.g., a component's sub-struct is written in-place). `Default` mode writes a reference or handle. This controls serialization depth — preventing accidental serialization of the entire world when a component holds a reference to a shared resource.

### 4.5 Custom Entity Serialization Hooks

Entities support custom serialization behavior through pre/post hooks:

```plaintext
EntitySerializer
  PreSerialize(entity: *Entity, mode: SerializeMode)
    // Called before field serialization begins
    // Mode: Serialize → prepare entity for writing
    // Mode: Deserialize → construct entity without default components
    //   (e.g., skip auto-creating Transform during deserialization)

  PostDeserialize(entity: *Entity)
    // Called after all fields are deserialized
    // Resolve cross-references, validate component set, fire hooks
```

**Construction control**: During deserialization, `PreSerialize` creates the entity with a special flag that suppresses default component auto-insertion (e.g., Transform). This prevents the deserializer from creating a Transform that will immediately be overwritten by the serialized data. After all components are deserialized, `PostDeserialize` validates the entity's component set and fires any deferred lifecycle hooks.

**Component clearing on re-deserialize**: When deserializing into an existing entity (e.g., hot-reload), `PreSerialize` clears the entity's component collection before repopulating. This ensures no stale components survive a reload.

## 5. Open Questions

<!-- TBD: Should the editor live-preview channel reuse the visual-graph IPC
     protocol (l2-visual-graph-editor-bridge) or define its own definition-push
     message set? -->

## Canonical References

<!-- MANDATORY for Stable status. List authoritative source files that downstream agents
     MUST read before implementing this spec. Use relative paths from project root.
     Stub state — fill with concrete files when implementation begins (Phase 6). -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

<!-- Empty table = no canonical sources yet. Populate one row per authoritative file
     when implementation lands (Phase 6). Stable promotion requires ≥1 row. -->

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-15 | Initial draft: extracted from `l1-definition-system.md` §4.9–§4.13 (editor integration, network boundary, serialization contract) per `/magic-analyze` SPEC_DECOMPOSE |
