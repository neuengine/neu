package definition

import (
	"fmt"
	"slices"
)

// validate runs all structure, type-reference (INV-4), and action checks on a
// decoded definition. Because every failure mode is caught here, a validated
// definition instantiates without error (INV-1).
func validate(def *Definition, types TypeResolver, actions *ActionRegistry) error {
	switch c := def.Content.(type) {
	case *UIDef:
		return validateUINode(&c.Root, types, actions)
	case *SceneDef:
		for i := range c.Entities {
			if err := validateSceneEntity(&c.Entities[i], types); err != nil {
				return err
			}
		}
		return nil
	case *FlowDef:
		return validateFlow(c, actions)
	case *TemplateDef:
		return validateComponents(c.Components, types)
	default:
		return ErrSchemaInvalid{Reason: "unknown content type"}
	}
}

func validateUINode(n *UINode, types TypeResolver, actions *ActionRegistry) error {
	if n.Type == "" {
		return ErrSchemaInvalid{Reason: "ui node missing type"}
	}
	if !types.ResolveType(n.Type) {
		return ErrUnknownType{Name: n.Type}
	}
	if n.OnClick != nil && !actions.Has(n.OnClick.Action) {
		return ErrUnknownAction{Type: n.OnClick.Action}
	}
	for i := range n.Children {
		if err := validateUINode(&n.Children[i], types, actions); err != nil {
			return err
		}
	}
	return nil
}

func validateSceneEntity(e *SceneEntity, types TypeResolver) error {
	if err := validateComponents(e.Components, types); err != nil {
		return err
	}
	for i := range e.Children {
		if err := validateSceneEntity(&e.Children[i], types); err != nil {
			return err
		}
	}
	return nil
}

func validateComponents(comps map[string]map[string]any, types TypeResolver) error {
	for name := range comps {
		if !types.ResolveType(name) {
			return ErrUnknownType{Name: name}
		}
	}
	return nil
}

func validateFlow(d *FlowDef, actions *ActionRegistry) error {
	if d.InitialState == "" {
		return ErrSchemaInvalid{Reason: "flow missing initial_state"}
	}
	if _, ok := d.States[d.InitialState]; !ok {
		return ErrSchemaInvalid{Reason: fmt.Sprintf("initial_state %q not present in states", d.InitialState)}
	}
	for _, s := range d.States {
		for _, a := range s.OnEnter {
			if !actions.Has(a.Action) {
				return ErrUnknownAction{Type: a.Action}
			}
		}
		for _, a := range s.OnExit {
			if !actions.Has(a.Action) {
				return ErrUnknownAction{Type: a.Action}
			}
		}
		for _, tr := range s.Transitions {
			if _, ok := d.States[tr.Target]; !ok {
				return ErrSchemaInvalid{Reason: fmt.Sprintf("transition target %q not present in states", tr.Target)}
			}
		}
	}
	return nil
}

// CheckIncludeDAG verifies that the include graph is acyclic via a three-colour
// DFS (INV-5). graph maps each definition path to the paths it includes; a back
// edge (revisiting a gray node) yields ErrDefinitionCycle. Iteration is sorted
// so the reported cycle node is deterministic.
func CheckIncludeDAG(graph map[string][]string) error {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(graph))

	var visit func(string) error
	visit = func(node string) error {
		color[node] = gray
		for _, dep := range graph[node] {
			switch color[dep] {
			case gray:
				return ErrDefinitionCycle{Path: dep}
			case white:
				if err := visit(dep); err != nil {
					return err
				}
			}
		}
		color[node] = black
		return nil
	}

	keys := make([]string, 0, len(graph))
	for k := range graph {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, node := range keys {
		if color[node] == white {
			if err := visit(node); err != nil {
				return err
			}
		}
	}
	return nil
}
