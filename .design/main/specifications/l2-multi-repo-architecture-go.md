# Multi-Repository Architecture — Go Implementation

**Version:** 0.2.0
**Status:** Stable
**Layer:** go
**Implements:** [l1-multi-repo-architecture.md](l1-multi-repo-architecture.md)

## Overview

Go-level design for the engine↔editor boundary: the two public packages
`pkg/editor/` (runtime extension interfaces) and `pkg/protocol/` (IPC wire
messages), the `//go:build editor` gating convention, and the architecture-guard
tests that mechanically enforce the L1 isolation invariants. This spec specifies
*only* the engine-side surface in the `neuengine` repository — the `editor`
repository is an external consumer and is out of scope.

## Related Specifications

- [l1-multi-repo-architecture.md](l1-multi-repo-architecture.md) — L1 concept specification (parent)
- [l2-app-framework-go.md](l2-app-framework-go.md) — `App` builder + plugin lifecycle that hosts `EditorInterface` as a service
- [l2-visual-graph-editor-bridge.md](l2-visual-graph-editor-bridge.md) — concrete `pkg/editor/graph.go` + `pkg/protocol/graph.go` consumer of this boundary

## 1. Motivation

The L1 spec defines a structural split into two repositories. The Go
implementation must make that boundary **mechanically enforceable**, not merely
documented:

- The only engine→editor channel is a set of interface-and-data-only packages
  under `pkg/`.
- Import-direction invariants must be CI-verifiable with concrete commands, not
  code review.
- A game that uses neither package must pay zero binary/runtime cost — achieved
  by ensuring both packages are dependency-free and side-effect-free (no
  `init()`, no global registration).

## 2. Constraints & Assumptions

- **Go 1.26.3+**, module path `github.com/neuengine/neu`.
- **`pkg/editor/` is interface-and-type only**: no function bodies containing
  business logic, no `internal/ecs` import, no `*World` field anywhere.
- **`pkg/protocol/` imports stdlib only**: permitted imports are limited to
  `encoding/json`, `bufio`, `io`, `fmt`, `errors`. Any other import is an
  INV-4 violation and fails the guard test.
- **`//go:build editor`** appears in the engine *only* in `internal/app` and
  `internal/definition` registration shims — nowhere else. Enforced by a grep
  guard.
- **No `Option[T]`**: L1 pseudo-code `Option[Range]` / `Option[GizmoHit]` map
  to the Go comma-ok idiom — pointer-or-nil for struct payloads
  (`*Range`), `(T, bool)` for value returns.
- **Editor repo out of scope**: INV-2 (editor has zero `internal/` imports) is
  enforced by Go module semantics (the `internal/` visibility rule) and the
  editor repository's own CI; this spec only asserts the engine side.

## 3. Core Invariants

> [!NOTE]
> See [l1-multi-repo-architecture.md §3](l1-multi-repo-architecture.md) for the
> technology-agnostic invariants INV-1…INV-6. Go-specific compliance is
> tabulated in §4.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: `neuengine` has zero imports of `editor` | CI guard: `go list -deps ./... ! grep github.com/neuengine/editor` must yield nothing. Backed by `TestNoEditorImports` using `golang.org/x/tools/go/packages`-free stdlib `go/build` walk (C-003: no external dep — uses `go list` via `os/exec` in the test). |
| **INV-2**: `editor` has zero `internal/` imports | Structural — Go forbids importing another module's `internal/`. Asserted in the editor repo's CI (out of scope here); engine side adds no escape hatch (no type-reexport of `internal/ecs` from `pkg/`). |
| **INV-3**: `pkg/editor/` is interfaces/types/consts only | `TestEditorPkgIsContractOnly`: parses every file in `pkg/editor/` via `go/ast`, asserts only `GenDecl` (type/const) and interface-method `FuncType` decls — zero `FuncDecl` with a body. Also asserts no import path containing `internal/`. |
| **INV-4**: `pkg/protocol/` has no engine deps | `TestProtocolStdlibOnly`: `go list -f '{{join .Imports "\n"}}' ./pkg/protocol` ⊆ stdlib allowlist. |
| **INV-5**: game importing neither pays zero overhead | Both packages contain no `init()` and no package-level mutable state. `TestNoPackageInit` asserts absence of `init` `FuncDecl`. Linker drops unreferenced interface types — verified by a size-delta build check in CI (`go build` with/without an import). |
| **INV-6**: editor world mutations go through `CommandBuffer` | `EditorInterface` (the service handed to `EditorPlugin.Build`) exposes only `Commands() *command.CommandBuffer` and read-only query handles — no `*World`, no direct sparse-set access. Compile-time: the interface simply has no method returning `*World`. |

## Go Packages

```
pkg/editor/      // unconditional compile; interfaces + data types only
  plugin.go
  inspector.go
  gizmo.go
  definition.go
  property.go    // PropertyInfo, PropertyList, Range, GizmoHit
pkg/protocol/    // stdlib-only; IPC wire messages
  hotreload.go
  diagnostics.go
  codec.go       // newline-delimited JSON framing
```

Neither package imports any other engine package. `pkg/editor/` may reference
shared value types (`Entity`, `TypeID`, `Ray3D`) only via the public `pkg/ecs`
and `pkg/math` aliases — never `internal/`.

## Type Definitions

### pkg/editor — Extension Interfaces

```go
// EditorPlugin is the single entry point. The engine invokes Build only
// during LEVEL_EDITOR init; absent in headless/production builds (INV-5).
type EditorPlugin interface {
    Build(api EditorInterface)
}

// EditorInterface is the capability handle passed to EditorPlugin.Build.
// It exposes ONLY deferred mutation + read access (INV-6) — no *World.
type EditorInterface interface {
    RegisterInspectorPlugin(InspectorPlugin)
    RegisterGizmoPlugin(GizmoPlugin)
    RegisterDefinitionPlugin(DefinitionEditorPlugin)
    Commands() *command.CommandBuffer
}

type InspectorPlugin interface {
    Handles(componentTypeID ecs.TypeID) bool
    Render(entity ecs.Entity, component ecs.DynamicObject) PropertyList
}

type GizmoPlugin interface {
    Handles(componentTypeID ecs.TypeID) bool
    Draw(entity ecs.Entity, component ecs.DynamicObject, gizmos GizmoWriter)
    // Interact returns (_, false) when the gizmo was not hit (L1 Option→comma-ok).
    Interact(entity ecs.Entity, component ecs.DynamicObject, ray math.Ray3D) (GizmoHit, bool)
}

type DefinitionEditorPlugin interface {
    Handles(defType DefinitionType) bool
    Edit(node DefinitionNode)
    GetInspectorProperties(node DefinitionNode) []EditorProperty
}
```

### pkg/editor — Data Types

```go
type PropertyInfo struct {
    Name        string
    DisplayName string
    TypeHint    string // "float32", "Vec3", "Handle<Image>", ...
    Value       any
    Editable    bool
    Range       *Range // nil ⇒ no range (L1 Option[Range])
}

type Range struct{ Min, Max float64 }
type PropertyList []PropertyInfo
type GizmoHit struct {
    Distance float32
    Handle   string // which gizmo sub-handle (axis, ring, ...)
}

// GizmoWriter mirrors the diagnostic-system Gizmos API; defined as an
// interface so pkg/editor stays implementation-free (INV-3).
type GizmoWriter interface {
    Line(from, to math.Vec3, color math.LinearRgba)
    Sphere(center math.Vec3, radius float32, color math.LinearRgba)
}
```

### pkg/protocol — Wire Messages

```go
// Every message carries a discriminator in the "type" JSON field.
type Kind string

const (
    KindHotReloadPrepare Kind = "HotReloadPrepare"
    KindHotReloadReady   Kind = "HotReloadReady"
    KindHotReloadFailed  Kind = "HotReloadFailed"
    KindShaderError      Kind = "ShaderError"
    KindShaderReloaded   Kind = "ShaderReloaded"
    KindReloadMetrics    Kind = "ReloadMetrics"
    KindNetworkAlert     Kind = "NetworkAlert"
    KindDiagnosticSnap   Kind = "DiagnosticSnapshot"
)

type HotReloadPrepare struct {
    Type         Kind   `json:"type"`
    SnapshotPath string `json:"snapshot_path"`
}

type HotReloadReady struct {
    Type         Kind   `json:"type"`
    SnapshotPath string `json:"snapshot_path"`
    EntityCount  uint32 `json:"entity_count"`
    SnapshotSize uint64 `json:"snapshot_size"`
}

type HotReloadFailed struct {
    Type   Kind   `json:"type"`
    Reason string `json:"reason"`
}

type ReloadMetrics struct {
    Type         Kind     `json:"type"`
    SnapshotMS   int64    `json:"snapshot_ms"`
    BuildMS      int64    `json:"build_ms"`
    RestoreMS    int64    `json:"restore_ms"`
    EntitiesLost []string `json:"entities_lost"`
}

type AlertLevel uint8

const (
    AlertWarning AlertLevel = iota
    AlertCritical
)

type NetworkAlert struct {
    Type    Kind       `json:"type"`
    Metric  string     `json:"metric"`
    Level   AlertLevel `json:"level"`
    Value   float64    `json:"value"`
    Message string     `json:"message"`
}

type DiagnosticSnapshot struct {
    Type      Kind               `json:"type"`
    Timestamp int64              `json:"timestamp"`
    Metrics   map[string]float64 `json:"metrics"`
}
```

## Key Methods

### pkg/protocol — Codec (newline-delimited JSON)

```
// Encode writes one message as a single JSON object + '\n'.
// msg must be one of the protocol structs (its Type field set).
func Encode(w io.Writer, msg any) error

// Decode reads the next newline-delimited message from r.
// It peeks the "type" field, then unmarshals into the concrete struct.
// Returns io.EOF at clean stream end.
func Decode(r *bufio.Reader) (any, Kind, error)

// Scanner wraps bufio.Scanner with the framing contract; one Scan == one msg.
type Scanner struct{ /* bufio.Scanner */ }
func NewScanner(r io.Reader) *Scanner
func (s *Scanner) Next() (any, Kind, bool, error)
```

Forward-compat rule (L1 §4.6): unknown `type` values and unknown JSON fields
are **not** errors — `Decode` returns `(nil, kind, ErrUnknownKind)` so the
reader can skip-and-continue; extra fields are ignored by `encoding/json`.

### internal/app — Build-Tag Gated Registration

```
// internal/app/editor_on.go   //go:build editor
//   registers the EditorInterface service during LEVEL_EDITOR init.
// internal/app/editor_off.go  //go:build !editor
//   provides a no-op stub so the field is always present (INV-5: the
//   no-op path has zero cost and no pkg/editor reference reaching plugins).
func registerEditorService(app *App) // both files define this symbol
```

## CI / Verification Strategy

These are the concrete commands the L1 §4.7 strategy maps to; they are the
`Verify` lines consumed by tasks T-2G01 / T-2G02:

```
go build ./pkg/protocol/                         # compiles standalone
go test  ./pkg/protocol/ -run TestProtocolRoundTrip
go test  ./pkg/editor/   -run TestEditorPkgIsContractOnly
go vet   ./...
go list -deps ./... | (! grep github.com/neuengine/editor)   # INV-1
grep -rn '//go:build editor' --include=*.go internal/ \
     | grep -v -E 'internal/(app|definition)/'  | (! grep .) # tag scope
```

`TestProtocolRoundTrip` is table-driven over every message Kind: marshal →
newline-frame → `Decode` → deep-equal the original (C-028 table-driven; C-005
`-race` clean — codec is stateless).

## Error Handling

| Condition | Behavior |
| :--- | :--- |
| Unknown message `type` on decode | Return `(nil, kind, ErrUnknownKind)`; caller skips, stream continues (forward-compat) |
| Malformed JSON line | Return wrapped `*json.SyntaxError`; caller logs and resyncs at next `\n` |
| Stream closed mid-message | `io.ErrUnexpectedEOF`; hot-reload orchestrator reports failure, launches nothing |
| `EditorInterface` requested in headless build | Service absent; `app.Services().Get[EditorInterface]()` returns `false`; registration silently skipped (INV-5) |

```go
var (
    ErrUnknownKind = errors.New("protocol: unknown message kind")
)
```

## Testing Strategy

- **Architecture guards** (the heart of this spec): `TestNoEditorImports`,
  `TestEditorPkgIsContractOnly`, `TestProtocolStdlibOnly`, `TestNoPackageInit`
  — all use `go/ast` / `go list`, zero external deps (C-003).
- **Codec round-trip**: table-driven over all `Kind`s, including unknown-kind
  and extra-field forward-compat cases.
- **Build-tag matrix**: `go build -tags editor ./...` and the default build
  both compile; the `!editor` stub is exercised by the headless path test.
- **No benchmarks**: this boundary is not on any hot path (IPC is per-reload,
  not per-frame).

## 7. Drawbacks & Alternatives

- **Drawback**: AST-based contract tests are stricter than `go vet` and can
  reject legitimate helper consts.
  **Alternative**: rely on code review.
  **Decision**: keep the AST guard — INV-3 is a structural guarantee the L1
  treats as load-bearing for the dogfooding argument; review is not mechanical.
- **Drawback**: newline-delimited JSON is verbose vs. a binary codec.
  **Alternative**: protobuf / gob.
  **Decision**: L1 §4.4.2 mandates stdlib `encoding/json` + `bufio` for
  operational simplicity pre-v1.0.0; binary codec is a post-1.0 optimization.
- **Drawback**: dropping the `A`-style explicit signal; `pkg/protocol`
  transport is Unix-socket/named-pipe only.
  **Alternative**: WebSocket transport now.
  **Decision**: L1 Open Question (DEFERRED — Phase 4, editor kickoff). The
  message types are transport-agnostic; adding WebSocket later is additive and
  non-breaking. Out of 0.1.0 scope.

## Canonical References

<!-- All paths verified on disk; INV-1…INV-6 mechanically verified by the
     architecture-guard suite (the C29 validator). L1 parent is Stable. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |
| [PLUGIN] | `pkg/editor/plugin.go` | `EditorPlugin` + `EditorInterface` (deferred-mutation handle, no `*World` — INV-6). |
| [GIZMO] | `pkg/editor/gizmo.go` | `GizmoPlugin`, `GizmoWriter`, `GizmoHit` (comma-ok `Interact`). |
| [DEFINITION] | `pkg/editor/definition.go` | `DefinitionEditorPlugin` (mirrors definition-system §4.10). |
| [PROPERTY] | `pkg/editor/property.go` | `PropertyInfo`, `PropertyList`, `Range`, `InspectorPlugin` data types. |
| [MESSAGES] | `pkg/protocol/messages.go` | `Kind` discriminator + 8 wire-message structs (hot-reload + diagnostics). |
| [CODEC] | `pkg/protocol/codec.go` | `Encode`/`Decode`/`Scanner` newline-delimited JSON, `ErrUnknownKind` forward-compat. |
| [GUARD_EDITOR] | `pkg/editor/editor_test.go` | `TestEditorPkgIsContractOnly` (INV-3), `TestNoEditorImports` (INV-1), `TestNoPackageInit` (INV-5), `TestEditorBuildClean`. |
| [GUARD_PROTOCOL] | `pkg/protocol/protocol_test.go` | `TestProtocolRoundTrip` (8 Kinds), `TestProtocolStdlibOnly` (INV-4), unknown-kind/extra-field forward-compat. |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-17 | Initial L2 draft — Go translation of l1-multi-repo-architecture v1.4.0. Concrete pkg/editor interfaces + pkg/protocol JSON wire messages, //go:build editor scoping, AST/`go list` architecture-guard tests mapping INV-1…INV-6. L1 Phase-4-deferred open questions kept out of 0.1.0 scope. |
| 0.2.0 | 2026-05-30 | **Draft → Stable** (`/magic.spec` ratification). L1 parent now Stable (layer constraint cleared); Canonical References populated with the on-disk `pkg/editor/` + `pkg/protocol/` sources and their guard tests. C29 satisfied: `go test ./pkg/editor/ ./pkg/protocol/` green — `TestEditorPkgIsContractOnly` (INV-3), `TestNoEditorImports` (INV-1), `TestNoPackageInit` (INV-5), `TestProtocolStdlibOnly` (INV-4), `TestProtocolRoundTrip` (8 Kinds), unknown-kind/extra-field forward-compat all pass. The boundary contract is mechanically enforced, not merely documented. |
