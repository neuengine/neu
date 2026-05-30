package definition

import (
	"encoding/json"
	"fmt"
	"io"
)

// Load reads, decodes, and validates a definition from r (INV-1: a definition
// returned without error is guaranteed to instantiate without runtime errors).
func Load(r io.Reader, types TypeResolver, actions *ActionRegistry) (Definition, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Definition{}, ErrSchemaInvalid{Reason: "read", Err: err}
	}
	return Decode(data, types, actions)
}

// Decode parses and validates definition bytes — the testable core of Load.
func Decode(data []byte, types TypeResolver, actions *ActionRegistry) (Definition, error) {
	var env rawEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return Definition{}, ErrSchemaInvalid{Reason: "json", Err: err}
	}
	kind, ok := ParseKind(env.Definition)
	if !ok {
		return Definition{}, ErrSchemaInvalid{
			Reason: fmt.Sprintf("unknown root kind %q (expected exactly one of ui/scene/flow/template)", env.Definition),
		}
	}
	content, includes, err := decodeContent(kind, env.Content)
	if err != nil {
		return Definition{}, err
	}
	def := Definition{
		Kind:     kind,
		Version:  env.Version,
		Metadata: env.Metadata,
		Includes: includes,
		Content:  content,
	}
	if err := validate(&def, types, actions); err != nil {
		return Definition{}, err
	}
	return def, nil
}

// decodeContent unmarshals the kind-specific content and returns the asset paths
// it references (the include-DAG edges, INV-5).
func decodeContent(kind Kind, raw json.RawMessage) (content any, includes []string, err error) {
	switch kind {
	case KindUI:
		var d UIDef
		if err := json.Unmarshal(raw, &d); err != nil {
			return nil, nil, ErrSchemaInvalid{Reason: "ui content", Err: err}
		}
		return &d, nil, nil
	case KindScene:
		var d SceneDef
		if err := json.Unmarshal(raw, &d); err != nil {
			return nil, nil, ErrSchemaInvalid{Reason: "scene content", Err: err}
		}
		return &d, sceneIncludes(d.Entities, nil), nil
	case KindFlow:
		var d FlowDef
		if err := json.Unmarshal(raw, &d); err != nil {
			return nil, nil, ErrSchemaInvalid{Reason: "flow content", Err: err}
		}
		return &d, flowIncludes(d), nil
	case KindTemplate:
		var d TemplateDef
		if err := json.Unmarshal(raw, &d); err != nil {
			return nil, nil, ErrSchemaInvalid{Reason: "template content", Err: err}
		}
		return &d, nil, nil
	default:
		return nil, nil, ErrSchemaInvalid{Reason: "unhandled kind"}
	}
}

// sceneIncludes collects template references from a scene entity tree.
func sceneIncludes(entities []SceneEntity, acc []string) []string {
	for _, e := range entities {
		if e.Template != "" {
			acc = append(acc, e.Template)
		}
		acc = sceneIncludes(e.Children, acc)
	}
	return acc
}

// flowIncludes collects the ui/scene asset references from a flow's states.
func flowIncludes(d FlowDef) []string {
	var acc []string
	for _, s := range d.States {
		if s.UI != "" {
			acc = append(acc, s.UI)
		}
		if s.Scene != "" {
			acc = append(acc, s.Scene)
		}
	}
	return acc
}
