package grapheditor_test

import (
	"testing"

	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/internal/grapheditor"
	"github.com/neuengine/neu/pkg/app/appface"
	"github.com/neuengine/neu/pkg/protocol"
	vg "github.com/neuengine/neu/pkg/visualgraph"
)

// captureBuilder is a minimal appface.Builder recording the resource and the
// system a plugin registers (avoids importing pkg/app → no cycle).
type captureBuilder struct {
	w        *world.World
	resource any
	system   scheduler.System
	schedule string
}

func newCaptureBuilder() *captureBuilder { return &captureBuilder{w: world.NewWorld()} }

func (b *captureBuilder) World() *world.World { return b.w }
func (b *captureBuilder) AddSystem(schedule string, sys scheduler.System) appface.Builder {
	b.schedule, b.system = schedule, sys
	return b
}
func (b *captureBuilder) AddSystems(_ string, _ ...scheduler.System) appface.Builder { return b }
func (b *captureBuilder) SetResource(v any) appface.Builder                          { b.resource = v; return b }
func (b *captureBuilder) InitResource(any) appface.Builder                           { return b }
func (b *captureBuilder) AddPlugin(appface.Plugin) appface.Builder                   { return b }
func (b *captureBuilder) AddPlugins(appface.PluginGroup) appface.Builder             { return b }

var _ appface.Builder = (*captureBuilder)(nil)

// --- Recorder ----------------------------------------------------------------

func TestRecorderMapsFrameAndError(t *testing.T) {
	t.Parallel()
	store := grapheditor.NewDebugStore()
	store.Open(graphID)
	rec := grapheditor.NewRecorder(store, graphID, 42)

	rec.RecordStep(vg.ExecutionFrame{
		NodeID: "add", NodeType: "math.Add", StepIndex: 3,
		PinValues: map[string]any{"sum": 9},
	})
	rec.RecordError("add", "boom")

	count, lastErr, pins := store.Inspect(graphID, "add")
	if count != 1 || lastErr != "boom" || pins["sum"] != 9 {
		t.Errorf("Inspect(add) = (%d, %q, %v), want (1, boom, sum=9)", count, lastErr, pins)
	}

	// The recorded editor frame carries a non-zero timestamp + the step index.
	tr := store.Trace(graphID, 0)
	if len(tr) != 1 || tr[0].StepIndex != 3 || tr[0].Timestamp == 0 {
		t.Errorf("trace = %+v, want one frame step=3 with a timestamp", tr)
	}

	// Recorder satisfies the visualgraph hook.
	var _ vg.TraceRecorder = rec
}

// --- SyncPlugin + system -----------------------------------------------------

func TestSyncPluginBuildRegistersSystem(t *testing.T) {
	t.Parallel()
	store := grapheditor.NewDebugStore()
	b := newCaptureBuilder()

	grapheditor.SyncPlugin{Store: store}.Build(b)

	if b.resource != store {
		t.Errorf("Build should publish the shared store, got %v", b.resource)
	}
	if b.schedule != appface.PostUpdate {
		t.Errorf("system schedule = %q, want PostUpdate", b.schedule)
	}
	if b.system == nil {
		t.Fatal("Build should register the DebugSync system")
	}
	if b.system.Name() != "grapheditor.DebugSync" {
		t.Errorf("system name = %q, want grapheditor.DebugSync", b.system.Name())
	}

	// A nil Store gets a fresh one (still publishes a resource).
	b2 := newCaptureBuilder()
	grapheditor.SyncPlugin{}.Build(b2)
	if _, ok := b2.resource.(*grapheditor.DebugStore); !ok {
		t.Errorf("nil-Store Build should publish a fresh *DebugStore, got %T", b2.resource)
	}
}

func TestSyncSystemEmitsProtocolMessages(t *testing.T) {
	t.Parallel()
	store := grapheditor.NewDebugStore()
	store.Open(graphID)
	store.SetBreakpoint(graphID, "bp")

	var got []any
	plugin := grapheditor.SyncPlugin{
		Store: store,
		Sink:  func(msg any) { got = append(got, msg) },
	}
	b := newCaptureBuilder()
	plugin.Build(b)

	// Record an ordinary step, a breakpointed step, and an error.
	rec := grapheditor.NewRecorder(store, graphID, 7)
	rec.RecordStep(vg.ExecutionFrame{NodeID: "n1", NodeType: "math.Add", StepIndex: 1})
	rec.RecordStep(vg.ExecutionFrame{NodeID: "bp", NodeType: "action.Log", StepIndex: 2})
	rec.RecordError("n1", "kaboom")

	// Run the registered system once.
	b.system.Run(b.w)

	if len(got) != 3 {
		t.Fatalf("emitted %d messages, want 3", len(got))
	}
	if tr, ok := got[0].(protocol.GraphExecutionTraceEvent); !ok {
		t.Errorf("msg[0] = %T, want GraphExecutionTraceEvent", got[0])
	} else if tr.GraphID != graphID || tr.EntityID != 7 || len(tr.Frames) != 1 || tr.Frames[0].NodeID != "n1" {
		t.Errorf("trace event = %+v, want graph=%s entity=7 frame n1", tr, graphID)
	}
	if hit, ok := got[1].(protocol.GraphBreakpointHit); !ok {
		t.Errorf("msg[1] = %T, want GraphBreakpointHit", got[1])
	} else if hit.NodeID != "bp" || hit.Frame.NodeType != "action.Log" {
		t.Errorf("breakpoint hit = %+v, want node bp / action.Log", hit)
	}
	if rerr, ok := got[2].(protocol.GraphRuntimeError); !ok {
		t.Errorf("msg[2] = %T, want GraphRuntimeError", got[2])
	} else if rerr.NodeID != "n1" || rerr.Error != "kaboom" {
		t.Errorf("runtime error = %+v, want node n1 / kaboom", rerr)
	}

	// Second run: queue already drained → no new messages.
	got = nil
	b.system.Run(b.w)
	if len(got) != 0 {
		t.Errorf("second run emitted %d, want 0 (queue drained)", len(got))
	}
}

func TestSyncSystemNilSinkStillDrains(t *testing.T) {
	t.Parallel()
	store := grapheditor.NewDebugStore()
	store.Open(graphID)
	b := newCaptureBuilder()
	grapheditor.SyncPlugin{Store: store}.Build(b) // nil Sink

	grapheditor.NewRecorder(store, graphID, 1).
		RecordStep(vg.ExecutionFrame{NodeID: "n", StepIndex: 1})

	b.system.Run(b.w) // must not panic, and drains the queue

	// Re-attaching draining proof: the pending queue is now empty.
	if ev := store.DrainEvents(); ev != nil {
		t.Errorf("nil-sink system should have drained the queue, got %+v", ev)
	}
}
