package visualgraph

import (
	"errors"
	"fmt"
	"testing"
)

// recSink records emitted commands for INV-1 verification.
type recSink struct{ ops []string }

func (s *recSink) Emit(op string, args map[string]any) {
	s.ops = append(s.ops, fmt.Sprintf("%s:%v", op, args["msg"]))
}

func execPin(id string, dir Direction) Pin { return Pin{ID: id, Direction: dir, Kind: Execution} }
func dataPin(id, typ string, dir Direction, def any) Pin {
	return Pin{ID: id, Name: id, Direction: dir, Kind: Data, DataType: typ, DefaultValue: def}
}

func TestNodeRegistry(t *testing.T) {
	t.Parallel()
	r := NewNodeRegistry()
	r.Register(NodeDescriptor{TypeName: "math.Add", DisplayName: "Add", Category: "Data"})
	r.Register(NodeDescriptor{TypeName: "action.Log", DisplayName: "Log", Category: "Action"})
	if r.Len() != 2 {
		t.Fatalf("Len = %d", r.Len())
	}
	if d, ok := r.Get("math.Add"); !ok || d.Category != "Data" {
		t.Errorf("Get = %+v,%v", d, ok)
	}
	if list := r.List(); len(list) != 2 || list[0].TypeName != "action.Log" {
		t.Errorf("List not sorted: %+v", list) // sorted: action.Log < math.Add
	}
	if dat := r.ListByCategory("Data"); len(dat) != 1 || dat[0].TypeName != "math.Add" {
		t.Errorf("ListByCategory = %+v", dat)
	}
	if found := r.Search("log"); len(found) != 1 {
		t.Errorf("Search(log) = %+v", found)
	}
	r.Unregister("math.Add")
	if r.Len() != 1 {
		t.Errorf("Len after unregister = %d", r.Len())
	}
}

func TestValidateGraphErrors(t *testing.T) {
	t.Parallel()
	// Unknown node.
	g := &GraphDefinition{Connections: []Connection{{FromNode: "x", FromPin: "o", ToNode: "y", ToPin: "i"}}}
	if err := ValidateGraph(g); !errors.Is(err, ErrUnknownNode) {
		t.Errorf("unknown node err = %v", err)
	}

	// Kind mismatch: Data output → Execution input.
	g2 := &GraphDefinition{
		Nodes: []Node{
			{ID: "a", Pins: []Pin{dataPin("o", "float32", Output, nil)}},
			{ID: "b", Pins: []Pin{execPin("i", Input)}},
		},
		Connections: []Connection{{FromNode: "a", FromPin: "o", ToNode: "b", ToPin: "i"}},
	}
	if err := ValidateGraph(g2); !errors.Is(err, ErrKindMismatch) {
		t.Errorf("kind mismatch err = %v", err)
	}

	// Type mismatch: float32 → Vec3.
	g3 := &GraphDefinition{
		Nodes: []Node{
			{ID: "a", Pins: []Pin{dataPin("o", "float32", Output, nil)}},
			{ID: "b", Pins: []Pin{dataPin("i", "Vec3", Input, nil)}},
		},
		Connections: []Connection{{FromNode: "a", FromPin: "o", ToNode: "b", ToPin: "i"}},
	}
	if err := ValidateGraph(g3); !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("type mismatch err = %v (INV-3)", err)
	}
}

func TestValidateGraphDataCycle(t *testing.T) {
	t.Parallel()
	// a.out → b.in, b.out → a.in : a data dependency cycle (INV-4).
	g := &GraphDefinition{
		Nodes: []Node{
			{ID: "a", Pins: []Pin{dataPin("in", "float32", Input, nil), dataPin("out", "float32", Output, nil)}},
			{ID: "b", Pins: []Pin{dataPin("in", "float32", Input, nil), dataPin("out", "float32", Output, nil)}},
		},
		Connections: []Connection{
			{FromNode: "a", FromPin: "out", ToNode: "b", ToPin: "in"},
			{FromNode: "b", FromPin: "out", ToNode: "a", ToPin: "in"},
		},
	}
	if err := ValidateGraph(g); !errors.Is(err, ErrGraphCycle) {
		t.Errorf("data cycle err = %v, want ErrGraphCycle (INV-4)", err)
	}
}

// dataEvalGraph: OnUpdate → Log; Add(2,3) → Log.msg. Expected emit "log:5".
func dataEvalGraph() *GraphDefinition {
	return &GraphDefinition{
		Nodes: []Node{
			{ID: "evt", Type: "event.OnUpdate", Pins: []Pin{execPin("exec_out", Output)}},
			{ID: "add", Type: "math.Add", Pins: []Pin{
				dataPin("a", "float32", Input, 2.0),
				dataPin("b", "float32", Input, 3.0),
				dataPin("result", "float32", Output, nil),
			}},
			{ID: "log", Type: "action.Log", Pins: []Pin{
				execPin("exec_in", Input), execPin("exec_out", Output),
				dataPin("msg", "any", Input, nil),
			}},
		},
		Connections: []Connection{
			{FromNode: "evt", FromPin: "exec_out", ToNode: "log", ToPin: "exec_in"},
			{FromNode: "add", FromPin: "result", ToNode: "log", ToPin: "msg"},
		},
	}
}

func TestInterpreterDataEvalAndSink(t *testing.T) {
	t.Parallel()
	g := dataEvalGraph()
	if err := ValidateGraph(g); err != nil {
		t.Fatalf("validate: %v", err)
	}
	in := NewInterpreter(0)
	var sink recSink
	if err := in.Run(g, "evt", &sink); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// INV-1: the Action node emitted exactly one command, with the lazily-pulled sum.
	if len(sink.ops) != 1 || sink.ops[0] != "log:5" {
		t.Errorf("sink ops = %v, want [log:5]", sink.ops)
	}
}

func TestInterpreterDeterministic(t *testing.T) {
	t.Parallel()
	in := NewInterpreter(0)
	run := func() []string {
		var s recSink
		_ = in.Run(dataEvalGraph(), "evt", &s)
		return s.ops
	}
	first := run()
	for range 20 {
		got := run()
		if len(got) != len(first) || (len(got) > 0 && got[0] != first[0]) {
			t.Errorf("INV-4: non-deterministic output %v vs %v", got, first)
		}
	}
}

func TestInterpreterBranch(t *testing.T) {
	t.Parallel()
	// OnUpdate → Branch(cond = Greater(5,3)=true) → "true" → Log.
	g := &GraphDefinition{
		Nodes: []Node{
			{ID: "evt", Type: "event.OnUpdate", Pins: []Pin{execPin("exec_out", Output)}},
			{ID: "gt", Type: "logic.Greater", Pins: []Pin{
				dataPin("a", "float32", Input, 5.0), dataPin("b", "float32", Input, 3.0),
				dataPin("result", "bool", Output, nil),
			}},
			{ID: "br", Type: "flow.Branch", Pins: []Pin{
				execPin("exec_in", Input), execPin("true", Output), execPin("false", Output),
				dataPin("cond", "bool", Input, false),
			}},
			{ID: "log", Type: "action.Log", Pins: []Pin{execPin("exec_in", Input), execPin("exec_out", Output), dataPin("msg", "any", Input, "yes")}},
		},
		Connections: []Connection{
			{FromNode: "evt", FromPin: "exec_out", ToNode: "br", ToPin: "exec_in"},
			{FromNode: "gt", FromPin: "result", ToNode: "br", ToPin: "cond"},
			{FromNode: "br", FromPin: "true", ToNode: "log", ToPin: "exec_in"},
		},
	}
	if err := ValidateGraph(g); err != nil {
		t.Fatalf("validate: %v", err)
	}
	var sink recSink
	if err := NewInterpreter(0).Run(g, "evt", &sink); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(sink.ops) != 1 || sink.ops[0] != "log:yes" {
		t.Errorf("branch sink = %v, want [log:yes]", sink.ops)
	}
}

func TestInterpreterStepLimit(t *testing.T) {
	t.Parallel()
	// Two Log nodes in an execution cycle → unbounded chain → step limit (INV-2).
	g := &GraphDefinition{
		Nodes: []Node{
			{ID: "evt", Type: "event.OnUpdate", Pins: []Pin{execPin("exec_out", Output)}},
			{ID: "a", Type: "action.Log", Pins: []Pin{execPin("exec_in", Input), execPin("exec_out", Output), dataPin("msg", "any", Input, "a")}},
			{ID: "b", Type: "action.Log", Pins: []Pin{execPin("exec_in", Input), execPin("exec_out", Output), dataPin("msg", "any", Input, "b")}},
		},
		Connections: []Connection{
			{FromNode: "evt", FromPin: "exec_out", ToNode: "a", ToPin: "exec_in"},
			{FromNode: "a", FromPin: "exec_out", ToNode: "b", ToPin: "exec_in"},
			{FromNode: "b", FromPin: "exec_out", ToNode: "a", ToPin: "exec_in"}, // cycle
		},
	}
	in := NewInterpreter(100) // small limit
	if err := in.Run(g, "evt", &recSink{}); !errors.Is(err, ErrStepLimit) {
		t.Errorf("exec cycle err = %v, want ErrStepLimit (INV-2)", err)
	}
}

func TestInterpreterHandlerCount(t *testing.T) {
	t.Parallel()
	if NewInterpreter(0).handlerCount() < 6 {
		t.Error("expected the built-in node set to be registered")
	}
	// Unknown entry node.
	if err := NewInterpreter(0).Run(&GraphDefinition{}, "ghost", &recSink{}); !errors.Is(err, ErrUnknownNode) {
		t.Errorf("unknown entry err = %v", err)
	}
}
