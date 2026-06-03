package grapheditor

import (
	"fmt"

	"github.com/neuengine/neu/pkg/editor"
	vg "github.com/neuengine/neu/pkg/visualgraph"
)

// GraphProvider resolves a graph definition by ID. The host app supplies it
// (e.g. from the loaded-graph registry), so the bridge stays decoupled from how
// and where graphs are stored.
type GraphProvider func(graphID string) (*vg.GraphDefinition, bool)

// EditorPlugin implements editor.GraphEditorPlugin, mapping live-edit callbacks
// onto graph validation and the shared DebugStore. Validation is advisory and
// never mutates the graph: an edit can be rejected before it is committed
// (§4.1's pre-commit guarantee).
type EditorPlugin struct {
	graphs GraphProvider
	store  *DebugStore
}

// NewEditorPlugin builds the editor gateway over a graph provider and the shared
// debug store.
func NewEditorPlugin(graphs GraphProvider, store *DebugStore) *EditorPlugin {
	return &EditorPlugin{graphs: graphs, store: store}
}

// OnGraphOpened prepares debug state for the graph the editor opened.
func (p *EditorPlugin) OnGraphOpened(graphID string) { p.store.Open(graphID) }

// OnGraphClosed releases the graph's debug state.
func (p *EditorPlugin) OnGraphClosed(graphID string) { p.store.Close(graphID) }

// OnNodeSelected returns the selected node's static type plus its current runtime
// values from the debug store.
func (p *EditorPlugin) OnNodeSelected(graphID, nodeID string) editor.NodeInspection {
	insp := editor.NodeInspection{NodeID: nodeID}
	if g, ok := p.resolve(graphID); ok {
		if n, ok := g.Node(nodeID); ok {
			insp.NodeType = n.Type
		}
	}
	count, lastErr, pins := p.store.Inspect(graphID, nodeID)
	insp.ExecutionCount = count
	insp.LastError = lastErr
	insp.PinValues = pins
	return insp
}

// OnConnectionChanged validates an added/removed connection before commit.
// Removals are always structurally safe; additions are checked exactly the way
// ValidateGraph checks an existing connection.
func (p *EditorPlugin) OnConnectionChanged(graphID string, change editor.ConnectionChange) editor.ValidationResult {
	if change.Action == editor.ConnectionRemove {
		return editor.ValidationResult{Valid: true}
	}
	g, ok := p.resolve(graphID)
	if !ok {
		return invalid("graph %q is not loaded", graphID)
	}
	return validateConnection(g, change.Connection)
}

// OnPropertyChanged validates a node-property edit before commit. An unknown
// property is a warning (the node may accept dynamic properties), not a hard
// rejection; a missing node or graph is an error.
func (p *EditorPlugin) OnPropertyChanged(graphID, nodeID, property string, value any) editor.ValidationResult {
	g, ok := p.resolve(graphID)
	if !ok {
		return invalid("graph %q is not loaded", graphID)
	}
	n, ok := g.Node(nodeID)
	if !ok {
		return invalid("node %q not found in graph %q", nodeID, graphID)
	}
	if _, known := n.Properties[property]; !known {
		return editor.ValidationResult{
			Valid:    true,
			Warnings: []string{fmt.Sprintf("property %q is not declared on node %q", property, nodeID)},
		}
	}
	return editor.ValidationResult{Valid: true}
}

func (p *EditorPlugin) resolve(graphID string) (*vg.GraphDefinition, bool) {
	if p.graphs == nil {
		return nil, false
	}
	return p.graphs(graphID)
}

// invalid builds a rejecting ValidationResult with one formatted error.
func invalid(format string, args ...any) editor.ValidationResult {
	return editor.ValidationResult{Valid: false, Errors: []string{fmt.Sprintf(format, args...)}}
}

// validateConnection checks a single proposed connection the way ValidateGraph
// checks each existing one: endpoints exist, Output → Input direction, matching
// kind, and (for data pins) assignable types. Pure — never mutates the graph.
func validateConnection(g *vg.GraphDefinition, c editor.Connection) editor.ValidationResult {
	from, ok := g.Node(c.FromNode)
	if !ok {
		return invalid("from-node %q not found", c.FromNode)
	}
	to, ok := g.Node(c.ToNode)
	if !ok {
		return invalid("to-node %q not found", c.ToNode)
	}
	fp, ok := from.Pin(c.FromPin)
	if !ok {
		return invalid("from-pin %s.%s not found", c.FromNode, c.FromPin)
	}
	tp, ok := to.Pin(c.ToPin)
	if !ok {
		return invalid("to-pin %s.%s not found", c.ToNode, c.ToPin)
	}
	if fp.Direction != vg.Output || tp.Direction != vg.Input {
		return invalid("connection must go Output → Input (%s.%s → %s.%s)",
			c.FromNode, c.FromPin, c.ToNode, c.ToPin)
	}
	if fp.Kind != tp.Kind {
		return invalid("connection mixes Execution and Data pins (%s.%s → %s.%s)",
			c.FromNode, c.FromPin, c.ToNode, c.ToPin)
	}
	if fp.Kind == vg.Data && !assignableType(fp.DataType, tp.DataType) {
		return invalid("data types not assignable: %s (%s) → %s (%s)",
			c.FromPin, fp.DataType, c.ToPin, tp.DataType)
	}
	return editor.ValidationResult{Valid: true}
}

var _ editor.GraphEditorPlugin = (*EditorPlugin)(nil)
