# Visual Graph Editor Bridge — Go Implementation

**Version:** 0.3.0
**Status:** Stable
**Layer:** go
**Implements:** [l1-visual-graph-system.md](l1-visual-graph-system.md)

## Overview

Go-level contract for the engine-side "door" that the external `editor`
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

The visual graph editor lives in a separate repository (`editor`) and must
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

> **Implementation Status (v0.2.0).** The **contract surface** is implemented +
> guard-tested: `pkg/editor/graph.go` (the three interfaces + editor-local DTOs)
> and `pkg/protocol/graph.go` (the four IPC messages + codec round-trip). Per the
> multi-repo decoupling precedent (cf. `pkg/editor/definition.go`'s opaque
> `DefinitionNode`), the interfaces are **self-contained**: entities are addressed
> by raw `uint64` and `OnGraphOpened` takes a `graphID string` (not a
> `GraphDefinition`) so the editor repo links a stable contract without importing
> the engine's graph types. The engine-side **wiring is deferred** to App
> integration: the concrete `GraphEditorPlugin`/`NodeRegistryQuery`/`GraphDebugger`
> implementations and the `graphDebugSyncSystem` (PostUpdate, editor-attached only)
> map live interpreter state onto these types and emit the IPC events.

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

<!-- Contract surface (v0.2.0). Engine-side wiring (graphDebugSyncSystem +
     concrete implementations) adds rows when App integration lands. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |
| editor-graph | pkg/editor/graph.go | `GraphEditorPlugin`/`NodeRegistryQuery`/`GraphDebugger` interfaces + editor-local DTOs (§4.1–§4.3) |
| protocol-graph | pkg/protocol/graph.go | `GraphBreakpointHit`/`GraphExecutionTraceEvent`/`GraphRuntimeError`/`GraphLiveUpdate` IPC messages (§4.4) |
| protocol-codec | pkg/protocol/codec.go | `Decode` dispatch for the graph message Kinds (round-trip) |
| editor-guard | pkg/editor/graph_test.go | Fake-implementer contract assertions + the multi-repo contract-only guard |
| protocol-graph-test | pkg/protocol/graph_test.go | Graph message round-trip + scanner (forward-compat) |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-15 | Initial draft: extracted from `l1-visual-graph-system.md` §4.7–§4.8 (Door interfaces + IPC protocol) per `/magic-analyze` SPEC_DECOMPOSE |
| 0.2.0 | 2026-06-01 | Contract surface implemented (`/magic.run` Track P04): `pkg/editor/graph.go` (3 interfaces + editor-local DTOs) + `pkg/protocol/graph.go` (4 IPC messages + 4 codec Decode cases). Reconciled to the multi-repo decoupling precedent — self-contained DTOs, `uint64` entity IDs, `OnGraphOpened(graphID string)` instead of a `GraphDefinition` parameter — so `pkg/editor` stays import-free of engine graph types (passes `TestEditorPkgIsContractOnly`); `pkg/protocol` stays stdlib-only (`TestProtocolStdlibOnly`). Canonical References populated; protocol round-trip + fake-implementer contract tests green (protocol 94.6%). Engine-side wiring (`graphDebugSyncSystem` + concrete impls) deferred to App integration. Status stays Draft pending that wiring. |
| 0.3.0 | 2026-06-03 | Promoted Draft → Stable (`/magic.task`). The engine-side wiring v0.2.0 deferred is now built (T-6P04·rem.1+.2): `internal/grapheditor` concrete `GraphEditorPlugin`/`NodeRegistryQuery`/`GraphDebugger` + `DebugStore` + the `PostUpdate` `graphDebugSyncSystem` emitting `pkg/protocol` graph IPC, plus the interpreter trace hook (`Interpreter.RunTraced` + `TraceRecorder`, defined in `pkg/visualgraph` to keep it editor/protocol-free). Validated end-to-end by `examples/editor` (T-6T07, hash-stable). L1 parent now Stable. |
