package definition

import "slices"

// Instantiate emits the commands that realize a validated definition. It writes
// only to the CommandSink — never the world directly (INV-2). It is infallible:
// validation already proved every type and reference resolves (INV-1), so there
// are no error branches here.
func Instantiate(def *Definition, sink CommandSink) {
	switch c := def.Content.(type) {
	case *UIDef:
		instantiateUINode(&c.Root, sink, noParent)
	case *SceneDef:
		for i := range c.Entities {
			instantiateSceneEntity(&c.Entities[i], sink, noParent)
		}
	case *FlowDef:
		// Entering the initial state runs its on_enter actions.
		if s, ok := c.States[c.InitialState]; ok {
			for _, a := range s.OnEnter {
				sink.RunAction(a)
			}
		}
	case *TemplateDef:
		// Templates are blueprints, referenced by other definitions — not
		// instantiated directly.
	}
}

// noParent marks a root entity (no ChildOf).
const noParent EntityRef = -1

func instantiateUINode(n *UINode, sink CommandSink, parent EntityRef) EntityRef {
	e := sink.SpawnEntity(n.ID)
	sink.InsertComponent(e, n.Type, n.Style)
	if n.Value != "" {
		sink.InsertComponent(e, "Text", map[string]any{"value": n.Value})
	}
	if parent != noParent {
		sink.SetParent(e, parent)
	}
	for i := range n.Children {
		instantiateUINode(&n.Children[i], sink, e)
	}
	return e
}

func instantiateSceneEntity(e *SceneEntity, sink CommandSink, parent EntityRef) EntityRef {
	ent := sink.SpawnEntity(e.Name)
	// Deterministic component order (maps iterate randomly).
	names := make([]string, 0, len(e.Components))
	for name := range e.Components {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		sink.InsertComponent(ent, name, e.Components[name])
	}
	if parent != noParent {
		sink.SetParent(ent, parent)
	}
	for i := range e.Children {
		instantiateSceneEntity(&e.Children[i], sink, ent)
	}
	return ent
}
