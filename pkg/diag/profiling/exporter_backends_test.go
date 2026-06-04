//go:build profiling

package profiling

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestChromeExporterTEF(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	c := NewChromeExporter(&buf)
	if err := c.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	// Span: 1000ns..3500ns → ts=1us, dur=2us; with one annotation.
	c.EmitSpan(Span{
		name:     "physics_step",
		category: CategoryPhysics,
		startNs:  1_000,
		endNs:    3_500,
		threadID: 2,
		metadata: []KeyValue{{Key: "entities", Value: 128}},
	})
	c.EmitFrameMark(7)
	if err := c.Shutdown(); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	var events []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &events); err != nil {
		t.Fatalf("output is not valid TEF JSON: %v\n%s", err, buf.String())
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	span := events[0]
	if span["name"] != "physics_step" || span["cat"] != "physics" || span["ph"] != "X" {
		t.Errorf("span event = %v", span)
	}
	if span["ts"] != float64(1) || span["dur"] != float64(2) {
		t.Errorf("ts/dur = %v/%v, want 1/2 microseconds", span["ts"], span["dur"])
	}
	if args, ok := span["args"].(map[string]any); !ok || args["entities"] != float64(128) {
		t.Errorf("args = %v", span["args"])
	}
	frame := events[1]
	if frame["ph"] != "i" || frame["name"] != "Frame 7" || frame["s"] != "g" {
		t.Errorf("frame event = %v", frame)
	}
}

func TestPprofExporterAggregates(t *testing.T) {
	t.Parallel()
	p := NewPprofExporter()
	_ = p.Init()
	p.EmitSpan(Span{name: "sys", startNs: 0, endNs: 100})
	p.EmitSpan(Span{name: "sys", startNs: 0, endNs: 300})
	p.EmitFrameMark(1) // no-op for pprof
	_ = p.Flush()
	_ = p.Shutdown()

	st, ok := p.Stat("sys")
	if !ok {
		t.Fatal("Stat(sys) not found")
	}
	if st.Count != 2 || st.TotalNs != 400 {
		t.Errorf("stat = %+v, want count=2 total=400", st)
	}
	if st.Mean() != 200 {
		t.Errorf("Mean = %v, want 200ns", st.Mean())
	}
	if _, ok := p.Stat("missing"); ok {
		t.Error("Stat(missing) should be absent")
	}
	// Empty stat Mean is zero (no divide-by-zero).
	if (PprofStat{}).Mean() != 0 {
		t.Error("empty PprofStat.Mean should be 0")
	}
}
