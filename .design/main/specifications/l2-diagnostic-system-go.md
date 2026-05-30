# Diagnostic System — Go Implementation

**Version:** 0.1.0
**Status:** Stable
**Layer:** go
**Implements:** [l1-diagnostic-system.md](l1-diagnostic-system.md)

## Overview

Go-level design for runtime diagnostics. A `DiagnosticsStore` resource holds named
metrics, each backed by a fixed-capacity ring buffer so per-frame `Push` is
allocation-free and rolling averages are deterministic (sample-count, not wall-clock).
All collection systems run in the `Last` schedule behind a run-condition that makes
them no-ops when no reader is registered (INV-1 zero-cost). Gizmos are an
immediate-mode vertex buffer drained by a dedicated `RenderFeature` after the scene and
before UI — pure visual, no world mutation (INV-2). Profiling spans compile to no-ops
without the `profiling` build tag (INV-4). Structured logging wraps `log/slog` with a
per-module level filter. The error-code surface defers to `l2-error-core-go`.

## Related Specifications

- [l1-diagnostic-system.md](l1-diagnostic-system.md) — L1 concept specification (parent)
- [l2-system-scheduling-go.md](l2-system-scheduling-go.md) — diagnostic systems run in the `Last` schedule; per-system timing spans
- [l2-render-core-go.md](l2-render-core-go.md) — gizmo + debug-overlay render passes reuse the `RenderFeature` interface
- [l2-error-core-go.md](l2-error-core-go.md) — the structured error-code registry/`--explain` surface consumes `errs.Lookup` (no duplication)
- [l2-app-framework-go.md](l2-app-framework-go.md) — `DiagnosticsPlugin` registration, schedule wiring
- [l2-math-system-go.md](l2-math-system-go.md) — `Vec3`/`Color` for gizmo geometry

## 1. Motivation

Observability must be present from day one but cost nothing when unused. Binding the L1
diagnostic concept to Go means: ring buffers instead of growing slices (deterministic,
0-alloc), `log/slog` instead of a bespoke logger (stdlib, structured), and build-tag
profiling instead of a runtime flag in the hot path (zero release cost). Gizmos and the
debug overlay reuse the render-core `RenderFeature` rather than inventing a parallel
draw path.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: `log/slog`, `runtime.MemStats`, `runtime/pprof` (goroutine-leak profile), generics for `RingBuffer[T]`.
- **C-003**: stdlib only. Tracy/Chrome-trace export writes a documented JSON/binary format via `encoding/json` + `encoding/binary`; no external profiler dep is linked.
- **INV-1 zero-cost**: collection systems are gated by a `hasReaders` run-condition; an unregistered/disabled diagnostic's `Push` returns immediately.
- **INV-3**: all diagnostic systems are added to the `Last` schedule only — they never declare ordering against gameplay systems.
- **INV-4**: profiling has two build-tagged implementations (`profiling` on/off); release builds exclude `runtime/pprof` span calls.
- Gizmo geometry is rebuilt each frame; retained gizmos hold an immutable vertex asset.

## 3. Core Invariants

> [!NOTE]
> See [l1-diagnostic-system.md §3](l1-diagnostic-system.md) for the technology-agnostic
> invariants (INV-1…INV-4). Go-specific compliance is tabulated in §4.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: zero overhead when no readers registered | Each `Diagnostic` tracks a reader count; `DiagnosticsStore.Push` early-returns when the metric is disabled or has zero readers. Collection systems carry a `RunIf(hasAnyReader)` condition, so the whole collection pass is skipped when nothing consumes diagnostics. |
| **INV-2**: gizmos never affect game state | The `Gizmos` system param appends to a per-frame `[]gizmoVertex` owned by a pooled buffer resource; it exposes no `*World` write path. A `gizmoFeature` (RenderFeature) drains the buffer in a draw pass after the scene, before UI. Buffer is cleared each frame N+1. |
| **INV-3**: all diagnostic systems in `Last`, never affect gameplay ordering | `DiagnosticsPlugin` adds every collection/log/overlay system to `Last` exclusively; none declare `Before`/`After` against non-diagnostic systems. |
| **INV-4**: profiling spans compile-time removable | `span.go` (`//go:build profiling`) wraps `pprof`/Tracy; `span_noop.go` (`//go:build !profiling`) makes `StartSpan`/`End` inlinable no-ops returning a zero `Span`. Release builds contain no span machinery. |

## Go Package

```
pkg/diag/
  store.go         // DiagnosticsStore, DiagnosticPath, Register/Push/Get
  diagnostic.go    // Diagnostic, DiagnosticEntry, RingBuffer[T], Average/Smoothed/Min/Max/Latest
  gizmos.go        // Gizmos interface, GizmoConfig, GizmoConfigStore, RetainedGizmo asset
  log.go           // slog wrapper: per-module level filter, structured fields, sinks
  span.go          // //go:build profiling   — StartSpan/End → pprof + Tracy/Chrome export
  span_noop.go     // //go:build !profiling  — no-op spans (0 cost)
  plugin.go        // DiagnosticsPlugin, LogDiagnosticsPlugin, built-in metric registration
internal/diag/
  builtins.go      // engine/fps, frame_time, entity_count, system/{name}, allocs, worker telemetry
  gizmofeature.go  // gizmoFeature: RenderFeature draining the vertex buffer (after scene, before UI)
  overlay.go       // debug overlay (F3): metrics text via the UI text path
  stepping.go      // //go:build debug — scheduler single-step driver
```

## Type Definitions

```go
type DiagnosticPath string // "engine/fps", "engine/system/Movement"

type DiagnosticEntry struct {
    Value     float64
    Timestamp gametime.Duration
}

// RingBuffer is a fixed-capacity circular buffer — 0-alloc after construction.
type RingBuffer[T any] struct {
    buf  []T
    head int
    size int
}
func NewRingBuffer[T any](capacity int) RingBuffer[T]
func (r *RingBuffer[T]) Push(v T)        // overwrites oldest when full
func (r *RingBuffer[T]) Len() int

type Diagnostic struct {
    Path       DiagnosticPath
    Suffix     string // "fps", "ms", "count"
    History    RingBuffer[DiagnosticEntry]
    MaxHistory int    // default 120
    IsEnabled  bool
    readers    int    // INV-1 gate
}
func (d *Diagnostic) Average() float64        // mean over sample count (deterministic)
func (d *Diagnostic) SmoothedAverage() float64 // EWMA
func (d *Diagnostic) Min() float64
func (d *Diagnostic) Max() float64
func (d *Diagnostic) Latest() (DiagnosticEntry, bool)

type DiagnosticsStore struct {
    metrics map[DiagnosticPath]*Diagnostic
    mu      sync.RWMutex
}

// Gizmos — immediate-mode drawing (INV-2, no world write).
type Gizmos interface {
    Line(start, end pkgmath.Vec3, c pkgmath.Color)
    Ray(origin, dir pkgmath.Vec3, c pkgmath.Color)
    Circle(center, normal pkgmath.Vec3, radius float32, c pkgmath.Color)
    Sphere(center pkgmath.Vec3, radius float32, c pkgmath.Color)
    Box(center, halfExtents pkgmath.Vec3, c pkgmath.Color)
    Arrow(start, end pkgmath.Vec3, c pkgmath.Color)
    Grid(origin, normal pkgmath.Vec3, cell float32, count int, c pkgmath.Color)
    Text(pos pkgmath.Vec3, s string, c pkgmath.Color)
}

type GizmoConfig struct {
    Enabled   bool
    LineWidth float32
    Color     pkgmath.Color
    DepthBias float32
    DepthTest bool
}

type LogLevel = slog.Level // Error/Warn/Info/Debug/Trace mapped onto slog levels

// Span — profiling handle (real under //go:build profiling, no-op otherwise).
type Span struct{ /* tag-dependent payload */ }
func StartSpan(name string) Span
func (s Span) End()
```

## Key Methods

```go
func (s *DiagnosticsStore) Register(d Diagnostic)            // duplicate path ⇒ Warning, not crash
func (s *DiagnosticsStore) Push(p DiagnosticPath, v float64) // INV-1: no-op if disabled/no readers
func (s *DiagnosticsStore) Get(p DiagnosticPath) (*Diagnostic, bool)
func (s *DiagnosticsStore) AddReader(p DiagnosticPath)       // bumps reader count → enables collection

func hasAnyReader(store *DiagnosticsStore) bool              // RunIf condition (INV-1)

func (p DiagnosticsPlugin) Build(app *app.App)               // registers built-ins in Last schedule
func SetModuleLevel(module string, lvl LogLevel)             // per-module slog filter
```

## Performance Strategy

- **0-alloc Push**: `RingBuffer` is pre-sized to `MaxHistory`; `Push` overwrites in place — no growth, no GC pressure on the per-frame metric path.
- **INV-1 short-circuit**: the `hasAnyReader` run-condition skips the entire collection pass when diagnostics are unconsumed; individual `Push` calls early-return on a disabled metric.
- **Pooled gizmo vertices**: the per-frame gizmo buffer is a `sync.Pool`-backed `[]gizmoVertex` reset by index-rewind (mirrors the render-core 0-alloc pattern), not reallocated.
- **Zero release profiling cost**: `span_noop.go` makes `StartSpan`/`End` empty inlinable calls; the `profiling` build tag is required to link any `pprof` machinery.
- **Deterministic averages**: fixed sample count means `Average()` is replay-stable (no wall-clock dependence), satisfying the L1 §4.2 determinism note.

## Error Handling

| Condition | Behavior |
| :--- | :--- |
| Duplicate `Register(path)` | `slog.Warn` + keep the existing metric (L1 §4.2: warning, not crash) |
| `Push` to an unregistered path | `slog.Debug` + no-op (lazy auto-register optional, off by default) |
| `--explain ECODE` for unknown code | defers to `errs.Lookup`; unknown ⇒ `errs.ErrCodeOutOfRange`-style message |
| Gizmo buffer overflow in a frame | grows the pooled buffer once (amortized); never drops vertices silently |
| Profiling export write failure | `slog.Warn`; diagnostics continue (export is best-effort) |

## Testing Strategy

- **INV-1**: with zero readers, a benchmark of the collection pass shows 0 allocs and the systems are skipped (run-condition false); `Push` to a disabled metric is a no-op.
- **RingBuffer**: table test for wrap-around, `Average`/`Min`/`Max`/`Latest` over partial and full buffers; `Average` is deterministic across runs.
- **INV-2**: a gizmo draw call performs no world mutation (query-count unchanged); the buffer clears between frames.
- **INV-3**: `DiagnosticsPlugin` adds systems only to `Last`; assert no ordering edges to gameplay systems.
- **INV-4**: build with `-tags profiling` ⇒ spans record; default build ⇒ `StartSpan` returns a zero `Span` and `End` is a no-op (benchmark shows 0 cost).
- **Logging**: `SetModuleLevel("render", Debug)` filters records below `Debug` for that module only.

## 7. Drawbacks & Alternatives

- **Drawback**: a fixed ring buffer caps history depth.
  **Alternative**: a growable slice.
  **Decision**: L1 §4.2 mandates a fixed sample count for deterministic averages and the cap keeps `Push` 0-alloc. Kept.
- **Drawback**: build-tag profiling means release binaries cannot profile without a rebuild.
  **Alternative**: a runtime profiling flag.
  **Decision**: INV-4 requires compile-time removal so the hot path pays nothing in shipping builds. A `profiling`-tagged build is the supported profiling path. Kept.
- **Drawback**: deferring error codes to `l2-error-core-go` splits the diagnostic surface across two packages.
  **Alternative**: a self-contained code registry here.
  **Decision**: one taxonomy avoids drift; diagnostic consumes `errs` rather than duplicating it. Kept.

## Canonical References

<!-- All paths verified on disk; validated by examples/diagnostic (hash ×20,
     T-6T06) + pkg/diag 94.6% / internal/diag 100% cov. C29 P6 gate closed. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |
| [DIAGNOSTIC] | `pkg/diag/diagnostic.go` | `RingBuffer[T]` (0-alloc Push), `Diagnostic` + Average/Smoothed/Min/Max/Latest. |
| [STORE] | `pkg/diag/store.go` | `DiagnosticsStore`, `Register`/`Push`/`AddReader`, `HasAnyReader` zero-cost gate (INV-1). |
| [GIZMOS] | `pkg/diag/gizmos.go` | `Gizmos` interface + `GizmoBuffer` (8-shape immediate-mode, 0-alloc Reset). |
| [LOG] | `pkg/diag/log.go` | `ModuleFilterHandler` per-module slog level filter. |
| [SPAN] | `pkg/diag/span.go` + `span_noop.go` | `//go:build profiling` spans with no-op release path (INV-4). |
| [FEATURE] | `internal/diag/gizmofeature.go` | `GizmoFeature` (RenderFeature) draining the buffer after scene/before UI (INV-2). |
| [EXAMPLE] | `examples/diagnostic/main.go` | Reader-gate + averages + gizmo geometry, hash-stable ×20 (T-6T06). |
| [TEST] | `pkg/diag/diag_test.go`, `internal/diag/gizmofeature_test.go` | RingBuffer wrap, reader-gate, gizmo 0-alloc, module filter, feature drain. |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-30 | Initial L2 draft — Go translation of l1-diagnostic-system v0.1.0. `DiagnosticsStore` + fixed-capacity `RingBuffer[T]` (0-alloc Push, deterministic averages), `hasAnyReader` run-condition for INV-1 zero-cost, pooled immediate-mode gizmo buffer drained by a `RenderFeature` (INV-2), `Last`-schedule-only systems (INV-3), build-tag `profiling` spans with no-op release path (INV-4), `log/slog` per-module filter, error codes deferred to `l2-error-core-go`. Authored ahead of Phase 6 (`/magic.spec`). Draft — L1 parent Draft + no implementation yet. |
