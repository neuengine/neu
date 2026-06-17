# Profiling Protocol

**Version:** 0.2.0
**Status:** Stable
**Layer:** concept

## Overview

The Profiling Protocol defines a standardized instrumentation layer for the engine. Systems, asset loads, render passes, and physics steps emit tracing spans in a common format. These spans are collected by a pluggable backend — the default targets Tracy (via its C API) for real-time visualization, with fallback exporters for `pprof` (Go-native) and `chrome://tracing` (JSON). All instrumentation is compile-time removable in release builds via `//go:build profile` tags, ensuring zero overhead in production.

## Related Specifications

- [diagnostic-system.md](l1-diagnostic-system.md) — DiagnosticsStore consumes aggregated span timings
- [system-scheduling.md](l1-system-scheduling.md) — Each system execution is wrapped in a span
- [task-system.md](l1-task-system.md) — Worker pool threads carry span context for task attribution
- [render-core.md](l1-render-core.md) — Render passes emit GPU timing spans
- [app-framework.md](l1-app-framework.md) — Profiling plugin registration and build tag isolation

## 1. Motivation

Go's built-in `pprof` captures CPU and memory profiles but lacks the real-time, frame-oriented visualization game developers need. When a frame stutters, the developer needs to see which system spiked, which archetype query was slow, and whether GC paused the main thread — all in a timeline view correlated with the frame boundary.

Without a profiling protocol:

- Each subsystem invents its own timing code, with incompatible output formats.
- Correlating a render stall with a physics spike requires manual log merging.
- Production builds carry instrumentation overhead even when profiling is off.
- Third-party tools (Tracy, Superluminal) cannot be plugged in without engine changes.

## 2. Constraints & Assumptions

- The profiling layer adds zero overhead when compiled without the `profile` build tag.
- Span creation must be allocation-free on the hot path (use sync.Pool for span objects).
- Tracy integration uses CGo for the C client library — behind `//go:build profile && cgo`.
- The `pprof` exporter uses only the Go standard library (no CGo dependency).
- The `chrome://tracing` exporter writes JSON to a file — no network dependency.
- Span context propagates through goroutines via `context.Context`, not thread-local storage.
- GPU timing spans are backend-specific and may not be available on all platforms.

## 3. Core Invariants

- **INV-1**: Profiling overhead is zero in release builds (compile-time exclusion via build tags).
- **INV-2**: Span creation on the hot path allocates zero bytes (pooled span objects).
- **INV-3**: Every frame boundary is marked with a `Frame` span, enabling frame-oriented timeline views.
- **INV-4**: Span nesting is strictly hierarchical — a child span cannot outlive its parent.
- **INV-5**: Exporter backends are pluggable — adding a new exporter does not modify the instrumentation API.

## 4. Detailed Design

### 4.1 Span API

The core instrumentation primitive is a `Span` — a named, timed region of execution:

```plaintext
Span
  name:       string          // human-readable label (e.g., "SystemUpdate:physics_step")
  category:   SpanCategory    // System | Render | Asset | Physics | GC | Custom
  start_ns:   int64           // monotonic nanosecond timestamp
  end_ns:     int64           // set on span.End()
  parent_id:  SpanID          // 0 if root span
  thread_id:  uint64          // OS thread ID for timeline placement
  metadata:   []KeyValue      // optional key-value pairs (entity count, query size, etc.)

SpanCategory:
  System       // ECS system execution
  Render       // render pass, draw call batch
  Asset        // asset load, compilation
  Physics      // physics step, collision detection
  GC           // Go garbage collection pause
  Custom       // user-defined
```

### 4.2 Instrumentation Macros

Since Go lacks macros, instrumentation uses a defer pattern with compile-time elimination:

```plaintext
// In profiling build (//go:build profile):
func BeginSpan(ctx context.Context, name string, cat SpanCategory) (context.Context, *Span)
func (s *Span) End()

// Usage in a system:
func physics_step(ctx context.Context, ...) {
    ctx, span := profiling.BeginSpan(ctx, "physics_step", SpanCategory.Physics)
    defer span.End()
    // ... system logic
}

// In release build (no profile tag):
// BeginSpan and End are replaced by no-op stubs that inline to nothing.
```

### 4.3 Frame Markers

The main loop emits frame boundary markers that profiling tools use to group spans into frames:

```plaintext
Main Loop:
  for !app.ShouldExit() {
      profiling.MarkFrame()      // emits a Frame span boundary
      app.Update()
  }

MarkFrame():
  // Close the previous frame span
  // Open a new frame span with incrementing frame number
  // Tracy: calls FrameMark()
  // chrome://tracing: emits an "I" (instant) event at frame boundary
```

### 4.4 Exporter Backends

```plaintext
ProfileExporter (interface)
  Init()
  EmitSpan(span: Span)
  EmitFrameMark(frame_number: uint64)
  Flush()
  Shutdown()

Implementations:
  TracyExporter       // CGo bridge to Tracy client C API
    - Requires: //go:build profile && cgo
    - Real-time streaming to Tracy profiler GUI
    - Supports: zones, frame marks, plots, memory tracking
    - Zone names are interned (static string pointers)

  PprofExporter       // Maps spans to pprof labels
    - Requires: //go:build profile
    - No CGo dependency — pure Go
    - Writes CPU/memory profiles via runtime/pprof
    - Spans become pprof labels for filtering in go tool pprof

  ChromeExporter      // JSON trace format for chrome://tracing
    - Requires: //go:build profile
    - Writes Trace Event Format (TEF) JSON to file
    - Supports: duration events, async events, instant events
    - File rotation: new file per session, configurable max size
    - Viewable in chrome://tracing or Perfetto UI
```

### 4.5 Automatic Instrumentation

The engine automatically instruments key subsystems without requiring manual span insertion:

```plaintext
Auto-instrumented spans:
  "Frame"                          // main loop frame boundary
  "Schedule:{name}"                // each named schedule execution
  "System:{name}"                  // each system function call
  "Query:{archetype_signature}"    // query iteration (if > threshold entities)
  "RenderPass:{name}"              // each render pass in the render graph
  "AssetLoad:{type}:{path}"        // asset load from disk
  "PhysicsStep"                    // fixed update physics tick
  "GC"                             // Go runtime GC pause (via runtime.SetFinalizer trick)
  "Extract"                        // SubApp data extraction phase

System scheduling integration:
  The DAG scheduler wraps each system dispatch in:
    ctx, span := BeginSpan(ctx, "System:" + system.Name(), SpanCategory.System)
    system.Run(ctx, world)
    span.End()
```

### 4.6 Memory Tracking

The profiling layer optionally tracks allocations per span:

```plaintext
MemoryTracker
  BeforeSpan():  snapshot = runtime.ReadMemStats()
  AfterSpan():   delta = runtime.ReadMemStats() - snapshot
                 span.metadata["alloc_bytes"] = delta.TotalAlloc
                 span.metadata["gc_pauses"]   = delta.NumGC

// Tracy: maps to TracyAlloc/TracyFree for per-zone memory graphs
// pprof: captured natively by heap profile
// chrome: emitted as counter events
```

### 4.7 GPU Timing

Render backends that support timestamp queries can report GPU-side timing:

```plaintext
GPUSpan
  name:       string
  queue:      GPUQueue          // Graphics | Compute | Transfer
  start_tick: uint64            // GPU timestamp counter
  end_tick:   uint64
  frequency:  uint64            // ticks per second (for conversion to ns)

GPUTimingCollector
  BeginQuery(name: string, queue: GPUQueue) -> GPUQueryHandle
  EndQuery(handle: GPUQueryHandle)
  ResolveQueries() -> []GPUSpan  // called once per frame, after GPU work completes
```

GPU spans are correlated with CPU spans via frame number. Tracy displays them on a separate GPU lane in the timeline.

### 4.8 Configuration

```plaintext
ProfilingConfig (resource)
  enabled:            bool              // master switch (also requires build tag)
  exporter:           ExporterType      // Tracy | Pprof | Chrome | Multi
  auto_instrument:    bool              // wrap systems/passes automatically
  memory_tracking:    bool              // track allocations per span
  gpu_timing:         bool              // collect GPU timestamp queries
  chrome_output_path: string            // file path for chrome exporter
  min_span_duration:  Duration          // skip spans shorter than this (noise filter)
  custom_categories:  []string          // user-defined category names
```

### 4.9 pprof Integration

The engine exposes standard Go pprof endpoints for integration with existing Go tooling:

```plaintext
When DiagnosticsPlugin is active:
  - CPU profile:    go tool pprof http://localhost:6060/debug/pprof/profile
  - Heap profile:   go tool pprof http://localhost:6060/debug/pprof/heap
  - Goroutine dump: go tool pprof http://localhost:6060/debug/pprof/goroutine
  - Block profile:  go tool pprof http://localhost:6060/debug/pprof/block

PprofExporter enriches these profiles with span labels:
  - pprof.Do(ctx, pprof.Labels("system", systemName), func(ctx) { ... })
  - Enables filtering: `go tool pprof -focus=physics_step`
```

## 5. Open Questions

- Should the Tracy exporter support Tracy's lock tracking for mutex contention visualization? *(non-blocking — Tracy is ADR-deferred under C32; additive when accepted)*
- Should spans carry a "budget" field (expected duration) for automatic regression detection? *(non-blocking — deferred to future revision)*
- How should GPU timing interact with async compute — separate lanes or merged timeline? *(non-blocking — GPU timing stubbed under ADR; deferred)*
- Should the profiling layer detect and annotate GC STW pauses automatically? *(non-blocking — deferred to future revision)*
- Is there value in a network exporter (UDP streaming) for profiling remote devices? *(non-blocking — deferred to future revision)*

## Canonical References

| Alias | Path | Purpose |
| :--- | :--- | :--- |
| span-api | pkg/diag/profiling/span.go | Span/SpanID types, BeginSpan/EndSpan/EmitSpan API surface |
| profile-on | pkg/diag/profiling/profile_on.go | Live pooled-span machinery (`//go:build profiling`) |
| profile-off | pkg/diag/profiling/profile_off.go | Inlinable no-op stubs for release builds (`//go:build !profiling`) |
| exporter-iface | pkg/diag/profiling/exporter.go | ProfileExporter interface |
| chrome-exporter | pkg/diag/profiling/exporter_chrome.go | Chrome TEF JSON writer |
| pprof-exporter | pkg/diag/profiling/exporter_pprof.go | pprof label aggregator |
| plugin | internal/profiling/plugin.go | ProfilingPlugin — App integration entry point |
| invariant-tests | pkg/diag/profiling/profiling_test.go | 0-alloc invariant + build-tag CI tests |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-03-28 | Initial draft: span API, Tracy/pprof/chrome exporters, auto-instrumentation |
| 0.2.0 | 2026-06-16 | Promoted Draft → Stable: Canonical References populated; open questions annotated non-blocking (Tracy ADR-deferred under C32; GPU timing stubbed). Implementation complete in `pkg/diag/profiling` (dual-tag spans, Chrome TEF + pprof exporters) and `internal/profiling` (ProfilingPlugin). |
