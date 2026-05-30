// Package definition implements the JSON declarative layer: a typed Envelope
// whose "definition" discriminator selects a ui/scene/flow/template interpreter.
// Files are validated up front (structure + type references + include-DAG
// acyclicity) so a validated definition always instantiates (INV-1), and
// interpreters emit only commands (INV-2) — never direct world access.
//
// Bootstrap: l2-definition-system-go Draft (Phase 6 Track A, C29 gate open).
package definition

import "encoding/json"

// Kind is the root definition type (the "definition" envelope field).
type Kind uint8

const (
	// KindUnknown is the zero value for an unrecognized definition kind.
	KindUnknown Kind = iota
	KindUI
	KindScene
	KindFlow
	KindTemplate
)

// ParseKind maps the envelope discriminator string to a Kind.
func ParseKind(s string) (Kind, bool) {
	switch s {
	case "ui":
		return KindUI, true
	case "scene":
		return KindScene, true
	case "flow":
		return KindFlow, true
	case "template":
		return KindTemplate, true
	default:
		return KindUnknown, false
	}
}

// String renders the kind. Total switch.
func (k Kind) String() string {
	switch k {
	case KindUI:
		return "ui"
	case KindScene:
		return "scene"
	case KindFlow:
		return "flow"
	case KindTemplate:
		return "template"
	default:
		return "unknown"
	}
}

// Metadata is the human-readable header used by the editor and asset browser.
type Metadata struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

// rawEnvelope is the on-disk JSON shape before validation.
type rawEnvelope struct {
	Definition string          `json:"definition"`
	Version    string          `json:"version"`
	Metadata   Metadata        `json:"metadata"`
	Content    json.RawMessage `json:"content"`
}

// Definition is a validated, ready-to-instantiate asset. Content holds the
// kind-specific model (*UIDef, *SceneDef, *FlowDef, or *TemplateDef). Includes
// lists the asset paths this definition references (the INV-5 DAG edges).
type Definition struct {
	Kind     Kind
	Version  string
	Metadata Metadata
	Includes []string
	Content  any
}
