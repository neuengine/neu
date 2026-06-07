package profiling

import (
	"errors"
	"testing"
	"time"
)

func TestSpanCategoryString(t *testing.T) {
	t.Parallel()
	cases := map[SpanCategory]string{
		CategoryCustom:   "custom",
		CategorySystem:   "system",
		CategoryRender:   "render",
		CategoryAsset:    "asset",
		CategoryPhysics:  "physics",
		CategoryGC:       "gc",
		SpanCategory(99): "custom",
	}
	for cat, want := range cases {
		if got := cat.String(); got != want {
			t.Errorf("SpanCategory(%d).String() = %q, want %q", cat, got, want)
		}
	}
}

func TestExporterTypeString(t *testing.T) {
	t.Parallel()
	cases := map[ExporterType]string{
		ExporterNone:     "none",
		ExporterPprof:    "pprof",
		ExporterChrome:   "chrome",
		ExporterMulti:    "multi",
		ExporterType(99): "none",
	}
	for e, want := range cases {
		if got := e.String(); got != want {
			t.Errorf("ExporterType(%d).String() = %q, want %q", e, got, want)
		}
	}
}

func TestSpanAccessorsAndDuration(t *testing.T) {
	t.Parallel()
	s := Span{
		name:     "physics_step",
		id:       7,
		parentID: 3,
		startNs:  1_000,
		endNs:    1_500,
		threadID: 42,
		category: CategoryPhysics,
		metadata: []KeyValue{{Key: "entities", Value: 128}},
	}
	if s.Name() != "physics_step" || s.ID() != 7 || s.ParentID() != 3 || s.ThreadID() != 42 {
		t.Errorf("accessor mismatch: %+v", s)
	}
	if s.Category() != CategoryPhysics || s.StartNanos() != 1_000 || s.EndNanos() != 1_500 {
		t.Errorf("accessor mismatch: %+v", s)
	}
	if got := s.Duration(); got != 500*time.Nanosecond {
		t.Errorf("Duration() = %v, want 500ns", got)
	}
	if md := s.Metadata(); len(md) != 1 || md[0].Key != "entities" {
		t.Errorf("Metadata() = %v", md)
	}
	// An open span (endNs == 0) has zero duration.
	open := Span{startNs: 1_000}
	if open.Duration() != 0 {
		t.Errorf("open span Duration() = %v, want 0", open.Duration())
	}
}

func TestDefaultProfilingConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultProfilingConfig()
	if cfg.Enabled {
		t.Error("default config should be dormant (Enabled=false)")
	}
	if cfg.Exporter != ExporterChrome {
		t.Errorf("default exporter = %v, want chrome", cfg.Exporter)
	}
	if !cfg.AutoInstrument || cfg.MemoryTracking || cfg.GPUTiming {
		t.Errorf("default toggles unexpected: %+v", cfg)
	}
	if cfg.ChromeOutputPath == "" {
		t.Error("default ChromeOutputPath should be set")
	}
}

// countingExporter records calls for MultiExporter fan-out assertions.
type countingExporter struct {
	failOn  string
	spans   int
	frames  int
	inits   int
	flushes int
	shuts   int
}

func (c *countingExporter) Init() error {
	c.inits++
	if c.failOn == "Init" {
		return errors.New("init fail")
	}
	return nil
}
func (c *countingExporter) EmitSpan(Span)        { c.spans++ }
func (c *countingExporter) EmitFrameMark(uint64) { c.frames++ }
func (c *countingExporter) Flush() error {
	c.flushes++
	if c.failOn == "Flush" {
		return errors.New("flush fail")
	}
	return nil
}
func (c *countingExporter) Shutdown() error {
	c.shuts++
	if c.failOn == "Shutdown" {
		return errors.New("shutdown fail")
	}
	return nil
}

func TestMultiExporterFanOut(t *testing.T) {
	t.Parallel()
	a, b := &countingExporter{}, &countingExporter{}
	// nil entries are dropped.
	m := NewMultiExporter(a, nil, b)
	if m.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", m.Len())
	}
	if err := m.Init(); err != nil {
		t.Errorf("Init() error = %v", err)
	}
	m.EmitSpan(Span{name: "x"})
	m.EmitSpan(Span{name: "y"})
	m.EmitFrameMark(1)
	if err := m.Flush(); err != nil {
		t.Errorf("Flush() error = %v", err)
	}
	if err := m.Shutdown(); err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
	for name, e := range map[string]*countingExporter{"a": a, "b": b} {
		if e.spans != 2 || e.frames != 1 || e.inits != 1 || e.flushes != 1 || e.shuts != 1 {
			t.Errorf("exporter %s call counts wrong: %+v", name, e)
		}
	}
}

func TestMultiExporterJoinsErrors(t *testing.T) {
	t.Parallel()
	good := &countingExporter{}
	bad := &countingExporter{failOn: "Init"}
	m := NewMultiExporter(good, bad)
	err := m.Init()
	if err == nil {
		t.Fatal("expected joined error from failing backend")
	}
	// The good backend was still initialised despite the bad one failing.
	if good.inits != 1 {
		t.Errorf("good backend Init not called: %+v", good)
	}
}

func TestNopExporter(t *testing.T) {
	t.Parallel()
	var n NopExporter
	if err := n.Init(); err != nil {
		t.Errorf("Init: %v", err)
	}
	n.EmitSpan(Span{name: "ignored"})
	n.EmitFrameMark(5)
	if err := n.Flush(); err != nil {
		t.Errorf("Flush: %v", err)
	}
	if err := n.Shutdown(); err != nil {
		t.Errorf("Shutdown: %v", err)
	}
}

// TestBeginSpanSafeWhenInactive runs in BOTH builds. Without the profiling tag,
// BeginSpan/End/MarkFrame are no-op stubs; with the tag but the default
// (disabled) config, they short-circuit on the noop span. Either way the API
// must be panic-free and return a usable, non-nil span.
func TestBeginSpanSafeWhenInactive(t *testing.T) {
	t.Parallel()
	ctx, span := BeginSpan(t.Context(), "inactive", CategorySystem)
	if span == nil {
		t.Fatal("BeginSpan returned a nil span")
	}
	span.Annotate("k", 1)
	span.End()
	MarkFrame()
	// The returned context is still usable.
	if ctx == nil {
		t.Fatal("BeginSpan returned a nil context")
	}
}

// TestInactiveZeroAlloc is the INV-1 cost guard. In the default (no-tag) build
// it measures the no-op stubs; in the profiling build it measures the
// disabled-config short-circuit. Both return the shared noop span with the
// context unchanged, so the full BeginSpan→End→MarkFrame sequence must allocate
// nothing — proving instrumentation is free when profiling is off.
func TestInactiveZeroAlloc(t *testing.T) {
	ctx := t.Context()
	allocs := testing.AllocsPerRun(1000, func() {
		c, s := BeginSpan(ctx, "hot", CategorySystem)
		s.End()
		MarkFrame()
		_ = c
	})
	if allocs != 0 {
		t.Errorf("inactive profiling allocated %.1f times/op, want 0 (INV-1)", allocs)
	}
}
