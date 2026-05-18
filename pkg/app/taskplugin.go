package app

import (
	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
	"github.com/neuengine/neu/pkg/task"
)

// TaskPlugin registers the bounded ComputePool, elastic IOPool, and
// MainThreadExecutor as World resources, and wires a PollMainThread system
// into the First schedule so the executor drains once per frame.
//
// Usage: call runtime.LockOSThread() and then exec.Bind() on the frame-loop
// goroutine before App.Run so INV-3 is satisfied:
//
//	app.AddPlugin(TaskPlugin{})
//	exec, _ := world.Resource[*task.MainThreadExecutor](app.World())
//	runtime.LockOSThread()
//	exec.Bind()
//	app.Run()
type TaskPlugin struct {
	Config task.TaskPoolConfig
}

// Build implements appface.Plugin.
func (p TaskPlugin) Build(app appface.Builder) {
	cp, io := task.NewTaskPools(p.Config)
	exec := task.NewMainThreadExecutor()

	w := app.World()
	world.SetResource(w, cp)
	world.SetResource(w, io)
	world.SetResource(w, exec)

	app.AddSystem(appface.First, scheduler.NewFuncSystem("task.PollMainThread", func(w *world.World) {
		// Resource[*T] returns **T when the stored value is a pointer; deref once.
		if ex, ok := world.Resource[*task.MainThreadExecutor](w); ok {
			(*ex).PollMainThread()
		}
	}))
}
