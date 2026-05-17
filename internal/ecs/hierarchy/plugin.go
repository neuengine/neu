package hierarchy

import (
	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
)

// HierarchyPlugin registers all hierarchy components, the transform
// propagation system, and required-component relationships.
// Add it to your App via app.AddPlugin(hierarchy.HierarchyPlugin{}).
type HierarchyPlugin struct{}

// Build implements appface.Plugin.
func (HierarchyPlugin) Build(app appface.Builder) {
	w := app.World()

	// Pre-register components so their IDs are stable before any entity spawns.
	world.RegisterComponent[ChildOf](w)
	world.RegisterComponent[Children](w)
	world.RegisterComponent[Transform](w)
	world.RegisterComponent[GlobalTransform](w)

	app.AddSystem(appface.PostUpdate, scheduler.NewFuncSystem(
		"hierarchy.PropagateTransforms",
		propagateTransforms,
	))
}
