package app

import (
	"github.com/neuengine/neu/internal/ecs/gametime"
	"github.com/neuengine/neu/internal/ecs/hierarchy"
	"github.com/neuengine/neu/internal/ecs/input"
	"github.com/neuengine/neu/internal/ecs/state"
	"github.com/neuengine/neu/pkg/app/appface"
)

// DefaultPlugins is a PluginGroup that registers the standard engine plugin
// set: time (60 Hz fixed step), hierarchy/transforms, input, and state-machine
// infrastructure. Add it to a new App with app.AddPlugins(DefaultPlugins{}).
type DefaultPlugins struct{}

// Plugins implements appface.PluginGroup.
func (DefaultPlugins) Plugins() []appface.Plugin {
	return []appface.Plugin{
		gametime.TimePlugin{},
		hierarchy.HierarchyPlugin{},
		input.InputPlugin{},
		state.StatePlugin{},
	}
}

// MinimalPlugins is a PluginGroup with no built-in plugins.
// Use when constructing a fully custom engine configuration.
type MinimalPlugins struct{}

// Plugins implements appface.PluginGroup.
func (MinimalPlugins) Plugins() []appface.Plugin { return nil }
