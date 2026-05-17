package editor

// DefinitionType identifies the kind of a definition node (e.g. "BehaviorGraph",
// "AnimationClip"). The editor uses this to dispatch to the correct plugin.
type DefinitionType string

// DefinitionNode is an opaque handle to a node within a graph definition.
// The engine resolves the concrete type at runtime.
type DefinitionNode interface {
	// ID returns the stable string identifier of this node.
	ID() string
	// Type returns the definition type discriminator.
	Type() DefinitionType
}
