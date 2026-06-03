package grapheditor

import (
	"fmt"

	"github.com/neuengine/neu/pkg/editor"
	vg "github.com/neuengine/neu/pkg/visualgraph"
)

// Debugger implements editor.GraphDebugger over the shared DebugStore. The
// engine's interpreter is non-invasive: it records an execution trace as it
// runs, and the debugger inspects / replays that trace rather than pausing live
// frames — a debugger must never stall the simulation loop. Stepping therefore
// walks the recorded trace via the store's replay cursor.
type Debugger struct {
	graphs GraphProvider
	store  *DebugStore
}

// NewDebugger builds the debugger over a graph provider and the shared store.
func NewDebugger(graphs GraphProvider, store *DebugStore) *Debugger {
	return &Debugger{graphs: graphs, store: store}
}

// SetBreakpoint marks a node as a breakpoint. Errors if the node is unknown or
// the graph is not open for debugging.
func (d *Debugger) SetBreakpoint(graphID, nodeID string) error {
	if !d.nodeExists(graphID, nodeID) {
		return fmt.Errorf("grapheditor: node %q not found in graph %q", nodeID, graphID)
	}
	if !d.store.SetBreakpoint(graphID, nodeID) {
		return fmt.Errorf("grapheditor: graph %q is not open for debugging", graphID)
	}
	return nil
}

// RemoveBreakpoint clears a node breakpoint, erroring if none was set.
func (d *Debugger) RemoveBreakpoint(graphID, nodeID string) error {
	if !d.store.RemoveBreakpoint(graphID, nodeID) {
		return fmt.Errorf("grapheditor: no breakpoint at %q in graph %q", nodeID, graphID)
	}
	return nil
}

// ListBreakpoints returns the breakpointed node IDs (sorted).
func (d *Debugger) ListBreakpoints(graphID string) []string {
	return d.store.ListBreakpoints(graphID)
}

// StepOver advances the replay cursor one recorded frame. A zero-value frame is
// returned once the trace is exhausted.
func (d *Debugger) StepOver(graphID string) editor.GraphExecutionFrame {
	f, _ := d.store.StepNext(graphID)
	return f
}

// StepInto behaves like StepOver until subgraph nodes are supported; the trace
// is already flattened across any subgraph boundary.
func (d *Debugger) StepInto(graphID string) editor.GraphExecutionFrame {
	return d.StepOver(graphID)
}

// Continue stops stepping by rewinding the replay cursor to the trace start.
func (d *Debugger) Continue(graphID string) error {
	d.store.ResetCursor(graphID)
	return nil
}

// GetExecutionTrace returns up to maxFrames recent execution frames.
func (d *Debugger) GetExecutionTrace(graphID string, maxFrames int) []editor.GraphExecutionFrame {
	return d.store.Trace(graphID, maxFrames)
}

// GetVariableValues returns the graph's declared variables with their defaults.
// Per-entity runtime variable tracking is deferred to interpreter instrumentation
// (T-6P04·rem.2); entityID is accepted now for a forward-compatible signature.
func (d *Debugger) GetVariableValues(graphID string, entityID uint64) map[string]any {
	g, ok := d.resolve(graphID)
	if !ok {
		return nil
	}
	out := make(map[string]any, len(g.Variables))
	for _, v := range g.Variables {
		out[v.Name] = v.Default
	}
	return out
}

// GetPinValue returns the latest recorded value at a node's pin, or nil if the
// node has not executed (or the pin carried no value). entityID is accepted for
// a forward-compatible signature; per-entity isolation lands with the trace hook.
func (d *Debugger) GetPinValue(graphID, nodeID, pinID string, entityID uint64) any {
	_, _, pins := d.store.Inspect(graphID, nodeID)
	if pins == nil {
		return nil
	}
	return pins[pinID]
}

func (d *Debugger) resolve(graphID string) (*vg.GraphDefinition, bool) {
	if d.graphs == nil {
		return nil, false
	}
	return d.graphs(graphID)
}

func (d *Debugger) nodeExists(graphID, nodeID string) bool {
	g, ok := d.resolve(graphID)
	if !ok {
		return false
	}
	_, ok = g.Node(nodeID)
	return ok
}

var _ editor.GraphDebugger = (*Debugger)(nil)
