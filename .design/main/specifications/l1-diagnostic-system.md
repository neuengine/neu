# Diagnostic System

**Version:** 0.1.0
**Status:** Stable
**Layer:** concept

## Overview

The diagnostic system provides runtime introspection tools for development and profiling. A `DiagnosticsStore` resource collects named metrics with rolling-window history. Built-in diagnostics cover FPS, frame time, entity counts, and per-system timing. Gizmos offer immediate-mode debug drawing as a visual overlay. Performance profiling wraps systems in tracing spans compatible with external tools. A structured log system provides per-module filtering. All diagnostic features are designed to be zero-cost when disabled or when no consumers are registered.

## Related Specifications

- [system-scheduling.md](l1-system-scheduling.md) — Diagnostic systems run in the Last schedule
- [render-core.md](l1-render-core.md) — Gizmo rendering and debug overlay pass
- [app-framework.md](l1-app-framework.md) — Plugin registration for diagnostic features
- [network-diagnostics.md](l1-network-diagnostics.md) — Networking-specific metrics, alerts, and debug overlay built on DiagnosticsStore

## 1. Motivation

Without instrumentation, diagnosing performance regressions or logic errors in a game is guesswork. A first-party diagnostic layer ensures every project has baseline observability from day one:

- Frame time spikes need to be detected and attributed to specific systems.
- Spatial relationships and collision shapes need visual debugging.
- Entity counts and resource usage must be trackable over time.
- Structured error codes make engine errors searchable and actionable.

## 2. Constraints & Assumptions

- Diagnostic overhead must be negligible when no readers are registered (zero-cost principle).
- All diagnostic systems run in the `Last` schedule and never affect gameplay system ordering.
- Gizmo drawing is purely visual and has no effect on game state.
- Profiling spans are compile-time removable in release builds via build tags.
- The profiling layer emits standard tracing spans consumable by external tools; the engine does not bundle its own profiler UI.

## 3. Core Invariants

- **INV-1**: Diagnostics collection has zero overhead when no readers are registered.
- **INV-2**: Gizmo drawing never affects game state (pure visual overlay).
- **INV-3**: All diagnostic systems are in the Last schedule and never affect gameplay systems.
- **INV-4**: Profiling spans are compile-time removable in release builds.

## 4. Detailed Design

### 4.1 DiagnosticsStore Resource

A global resource holding all registered diagnostics:

```
DiagnosticsStore {
    Metrics: map[DiagnosticPath]Diagnostic
}
```

Systems access it as a resource: `ResMut[DiagnosticsStore]`. Thread-safe for concurrent reads; writes are serialized by system scheduling.

### 4.2 Diagnostic

A named measurement with a rolling history buffer:

```
Diagnostic {
    Path:        DiagnosticPath   // e.g., "engine/fps", "engine/frame_time"
    Suffix:      string           // Unit label: "fps", "ms", "count"
    History:     RingBuffer[DiagnosticEntry]
    MaxHistory:  int              // Default 120 entries
    IsEnabled:   bool
}

DiagnosticEntry {
    Value:     float64
    Timestamp: Time
}
```

Provides computed accessors:

- `Average()` — Mean over the history buffer.
- `SmoothedAverage()` — Exponentially weighted moving average.
- `Min()`, `Max()` — Extremes within the buffer.
- `Latest()` — Most recent entry.

A diagnostic metric path is unique; registering a duplicate path is a warning, not a crash. Rolling-window averages use a fixed sample count, not wall-clock time, so they are deterministic under replay.

### 4.3 Built-in Diagnostics

Registered automatically by the DiagnosticsPlugin:

| Metric | Path | Source |
| :--- | :--- | :--- |
| Frame time | `engine/frame_time` | Wall-clock delta between frames |
| Frames per second | `engine/fps` | Computed as 1/delta |
| Entity count | `engine/entity_count` | World entity metadata |
| System CPU time | `engine/system/{name}` | Scheduler tracing spans |
| System Allocations | `engine/system/{name}/allocs` | `runtime.MemStats` before/after system |
| Thread Idle Time | `engine/worker/idle_time` | TaskPool worker telemetry |
| Goroutine Leaks | `engine/worker/goroutine_leaks` | `runtime/pprof` goroutineleak profile (Go 1.26+) |
| CPU Throttling | `engine/hw/throttling` | Platform-specific cooling/freq metrics |

### 4.4 Custom Diagnostics

Users register new diagnostics and push values each frame:

```
store.Register(Diagnostic{
    Path:       "game/enemies_alive",
    Suffix:     "count",
    MaxHistory: 60,
})

store.Push("game/enemies_alive", float64(len(enemies)))
```

Custom diagnostics behave identically to built-in ones — same history buffer, same accessors.

### 4.5 LogDiagnosticsPlugin

Periodically (default every 2 seconds) logs all enabled diagnostics to the console in a formatted table. Configurable filter by path prefix and output interval.

### 4.6 Gizmos — Immediate Mode

The `Gizmos` system parameter provides drawing commands:

```
Gizmos interface {
    Line(start, end Vec3, color Color)
    Ray(origin Vec3, direction Vec3, color Color)
    Circle(center Vec3, normal Vec3, radius float32, color Color)
    Sphere(center Vec3, radius float32, color Color)
    Box(center Vec3, halfExtents Vec3, color Color)
    Arrow(start, end Vec3, color Color)
    Grid(origin Vec3, normal Vec3, cellSize float32, count int, color Color)
    Text(position Vec3, text string, color Color)
}
```

All calls append vertices to a per-frame buffer. A dedicated render pass draws this buffer as line-list or triangle-list geometry. Gizmo geometry is rendered after the main scene but before UI. Calls issued in frame N are visible in frame N and discarded before frame N+1.

### 4.7 GizmoConfigStore

```
GizmoConfigStore {
    Groups: map[GizmoGroupId]GizmoConfig
}

GizmoConfig {
    Enabled:    bool
    LineWidth:  float32
    Color:      Color       // default color for the group
    DepthBias:  float32
    DepthTest:  bool        // whether gizmos are occluded by scene geometry
}
```

Multiple gizmo groups (e.g., "physics", "navigation", "custom") can be toggled independently at runtime. Toggling a group hides or shows all its gizmos without changing calling code.

### 4.8 Retained Gizmos

For geometry that should persist across frames (e.g., a navigation mesh overlay), a `RetainedGizmo` asset holds the vertex data. It is drawn every frame until the asset handle is dropped. Updates replace the entire vertex buffer.

### 4.9 Performance Profiling

Every system execution is wrapped in a tracing span:

```
span := profiling.StartSpan("MovementSystem")
defer span.End()
```

- **Build tags**: `//go:build profiling` — all span calls compile to no-ops without this tag (INV-4).
- **Tracy integration**: Spans can be exported to Tracy-compatible format for visual profiling.
- **Chrome tracing**: JSON trace event format for chrome://tracing.
- **Runtime toggle**: Profiling can be enabled/disabled at runtime when compiled with the profiling tag.
- Schedule-level spans wrap entire schedule runs in addition to individual systems.

### 4.10 Debug Overlay

An optional on-screen text overlay showing key metrics:

- FPS counter (top-left by default).
- Entity count.
- Per-system timing breakdown.
- Toggled with a configurable key (F3 by default) or programmatically.

The overlay is rendered using the UI system's text facilities but managed by the diagnostic plugin.

### 4.11 Log System

Structured logging with severity levels:

```
LogLevel:
    Error   // Unrecoverable or critical failures
    Warn    // Recoverable issues or deprecated usage
    Info    // Noteworthy runtime events
    Debug   // Developer-oriented detail
    Trace   // Fine-grained execution flow
```

- **Per-module filters**: `log.SetFilter("render", LogLevel.Debug)` — configure verbosity per subsystem.
- **Structured fields**: `log.Info("entity spawned", "id", entity.ID, "archetype", archetype.Name)`.
- **Output sinks**: Console (default), file, custom sink interface.
- Default level is Info; configurable at startup and runtime.

### 4.12 Error Code Registry

Engine errors use structured codes:

```
E0001: Duplicate system name "{name}" in schedule "{schedule}".
       Each system must have a unique name within its schedule.
       Solution: Rename one of the conflicting systems.
```

The registry maps codes to descriptions and suggested solutions. A `--explain E0001` CLI flag prints the full entry. User plugins can register their own code ranges. Error codes are globally unique and monotonically increasing; retired codes are never reused.

### 4.13 Debug Stepping

When enabled, the scheduler pauses before each system and waits for a "step" input (keyboard shortcut or API call). A UI overlay shows the current system name, its dependencies, and the schedule graph position. This mode is intended for local debugging only and does not alter system execution order.

## 5. Open Questions

- Should gizmo persistence (draw for N frames) be supported beyond retained gizmos?
- How should diagnostics be exposed in shipping builds for telemetry purposes without the full dev overhead?
- Should the profiling system support GPU timing queries in addition to CPU spans?
- Can the error code registry be generated from source annotations at build time?
- How should diagnostic data be exposed for external tools (e.g., a web dashboard)?

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
| 0.1.0 | 2026-03-25 | Initial draft |
| — | — | Planned examples: `examples/diagnostic/` |
