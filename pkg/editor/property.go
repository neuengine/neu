package editor

// PropertyInfo describes a single inspectable field of a component.
type PropertyInfo struct {
	Name        string
	DisplayName string
	TypeHint    string // "float32", "Vec3", "Handle<Image>", ...
	Value       any
	Editable    bool
	Range       *Range // nil ⇒ no range constraint (L1 Option[Range] → comma-ok equivalent)
}

// Range constrains a numeric PropertyInfo to [Min, Max].
type Range struct{ Min, Max float64 }

// PropertyList is an ordered collection of PropertyInfo entries for display
// in the inspector panel.
type PropertyList []PropertyInfo

// EditorProperty is a simplified property descriptor used in definition node
// inspection (see DefinitionEditorPlugin.GetInspectorProperties).
type EditorProperty struct {
	Name  string
	Value any
}
