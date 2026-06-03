package main

import (
	"testing"

	"github.com/neuengine/neu/internal/grapheditor"
	vg "github.com/neuengine/neu/pkg/visualgraph"
)

// TestEditorHashStable asserts the end-to-end validation is deterministic across
// ≥20 runs — the invariant cmd/examplecheck relies on (C29 P6 gate).
func TestEditorHashStable(t *testing.T) {
	t.Parallel()
	first, err := run()
	if err != nil {
		t.Fatalf("run(): %v", err)
	}
	for i := range 20 {
		got, err := run()
		if err != nil {
			t.Fatalf("run #%d: %v", i+1, err)
		}
		if got != first {
			t.Errorf("run #%d: hash = %d, want %d (non-deterministic)", i+1, got, first)
		}
	}
}

// nopSink is a visualgraph.CommandSink that discards Action emissions.
type nopSink struct{}

func (nopSink) Emit(string, map[string]any) {}

// TestGraphTraceLivePath validates the live interpreter → Recorder → DebugStore
// path (the runtime half of the graph-debug feature). Timestamps are wall-clock
// so the trace is asserted by structure, not hashed.
func TestGraphTraceLivePath(t *testing.T) {
	t.Parallel()
	g := &vg.GraphDefinition{
		ID: "g",
		Nodes: []vg.Node{
			{ID: "evt", Type: "event.OnUpdate", Pins: []vg.Pin{
				{ID: "exec_out", Direction: vg.Output, Kind: vg.Execution},
			}},
			{ID: "log", Type: "action.Log", Pins: []vg.Pin{
				{ID: "exec_in", Direction: vg.Input, Kind: vg.Execution},
				{ID: "exec_out", Direction: vg.Output, Kind: vg.Execution},
				{ID: "msg", DataType: "any", Direction: vg.Input, Kind: vg.Data},
			}},
		},
		Connections: []vg.Connection{
			{FromNode: "evt", FromPin: "exec_out", ToNode: "log", ToPin: "exec_in"},
		},
	}
	if err := vg.ValidateGraph(g); err != nil {
		t.Fatalf("validate: %v", err)
	}

	store := grapheditor.NewDebugStore()
	store.Open("g")
	rec := grapheditor.NewRecorder(store, "g", 1)

	in := vg.NewInterpreter(0)
	if err := in.RunTraced(g, "evt", nopSink{}, rec); err != nil {
		t.Fatalf("RunTraced: %v", err)
	}

	// The exec chain evt → log recorded two frames into the store.
	trace := store.Trace("g", 0)
	if len(trace) != 2 {
		t.Fatalf("recorded %d frames, want 2 (evt → log)", len(trace))
	}
	if trace[0].NodeID != "evt" || trace[1].NodeID != "log" {
		t.Errorf("frame node order = [%s, %s], want [evt, log]", trace[0].NodeID, trace[1].NodeID)
	}
	if c, _, _ := store.Inspect("g", "log"); c != 1 {
		t.Errorf("log execution count = %d, want 1", c)
	}
}
