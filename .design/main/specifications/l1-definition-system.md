# Definition System

**Version:** 0.5.0
**Status:** Stable
**Layer:** concept

## Overview

The Definition System is a JSON-based declarative configuration layer that serves as a bridge between engine code and visual tooling. It allows UI layouts, game scenarios, entity templates, and behavioral flows to be described in structured JSON files rather than compiled Go code. The engine interprets these definitions at runtime, constructing entities, components, and state transitions from data.

This is the foundation for data-driven game development: **code defines capabilities, data defines content**. A game designer can create menus, configure loading screens, wire up game flow, and compose entity templates without writing or compiling Go. A GUI editor reads and writes the same JSON files, making "visual programming" possible.

## Related Specifications

- [type-registry.md](l1-type-registry.md) — Maps JSON type names to Go struct types at runtime
- [ui-system.md](l1-ui-system.md) — UI tree and style definitions expressed in JSON
- [scene-system.md](l1-scene-system.md) — Entity/component snapshots in definition format
- [state-system.md](l1-state-system.md) — State machine flows described declaratively
- [app-framework.md](l1-app-framework.md) — App configuration and plugin setup from definitions
- [asset-system.md](l1-asset-system.md) — Definition files loaded as assets with hot-reload
- [component-system.md](l1-component-system.md) — Component values deserialized from JSON via registry
- [hierarchy-system.md](l1-hierarchy-system.md) — Parent-child trees expressed in nested JSON structure
- [ai-assistant-system.md](l1-ai-assistant-system.md) — AI agents read and write definition files
- [visual-graph-system.md](l1-visual-graph-system.md) — Visual graphs are a definition type ("graph") loaded by this system
- [definition-integration.md](l1-definition-integration.md) — Editor integration, network boundary & serialization contract (extracted §4.9–§4.13)

## 1. Motivation

Every game engine eventually faces the same problem: the gap between code and content. Programmers write systems in Go; designers want to tweak menus, adjust game flow, and compose scenes without a compiler. Without a declarative data layer:

- Every UI change requires a code edit, recompile, and restart.
- Game flow (loading screen → main menu → level select → gameplay) is hardcoded in state machine setup functions.
- Entity templates (prefabs) exist only as Go structs, invisible to visual editors.
- Scene files store raw component data but lack higher-level semantics (styling, flow, conditions).
- A GUI editor has no standard format to read or produce.

The Definition System solves this by introducing a single, unified JSON format that the engine interprets at runtime. The same format serves hand-authored files, GUI editor output, and programmatic generation.

## 2. Constraints & Assumptions

- JSON is the sole serialization format for definitions (human-readable, editor-friendly, stdlib `encoding/json`).
- All types referenced in definitions must be registered in the TypeRegistry.
- Definitions are loaded as assets — they participate in hot-reload, dependency tracking, and asset lifecycle.
- Definitions are declarative and side-effect-free. They describe **what** to create, not **how** to create it.
- The definition runtime interprets files each time they are instantiated (no ahead-of-time compilation).
- Definitions can reference other definitions by asset path (composition, not inheritance).
- A definition file has exactly one root type: `ui`, `scene`, `flow`, or `template`.

## 3. Core Invariants

- **INV-1**: A definition file that passes schema validation will always instantiate without runtime errors (fail-fast at validation, not at instantiation).
- **INV-2**: Definitions never bypass the ECS — they produce entities, components, and resources through the standard command pipeline.
- **INV-3**: Hot-reloading a definition file re-instantiates the affected content without restarting the app.
- **INV-4**: Every value in a definition file maps to a registered type or a built-in primitive (no opaque blobs).
- **INV-5**: Definitions are composable — a definition can include other definitions by reference, forming a DAG (cycles are detected and rejected at load time).

## 4. Detailed Design

### 4.1 Definition File Structure

Every definition file has a common envelope:

```plaintext
{
    "definition": "ui" | "scene" | "flow" | "template",
    "version": "1.0",
    "metadata": {
        "name": "main_menu",
        "description": "Main menu layout",
        "tags": ["menu", "ui"]
    },
    "content": { ... }
}
```

- `definition` — The root type, determining which interpreter processes the content.
- `version` — Schema version for forward compatibility.
- `metadata` — Human-readable info, used by editor and asset browser.
- `content` — The actual definition payload, structure varies by type.

### 4.2 UI Definitions

UI definitions describe a tree of UI nodes with layout and style properties. Analogous to HTML+CSS but in a single JSON structure.

```plaintext
{
    "definition": "ui",
    "content": {
        "root": {
            "type": "Node",
            "style": {
                "display": "flex",
                "flex_direction": "column",
                "justify_content": "center",
                "align_items": "center",
                "width": "100%",
                "height": "100%",
                "background_color": "#1a1a2e"
            },
            "children": [
                {
                    "type": "Text",
                    "value": "My Game",
                    "style": {
                        "font_size": 48,
                        "color": "#e94560",
                        "margin_bottom": 40
                    }
                },
                {
                    "type": "Button",
                    "id": "btn_play",
                    "style": {
                        "width": 200,
                        "height": 50,
                        "background_color": "#0f3460",
                        "border_radius": 8
                    },
                    "children": [
                        {
                            "type": "Text",
                            "value": "Play",
                            "style": { "font_size": 24, "color": "#ffffff" }
                        }
                    ],
                    "on_click": { "action": "transition", "target": "gameplay" }
                },
                {
                    "type": "Button",
                    "id": "btn_quit",
                    "style": {
                        "width": 200,
                        "height": 50,
                        "background_color": "#533483",
                        "border_radius": 8,
                        "margin_top": 10
                    },
                    "children": [
                        {
                            "type": "Text",
                            "value": "Quit",
                            "style": { "font_size": 24, "color": "#ffffff" }
                        }
                    ],
                    "on_click": { "action": "quit" }
                }
            ]
        }
    }
}
```

**Key properties:**

- `type` — Node type from the UI system (`Node`, `Text`, `Button`, `ImageNode`, `ScrollView`).
- `style` — Flat key-value map of style properties. Mirrors the `Style` component fields (flexbox layout + visual).
- `children` — Nested array defining the UI hierarchy (maps to ECS ChildOf relationships).
- `id` — Optional identifier for referencing this node from code or other definitions.
- `on_click`, `on_hover` — Event binding declarations (see 4.6 Action Bindings).

**Style properties** map directly to the UI system's Style component: `display`, `flex_direction`, `justify_content`, `align_items`, `width`, `height`, `margin`, `padding`, `border`, `background_color`, `border_color`, `border_radius`, `font_size`, `color`, `gap`, `overflow`, etc.

### 4.3 Scene Definitions

Scene definitions describe entity hierarchies with component data. An extension of the existing DynamicScene format with richer semantics.

```plaintext
{
    "definition": "scene",
    "content": {
        "entities": [
            {
                "name": "player",
                "components": {
                    "Transform": { "translation": [0, 1, 0], "rotation": [0, 0, 0, 1], "scale": [1, 1, 1] },
                    "MeshRenderer": { "mesh": "assets/models/character.glb", "material": "assets/materials/player.json" },
                    "Health": { "max": 100, "current": 100 },
                    "Player": {}
                },
                "children": [
                    {
                        "name": "weapon_slot",
                        "components": {
                            "Transform": { "translation": [0.5, 0.8, 0.2] }
                        }
                    }
                ]
            },
            {
                "name": "camera",
                "components": {
                    "Transform": { "translation": [0, 5, -10] },
                    "Camera": { "projection": "perspective", "fov": 60 }
                }
            }
        ]
    }
}
```

**Integration with Scene System:**

- Component names resolve through the TypeRegistry to ComponentIDs.
- Component values are deserialized via reflection using registered field metadata.
- `children` nesting maps to ChildOf relationships.
- Asset references (e.g., `"assets/models/character.glb"`) resolve to Handle types through the Asset System.
- Entity names become `Name` components for debugging and cross-referencing.

### 4.4 Flow Definitions

Flow definitions describe game state graphs — the high-level scenario of the application. This replaces hardcoded state machine setup with a declarative graph.

```plaintext
{
    "definition": "flow",
    "content": {
        "initial_state": "loading",
        "states": {
            "loading": {
                "ui": "assets/ui/loading_screen.json",
                "on_enter": [
                    { "action": "load_assets", "manifest": "assets/manifests/core.json" }
                ],
                "transitions": [
                    { "event": "assets_loaded", "target": "main_menu" }
                ]
            },
            "main_menu": {
                "ui": "assets/ui/main_menu.json",
                "systems": ["menu_animation_system"],
                "transitions": [
                    { "event": "button:btn_play", "target": "gameplay" },
                    { "event": "button:btn_quit", "target": "quit" }
                ]
            },
            "gameplay": {
                "scene": "assets/scenes/level_01.json",
                "ui": "assets/ui/hud.json",
                "systems": ["player_movement", "enemy_ai", "combat"],
                "transitions": [
                    { "event": "player_died", "target": "game_over" },
                    { "event": "key:escape", "target": "pause_menu" }
                ]
            },
            "pause_menu": {
                "ui": "assets/ui/pause_menu.json",
                "overlay": true,
                "transitions": [
                    { "event": "button:btn_resume", "target": "gameplay" },
                    { "event": "button:btn_main_menu", "target": "main_menu" },
                    { "event": "key:escape", "target": "gameplay" }
                ]
            },
            "game_over": {
                "ui": "assets/ui/game_over.json",
                "on_enter": [
                    { "action": "despawn_scene" }
                ],
                "transitions": [
                    { "event": "button:btn_retry", "target": "gameplay" },
                    { "event": "button:btn_main_menu", "target": "main_menu" }
                ]
            },
            "quit": {
                "on_enter": [
                    { "action": "exit_app" }
                ]
            }
        }
    }
}
```

**Each state declares:**

- `ui` — Asset path to a UI definition file to show while in this state.
- `scene` — Asset path to a scene definition to spawn on enter.
- `systems` — Named systems to activate only during this state (maps to RunConditions).
- `overlay` — If true, the previous state's content remains visible underneath.
- `on_enter` / `on_exit` — Action lists executed on state transition.
- `transitions` — Event→target mappings defining edges in the state graph.

**Integration with State System:**

- Each state name maps to a value in an engine-managed `FlowState` enum.
- Transitions generate the same `NextState` writes as code-driven state machines.
- `on_enter`/`on_exit` actions are executed as Commands during the StateTransition schedule.
- `DespawnOnExit` behavior is automatic — entities spawned by a state's scene/ui are tagged and cleaned up on exit.

### 4.5 Template Definitions

Template definitions describe reusable entity blueprints (prefabs). They are not instantiated directly — other definitions or code reference them.

```plaintext
{
    "definition": "template",
    "content": {
        "name": "enemy_goblin",
        "components": {
            "Health": { "max": 30, "current": 30 },
            "Speed": { "value": 3.5 },
            "Enemy": { "aggro_range": 10.0 },
            "AI": { "behavior": "patrol" }
        },
        "overridable": ["Health.max", "Speed.value", "AI.behavior"]
    }
}
```

- Templates are loaded as assets and referenced by path.
- Scene definitions can reference templates: `{ "template": "assets/templates/enemy_goblin.json", "overrides": { "Health.max": 50 } }`
- `overridable` declares which fields a consumer may customize (editor hint, not runtime enforcement).

### 4.6 Action Bindings

Actions are the verbs of the definition system — they connect declarative data to engine behavior:

| Action | Description | Parameters |
| :--- | :--- | :--- |
| `transition` | Trigger a flow state transition | `target`: state name |
| `quit` | Request app exit | — |
| `spawn` | Spawn an entity from a template | `template`: asset path, `overrides`: map |
| `despawn` | Despawn entities by tag or name | `target`: name or tag |
| `despawn_scene` | Despawn all entities from the current scene | — |
| `load_assets` | Begin loading an asset manifest | `manifest`: asset path |
| `play_audio` | Play a sound effect or music | `source`: asset path, `spatial`: bool |
| `set_resource` | Set a resource value | `type`: type name, `value`: map |
| `send_event` | Send a typed event | `type`: event name, `data`: map |
| `log` | Log a message (debug) | `message`: string, `level`: string |

Actions are extensible — plugins can register custom action types with the Definition System.

### 4.7 Binding Pipeline

The path from JSON file to live ECS entities:

```mermaid
graph LR
    A["JSON File"] --> B["Asset Loader"]
    B --> C["Schema Validator"]
    C --> D["Definition Asset"]
    D --> E["Interpreter"]
    E --> F["Commands"]
    F --> G["World"]

    H["TypeRegistry"] --> E
    I["Asset Server"] --> E
```

1. **Load**: Asset System loads the JSON file via a `DefinitionLoader`.
2. **Validate**: Schema validator checks structure, type references, and DAG integrity (no circular includes).
3. **Store**: Parsed definition is stored as a typed asset (`Handle[UIDefinition]`, `Handle[FlowDefinition]`, etc.).
4. **Interpret**: When instantiated, the interpreter walks the definition tree and emits Commands.
5. **Apply**: Commands create entities, insert components, set up hierarchy, register state transitions.

The TypeRegistry resolves all string type names to Go types. The Asset Server resolves all asset path references to Handles.

### 4.8 Hot-Reload Workflow

Because definitions are assets, they participate in the standard hot-reload pipeline:

1. File watcher detects change to a `.json` definition file.
2. Asset System reloads the file and fires `AssetEvent[T]::Modified`.
3. The definition interpreter detects the modification event.
4. For UI definitions: the existing UI entity tree is despawned and re-instantiated from the updated definition.
5. For flow definitions: the flow graph is updated in-place; current state is preserved if it still exists.
6. For scene definitions: the scene is re-instantiated (or flagged for manual re-spawn).

This enables the core workflow: **edit JSON in editor → save → see changes in running game**.

### 4.9 Editor, Network & Serialization Integration

The editor-integration surface, network-transmission boundary, and the
deterministic serialization contract (order-based field serialization, custom
entity serialization hooks) are specified separately to keep this spec focused
on the definition content model:

> **→ [l1-definition-integration.md](l1-definition-integration.md)** —
> §4.1 Editor Integration Points · §4.2 Editor Plugin Integration
> (`DefinitionEditorInterface`, `handles()` dispatch, property revert/default,
> dynamic property list) · §4.3 Network Boundary (definitions are
> process-local) · §4.4 Order-Based Field Serialization · §4.5 Custom Entity
> Serialization Hooks.

Conceptual guarantees retained here: definitions are a data contract a future
GUI editor consumes, they never cross a network boundary at runtime, and their
serialization order is version-stable.

## 5. Open Questions

- Should definitions support expressions/formulas (e.g., `"width": "parent.width * 0.5"`) or stay purely static?
- Should there be a binary definition format for shipped games (faster parsing, smaller files)?
- How deep should template inheritance go (single level or multi-level)?
- Should flow definitions support parallel states (two active simultaneously)?
- Localization: should text values support i18n keys (e.g., `"value": "@menu.play_button"`) natively?

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
| 0.1.0 | 2026-03-26 | Initial draft — captures data-driven design vision for JSON declarative layer |
| 0.2.0 | 2026-03-26 | Added network boundary section — definitions are local-only, no cross-process transmission |
| 0.3.0 | 2026-03-26 | Added editor plugin integration: stable API, handles() dispatch, property revert/default, dynamic property list |
| 0.4.0 | 2026-03-26 | Added order-based field serialization, custom entity serialization hooks |
| 0.5.0 | 2026-05-15 | Extracted §4.9–§4.13 (editor/network/serialization) into `l1-definition-integration.md` to clear SPEC_DECOMPOSE (507→~400 lines); §4.9 now a pointer subsection |
| — | — | Planned examples: `examples/config/` |
