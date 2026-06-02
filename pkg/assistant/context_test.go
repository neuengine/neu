//go:build editor

package assistant

import "testing"

// fullSources returns a ContextSources with every accessor populated, plus a
// pointer to a call-counter map so tests can assert which sources were invoked.
func fullSources(called map[string]bool) ContextSources {
	return ContextSources{
		SelectedEntities: func() []EntityInfo {
			called["entities"] = true
			return []EntityInfo{{ID: 1, Name: "Player", Components: []string{"Transform", "Health"}}}
		},
		ActiveScene: func() SceneInfo {
			called["scene"] = true
			return SceneInfo{Name: "main", EntityCount: 3}
		},
		RecentCommands: func() []CommandRecord {
			called["commands"] = true
			return []CommandRecord{{Kind: "spawn", Target: 1}}
		},
		TypeRegistry: func() TypeRegistrySummary {
			called["types"] = true
			return TypeRegistrySummary{ComponentTypes: []string{"Transform", "Health"}}
		},
		Diagnostics: func() DiagnosticsSummary {
			called["diag"] = true
			return DiagnosticsSummary{Metrics: map[string]float64{"fps": 60}}
		},
	}
}

func TestContextProvider_FullCapabilities(t *testing.T) {
	t.Parallel()
	called := map[string]bool{}
	p := NewContextProvider(fullSources(called))

	caps := ReadScenes | ReadTypeRegistry | ReadDiagnostics
	ec := p.GetEditorContext(caps)

	if len(ec.SelectedEntities) != 1 || ec.SelectedEntities[0].Name != "Player" {
		t.Errorf("SelectedEntities = %+v", ec.SelectedEntities)
	}
	if ec.ActiveScene.Name != "main" || ec.ActiveScene.EntityCount != 3 {
		t.Errorf("ActiveScene = %+v", ec.ActiveScene)
	}
	if len(ec.RecentCommands) != 1 || ec.RecentCommands[0].Kind != "spawn" {
		t.Errorf("RecentCommands = %+v", ec.RecentCommands)
	}
	if len(ec.TypeRegistry.ComponentTypes) != 2 {
		t.Errorf("TypeRegistry = %+v", ec.TypeRegistry)
	}
	if ec.Diagnostics.Metrics["fps"] != 60 {
		t.Errorf("Diagnostics = %+v", ec.Diagnostics)
	}
}

func TestContextProvider_FiltersByCapability(t *testing.T) {
	t.Parallel()
	called := map[string]bool{}
	p := NewContextProvider(fullSources(called))

	// Only ReadTypeRegistry granted: scene/diagnostics fields must stay empty,
	// and their source accessors must not even be called (INV-2 read-side).
	ec := p.GetEditorContext(ReadTypeRegistry)

	if ec.SelectedEntities != nil || ec.RecentCommands != nil {
		t.Error("scene-gated fields should be empty without ReadScenes")
	}
	if ec.ActiveScene != (SceneInfo{}) {
		t.Errorf("ActiveScene should be zero, got %+v", ec.ActiveScene)
	}
	if ec.Diagnostics.Metrics != nil {
		t.Error("Diagnostics should be empty without ReadDiagnostics")
	}
	if len(ec.TypeRegistry.ComponentTypes) != 2 {
		t.Error("TypeRegistry should be populated with ReadTypeRegistry")
	}
	// Source-call assertions: only the type accessor ran.
	if called["entities"] || called["scene"] || called["commands"] || called["diag"] {
		t.Errorf("ungated sources were invoked: %+v", called)
	}
	if !called["types"] {
		t.Error("type registry source should have been invoked")
	}
}

func TestContextProvider_DiagnosticsOnly(t *testing.T) {
	t.Parallel()
	called := map[string]bool{}
	p := NewContextProvider(fullSources(called))

	ec := p.GetEditorContext(ReadDiagnostics)
	if ec.Diagnostics.Metrics["fps"] != 60 {
		t.Errorf("Diagnostics should be populated, got %+v", ec.Diagnostics)
	}
	if ec.TypeRegistry.ComponentTypes != nil {
		t.Error("TypeRegistry should be empty without ReadTypeRegistry")
	}
}

func TestContextProvider_NoCapabilities(t *testing.T) {
	t.Parallel()
	called := map[string]bool{}
	p := NewContextProvider(fullSources(called))

	ec := p.GetEditorContext(0)
	if ec.SelectedEntities != nil || ec.TypeRegistry.ComponentTypes != nil || ec.Diagnostics.Metrics != nil {
		t.Errorf("no capabilities ⇒ empty context, got %+v", ec)
	}
	if len(called) != 0 {
		t.Errorf("no sources should be invoked with zero caps, got %+v", called)
	}
}

func TestContextProvider_NilSources(t *testing.T) {
	t.Parallel()
	// All accessors nil but capabilities granted: must not panic, returns zero.
	p := NewContextProvider(ContextSources{})
	ec := p.GetEditorContext(ReadScenes | ReadTypeRegistry | ReadDiagnostics)
	if len(ec.SelectedEntities) != 0 || ec.ActiveScene != (SceneInfo{}) || ec.Diagnostics.Metrics != nil {
		t.Errorf("nil sources should yield a zero context, got %+v", ec)
	}
}
