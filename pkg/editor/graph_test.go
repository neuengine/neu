package editor_test

import (
	"testing"

	"github.com/neuengine/neu/pkg/editor"
)

// fakeBridge implements all three graph-bridge interfaces, asserting at compile
// time that the contract is satisfiable from outside the package (as the editor
// repo would) and giving the DTOs a usage exercise.
type fakeBridge struct {
	breakpoints map[string]bool
}

var (
	_ editor.GraphEditorPlugin = (*fakeBridge)(nil)
	_ editor.NodeRegistryQuery = (*fakeBridge)(nil)
	_ editor.GraphDebugger     = (*fakeBridge)(nil)
)

func (*fakeBridge) OnGraphOpened(string) {}
func (*fakeBridge) OnGraphClosed(string) {}
func (*fakeBridge) OnNodeSelected(graphID, nodeID string) editor.NodeInspection {
	return editor.NodeInspection{NodeID: nodeID, NodeType: "math.Add", ExecutionCount: 1, PinValues: map[string]any{"result": 5}}
}
func (*fakeBridge) OnConnectionChanged(_ string, c editor.ConnectionChange) editor.ValidationResult {
	// Reject a self-connection as an example validation.
	if c.Connection.FromNode == c.Connection.ToNode {
		return editor.ValidationResult{Valid: false, Errors: []string{"self-connection"}}
	}
	return editor.ValidationResult{Valid: true}
}
func (*fakeBridge) OnPropertyChanged(_, _, _ string, _ any) editor.ValidationResult {
	return editor.ValidationResult{Valid: true}
}

func (*fakeBridge) ListAllNodes() []editor.NodeDescriptor {
	return []editor.NodeDescriptor{{TypeName: "math.Add", DisplayName: "Add", Category: "Math", Pins: []editor.PinDescriptor{
		{ID: "a", DataType: "float32", Direction: editor.Input},
		{ID: "out", DataType: "float32", Direction: editor.Output},
	}}}
}
func (b *fakeBridge) SearchNodes(query, _ string) []editor.NodeDescriptor {
	if query == "" {
		return nil
	}
	return b.ListAllNodes()
}
func (b *fakeBridge) GetNodeDescriptor(typeName string) (editor.NodeDescriptor, bool) {
	if typeName == "math.Add" {
		return b.ListAllNodes()[0], true
	}
	return editor.NodeDescriptor{}, false
}
func (b *fakeBridge) GetCompatibleNodes(string, editor.Direction) []editor.NodeDescriptor {
	return b.ListAllNodes()
}
func (*fakeBridge) GetTypeHierarchy(string) []string { return []string{"any"} }

func (b *fakeBridge) SetBreakpoint(_, nodeID string) error {
	if b.breakpoints == nil {
		b.breakpoints = map[string]bool{}
	}
	b.breakpoints[nodeID] = true
	return nil
}
func (b *fakeBridge) RemoveBreakpoint(_, nodeID string) error {
	delete(b.breakpoints, nodeID)
	return nil
}
func (b *fakeBridge) ListBreakpoints(string) []string {
	out := make([]string, 0, len(b.breakpoints))
	for n := range b.breakpoints {
		out = append(out, n)
	}
	return out
}
func (*fakeBridge) StepOver(string) editor.GraphExecutionFrame {
	return editor.GraphExecutionFrame{StepIndex: 1}
}
func (*fakeBridge) StepInto(string) editor.GraphExecutionFrame {
	return editor.GraphExecutionFrame{StepIndex: 2}
}
func (*fakeBridge) Continue(string) error { return nil }
func (*fakeBridge) GetExecutionTrace(string, int) []editor.GraphExecutionFrame {
	return []editor.GraphExecutionFrame{{NodeID: "n1"}}
}
func (*fakeBridge) GetVariableValues(string, uint64) map[string]any { return map[string]any{"hp": 100} }
func (*fakeBridge) GetPinValue(_, _, _ string, _ uint64) any        { return 42 }

// TestGraphBridgeContract exercises the bridge DTOs through a fake implementer,
// confirming the editor-facing contract is usable end to end.
func TestGraphBridgeContract(t *testing.T) {
	t.Parallel()
	var b editor.GraphEditorPlugin = &fakeBridge{}

	insp := b.OnNodeSelected("g", "n1")
	if insp.NodeType != "math.Add" || insp.PinValues["result"] != 5 {
		t.Errorf("inspection = %+v", insp)
	}
	// Validation rejects a self-connection (advisory, no mutation).
	bad := b.OnConnectionChanged("g", editor.ConnectionChange{
		Action: editor.ConnectionAdd, Connection: editor.Connection{FromNode: "n1", ToNode: "n1"},
	})
	if bad.Valid || len(bad.Errors) == 0 {
		t.Errorf("self-connection should be invalid: %+v", bad)
	}
	ok := b.OnConnectionChanged("g", editor.ConnectionChange{
		Connection: editor.Connection{FromNode: "n1", ToNode: "n2"},
	})
	if !ok.Valid {
		t.Errorf("valid connection rejected: %+v", ok)
	}

	dbg := b.(editor.GraphDebugger)
	if err := dbg.SetBreakpoint("g", "n1"); err != nil {
		t.Fatalf("SetBreakpoint: %v", err)
	}
	if got := dbg.ListBreakpoints("g"); len(got) != 1 || got[0] != "n1" {
		t.Errorf("breakpoints = %v", got)
	}

	reg := b.(editor.NodeRegistryQuery)
	if _, ok := reg.GetNodeDescriptor("math.Add"); !ok {
		t.Error("math.Add should be a known node")
	}
	if _, ok := reg.GetNodeDescriptor("nope"); ok {
		t.Error("unknown node should report ok=false")
	}
}
