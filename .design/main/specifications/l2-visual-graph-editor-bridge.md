# Visual Graph Editor Bridge — Go Implementation

**Version:** 0.1.0
**Status:** Draft
**Layer:** go
**Implements:** [l1-visual-graph-system.md](l1-visual-graph-system.md)

## Overview

Go-level contract for the engine-side "door" that the external `bolteditor`
repository implements to drive a Blueprint-style graph UI. This spec covers the
`pkg/editor/graph.go` interfaces (`GraphEditorPlugin`, `NodeRegistryQuery`,
`GraphDebugger`) and the `pkg/protocol/graph.go` IPC message extensions used for
editor↔engine debugging communication.

Extracted from [l1-visual-graph-system.md](l1-visual-graph-system.md) §4.7–§4.8
to keep the L1 concept spec technology-neutral (L1 Purity) and to give the
editor-integration surface its own versionable contract.

## Related Specifications

- [l1-visual-graph-system.md](l1-visual-graph-system.md) — L1 concept specification (parent)
- [l1-multi-repo-architecture.md](l1-multi-repo-architecture.md) — `pkg/editor/` + `pkg/protocol/` public boundary
- [l1-definition-system.md](l1-definition-system.md) — graph definition type loaded through the editor

## 1. Motivation

The visual graph editor lives in a separate repository (`bolteditor`) and must
not depend on engine internals. The engine therefore exposes a narrow,
Go-typed interface set under `pkg/editor/` plus a versioned IPC protocol under
`pkg/protocol/`. This is the only sanctioned interaction path between the editor
and the runtime graph interpreter.

## 2. Constraints & Assumptions

- The editor process is out-of-process; all live interaction crosses the IPC
  protocol. In-process interfaces are used by the engine to populate IPC events.
- Interface implementations are registered only when an `EditorInterface`
  service is present (editor attached); headless runtime registers nothing.
- Validation results never mutate graph state — they only report compatibility.

## 3. Invariant Compliance

| L1 Invariant (l1-visual-graph-system) | Go Realization |
| :--- | :--- |
| Engine exposes a closed "door"; editor has no other entry point | All editor access funneled through the three `pkg/editor/graph.go` interfaces |
| Runtime ignores editor-only metadata | `editor_metadata` not consumed here; only runtime values surfaced via `NodeInspection` |
| Graph mutations are validated before apply | `OnConnectionChanged` / `OnPropertyChanged` return `ValidationResult` prior to commit |
| Debugging is opt-in and non-invasive | `graphDebugSyncSystem` registered in `PostUpdate` only when editor connected |

## 4. Detailed Design

### 4.1 GraphEditorPlugin Interface

```plaintext
// pkg/editor/graph.go

GraphEditorPlugin (interface)
  OnGraphOpened(graph: GraphDefinition)
    // Called when the editor opens a graph for editing.
    // Allows the engine to prepare debugging state.

  OnGraphClosed(graphID: string)
    // Called when the editor closes a graph.
    // Engine cleans up debug state.

  OnNodeSelected(graphID: string, nodeID: string) -> NodeInspection
    // Called when the editor selects a node.
    // Returns current runtime values for inspection.

  OnConnectionChanged(graphID: string, change: ConnectionChange) -> ValidationResult
    // Called when the editor adds/removes a connection.
    // Engine validates type compatibility and returns errors or warnings.

  OnPropertyChanged(graphID: string, nodeID: string, property: string, value: any) -> ValidationResult
    // Called when the editor changes a node property.
    // Engine validates the value.

NodeInspection
  node_id:       string
  node_type:     string
  pin_values:    map[string]any          // current runtime values at each pin
  execution_count: uint64                // how many times this node has executed
  last_error:    string                  // last error if any

ConnectionChange
  action:       Add | Remove
  connection:   Connection

ValidationResult
  valid:        bool
  errors:       []string
  warnings:     []string
```

### 4.2 NodeRegistryQuery Interface

```plaintext
// pkg/editor/graph.go

NodeRegistryQuery (interface)
  ListAllNodes() -> []NodeDescriptor
    // Returns all registered node types for the editor's node palette.

  SearchNodes(query: string, category: string) -> []NodeDescriptor
    // Filtered search for the "add node" context menu.

  GetNodeDescriptor(typeName: string) -> Option[NodeDescriptor]
    // Full descriptor for a specific node type.

  GetCompatibleNodes(pinType: string, pinDirection: Direction) -> []NodeDescriptor
    // Given a pin the user is dragging from, return nodes with compatible pins.
    // This powers the "drag a wire and release" → context menu workflow.

  GetTypeHierarchy(typeName: string) -> []string
    // Returns assignable types (e.g., Vec3 is assignable to any, float32 is assignable to float64).
    // Used for connection validation and smart suggestions.
```

### 4.3 GraphDebugger Interface

```plaintext
// pkg/editor/graph.go

GraphDebugger (interface)
  SetBreakpoint(graphID: string, nodeID: string) -> error
  RemoveBreakpoint(graphID: string, nodeID: string) -> error
  ListBreakpoints(graphID: string) -> []string

  StepOver(graphID: string) -> GraphExecutionFrame
  StepInto(graphID: string) -> GraphExecutionFrame     // enters subgraph
  Continue(graphID: string) -> error

  GetExecutionTrace(graphID: string, maxFrames: int) -> []GraphExecutionFrame
  GetVariableValues(graphID: string, entityID: Entity) -> map[string]any
  GetPinValue(graphID: string, nodeID: string, pinID: string, entityID: Entity) -> any

GraphExecutionFrame
  node_id:       string
  node_type:     string
  pin_values:    map[string]any
  step_index:    uint32
  timestamp:     int64
```

### 4.4 IPC Protocol Extensions

The graph debugging protocol extends `pkg/protocol/` for editor-engine communication:

```plaintext
// pkg/protocol/graph.go

GraphBreakpointHit
  // Engine → Editor. Execution paused at a breakpoint.
  graph_id:     string
  node_id:      string
  entity_id:    uint64
  frame:        GraphExecutionFrame

GraphExecutionTraceEvent
  // Engine → Editor. Real-time execution trace (when tracing is enabled).
  graph_id:     string
  entity_id:    uint64
  frames:       []GraphExecutionFrame

GraphRuntimeError
  // Engine → Editor. A graph encountered a runtime error.
  graph_id:     string
  node_id:      string
  entity_id:    uint64
  error:        string
  pin_values:   map[string]any

GraphLiveUpdate
  // Editor → Engine. Push a graph modification for immediate preview.
  graph_id:       string
  change_type:    NodeAdded | NodeRemoved | ConnectionAdded | ConnectionRemoved | PropertyChanged
  payload:        any                    // change-specific data
```

## 5. Open Questions

<!-- TBD: Should GraphLiveUpdate carry a schema version so the editor and engine
     can negotiate protocol compatibility across versions? -->
<!-- TBD: Should breakpoint state survive engine hot-reload, or be re-pushed by
     the editor on reconnect? -->

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
| 0.1.0 | 2026-05-15 | Initial draft: extracted from `l1-visual-graph-system.md` §4.7–§4.8 (Door interfaces + IPC protocol) per `/magic-analyze` SPEC_DECOMPOSE |
