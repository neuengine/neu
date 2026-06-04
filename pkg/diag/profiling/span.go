// Package profiling defines the engine's instrumentation protocol: hierarchical,
// frame-oriented timing spans emitted to pluggable exporter backends.
//
// The package is split by build tag. This file and its siblings (config.go,
// exporter.go) are UNTAGGED — they compile in every build so that the data
// types (Span, SpanCategory, ProfilingConfig, ProfileExporter) remain
// referenceable even in release binaries. The live instrumentation API
// (BeginSpan, End, MarkFrame) has two implementations selected at compile time:
//
//   - profile_on.go  (//go:build profiling)  — pooled spans, context parent
//     chain, real exporter dispatch.
//   - profile_off.go (//go:build !profiling) — inlinable no-op stubs, so a
//     release build links zero instrumentation cost (INV-1).
//
// Per the engine's zero-dependency rule (C-003 / RULES.md C32) only stdlib
// exporters ship: a runtime/pprof label bridge and a chrome://tracing (Trace
// Event Format) JSON writer. The Tracy CGo backend named by the L1 spec is
// rejected as a third-party dependency and left as a drop-in behind the
// ProfileExporter interface (INV-5).
//
// Build-tag note: the engine's existing span seed in package diag uses the
// `profiling` tag; this package follows that convention rather than the `profile`
// tag the draft L2 spec used. [Bootstrap]
package profiling

import "time"

// SpanID identifies a span within a process run. Zero is reserved for "no
// parent" (a root span). IDs are assigned monotonically by BeginSpan.
type SpanID uint64

// SpanCategory classifies a span for timeline grouping and exporter colouring.
type SpanCategory uint8

const (
	// CategoryCustom is the default for user-defined spans and frame markers.
	CategoryCustom SpanCategory = iota
	// CategorySystem marks an ECS system execution.
	CategorySystem
	// CategoryRender marks a render pass or draw-call batch.
	CategoryRender
	// CategoryAsset marks an asset load or compilation.
	CategoryAsset
	// CategoryPhysics marks a physics step or collision pass.
	CategoryPhysics
	// CategoryGC marks a Go garbage-collection pause.
	CategoryGC
)

// String returns the canonical lowercase category name used by exporters.
func (c SpanCategory) String() string {
	switch c {
	case CategorySystem:
		return "system"
	case CategoryRender:
		return "render"
	case CategoryAsset:
		return "asset"
	case CategoryPhysics:
		return "physics"
	case CategoryGC:
		return "gc"
	case CategoryCustom:
		return "custom"
	default:
		return "custom"
	}
}

// KeyValue is an optional annotation attached to a span (entity count, query
// size, allocation delta, etc.).
type KeyValue struct {
	Key   string
	Value any
}

// Span is a named, timed region of execution. It is pure data: the live
// instrumentation path in profile_on.go mutates the unexported fields, while
// exporters read the accessors below. A Span is emitted to the exporter by
// value (never by pointer) so the pooled instance can be recycled immediately
// after End — exporters that buffer must copy what they need.
type Span struct {
	metadata []KeyValue
	name     string
	id       SpanID
	parentID SpanID
	startNs  int64
	endNs    int64
	threadID uint64
	category SpanCategory
}

// Name returns the span's human-readable label.
func (s *Span) Name() string { return s.name }

// Category returns the span's classification.
func (s *Span) Category() SpanCategory { return s.category }

// ID returns the span's identifier.
func (s *Span) ID() SpanID { return s.id }

// ParentID returns the enclosing span's ID, or 0 for a root span.
func (s *Span) ParentID() SpanID { return s.parentID }

// ThreadID returns the timeline-lane identifier captured at BeginSpan.
func (s *Span) ThreadID() uint64 { return s.threadID }

// Metadata returns the span's annotations. The slice aliases the span's
// backing array; callers that retain it past the span's recycle must copy.
func (s *Span) Metadata() []KeyValue { return s.metadata }

// StartNanos returns the span's start timestamp in monotonic nanoseconds.
func (s *Span) StartNanos() int64 { return s.startNs }

// EndNanos returns the span's end timestamp, or 0 if the span is still open.
func (s *Span) EndNanos() int64 { return s.endNs }

// Duration returns the elapsed time between BeginSpan and End. It is zero for a
// span that has not yet ended.
func (s *Span) Duration() time.Duration {
	if s.endNs == 0 {
		return 0
	}
	return time.Duration(s.endNs - s.startNs)
}
