// Package visualgraph provides the engine-side runtime for node-based visual
// programming: the graph data model, node registry, load-time validation, and
// the interpreter. The GUI editor lives in the external editor repository and
// drives this through pkg/editor interfaces ("the door"); the runtime here
// loads and executes graph definitions headlessly, with or without an editor.
//
// Bootstrap: l1-visual-graph-system + l2-visual-graph-editor-bridge Draft
// (Phase 6 Track P, C29 gate open).
package visualgraph

// Direction is a pin's data-flow direction.
type Direction uint8

const (
	Input Direction = iota
	Output
)

// Kind distinguishes control-flow (Execution) pins from value (Data) pins.
type Kind uint8

const (
	// Execution pins order node execution (the imperative control chain).
	Execution Kind = iota
	// Data pins carry typed values, evaluated lazily when a consumer pulls them.
	Data
)

// Pin is an input or output socket on a node.
type Pin struct {
	DefaultValue any
	ID           string
	Name         string
	DataType     string
	Direction    Direction
	Kind         Kind
}

// Node is one node in a graph: a registered type plus its pins and constant
// properties.
type Node struct {
	Properties map[string]any
	ID         string
	Type       string
	Pins       []Pin
}

// Pin returns the pin with the given id, or false.
func (n Node) Pin(id string) (Pin, bool) {
	for _, p := range n.Pins {
		if p.ID == id {
			return p, true
		}
	}
	return Pin{}, false
}

// Connection links an output pin to an input pin.
type Connection struct {
	FromNode string
	FromPin  string
	ToNode   string
	ToPin    string
}

// VariableDecl is a graph-local variable declaration.
type VariableDecl struct {
	Default any
	Name    string
	Type    string
}

// GraphDefinition is the runtime graph model (the parsed "graph" definition).
type GraphDefinition struct {
	ID          string
	Name        string
	Variables   []VariableDecl
	Nodes       []Node
	Connections []Connection
}

// Node returns the node with the given id, or false.
func (g *GraphDefinition) Node(id string) (Node, bool) {
	for _, n := range g.Nodes {
		if n.ID == id {
			return n, true
		}
	}
	return Node{}, false
}
