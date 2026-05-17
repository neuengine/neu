package input

import (
	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
)

// InputPlugin registers all input resources and the per-frame clear system.
// Resources registered: ButtonInput[KeyCode], ButtonInput[MouseButton],
// ButtonInput[GamepadButton], AxisInput[GamepadAxis], CursorPosition.
// System registered: input.ClearInputState in PreUpdate.
type InputPlugin struct{}

// Build implements appface.Plugin.
func (InputPlugin) Build(app appface.Builder) {
	w := app.World()

	world.SetResource(w, NewButtonInput[KeyCode]())
	world.SetResource(w, NewButtonInput[MouseButton]())
	world.SetResource(w, NewButtonInput[GamepadButton]())
	world.SetResource(w, NewAxisInput[GamepadAxis]())
	world.SetResource(w, CursorPosition{})

	app.AddSystem(appface.PreUpdate, scheduler.NewFuncSystem(
		"input.ClearInputState",
		clearInputState,
	))
}

// clearInputState resets per-frame JustPressed/JustReleased sets on all button
// inputs at the start of PreUpdate, before platform events are injected.
func clearInputState(w *world.World) {
	if kb, ok := world.Resource[*ButtonInput[KeyCode]](w); ok {
		(*kb).Clear()
	}
	if mb, ok := world.Resource[*ButtonInput[MouseButton]](w); ok {
		(*mb).Clear()
	}
	if gb, ok := world.Resource[*ButtonInput[GamepadButton]](w); ok {
		(*gb).Clear()
	}
}
