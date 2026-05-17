package state

import "github.com/neuengine/neu/pkg/app/appface"

// StatePlugin is a minimal plugin that provides state-system infrastructure.
// State types are registered per-type via InitState[S](app, initial).
type StatePlugin struct{}

// Build implements appface.Plugin. Currently a no-op — infrastructure is
// wired by InitState[S] calls on the builder.
func (StatePlugin) Build(_ appface.Builder) {}
