package event

import (
	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
)

// EventsPlugin wires per-frame event maintenance into the App loop: a
// First-schedule system rotates every registered EventBus's double buffer
// ([SwapAll]) and runs message-channel cleanup ([CleanupAll]). It runs at frame
// start, so events a system sends in one frame are promoted to the readable
// ("previous") buffer for the next frame's readers — the engine-loop half of
// the double-buffered delivery contract.
//
// Without this plugin the buses are inert in an App loop: writes accumulate in
// the current buffer but never rotate to where readers observe them. The system
// is a no-op when no events are registered.
type EventsPlugin struct{}

// Build implements appface.Plugin.
func (EventsPlugin) Build(app appface.Builder) {
	app.AddSystem(appface.First, scheduler.NewFuncSystem("ecs.SwapEvents", func(w *world.World) {
		SwapAll(w)
		CleanupAll(w)
	}))
}
