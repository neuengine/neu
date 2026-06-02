//go:build editor

package assistant

// EditorContext is the capability-filtered snapshot of editor state handed to an
// agent on request (L1 §4.4). It is assembled on demand — never the whole world
// per message — and only the fields the agent has capability to read are
// populated; the rest stay zero (L1 §4.4 filtering).
//
// Per L1 §4.9, no field ever carries credentials, API keys, or .env contents:
// the provider sources only structural metadata (entity/scene/type summaries,
// command kinds, numeric diagnostics), so there is no path for secrets to leak
// even when an agent holds FileSystemAccess.
type EditorContext struct {
	SelectedEntities []EntityInfo        // gated by ReadScenes
	ActiveScene      SceneInfo           // gated by ReadScenes
	RecentCommands   []CommandRecord     // gated by ReadScenes (world-mutation history)
	TypeRegistry     TypeRegistrySummary // gated by ReadTypeRegistry
	Diagnostics      DiagnosticsSummary  // gated by ReadDiagnostics
}

// EntityInfo is a structural summary of one selected entity (no field values, so
// no secrets — L1 §4.9).
type EntityInfo struct {
	Name       string
	Components []string
	ID         uint64
}

// SceneInfo summarises the active scene.
type SceneInfo struct {
	Name        string
	EntityCount int
}

// TypeRegistrySummary lists the component type names available to an agent for
// suggestion/autocomplete.
type TypeRegistrySummary struct {
	ComponentTypes []string
}

// CommandRecord is one recent user action, for intent inference (L1 §4.4).
type CommandRecord struct {
	Kind   string
	Target uint64
}

// DiagnosticsSummary is a point-in-time numeric metrics snapshot (no log text,
// so no secrets).
type DiagnosticsSummary struct {
	Metrics map[string]float64
}

// ContextSources are the editor-supplied accessors the provider folds into an
// EditorContext. Each is injected so pkg/assistant stays decoupled from the
// concrete world / type-registry / diagnostic types (the same injection pattern
// used by the definition instantiation adapter). A nil accessor yields the zero
// value for its field — partial wiring is safe.
type ContextSources struct {
	SelectedEntities func() []EntityInfo
	ActiveScene      func() SceneInfo
	RecentCommands   func() []CommandRecord
	TypeRegistry     func() TypeRegistrySummary
	Diagnostics      func() DiagnosticsSummary
}

// ContextProvider assembles a capability-filtered EditorContext on demand
// (L1 §4.4). It holds only the injected source accessors.
type ContextProvider struct {
	sources ContextSources
}

// NewContextProvider returns a provider backed by the given sources.
func NewContextProvider(sources ContextSources) *ContextProvider {
	return &ContextProvider{sources: sources}
}

// GetEditorContext assembles the context for an agent holding caps. Only the
// fields whose gating capability is granted are populated; every other field is
// left zero (INV-2 read-side: an agent never receives data it cannot read).
func (p *ContextProvider) GetEditorContext(caps Capability) EditorContext {
	var ec EditorContext

	if caps.Has(ReadScenes) {
		if f := p.sources.SelectedEntities; f != nil {
			ec.SelectedEntities = f()
		}
		if f := p.sources.ActiveScene; f != nil {
			ec.ActiveScene = f()
		}
		if f := p.sources.RecentCommands; f != nil {
			ec.RecentCommands = f()
		}
	}

	if caps.Has(ReadTypeRegistry) {
		if f := p.sources.TypeRegistry; f != nil {
			ec.TypeRegistry = f()
		}
	}

	if caps.Has(ReadDiagnostics) {
		if f := p.sources.Diagnostics; f != nil {
			ec.Diagnostics = f()
		}
	}

	return ec
}
