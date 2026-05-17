// Package appface defines the narrow interfaces plugins use to configure the App.
// It lives in a sub-package so that internal plugin packages can import it
// without creating an import cycle with pkg/app (which imports concrete plugins
// for DefaultPlugins).
package appface

import (
	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
)

// Builder is the subset of App methods that Plugin.Build uses to configure
// the application. *app.App implements this interface.
type Builder interface {
	// World returns a reference to the main ECS World.
	World() *world.World
	// AddSystem registers a system in the named schedule. Auto-creates the
	// schedule if it does not exist.
	AddSystem(schedule string, system scheduler.System) Builder
	// AddSystems registers multiple systems in the named schedule.
	AddSystems(schedule string, systems ...scheduler.System) Builder
	// SetResource stores a singleton resource in the World (overwrites existing).
	SetResource(value any) Builder
	// InitResource stores the zero value of a resource type if not already set.
	InitResource(value any) Builder
	// AddPlugin adds a single plugin (idempotent).
	AddPlugin(p Plugin) Builder
	// AddPlugins adds an ordered group of plugins.
	AddPlugins(g PluginGroup) Builder
}

// Plugin is the primary extension interface. Build configures the app with
// systems, resources, and nested plugins.
type Plugin interface {
	Build(app Builder)
}

// PluginGroup is an ordered, optionally-filtered collection of plugins.
type PluginGroup interface {
	Plugins() []Plugin
}

// Standard schedule labels used by the engine and plugins.
// String constants so plugin packages can reference them without importing pkg/app.
const (
	PreStartup      = "PreStartup"
	Startup         = "Startup"
	PostStartup     = "PostStartup"
	First           = "First"
	PreUpdate       = "PreUpdate"
	StateTransition = "StateTransition"
	FixedMainLoop   = "FixedMainLoop"
	FixedPreUpdate  = "FixedPreUpdate"
	FixedUpdate     = "FixedUpdate"
	FixedPostUpdate = "FixedPostUpdate"
	Update          = "Update"
	PostUpdate      = "PostUpdate"
	Last            = "Last"
	PreShutdown     = "PreShutdown"
	Shutdown        = "Shutdown"
)
