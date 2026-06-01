package diag

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	pkgmath "github.com/neuengine/neu/pkg/math"
)

func TestRingBufferWrapAndOrder(t *testing.T) {
	t.Parallel()
	r := NewRingBuffer[int](3)
	if r.Cap() != 3 || r.Len() != 0 {
		t.Fatalf("new ring: cap=%d len=%d", r.Cap(), r.Len())
	}
	for i := 1; i <= 5; i++ { // 4,5 overwrite 1,2 → buffer holds 3,4,5
		r.Push(i)
	}
	if r.Len() != 3 {
		t.Fatalf("len after overflow = %d, want 3", r.Len())
	}
	var got []int
	r.ForEach(func(v int) { got = append(got, v) })
	if len(got) != 3 || got[0] != 3 || got[2] != 5 {
		t.Errorf("ForEach order = %v, want [3 4 5]", got)
	}
	if last, ok := r.Latest(); !ok || last != 5 {
		t.Errorf("Latest = %d,%v want 5,true", last, ok)
	}
}

func TestRingBufferEmpty(t *testing.T) {
	t.Parallel()
	r := NewRingBuffer[int](0) // clamped to 1
	if r.Cap() != 1 {
		t.Errorf("zero cap clamped to %d, want 1", r.Cap())
	}
	if _, ok := r.Latest(); ok {
		t.Error("Latest on empty should be false")
	}
	r.ForEach(func(int) { t.Error("ForEach on empty must not call fn") })
}

func TestDiagnosticStats(t *testing.T) {
	t.Parallel()
	d := NewDiagnostic("engine/test", "ms", 10)
	for _, v := range []float64{10, 20, 30} {
		d.record(v, 0)
	}
	if avg := d.Average(); avg != 20 {
		t.Errorf("Average = %v, want 20", avg)
	}
	if d.Min() != 10 || d.Max() != 30 {
		t.Errorf("Min/Max = %v/%v, want 10/30", d.Min(), d.Max())
	}
	if e, ok := d.Latest(); !ok || e.Value != 30 {
		t.Errorf("Latest = %+v,%v", e, ok)
	}
	// Smoothed lies between samples, biased toward the latest.
	if s := d.SmoothedAverage(); s <= 10 || s >= 30 {
		t.Errorf("SmoothedAverage = %v, expected within (10,30)", s)
	}
}

func TestDiagnosticEmptyStats(t *testing.T) {
	t.Parallel()
	d := NewDiagnostic("empty", "", 0) // default history
	if d.Average() != 0 || d.Min() != 0 || d.Max() != 0 {
		t.Error("empty diagnostic stats should be 0")
	}
	if d.history.Cap() != DefaultHistory {
		t.Errorf("default history = %d, want %d", d.history.Cap(), DefaultHistory)
	}
}

func TestStoreReaderGate(t *testing.T) {
	t.Parallel()
	s := NewDiagnosticsStore()
	d := NewDiagnostic("game/enemies", "count", 5)
	s.Register(d)

	// INV-1: no reader → Push is a no-op.
	s.Push("game/enemies", 7)
	if d.Len() != 0 {
		t.Fatalf("Push with no reader recorded %d samples, want 0", d.Len())
	}
	if s.HasAnyReader() {
		t.Error("HasAnyReader should be false before AddReader")
	}

	// With a reader, Push records.
	s.AddReader("game/enemies")
	if !s.HasAnyReader() {
		t.Error("HasAnyReader should be true after AddReader")
	}
	s.Push("game/enemies", 7)
	if d.Len() != 1 {
		t.Errorf("Push with reader recorded %d, want 1", d.Len())
	}

	// Disabled → no-op even with a reader.
	s.SetEnabled("game/enemies", false)
	s.Push("game/enemies", 9)
	if d.Len() != 1 {
		t.Errorf("Push while disabled recorded; len=%d want 1", d.Len())
	}

	// Unregistered path → no-op, no panic.
	s.Push("does/not/exist", 1)
}

func TestStoreDuplicateRegister(t *testing.T) {
	t.Parallel()
	s := NewDiagnosticsStore()
	s.Register(NewDiagnostic("dup", "", 5))
	s.Register(NewDiagnostic("dup", "", 5)) // warned + ignored
	if s.Len() != 1 {
		t.Errorf("store Len = %d after duplicate, want 1", s.Len())
	}
}

func TestGizmoBufferGeometry(t *testing.T) {
	t.Parallel()
	b := NewGizmoBuffer()
	white := pkgmath.LinearRgba{R: 1, G: 1, B: 1, A: 1}

	b.Line(pkgmath.Vec3{}, pkgmath.Vec3{X: 1}, white)
	if len(b.Lines()) != 2 {
		t.Errorf("Line → %d verts, want 2", len(b.Lines()))
	}
	b.Reset()

	b.Box(pkgmath.Vec3{}, pkgmath.Vec3{X: 1, Y: 1, Z: 1}, white)
	if len(b.Lines()) != 24 { // 12 edges × 2 verts
		t.Errorf("Box → %d verts, want 24", len(b.Lines()))
	}
	b.Reset()

	b.Sphere(pkgmath.Vec3{}, 1, white)
	if len(b.Lines()) != 3*circleSegments*2 {
		t.Errorf("Sphere → %d verts, want %d", len(b.Lines()), 3*circleSegments*2)
	}
	b.Reset()

	b.Ray(pkgmath.Vec3{}, pkgmath.Vec3{Y: 2}, white)
	b.Arrow(pkgmath.Vec3{}, pkgmath.Vec3{X: 3}, white)
	b.Grid(pkgmath.Vec3{}, pkgmath.Vec3{Y: 1}, 1, 2, white)
	if len(b.Lines()) == 0 {
		t.Error("Ray/Arrow/Grid produced no geometry")
	}

	b.Text(pkgmath.Vec3{Y: 1}, "fps: 60", white)
	if len(b.Texts()) != 1 || b.Texts()[0].Text != "fps: 60" {
		t.Errorf("Text not recorded: %+v", b.Texts())
	}

	b.Reset()
	if len(b.Lines()) != 0 || len(b.Texts()) != 0 {
		t.Error("Reset should clear lines and texts")
	}
}

func TestGizmoBufferResetReuse(t *testing.T) {
	// Not parallel: testing.AllocsPerRun must not run during parallel tests.
	b := NewGizmoBuffer()
	white := pkgmath.LinearRgba{A: 1}
	for range 50 { // warm capacity
		b.Box(pkgmath.Vec3{}, pkgmath.Vec3{X: 1, Y: 1, Z: 1}, white)
	}
	b.Reset()
	allocs := testing.AllocsPerRun(20, func() {
		b.Reset()
		for range 50 {
			b.Box(pkgmath.Vec3{}, pkgmath.Vec3{X: 1, Y: 1, Z: 1}, white)
		}
	})
	if allocs > 0 {
		t.Errorf("Reset+refill allocated %v/op, want 0 (capacity reuse)", allocs)
	}
}

func TestModuleFilterHandler(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	base := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: LevelTrace})
	h := NewModuleFilterHandler(base, slog.LevelInfo)
	h.SetModuleLevel("render", slog.LevelDebug)
	log := slog.New(h)

	log.Debug("render detail", "module", "render")   // passes (render→Debug)
	log.Debug("physics detail", "module", "physics") // dropped (default Info)
	log.Info("general")                              // passes (no module, Info >= default Info)

	out := buf.String()
	if !strings.Contains(out, "render detail") {
		t.Error("render Debug record should pass the filter")
	}
	if strings.Contains(out, "physics detail") {
		t.Error("physics Debug record should be dropped by the default Info filter")
	}
	if !strings.Contains(out, "general") {
		t.Error("module-less Info record should pass")
	}
}

func TestSpanNoop(t *testing.T) {
	t.Parallel()
	// Default (non-profiling) build: StartSpan/End must be safe no-ops.
	s := StartSpan("region")
	s.End()
}
