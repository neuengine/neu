package app

import "github.com/neuengine/neu/pkg/app/appface"

// PluginFn adapts a plain function to the appface.Plugin interface.
// Useful for one-off plugins that don't warrant a named struct.
type PluginFn func(app appface.Builder)

// Build implements appface.Plugin.
func (f PluginFn) Build(app appface.Builder) { f(app) }

// PluginGroupBuilder assembles an ordered appface.PluginGroup via Add calls.
type PluginGroupBuilder struct {
	plugins []appface.Plugin
}

// NewPluginGroup returns an empty PluginGroupBuilder.
func NewPluginGroup() *PluginGroupBuilder { return &PluginGroupBuilder{} }

// Add appends p to the group. Returns the receiver for chaining.
func (b *PluginGroupBuilder) Add(p appface.Plugin) *PluginGroupBuilder {
	b.plugins = append(b.plugins, p)
	return b
}

// Plugins implements appface.PluginGroup.
func (b *PluginGroupBuilder) Plugins() []appface.Plugin { return b.plugins }
