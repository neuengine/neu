# AI Assistant System — Go Implementation

**Version:** 0.2.0
**Status:** Stable
**Layer:** go
**Implements:** [l1-ai-assistant-system.md](l1-ai-assistant-system.md)

## Overview

Go-level design for editor-facing AI agents (all `//go:build editor`). An
`AssistantManager` resource holds per-agent connections + capability sets and a
request log. Agents speak a JSON `AgentMessage` protocol over pluggable
transports: `stdio` (subprocess) and `http` are implemented; `websocket` is
**ADR-gated** (C-003 — a WS dependency or a minimal RFC6455 stdlib impl is a
deferred opt-in, not part of the realized v1 surface). A `ContextProvider`
assembles a capability-filtered `EditorContext` on demand. All agent world
mutations are translated into plugin-ID + request-ID-tagged Commands and grouped
into one undo step (INV-1/INV-4). Every agent call is asynchronous with a
timeout so a slow/unreachable backend never blocks the editor loop (INV-5).

## Related Specifications

- [l1-ai-assistant-system.md](l1-ai-assistant-system.md) — L1 concept specification (parent)
- [l2-plugin-distribution-go.md](l2-plugin-distribution-go.md) — agents are editor plugins; this transport/capability contract is the universal plugin protocol
- [l2-app-framework-go.md](l2-app-framework-go.md) — registers as an `EditorPlugin` at `LEVEL_EDITOR`
- [l2-command-system-go.md](l2-command-system-go.md) — agent mutations become tagged Commands (INV-1/INV-4)
- [l2-task-system-go.md](l2-task-system-go.md) — agent I/O runs on the worker pool; `context.Context` cancellation (INV-5)
- [l2-definition-system-go.md](l2-definition-system-go.md) — `generate_ui`/`generate_scene` emit definition documents
- [l2-type-registry-go.md](l2-type-registry-go.md) — `ContextProvider` summarizes component metadata for agents

## 1. Motivation

The L1 standardizes editor↔agent integration. The Go binding's job: one JSON
protocol over three transports, a capability bitfield checked before every
action, and a modification path that reuses the Command pipeline so AI edits are
undoable and auditable like any user edit. Editor-only via the `editor` build
tag — shipped games and headless builds link none of it.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: `encoding/json`, `os/exec` (stdio), `net/http` + `nhooyr.io/websocket`-or-stdlib candidate (ADR) for the WS transport; `context.Context` everywhere.
- **`//go:build editor`**: the entire package is gated; non-editor builds exclude it (INV: no AI in shipped games).
- **C-003**: stdio + HTTP are stdlib; a WebSocket dep requires an ADR (or a minimal RFC6455 stdlib impl).
- The protocol envelope reuses `pkg/protocol`-style newline-delimited JSON for stdio (shared with plugin distribution).
- No AI model is bundled; the backend is external (INV: engine ships the protocol, not the model).

## 3. Core Invariants

> [!NOTE]
> See [l1-ai-assistant-system.md §3](l1-ai-assistant-system.md) for INV-1…INV-5.
> Go-specific compliance is tabulated in §4.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: agents never bypass the Command pipeline | `AssistantManager.applyModification` converts an agent result into `Command`s issued via the tagged `CommandIssuer` from `pkg/plugin`; there is no `*World` path. |
| **INV-2**: actions beyond granted capabilities rejected + logged | A `Capability` bitfield per agent; `checkCap(agent, cap)` gates each method dispatch and each `EditorContext` field; a violation returns `ErrCapabilityDenied` and appends an audit entry. |
| **INV-3**: agents load/unload at editor runtime | Agents are `EditorPlugin`s managed by the plugin distribution loader; enable/disable toggles the connection without an engine restart. |
| **INV-4**: AI modifications tagged with agent ID + request ID, grouped for undo | Commands carry `(AgentID, RequestID)`; the command buffer groups one request's commands into a single undo transaction. |
| **INV-5**: network failure never crashes/freezes the editor | Every `Connection.Send`/request runs on the worker pool under a `context.WithTimeout`; transport errors return `ErrAgentUnavailable`; the main loop never blocks on a connection. |

## Go Package

```
pkg/assistant/                 // //go:build editor
  manager.go                   // AssistantManager resource, request log, dispatch
  message.go                   // AgentMessage, MessageType, AgentError (JSON)
  capability.go                // Capability bitfield (Read*/Write*/Advanced)
  context.go                   // ContextProvider, EditorContext (capability-filtered)
  methods.go                   // standard method names + param/result schemas
  registry.go                  // AgentRegistry: scan .agents/ + user dir, AgentManifest
  plugin.go                    // AIAssistantPlugin (EditorPlugin) wiring
  transport/                   // //go:build editor
    transport.go               // Transport, Connection interfaces
    stdio.go                   // subprocess stdin/stdout, newline-delimited JSON
    websocket.go               // long-lived WS connection
    http.go                    // stateless request/response
```

## Type Definitions

```go
type AgentID string
type RequestID string

type MessageType uint8
const (MsgRequest MessageType = iota; MsgResponse; MsgNotification; MsgError)

type AgentMessage struct {
    ID     string         `json:"id"`
    Type   MessageType    `json:"type"`
    Method string         `json:"method,omitempty"`
    Params map[string]any `json:"params,omitempty"`
    Result any            `json:"result,omitempty"`
    Error  *AgentError    `json:"error,omitempty"`
}

type Capability uint32
const (
    ReadTypeRegistry Capability = 1 << iota
    ReadScenes; ReadDefinitions; ReadAssetManifest; ReadDiagnostics
    WriteDefinitions; WriteScenes; SpawnEntities; ModifyComponents; ExecuteCommands
    FileSystemAccess; NetworkAccess; CodeGeneration
)
func (c Capability) Has(f Capability) bool { return c&f == f }

type Transport interface {
    Connect(endpoint string) (Connection, error)
    Close()
}
type Connection interface {
    Send(ctx context.Context, m AgentMessage) error
    Receive(ctx context.Context) (AgentMessage, error)
    IsAlive() bool
}

type EditorContext struct { // assembled on demand, filtered by capability
    SelectedEntities []EntityInfo
    ActiveScene      SceneInfo
    TypeRegistry     TypeRegistrySummary
    RecentCommands   []CommandRecord
    Diagnostics      DiagnosticsSummary
}

var (
    ErrCapabilityDenied = errors.New("assistant: capability not granted")
    ErrAgentUnavailable = errors.New("assistant: agent unreachable or timed out")
)
```

## Performance Strategy

- **Off-main-loop I/O**: transports run on `task.IOPool`; the editor schedule never blocks on `Send`/`Receive`.
- **On-demand context**: `EditorContext` is assembled only when an agent requests it, and only the capability-permitted fields — never the whole world per message (L1 §4.4).
- **Bounded requests**: every request carries a timeout; `map[RequestID]context.CancelFunc` (mutex-guarded) lets the editor cancel in flight; goroutines return within one RTT.
- **Rate limiting**: a per-agent token bucket (default 60 req/min) caps runaway agents before they reach a transport.

## Error Handling

| Condition | Behavior |
| :--- | :--- |
| Method beyond granted capabilities | `ErrCapabilityDenied` + audit entry (INV-2) |
| Transport timeout / disconnect | `ErrAgentUnavailable`; agent marked disconnected; editor degrades gracefully (INV-5) |
| Malformed `AgentMessage` JSON | wrapped decode error; message dropped, connection resynced at next newline |
| Rate limit exceeded | explicit rate-limit error with retry-after |
| Credentials/sensitive data in context | `ContextProvider` never includes `.env`/keys even with `FileSystemAccess` (L1 §4.9) |

## Testing Strategy

- **Capability gate** (INV-2): each standard method requires its declared capability; an under-privileged agent ⇒ `ErrCapabilityDenied` + one audit entry.
- **Command tagging** (INV-1/INV-4): a `generate_scene` result emits Commands tagged `(agent, request)` grouped into one undo step (recording command buffer).
- **Async/timeout** (INV-5): a fake transport that never responds ⇒ `ErrAgentUnavailable` within the timeout; no goroutine leak (`-race` + leak check).
- **Transport parity**: the same `AgentMessage` round-trips identically over stdio/ws/http fakes.
- **Context filtering**: an agent without `ReadDiagnostics` gets an `EditorContext` with `Diagnostics` empty.

## 7. Drawbacks & Alternatives

- **Drawback**: a custom JSON protocol diverges from MCP/LSP.
  **Alternative**: adopt Model Context Protocol.
  **Decision**: L1 §5 open question; the envelope is intentionally MCP-shaped (id/method/params/result) so an MCP adapter is additive. Kept engine-native for v1.
- **Drawback**: three transports triple the surface.
  **Alternative**: stdio only.
  **Decision**: L1 §4.2 requires all three (local subprocess, long-lived service, stateless API); they share one `Connection` contract so dispatch is transport-agnostic. Kept.

## Canonical References

<!-- MANDATORY for Stable status. Stub — populate when implementation lands
     (Phase 6 Track H). Blocked until: (1) l1-ai-assistant-system Stable; (2)
     pkg/assistant + transports implemented with tests + a validating example
     (examples/app/ai_assistant_stub/). -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-30 | Initial L2 draft — Go translation of l1-ai-assistant-system v0.2.0. `//go:build editor` `pkg/assistant`: `AssistantManager`, JSON `AgentMessage` over stdio/ws/http transports sharing one `Connection` contract, `Capability` bitfield gating methods + `EditorContext` fields (INV-2), Command-pipeline modification tagged agent+request grouped for undo (INV-1/INV-4), worker-pool async with per-request cancel + timeout (INV-5), `AgentRegistry` directory scan. Authored ahead of Phase 6 Track H (`/magic.spec`). Draft — L1 parent Draft + no implementation yet. |
| 0.2.0 | 2026-06-03 | Promoted Draft → Stable (`/magic.task`). Realized in `pkg/assistant` (`//go:build editor`): `AssistantManager`, JSON `AgentMessage` over the `Connection` contract with `StdioConnection` + `HTTPTransport`/`HTTPConnection` + in-memory transports, `Capability` gating + `EditorContext`/`ContextProvider` (INV-2), per-request cancel/timeout (INV-5). **Narrowed (overclaim correction):** the `websocket` transport is ADR-gated (C-003) and explicitly NOT part of the realized v1 surface — promotion is on `stdio`+`http`; ws follows its ADR. L1 parent now Stable. |
