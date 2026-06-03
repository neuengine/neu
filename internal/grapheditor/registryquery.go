package grapheditor

import (
	"github.com/neuengine/neu/pkg/editor"
	vg "github.com/neuengine/neu/pkg/visualgraph"
)

// RegistryQuery adapts a *visualgraph.NodeRegistry to editor.NodeRegistryQuery,
// translating the runtime descriptors into the editor-local DTOs so no engine
// type crosses the boundary. It powers the editor's node palette and the
// drag-to-connect compatibility hints (§4.2).
type RegistryQuery struct {
	reg *vg.NodeRegistry
}

// NewRegistryQuery wraps a node registry as an editor query gateway.
func NewRegistryQuery(reg *vg.NodeRegistry) *RegistryQuery {
	return &RegistryQuery{reg: reg}
}

// assignableType mirrors visualgraph's structural assignability (unexported
// there): an empty or "any" input accepts anything, otherwise types must match.
func assignableType(outType, inType string) bool {
	return inType == "" || inType == "any" || outType == inType
}

func toEditorDirection(d vg.Direction) editor.Direction {
	if d == vg.Output {
		return editor.Output
	}
	return editor.Input
}

func toEditorPin(p vg.PinDescriptor) editor.PinDescriptor {
	return editor.PinDescriptor{
		ID:        p.ID,
		Name:      p.Name,
		DataType:  p.DataType,
		Direction: toEditorDirection(p.Direction),
		Execution: p.Kind == vg.Execution,
	}
}

func toEditorNode(d vg.NodeDescriptor) editor.NodeDescriptor {
	pins := make([]editor.PinDescriptor, len(d.Pins))
	for i, p := range d.Pins {
		pins[i] = toEditorPin(p)
	}
	return editor.NodeDescriptor{
		TypeName:    d.TypeName,
		DisplayName: d.DisplayName,
		Category:    d.Category,
		Description: d.Description,
		Pins:        pins,
	}
}

func toEditorNodes(ds []vg.NodeDescriptor) []editor.NodeDescriptor {
	out := make([]editor.NodeDescriptor, len(ds))
	for i, d := range ds {
		out[i] = toEditorNode(d)
	}
	return out
}

// ListAllNodes returns every registered node type for the palette (deterministic
// order — the registry sorts by type name).
func (q *RegistryQuery) ListAllNodes() []editor.NodeDescriptor {
	return toEditorNodes(q.reg.List())
}

// SearchNodes filters the palette by a free-text query and/or category. An empty
// query matches all node types; an empty category matches all categories.
func (q *RegistryQuery) SearchNodes(query, category string) []editor.NodeDescriptor {
	var src []vg.NodeDescriptor
	if query == "" {
		src = q.reg.List()
	} else {
		src = q.reg.Search(query)
	}
	out := make([]editor.NodeDescriptor, 0, len(src))
	for _, d := range src {
		if category != "" && d.Category != category {
			continue
		}
		out = append(out, toEditorNode(d))
	}
	return out
}

// GetNodeDescriptor returns the descriptor for a type, or ok=false.
func (q *RegistryQuery) GetNodeDescriptor(typeName string) (editor.NodeDescriptor, bool) {
	d, ok := q.reg.Get(typeName)
	if !ok {
		return editor.NodeDescriptor{}, false
	}
	return toEditorNode(d), true
}

// GetCompatibleNodes returns nodes that have a data pin able to connect to a
// dragged pin of pinType in direction dir. A drag from an Output pin seeks nodes
// with an assignable Input pin (and vice versa); execution pins are not
// type-matched and so are excluded.
func (q *RegistryQuery) GetCompatibleNodes(pinType string, dir editor.Direction) []editor.NodeDescriptor {
	want := editor.Input
	if dir == editor.Input {
		want = editor.Output
	}
	var out []editor.NodeDescriptor
	for _, d := range q.reg.List() {
		for _, p := range d.Pins {
			ep := toEditorPin(p)
			if ep.Direction != want || ep.Execution {
				continue
			}
			// A dragged Output feeds the candidate Input; a dragged Input is fed
			// by the candidate Output. assignableType is (outType, inType).
			var ok bool
			if dir == editor.Output {
				ok = assignableType(pinType, ep.DataType)
			} else {
				ok = assignableType(ep.DataType, pinType)
			}
			if ok {
				out = append(out, toEditorNode(d))
				break
			}
		}
	}
	return out
}

// GetTypeHierarchy returns the types assignable from typeName. The structural
// rule mirrors visualgraph: a concrete type plus the universal "any"; the editor
// refines this with its own TypeRegistry.
func (q *RegistryQuery) GetTypeHierarchy(typeName string) []string {
	if typeName == "" || typeName == "any" {
		return []string{"any"}
	}
	return []string{typeName, "any"}
}

var _ editor.NodeRegistryQuery = (*RegistryQuery)(nil)
