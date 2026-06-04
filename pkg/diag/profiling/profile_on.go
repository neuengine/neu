//go:build profiling

package profiling

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// procStart anchors the monotonic clock. Timestamps are nanoseconds since
// process start, which keeps Chrome-trace timestamps small and positive.
var procStart = time.Now()

// nowNanos returns monotonic nanoseconds since process start. time.Since reads
// the monotonic clock, so spans are immune to wall-clock adjustments.
func nowNanos() int64 { return int64(time.Since(procStart)) }

// spanPool recycles *Span objects so span creation is allocation-free on the
// hot path after warm-up (INV-2). End returns the span here after the exporter
// has copied its value.
var spanPool = sync.Pool{New: func() any { return new(Span) }}

// noopSpan is returned when profiling is built in but disabled at runtime. It
// is never pooled; End/Annotate short-circuit on it.
var noopSpan = new(Span)

var (
	nextID         atomic.Uint64
	frameNum       atomic.Uint64
	activeExporter ProfileExporter = NopExporter{}
	activeConfig                   = DefaultProfilingConfig()
	threadIDFunc                   = func() uint64 { return 0 }
)

// ctxKey is the private context key carrying the current span ID for parent
// linkage. A struct key avoids collisions with other context values.
type ctxKey struct{}

// SetExporter installs the active span sink. Called once by ProfilingPlugin at
// build time. A nil exporter is ignored (the NopExporter remains).
func SetExporter(e ProfileExporter) {
	if e != nil {
		activeExporter = e
	}
}

// SetConfig installs the runtime configuration (Enabled gate, MinSpanDuration
// filter, exporter selection).
func SetConfig(c ProfilingConfig) { activeConfig = c }

// SetThreadIDFunc installs a timeline-lane identifier source. The default
// returns 0 (single lane); a host with a goroutine/thread-id mechanism may
// inject a real one. Kept injectable because pure Go exposes no allocation-free
// thread id (C-003 forbids a goid dependency).
func SetThreadIDFunc(f func() uint64) {
	if f != nil {
		threadIDFunc = f
	}
}

// reset clears a span for reuse from the pool. metadata's backing array is
// retained (truncated) so a recycled span does not pin annotation values.
func (s *Span) reset() {
	clear(s.metadata)
	s.metadata = s.metadata[:0]
	s.name = ""
	s.id = 0
	s.parentID = 0
	s.startNs = 0
	s.endNs = 0
	s.threadID = 0
	s.category = CategoryCustom
}

func spanFromContext(ctx context.Context) SpanID {
	if v := ctx.Value(ctxKey{}); v != nil {
		return v.(SpanID)
	}
	return 0
}

// BeginSpan opens a span named `name` of category `cat`, parented to whatever
// span is carried by ctx. It returns a child context (so nested BeginSpan calls
// link correctly — INV-4) and the span. The canonical usage is:
//
//	ctx, span := profiling.BeginSpan(ctx, "physics_step", profiling.CategoryPhysics)
//	defer span.End()
//
// When profiling is built in but disabled (config.Enabled == false) it returns
// ctx unchanged and the shared noop span — no pool traffic.
//
// Allocation note (INV-2): the span object itself is pooled and reused
// (zero-alloc). The returned child context costs one small context.WithValue
// allocation per nested span — the one unavoidable cost of context-based
// hierarchy in pure Go. The span payload churn that C27 guards is zero-alloc.
func BeginSpan(ctx context.Context, name string, cat SpanCategory) (context.Context, *Span) {
	if !activeConfig.Enabled {
		return ctx, noopSpan
	}
	s := spanPool.Get().(*Span)
	s.reset()
	s.name = name
	s.category = cat
	s.id = SpanID(nextID.Add(1))
	s.parentID = spanFromContext(ctx)
	s.threadID = threadIDFunc()
	s.startNs = nowNanos()
	return context.WithValue(ctx, ctxKey{}, s.id), s
}

// End closes the span, emits it to the active exporter by value, and returns it
// to the pool. Spans shorter than MinSpanDuration are dropped (noise filter)
// without reaching the exporter. End on the noop span is a no-op.
func (s *Span) End() {
	if s == noopSpan {
		return
	}
	s.endNs = nowNanos()
	if d := activeConfig.MinSpanDuration; d > 0 && time.Duration(s.endNs-s.startNs) < d {
		spanPool.Put(s)
		return
	}
	activeExporter.EmitSpan(*s) // value copy — pooled span is recycled below
	spanPool.Put(s)
}

// Annotate appends a key/value annotation to the span (entity count, query
// size, etc.). No-op on the noop span. The annotation aliases pooled memory and
// is valid only until End.
func (s *Span) Annotate(key string, value any) {
	if s == noopSpan {
		return
	}
	s.metadata = append(s.metadata, KeyValue{Key: key, Value: value})
}

// MarkFrame records a frame boundary, incrementing the frame counter and
// emitting a frame mark to the exporter (INV-3). Driven once per frame by the
// ProfilingPlugin's Last-schedule system. No-op while disabled.
func MarkFrame() {
	if !activeConfig.Enabled {
		return
	}
	activeExporter.EmitFrameMark(frameNum.Add(1))
}

// FrameNumber returns the current frame counter (for exporters and tests).
func FrameNumber() uint64 { return frameNum.Load() }
