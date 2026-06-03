package grapheditor_test

import (
	"reflect"
	"sync"
	"testing"

	"github.com/neuengine/neu/internal/grapheditor"
	"github.com/neuengine/neu/pkg/editor"
	vg "github.com/neuengine/neu/pkg/visualgraph"
)

// --- fixtures ----------------------------------------------------------------

const graphID = "g1"

// testRegistry builds a registry with one node per pin shape the tests exercise:
// a data node (math.Add), an event/exec node (event.Start), and a mixed
// action node (action.Log).
func testRegistry() *vg.NodeRegistry {
	r := vg.NewNodeRegistry()
	r.Register(vg.NodeDescriptor{
		TypeName: "math.Add", DisplayName: "Add", Category: "Data",
		Pins: []vg.PinDescriptor{
			{ID: "a", Name: "A", DataType: "int", Direction: vg.Input, Kind: vg.Data},
			{ID: "b", Name: "B", DataType: "int", Direction: vg.Input, Kind: vg.Data},
			{ID: "sum", Name: "Sum", DataType: "int", Direction: vg.Output, Kind: vg.Data},
		},
	})
	r.Register(vg.NodeDescriptor{
		TypeName: "event.Start", DisplayName: "Start", Category: "Event",
		Pins: []vg.PinDescriptor{
			{ID: "out", Name: "Out", Direction: vg.Output, Kind: vg.Execution},
		},
	})
	r.Register(vg.NodeDescriptor{
		TypeName: "action.Log", DisplayName: "Log", Category: "Action",
		Pins: []vg.PinDescriptor{
			{ID: "in", Name: "In", Direction: vg.Input, Kind: vg.Execution},
			{ID: "msg", Name: "Msg", DataType: "string", Direction: vg.Input, Kind: vg.Data},
		},
	})
	return r
}

func testGraph() *vg.GraphDefinition {
	return &vg.GraphDefinition{
		ID:   graphID,
		Name: "Test",
		Variables: []vg.VariableDecl{
			{Name: "score", Type: "int", Default: 0},
			{Name: "label", Type: "string", Default: "hi"},
		},
		Nodes: []vg.Node{
			{
				ID:   "start",
				Type: "event.Start",
				Pins: []vg.Pin{{ID: "out", Name: "Out", Direction: vg.Output, Kind: vg.Execution}},
			},
			{
				ID:         "add",
				Type:       "math.Add",
				Properties: map[string]any{"label": "adder"},
				Pins: []vg.Pin{
					{ID: "a", Name: "A", DataType: "int", Direction: vg.Input, Kind: vg.Data},
					{ID: "b", Name: "B", DataType: "int", Direction: vg.Input, Kind: vg.Data},
					{ID: "sum", Name: "Sum", DataType: "int", Direction: vg.Output, Kind: vg.Data},
				},
			},
			{
				ID:   "log",
				Type: "action.Log",
				Pins: []vg.Pin{
					{ID: "in", Name: "In", Direction: vg.Input, Kind: vg.Execution},
					{ID: "msg", Name: "Msg", DataType: "string", Direction: vg.Input, Kind: vg.Data},
					{ID: "anyIn", Name: "AnyIn", DataType: "any", Direction: vg.Input, Kind: vg.Data},
				},
			},
		},
	}
}

func provider(g *vg.GraphDefinition) grapheditor.GraphProvider {
	return func(id string) (*vg.GraphDefinition, bool) {
		if g != nil && id == g.ID {
			return g, true
		}
		return nil, false
	}
}

func frame(node string, step uint32, pins map[string]any) editor.GraphExecutionFrame {
	return editor.GraphExecutionFrame{NodeID: node, PinValues: pins, StepIndex: step, Timestamp: int64(step)}
}

// --- RegistryQuery -----------------------------------------------------------

func TestRegistryQueryListAndSearch(t *testing.T) {
	t.Parallel()
	q := grapheditor.NewRegistryQuery(testRegistry())

	all := q.ListAllNodes()
	if len(all) != 3 {
		t.Fatalf("ListAllNodes = %d, want 3", len(all))
	}
	// Sorted by type name (registry guarantee).
	wantOrder := []string{"action.Log", "event.Start", "math.Add"}
	for i, d := range all {
		if d.TypeName != wantOrder[i] {
			t.Errorf("node[%d] = %q, want %q", i, d.TypeName, wantOrder[i])
		}
	}

	if got := q.SearchNodes("add", ""); len(got) != 1 || got[0].TypeName != "math.Add" {
		t.Errorf("SearchNodes(add) = %+v, want [math.Add]", got)
	}
	if got := q.SearchNodes("", "Event"); len(got) != 1 || got[0].TypeName != "event.Start" {
		t.Errorf("SearchNodes(,Event) = %+v, want [event.Start]", got)
	}
	if got := q.SearchNodes("", ""); len(got) != 3 {
		t.Errorf("SearchNodes(,) = %d, want 3", len(got))
	}
	// Query matches math.Add but category filter excludes it.
	if got := q.SearchNodes("add", "Action"); len(got) != 0 {
		t.Errorf("SearchNodes(add,Action) = %+v, want empty", got)
	}
}

func TestRegistryQueryGetDescriptorTranslatesPins(t *testing.T) {
	t.Parallel()
	q := grapheditor.NewRegistryQuery(testRegistry())

	if _, ok := q.GetNodeDescriptor("ghost"); ok {
		t.Error("GetNodeDescriptor(ghost) ok=true, want false")
	}

	d, ok := q.GetNodeDescriptor("event.Start")
	if !ok {
		t.Fatal("GetNodeDescriptor(event.Start) not found")
	}
	if len(d.Pins) != 1 {
		t.Fatalf("event.Start pins = %d, want 1", len(d.Pins))
	}
	out := d.Pins[0]
	if out.Direction != editor.Output || !out.Execution {
		t.Errorf("event.Start.out = %+v, want Output execution pin", out)
	}

	add, _ := q.GetNodeDescriptor("math.Add")
	for _, p := range add.Pins {
		if p.Execution {
			t.Errorf("math.Add pin %q should be a data pin, got Execution=true", p.ID)
		}
	}
}

func TestRegistryQueryCompatibleNodes(t *testing.T) {
	t.Parallel()
	q := grapheditor.NewRegistryQuery(testRegistry())

	names := func(ds []editor.NodeDescriptor) []string {
		out := make([]string, len(ds))
		for i, d := range ds {
			out[i] = d.TypeName
		}
		return out
	}

	// Dragging from an int Output → nodes with an assignable int Input.
	if got := names(q.GetCompatibleNodes("int", editor.Output)); !reflect.DeepEqual(got, []string{"math.Add"}) {
		t.Errorf("compatible(int, Output) = %v, want [math.Add]", got)
	}
	// Dragging into an int Input → nodes with an Output assignable to int.
	if got := names(q.GetCompatibleNodes("int", editor.Input)); !reflect.DeepEqual(got, []string{"math.Add"}) {
		t.Errorf("compatible(int, Input) = %v, want [math.Add]", got)
	}
	// Dragging from a string Output → action.Log's string Input.
	if got := names(q.GetCompatibleNodes("string", editor.Output)); !reflect.DeepEqual(got, []string{"action.Log"}) {
		t.Errorf("compatible(string, Output) = %v, want [action.Log]", got)
	}
}

func TestRegistryQueryTypeHierarchy(t *testing.T) {
	t.Parallel()
	q := grapheditor.NewRegistryQuery(testRegistry())
	cases := map[string][]string{
		"int": {"int", "any"},
		"any": {"any"},
		"":    {"any"},
	}
	for in, want := range cases {
		if got := q.GetTypeHierarchy(in); !reflect.DeepEqual(got, want) {
			t.Errorf("GetTypeHierarchy(%q) = %v, want %v", in, got, want)
		}
	}
}

// --- EditorPlugin ------------------------------------------------------------

func TestEditorPluginOpenCloseAndInspect(t *testing.T) {
	t.Parallel()
	store := grapheditor.NewDebugStore()
	p := grapheditor.NewEditorPlugin(provider(testGraph()), store)

	p.OnGraphOpened(graphID)
	if !store.IsOpen(graphID) {
		t.Fatal("graph should be open after OnGraphOpened")
	}

	// Record a frame so inspection has runtime data.
	store.RecordFrame(graphID, 7, frame("add", 1, map[string]any{"sum": 5}))

	insp := p.OnNodeSelected(graphID, "add")
	if insp.NodeType != "math.Add" {
		t.Errorf("NodeType = %q, want math.Add", insp.NodeType)
	}
	if insp.ExecutionCount != 1 {
		t.Errorf("ExecutionCount = %d, want 1", insp.ExecutionCount)
	}
	if insp.PinValues["sum"] != 5 {
		t.Errorf("PinValues[sum] = %v, want 5", insp.PinValues["sum"])
	}

	// Unknown node: still returns its ID, no type.
	unknown := p.OnNodeSelected(graphID, "ghost")
	if unknown.NodeID != "ghost" || unknown.NodeType != "" {
		t.Errorf("OnNodeSelected(ghost) = %+v, want NodeID=ghost NodeType=empty", unknown)
	}

	p.OnGraphClosed(graphID)
	if store.IsOpen(graphID) {
		t.Error("graph should be closed after OnGraphClosed")
	}
}

func TestEditorPluginConnectionValidation(t *testing.T) {
	t.Parallel()
	p := grapheditor.NewEditorPlugin(provider(testGraph()), grapheditor.NewDebugStore())

	conn := func(fn, fp, tn, tp string) editor.Connection {
		return editor.Connection{FromNode: fn, FromPin: fp, ToNode: tn, ToPin: tp}
	}
	add := func(c editor.Connection) editor.ValidationResult {
		return p.OnConnectionChanged(graphID, editor.ConnectionChange{Connection: c, Action: editor.ConnectionAdd})
	}

	// Remove is always valid.
	if r := p.OnConnectionChanged(graphID, editor.ConnectionChange{Action: editor.ConnectionRemove}); !r.Valid {
		t.Error("ConnectionRemove should always be valid")
	}

	// Valid exec + valid data (int → any).
	if r := add(conn("start", "out", "log", "in")); !r.Valid {
		t.Errorf("exec connection should be valid: %+v", r)
	}
	if r := add(conn("add", "sum", "log", "anyIn")); !r.Valid {
		t.Errorf("int→any data connection should be valid: %+v", r)
	}

	invalidCases := map[string]editor.Connection{
		"from-node missing": conn("ghost", "out", "log", "in"),
		"to-node missing":   conn("start", "out", "ghost", "in"),
		"from-pin missing":  conn("start", "ghost", "log", "in"),
		"to-pin missing":    conn("start", "out", "log", "ghost"),
		"direction wrong":   conn("log", "in", "add", "a"),      // Input → ...
		"kind mismatch":     conn("start", "out", "log", "msg"), // exec → data
		"type mismatch":     conn("add", "sum", "log", "msg"),   // int → string
	}
	for name, c := range invalidCases {
		if r := add(c); r.Valid || len(r.Errors) == 0 {
			t.Errorf("%s: expected invalid with errors, got %+v", name, r)
		}
	}

	// Graph not loaded.
	if r := add(conn("start", "out", "log", "in")); r.Valid {
		// sanity: this *is* loaded; now test the unloaded path separately.
		_ = r
	}
	pUnloaded := grapheditor.NewEditorPlugin(provider(nil), grapheditor.NewDebugStore())
	if r := pUnloaded.OnConnectionChanged(graphID, editor.ConnectionChange{Action: editor.ConnectionAdd}); r.Valid {
		t.Error("unloaded graph add-connection should be invalid")
	}
}

func TestEditorPluginPropertyValidation(t *testing.T) {
	t.Parallel()
	p := grapheditor.NewEditorPlugin(provider(testGraph()), grapheditor.NewDebugStore())

	// Known property on node "add".
	if r := p.OnPropertyChanged(graphID, "add", "label", "x"); !r.Valid || len(r.Warnings) != 0 {
		t.Errorf("known property = %+v, want valid no-warning", r)
	}
	// Unknown property → valid with a warning.
	r := p.OnPropertyChanged(graphID, "add", "ghostProp", 1)
	if !r.Valid || len(r.Warnings) == 0 {
		t.Errorf("unknown property = %+v, want valid+warning", r)
	}
	// Missing node → invalid.
	if r := p.OnPropertyChanged(graphID, "ghost", "label", 1); r.Valid {
		t.Error("missing node should be invalid")
	}
	// Graph not loaded → invalid.
	pUnloaded := grapheditor.NewEditorPlugin(provider(nil), grapheditor.NewDebugStore())
	if r := pUnloaded.OnPropertyChanged(graphID, "add", "label", 1); r.Valid {
		t.Error("unloaded graph property change should be invalid")
	}
}

// --- Debugger ----------------------------------------------------------------

func TestDebuggerBreakpoints(t *testing.T) {
	t.Parallel()
	store := grapheditor.NewDebugStore()
	d := grapheditor.NewDebugger(provider(testGraph()), store)

	// Graph not open yet → node exists but store rejects.
	if err := d.SetBreakpoint(graphID, "add"); err == nil {
		t.Error("SetBreakpoint on a not-open graph should error")
	}

	store.Open(graphID)
	if err := d.SetBreakpoint(graphID, "add"); err != nil {
		t.Errorf("SetBreakpoint(add) = %v, want nil", err)
	}
	if err := d.SetBreakpoint(graphID, "log"); err != nil {
		t.Errorf("SetBreakpoint(log) = %v, want nil", err)
	}
	// Unknown node.
	if err := d.SetBreakpoint(graphID, "ghost"); err == nil {
		t.Error("SetBreakpoint(ghost) should error")
	}

	if bps := d.ListBreakpoints(graphID); !reflect.DeepEqual(bps, []string{"add", "log"}) {
		t.Errorf("ListBreakpoints = %v, want [add log] (sorted)", bps)
	}

	if err := d.RemoveBreakpoint(graphID, "add"); err != nil {
		t.Errorf("RemoveBreakpoint(add) = %v, want nil", err)
	}
	if err := d.RemoveBreakpoint(graphID, "add"); err == nil {
		t.Error("RemoveBreakpoint(add) twice should error")
	}
}

func TestDebuggerSteppingAndTrace(t *testing.T) {
	t.Parallel()
	store := grapheditor.NewDebugStore()
	d := grapheditor.NewDebugger(provider(testGraph()), store)
	store.Open(graphID)

	store.RecordFrame(graphID, 1, frame("start", 0, nil))
	store.RecordFrame(graphID, 1, frame("add", 1, map[string]any{"sum": 9}))
	store.RecordFrame(graphID, 1, frame("log", 2, nil))

	if tr := d.GetExecutionTrace(graphID, 0); len(tr) != 3 {
		t.Fatalf("trace len = %d, want 3", len(tr))
	}
	if tr := d.GetExecutionTrace(graphID, 2); len(tr) != 2 || tr[0].NodeID != "add" {
		t.Errorf("trace(max=2) = %+v, want last 2 [add, log]", tr)
	}

	// Step through the recorded trace.
	if f := d.StepOver(graphID); f.NodeID != "start" {
		t.Errorf("step 1 = %q, want start", f.NodeID)
	}
	if f := d.StepInto(graphID); f.NodeID != "add" { // StepInto == StepOver for now
		t.Errorf("step 2 = %q, want add", f.NodeID)
	}
	if f := d.StepOver(graphID); f.NodeID != "log" {
		t.Errorf("step 3 = %q, want log", f.NodeID)
	}
	// Exhausted → zero frame.
	if f := d.StepOver(graphID); f.NodeID != "" {
		t.Errorf("step past end = %q, want empty", f.NodeID)
	}
	// Continue rewinds the cursor.
	if err := d.Continue(graphID); err != nil {
		t.Errorf("Continue = %v", err)
	}
	if f := d.StepOver(graphID); f.NodeID != "start" {
		t.Errorf("after Continue, step 1 = %q, want start", f.NodeID)
	}

	// Pin value from the latest recorded frame.
	if v := d.GetPinValue(graphID, "add", "sum", 1); v != 9 {
		t.Errorf("GetPinValue(add.sum) = %v, want 9", v)
	}
	if v := d.GetPinValue(graphID, "start", "x", 1); v != nil {
		t.Errorf("GetPinValue(start.x) = %v, want nil", v)
	}
}

func TestDebuggerVariableValues(t *testing.T) {
	t.Parallel()
	d := grapheditor.NewDebugger(provider(testGraph()), grapheditor.NewDebugStore())

	vars := d.GetVariableValues(graphID, 42)
	if vars["score"] != 0 || vars["label"] != "hi" {
		t.Errorf("GetVariableValues = %+v, want score=0 label=hi", vars)
	}
	// Unloaded graph → nil.
	dUnloaded := grapheditor.NewDebugger(provider(nil), grapheditor.NewDebugStore())
	if vars := dUnloaded.GetVariableValues(graphID, 1); vars != nil {
		t.Errorf("GetVariableValues(unloaded) = %+v, want nil", vars)
	}
}

// --- DebugStore --------------------------------------------------------------

func TestDebugStoreEventsAndBreakpointFlag(t *testing.T) {
	t.Parallel()
	s := grapheditor.NewDebugStore()

	// Recording into a closed graph is a no-op.
	s.RecordFrame(graphID, 1, frame("add", 0, nil))
	if ev := s.DrainEvents(); ev != nil {
		t.Errorf("closed-graph record should queue nothing, got %+v", ev)
	}

	s.Open(graphID)
	s.SetBreakpoint(graphID, "add")
	s.RecordFrame(graphID, 5, frame("add", 0, map[string]any{"sum": 1}))
	s.RecordFrame(graphID, 5, frame("log", 1, nil))

	ev := s.DrainEvents()
	if len(ev) != 2 {
		t.Fatalf("DrainEvents = %d events, want 2", len(ev))
	}
	if !ev[0].Breakpoint {
		t.Error("first event (breakpointed node) should have Breakpoint=true")
	}
	if ev[1].Breakpoint {
		t.Error("second event (no breakpoint) should have Breakpoint=false")
	}
	// Drain clears the queue.
	if ev := s.DrainEvents(); ev != nil {
		t.Errorf("second drain = %+v, want nil", ev)
	}
}

func TestDebugStoreRecordError(t *testing.T) {
	t.Parallel()
	s := grapheditor.NewDebugStore()
	s.Open(graphID)
	s.RecordFrame(graphID, 3, frame("add", 0, map[string]any{"sum": 2}))
	s.RecordError(graphID, 3, "add", "boom")

	count, lastErr, pins := s.Inspect(graphID, "add")
	if count != 1 || lastErr != "boom" || pins["sum"] != 2 {
		t.Errorf("Inspect(add) = (%d, %q, %v), want (1, boom, sum=2)", count, lastErr, pins)
	}

	ev := s.DrainEvents()
	if len(ev) != 2 || ev[1].Err != "boom" || ev[1].Frame.NodeID != "add" {
		t.Errorf("error event = %+v, want Err=boom NodeID=add", ev)
	}

	// Inspect on a closed graph is zero-valued.
	if c, e, p := s.Inspect("nope", "add"); c != 0 || e != "" || p != nil {
		t.Errorf("Inspect(closed) = (%d, %q, %v), want zero", c, e, p)
	}
}

func TestDebugStoreTraceRingCap(t *testing.T) {
	t.Parallel()
	s := grapheditor.NewDebugStore()
	s.Open(graphID)
	total := grapheditor.DefaultMaxTrace + 50
	for i := range total {
		s.RecordFrame(graphID, 1, frame("n", uint32(i), nil))
	}
	tr := s.Trace(graphID, 0)
	if len(tr) != grapheditor.DefaultMaxTrace {
		t.Fatalf("trace len = %d, want capped at %d", len(tr), grapheditor.DefaultMaxTrace)
	}
	// The ring keeps the most-recent frames: first retained is index 50.
	if tr[0].StepIndex != 50 {
		t.Errorf("oldest retained StepIndex = %d, want 50", tr[0].StepIndex)
	}
	// Trace on a closed graph is nil.
	if tr := s.Trace("nope", 0); tr != nil {
		t.Errorf("Trace(closed) = %+v, want nil", tr)
	}
}

func TestDebugStoreConcurrentRecordAndQuery(t *testing.T) {
	t.Parallel()
	s := grapheditor.NewDebugStore()
	s.Open(graphID)

	var wg sync.WaitGroup
	wg.Go(func() {
		for i := range 200 {
			s.RecordFrame(graphID, 1, frame("add", uint32(i), map[string]any{"sum": i}))
		}
	})
	wg.Go(func() {
		for range 200 {
			_, _, _ = s.Inspect(graphID, "add")
			_ = s.Trace(graphID, 10)
			_ = s.DrainEvents()
		}
	})
	wg.Wait()
}
