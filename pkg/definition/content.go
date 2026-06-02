package definition

import "encoding/json"

// UINode is a node in a UI definition tree. Type names a UI widget ("Node",
// "Text", "Button", ...); Style is a flat property map mirroring the ui.Style
// fields; Children nest to form the ChildOf hierarchy.
type UINode struct {
	Style    map[string]any `json:"style,omitempty"`
	OnClick  *Action        `json:"on_click,omitempty"`
	Type     string         `json:"type"`
	ID       string         `json:"id,omitempty"`
	Value    string         `json:"value,omitempty"`
	Children []UINode       `json:"children,omitempty"`
}

// UIDef is a ui definition's content: a single root node tree.
type UIDef struct {
	Root UINode `json:"root"`
}

// SceneEntity is one entity in a scene definition: named, with a component map
// (type name → value) and nested children.
type SceneEntity struct {
	Components map[string]map[string]any `json:"components"`
	Name       string                    `json:"name"`
	Template   string                    `json:"template,omitempty"`
	Children   []SceneEntity             `json:"children,omitempty"`
}

// SceneDef is a scene definition's content: a list of root entities.
type SceneDef struct {
	Entities []SceneEntity `json:"entities"`
}

// FlowTransition is an event→target edge in a flow state graph.
type FlowTransition struct {
	Event  string `json:"event"`
	Target string `json:"target"`
}

// FlowStateDef is one state in a flow definition.
type FlowStateDef struct {
	UI          string           `json:"ui,omitempty"`
	Scene       string           `json:"scene,omitempty"`
	Systems     []string         `json:"systems,omitempty"`
	OnEnter     []Action         `json:"on_enter,omitempty"`
	OnExit      []Action         `json:"on_exit,omitempty"`
	Transitions []FlowTransition `json:"transitions,omitempty"`
	Overlay     bool             `json:"overlay,omitempty"`
}

// FlowDef is a flow definition's content: an initial state plus a state graph.
type FlowDef struct {
	States       map[string]FlowStateDef `json:"states"`
	InitialState string                  `json:"initial_state"`
}

// TemplateDef is a reusable entity blueprint (prefab). It is referenced by other
// definitions, not instantiated directly. Overridable lists editor-hint fields.
type TemplateDef struct {
	Name        string                    `json:"name"`
	Components  map[string]map[string]any `json:"components"`
	Overridable []string                  `json:"overridable,omitempty"`
}

// Action is a declarative verb connecting data to engine behavior (L1 §4.6).
// The "action" key names the verb; all other keys become Params.
type Action struct {
	Params map[string]any
	Action string
}

// UnmarshalJSON captures the "action" verb and folds the remaining keys into
// Params, so an action object like {"action":"transition","target":"x"} decodes
// to {Action:"transition", Params:{"target":"x"}}.
func (a *Action) UnmarshalJSON(data []byte) error {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	if v, ok := m["action"].(string); ok {
		a.Action = v
	}
	delete(m, "action")
	a.Params = m
	return nil
}
