package visualgraph

import (
	"errors"
	"fmt"
)

// ErrStepLimit is returned when a single execution pass exceeds the node-step
// limit (INV-2: bounded execution — a runaway graph never freezes the frame).
var ErrStepLimit = errors.New("visualgraph: node-step limit exceeded")

// ErrNoHandler is returned when a node's type has no registered behaviour.
var ErrNoHandler = errors.New("visualgraph: no handler for node type")

// CommandSink receives the side effects of Action nodes. All world mutations go
// through it (INV-1) — the engine adapts it to the ECS CommandBuffer; tests use
// a recorder.
type CommandSink interface {
	Emit(op string, args map[string]any)
}

// NodeBehavior is a node type's runtime logic. A pure Data node sets Eval; an
// Execution node (Action/Flow/Event) sets Exec and returns the execution-output
// pin to follow next ("" ends the chain).
type NodeBehavior struct {
	Eval func(props, inputs map[string]any) map[string]any
	Exec func(props, inputs map[string]any, sink CommandSink) (nextPin string, err error)
}

// ExecutionFrame is a neutral record of one execution step, emitted to a
// TraceRecorder. It is self-contained (no editor/protocol import) so the
// debugger bridge maps it onto its own frame type — keeping this package free of
// any editor/IPC dependency.
type ExecutionFrame struct {
	PinValues map[string]any
	NodeID    string
	NodeType  string
	StepIndex uint32
}

// TraceRecorder observes execution steps for debugging. It is passed into
// [Interpreter.RunTraced] (not stored on the interpreter) so concurrent passes
// for different graphs/entities each record into their own recorder. A nil
// recorder disables tracing at zero cost.
type TraceRecorder interface {
	RecordStep(ExecutionFrame)
}

// DefaultStepLimit bounds a pass when none is configured (L1 §4.4).
const DefaultStepLimit = 10000

// Interpreter executes validated graphs. It is safe to reuse across graphs;
// per-pass state (the data-evaluation memo) is local to Run.
type Interpreter struct {
	handlers  map[string]NodeBehavior
	stepLimit int
}

// NewInterpreter returns an interpreter with the built-in node set registered.
func NewInterpreter(stepLimit int) *Interpreter {
	if stepLimit <= 0 {
		stepLimit = DefaultStepLimit
	}
	in := &Interpreter{handlers: make(map[string]NodeBehavior), stepLimit: stepLimit}
	registerBuiltins(in)
	return in
}

// Register adds or replaces a node type's behaviour.
func (in *Interpreter) Register(typeName string, b NodeBehavior) {
	in.handlers[typeName] = b
}

// run holds per-pass execution state.
type run struct {
	in    *Interpreter
	g     *GraphDefinition
	sink  CommandSink
	rec   TraceRecorder
	memo  map[string]map[string]any // node ID → evaluated data outputs (lazy, once per pass)
	steps int
}

// Run executes the graph starting at entryNodeID, following the execution chain
// and lazily evaluating data inputs. Side effects are emitted to sink (INV-1).
// Execution is bounded by the step limit (INV-2) and deterministic given the
// same graph + sink (INV-4).
func (in *Interpreter) Run(g *GraphDefinition, entryNodeID string, sink CommandSink) error {
	return in.RunTraced(g, entryNodeID, sink, nil)
}

// RunTraced behaves like Run but reports each executed step to rec (a debugger
// hook). A nil rec is identical to Run, so the trace path is zero-overhead when
// no editor is attached.
func (in *Interpreter) RunTraced(g *GraphDefinition, entryNodeID string, sink CommandSink, rec TraceRecorder) error {
	entry, ok := g.Node(entryNodeID)
	if !ok {
		return fmt.Errorf("%w: entry %q", ErrUnknownNode, entryNodeID)
	}
	r := &run{in: in, g: g, sink: sink, rec: rec, memo: make(map[string]map[string]any)}
	return r.execChain(entry)
}

// execChain follows execution-pin connections from node until the chain ends or
// the step limit is hit.
func (r *run) execChain(node Node) error {
	cur := node
	for {
		r.steps++
		if r.steps > r.in.stepLimit {
			return ErrStepLimit
		}
		b, ok := r.in.handlers[cur.Type]
		if !ok || b.Exec == nil {
			return fmt.Errorf("%w: %q (node %s)", ErrNoHandler, cur.Type, cur.ID)
		}
		inputs := r.pullDataInputs(cur)
		if r.rec != nil {
			r.rec.RecordStep(ExecutionFrame{
				PinValues: inputs,
				NodeID:    cur.ID,
				NodeType:  cur.Type,
				StepIndex: uint32(r.steps),
			})
		}
		nextPin, err := b.Exec(cur.Properties, inputs, r.sink)
		if err != nil {
			return err
		}
		if nextPin == "" {
			return nil // chain end
		}
		nextNode, ok := r.execTarget(cur.ID, nextPin)
		if !ok {
			return nil // no downstream node on that pin — chain end
		}
		cur = nextNode
	}
}

// pullDataInputs gathers a node's Data input values, lazily evaluating upstream
// data nodes; an unconnected input falls back to its pin default.
func (r *run) pullDataInputs(node Node) map[string]any {
	inputs := make(map[string]any)
	for _, pin := range node.Pins {
		if pin.Direction != Input || pin.Kind != Data {
			continue
		}
		if fromNode, fromPin, ok := r.dataSource(node.ID, pin.ID); ok {
			out := r.evalData(fromNode)
			inputs[pin.ID] = out[fromPin]
		} else {
			inputs[pin.ID] = pin.DefaultValue
		}
	}
	return inputs
}

// evalData lazily evaluates a data node's outputs, memoized per pass (the data
// graph is acyclic — guaranteed by ValidateGraph — so this terminates).
func (r *run) evalData(nodeID string) map[string]any {
	if out, done := r.memo[nodeID]; done {
		return out
	}
	node, ok := r.g.Node(nodeID)
	if !ok {
		return nil
	}
	b := r.in.handlers[node.Type]
	var out map[string]any
	if b.Eval != nil {
		out = b.Eval(node.Properties, r.pullDataInputs(node))
	} else {
		out = map[string]any{}
	}
	r.memo[nodeID] = out
	return out
}

// dataSource returns the (node, pin) feeding a Data input pin, if connected.
func (r *run) dataSource(toNode, toPin string) (string, string, bool) {
	for _, c := range r.g.Connections {
		if c.ToNode == toNode && c.ToPin == toPin {
			if from, ok := r.g.Node(c.FromNode); ok {
				if p, ok := from.Pin(c.FromPin); ok && p.Kind == Data {
					return c.FromNode, c.FromPin, true
				}
			}
		}
	}
	return "", "", false
}

// execTarget returns the node connected to an execution-output pin, if any.
func (r *run) execTarget(fromNode, fromPin string) (Node, bool) {
	for _, c := range r.g.Connections {
		if c.FromNode == fromNode && c.FromPin == fromPin {
			if from, ok := r.g.Node(fromNode); ok {
				if p, ok := from.Pin(fromPin); ok && p.Kind == Execution {
					return r.g.Node(c.ToNode)
				}
			}
		}
	}
	return Node{}, false
}

// Steps reports how many nodes the last Run executed (test/diagnostic hook).
func (in *Interpreter) handlerCount() int { return len(in.handlers) }
