//go:build profiling

package profiling

import (
	"context"
	"testing"
	"time"
)

// captureExporter records emitted spans and frame marks by value for assertions.
type captureExporter struct {
	spans  []Span
	frames []uint64
}

func (c *captureExporter) Init() error            { return nil }
func (c *captureExporter) EmitSpan(s Span)        { c.spans = append(c.spans, s) }
func (c *captureExporter) EmitFrameMark(f uint64) { c.frames = append(c.frames, f) }
func (c *captureExporter) Flush() error           { return nil }
func (c *captureExporter) Shutdown() error        { return nil }

// activate installs an enabled config + capture exporter and restores the
// previous globals on cleanup. Tests touching package globals must not run in
// parallel.
func activate(t *testing.T, cfg ProfilingConfig) *captureExporter {
	t.Helper()
	prevCfg, prevExp := activeConfig, activeExporter
	cap := &captureExporter{}
	SetConfig(cfg)
	SetExporter(cap)
	t.Cleanup(func() {
		activeConfig = prevCfg
		activeExporter = prevExp
	})
	return cap
}

func enabledConfig() ProfilingConfig {
	cfg := DefaultProfilingConfig()
	cfg.Enabled = true
	return cfg
}

func TestBeginEndEmitsSpan(t *testing.T) {
	cap := activate(t, enabledConfig())
	ctx, span := BeginSpan(context.Background(), "load", CategoryAsset)
	if span == noopSpan {
		t.Fatal("enabled BeginSpan returned the noop span")
	}
	if ctx == context.Background() {
		t.Error("enabled BeginSpan should return a child context")
	}
	span.Annotate("bytes", 4096)
	span.End()
	if len(cap.spans) != 1 {
		t.Fatalf("got %d emitted spans, want 1", len(cap.spans))
	}
	got := cap.spans[0]
	if got.Name() != "load" || got.Category() != CategoryAsset {
		t.Errorf("emitted span = %+v", got)
	}
	if got.EndNanos() < got.StartNanos() {
		t.Errorf("end %d before start %d", got.EndNanos(), got.StartNanos())
	}
	if md := got.Metadata(); len(md) != 1 || md[0].Key != "bytes" {
		t.Errorf("metadata = %v", md)
	}
}

func TestParentLinkage(t *testing.T) {
	cap := activate(t, enabledConfig())
	ctx, parent := BeginSpan(context.Background(), "outer", CategorySystem)
	_, child := BeginSpan(ctx, "inner", CategorySystem)
	child.End()
	parent.End()
	// child emitted first (ended first); its parentID must equal parent.id.
	if len(cap.spans) != 2 {
		t.Fatalf("got %d spans, want 2", len(cap.spans))
	}
	innerEmit, outerEmit := cap.spans[0], cap.spans[1]
	if innerEmit.Name() != "inner" || outerEmit.Name() != "outer" {
		t.Fatalf("emit order wrong: %q then %q", innerEmit.Name(), outerEmit.Name())
	}
	if innerEmit.ParentID() != outerEmit.ID() {
		t.Errorf("child parentID = %d, want parent id %d", innerEmit.ParentID(), outerEmit.ID())
	}
	if outerEmit.ParentID() != 0 {
		t.Errorf("root span parentID = %d, want 0", outerEmit.ParentID())
	}
}

func TestMinSpanDurationFilter(t *testing.T) {
	cfg := enabledConfig()
	cfg.MinSpanDuration = time.Hour // nothing is this long → all filtered
	cap := activate(t, cfg)
	_, span := BeginSpan(context.Background(), "tiny", CategoryCustom)
	span.End()
	if len(cap.spans) != 0 {
		t.Errorf("sub-threshold span was emitted: %d", len(cap.spans))
	}
}

func TestMarkFrameEmits(t *testing.T) {
	cap := activate(t, enabledConfig())
	start := FrameNumber()
	MarkFrame()
	MarkFrame()
	if len(cap.frames) != 2 {
		t.Fatalf("got %d frame marks, want 2", len(cap.frames))
	}
	if cap.frames[1] != cap.frames[0]+1 {
		t.Errorf("frame numbers not monotonic: %v", cap.frames)
	}
	if FrameNumber() != start+2 {
		t.Errorf("FrameNumber = %d, want %d", FrameNumber(), start+2)
	}
}

func TestDisabledShortCircuits(t *testing.T) {
	cap := activate(t, DefaultProfilingConfig()) // Enabled=false
	ctx, span := BeginSpan(context.Background(), "x", CategorySystem)
	if span != noopSpan {
		t.Error("disabled BeginSpan should return the noop span")
	}
	if ctx != context.Background() {
		t.Error("disabled BeginSpan should return ctx unchanged")
	}
	span.End()
	MarkFrame()
	if len(cap.spans) != 0 || len(cap.frames) != 0 {
		t.Errorf("disabled profiling emitted: spans=%d frames=%d", len(cap.spans), len(cap.frames))
	}
}

// TestSpanPoolZeroAlloc proves the span-payload path (Get → reset → emit value
// → Put) is allocation-free (INV-2 / C27). It measures End on a pre-acquired
// pooled span plus a no-op exporter, isolating the pool mechanics from the
// context.WithValue parent-link allocation measured separately below.
func TestSpanPoolZeroAlloc(t *testing.T) {
	cfg := enabledConfig()
	prevCfg, prevExp := activeConfig, activeExporter
	SetConfig(cfg)
	SetExporter(NopExporter{})
	t.Cleanup(func() { activeConfig = prevCfg; activeExporter = prevExp })

	// Warm the pool.
	for range 4 {
		s := spanPool.Get().(*Span)
		s.reset()
		s.startNs = nowNanos()
		s.endNs = nowNanos()
		activeExporter.EmitSpan(*s)
		spanPool.Put(s)
	}
	allocs := testing.AllocsPerRun(1000, func() {
		s := spanPool.Get().(*Span)
		s.reset()
		s.name = "hot"
		s.category = CategorySystem
		s.startNs = nowNanos()
		s.endNs = nowNanos()
		activeExporter.EmitSpan(*s) // value copy, NopExporter discards
		spanPool.Put(s)
	})
	if allocs != 0 {
		t.Errorf("span pool path allocated %.1f times/op, want 0 (INV-2)", allocs)
	}
}

// TestBeginEndBoundedAlloc documents that the full BeginSpan→End round-trip
// costs at most one allocation — the context.WithValue parent-link wrapper —
// not the span payload. This pins exactly where the single allocation lives.
func TestBeginEndBoundedAlloc(t *testing.T) {
	cfg := enabledConfig()
	prevCfg, prevExp := activeConfig, activeExporter
	SetConfig(cfg)
	SetExporter(NopExporter{})
	t.Cleanup(func() { activeConfig = prevCfg; activeExporter = prevExp })

	ctx := context.Background()
	allocs := testing.AllocsPerRun(1000, func() {
		_, s := BeginSpan(ctx, "hot", CategorySystem)
		s.End()
	})
	if allocs > 1 {
		t.Errorf("BeginSpan→End allocated %.1f times/op, want ≤1 (context wrapper only)", allocs)
	}
}
