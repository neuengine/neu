package plugin

import (
	"fmt"
	"log/slog"

	"github.com/neuengine/neu/pkg/app/appface"
	pkgplugin "github.com/neuengine/neu/pkg/plugin"
)

// LoadInProcess looks up the compile-time factory registered for id, calls
// Build(app) (and Ready(app) if the plugin implements appface.FullPlugin), and
// advances the manager state to StateActive. T-6N03b.
//
// Finish and Cleanup are driven later — either by the App's FullPlugin lifecycle
// (when registered via AddPlugin) or explicitly by the caller on shutdown.
//
// Returns the live Plugin instance so the caller can store it and invoke
// Finish/Cleanup at the appropriate time outside the App lifecycle.
//
// Errors:
//   - factory not registered: plugin cannot be loaded in-process
//   - id not in manager registry: must Register() before Loading
func LoadInProcess(m *Manager, id pkgplugin.PluginID, app appface.Builder) (appface.Plugin, error) {
	factory, ok := LookupFactory(id)
	if !ok {
		return nil, fmt.Errorf("plugin: no factory registered for %q (must call RegisterFactory at link time)", id)
	}
	if _, registered := m.State(id); !registered {
		return nil, fmt.Errorf("plugin: %q not registered in manager; call Manager.Register first", id)
	}

	m.SetState(id, pkgplugin.StateLoading)
	p := factory()
	p.Build(app)

	if fp, ok := p.(appface.FullPlugin); ok {
		fp.Ready(app)
	}

	m.SetState(id, pkgplugin.StateActive)
	slog.Info("plugin loaded in-process", "id", id)
	return p, nil
}

// FinishInProcess drives Finish → Cleanup on a previously LoadInProcess-ed
// plugin (if it implements appface.FullPlugin) and marks it Disabled.
func FinishInProcess(m *Manager, id pkgplugin.PluginID, p appface.Plugin, app appface.Builder) {
	if fp, ok := p.(appface.FullPlugin); ok {
		fp.Finish(app)
		fp.Cleanup(app)
	}
	m.SetState(id, pkgplugin.StateDisabled)
	slog.Info("plugin finished in-process", "id", id)
}
