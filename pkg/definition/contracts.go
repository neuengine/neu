package definition

// TypeResolver reports whether a type name is registered. The engine's
// TypeRegistry implements it; tests provide a stub. Defined here (at the
// consumer) so the definition package stays decoupled from the registry
// implementation (INV-4 type-reference checks go through this).
type TypeResolver interface {
	ResolveType(name string) bool
}

// EntityRef is an opaque handle to an entity created during one instantiation.
// It is local to a CommandSink session and maps to a real ECS entity by the
// adapter that wires definitions to commands.
type EntityRef int

// CommandSink is the narrow command surface interpreters emit to (INV-2: no
// *World access). The engine adapter forwards these to ecs.Commands; tests use
// a recording implementation to prove no direct world mutation occurs.
type CommandSink interface {
	// SpawnEntity reserves a new entity, returning a session-local reference.
	SpawnEntity(name string) EntityRef
	// InsertComponent attaches a component (by registered type name) with a
	// decoded value map to an entity.
	InsertComponent(e EntityRef, typeName string, value map[string]any)
	// SetParent establishes a ChildOf relationship.
	SetParent(child, parent EntityRef)
	// RunAction queues a declarative action (transition, spawn, play_audio, ...).
	RunAction(a Action)
}

// ActionRegistry tracks the set of known action types (INV: unknown action ⇒
// validation error). Plugins register custom actions here.
type ActionRegistry struct {
	known map[string]struct{}
}

// NewActionRegistry returns a registry pre-populated with the built-in actions
// (L1 §4.6).
func NewActionRegistry() *ActionRegistry {
	r := &ActionRegistry{known: make(map[string]struct{})}
	for _, a := range []string{
		"transition", "quit", "spawn", "despawn", "despawn_scene",
		"load_assets", "play_audio", "set_resource", "send_event", "log",
	} {
		r.known[a] = struct{}{}
	}
	return r
}

// Register adds a custom action type.
func (r *ActionRegistry) Register(actionType string) { r.known[actionType] = struct{}{} }

// Has reports whether an action type is registered.
func (r *ActionRegistry) Has(actionType string) bool {
	_, ok := r.known[actionType]
	return ok
}
