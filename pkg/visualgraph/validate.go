package visualgraph

import (
	"errors"
	"fmt"
)

// Validation sentinel errors.
var (
	ErrUnknownNode   = errors.New("visualgraph: connection references unknown node")
	ErrUnknownPin    = errors.New("visualgraph: connection references unknown pin")
	ErrPinDirection  = errors.New("visualgraph: connection must go Output → Input")
	ErrKindMismatch  = errors.New("visualgraph: connection mixes Execution and Data pins")
	ErrTypeMismatch  = errors.New("visualgraph: data pin types are not assignable")
	ErrGraphCycle    = errors.New("visualgraph: data dependency cycle detected")
)

// assignable reports whether an output data type can feed an input data type.
// "any" (or empty) on the input accepts anything (INV-3 is structural; the
// editor refines with the TypeRegistry).
func assignable(outType, inType string) bool {
	return inType == "" || inType == "any" || outType == inType
}

// ValidateGraph checks every connection's endpoints, direction, kind, and (for
// Data connections) type compatibility (INV-3), then verifies the data
// dependency graph is acyclic so lazy pull terminates (INV-4). A graph that
// passes validation can be interpreted without structural runtime errors.
func ValidateGraph(g *GraphDefinition) error {
	for _, c := range g.Connections {
		from, ok := g.Node(c.FromNode)
		if !ok {
			return fmt.Errorf("%w: %q", ErrUnknownNode, c.FromNode)
		}
		to, ok := g.Node(c.ToNode)
		if !ok {
			return fmt.Errorf("%w: %q", ErrUnknownNode, c.ToNode)
		}
		fp, ok := from.Pin(c.FromPin)
		if !ok {
			return fmt.Errorf("%w: %s.%s", ErrUnknownPin, c.FromNode, c.FromPin)
		}
		tp, ok := to.Pin(c.ToPin)
		if !ok {
			return fmt.Errorf("%w: %s.%s", ErrUnknownPin, c.ToNode, c.ToPin)
		}
		if fp.Direction != Output || tp.Direction != Input {
			return fmt.Errorf("%w: %s.%s → %s.%s", ErrPinDirection, c.FromNode, c.FromPin, c.ToNode, c.ToPin)
		}
		if fp.Kind != tp.Kind {
			return fmt.Errorf("%w: %s.%s → %s.%s", ErrKindMismatch, c.FromNode, c.FromPin, c.ToNode, c.ToPin)
		}
		if fp.Kind == Data && !assignable(fp.DataType, tp.DataType) {
			return fmt.Errorf("%w: %s (%s) → %s (%s)", ErrTypeMismatch, c.FromPin, fp.DataType, c.ToPin, tp.DataType)
		}
	}
	return checkDataAcyclic(g)
}

// checkDataAcyclic detects a cycle in the data-dependency graph (edges point
// from a node to the nodes whose outputs it consumes). A cycle would make lazy
// data evaluation recurse forever (INV-4 / bounded execution).
func checkDataAcyclic(g *GraphDefinition) error {
	// deps[node] = nodes it pulls data from (upstream).
	deps := make(map[string][]string)
	for _, c := range g.Connections {
		from, _ := g.Node(c.FromNode)
		if p, ok := from.Pin(c.FromPin); ok && p.Kind == Data {
			deps[c.ToNode] = append(deps[c.ToNode], c.FromNode)
		}
	}
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int)
	var visit func(string) error
	visit = func(node string) error {
		color[node] = gray
		for _, up := range deps[node] {
			switch color[up] {
			case gray:
				return fmt.Errorf("%w: at %q", ErrGraphCycle, up)
			case white:
				if err := visit(up); err != nil {
					return err
				}
			}
		}
		color[node] = black
		return nil
	}
	for _, n := range g.Nodes {
		if color[n.ID] == white {
			if err := visit(n.ID); err != nil {
				return err
			}
		}
	}
	return nil
}
