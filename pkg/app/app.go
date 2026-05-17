// Package app provides the top-level engine entry point. App owns the main
// World, a named-schedule runner, and a plugin registry.
package app

import (
	"reflect"

	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
)

// RunMode controls how App.Run executes the main loop.
type RunMode uint8

const (
	// RunLoop runs the main schedule continuously until exit is requested.
	RunLoop RunMode = iota
	// RunOnce runs startup + one frame + cleanup, then returns.
	RunOnce
)

// RunnerFn is a customizable game-loop function.
type RunnerFn func(app *App) error

// ExtractFn copies data from the main World to a SubApp's World.
type ExtractFn func(main *world.World, sub *world.World)

// scheduleOrder defines the per-frame execution order.
var startupOrder = []string{
	appface.PreStartup,
	appface.Startup,
	appface.PostStartup,
}

var updateOrder = []string{
	appface.First,
	appface.PreUpdate,
	appface.StateTransition,
	appface.FixedMainLoop,
	appface.Update,
	appface.PostUpdate,
	appface.Last,
}

// App is the top-level engine entry point.
// It implements appface.Builder so plugins can configure it generically.
type App struct {
	w          *world.World
	schedules  map[string]*scheduler.Schedule
	executor   *scheduler.SequentialExecutor
	registry   *pluginRegistry
	subApps    map[string]*SubApp
	runner     RunnerFn
	runMode    RunMode
	shouldExit bool
}

// NewApp creates an App with an empty World and default schedules.
func NewApp() *App {
	a := &App{
		w:         world.NewWorld(),
		schedules: make(map[string]*scheduler.Schedule),
		executor:  scheduler.NewSequentialExecutor(),
		registry:  newPluginRegistry(),
		subApps:   make(map[string]*SubApp),
	}
	a.runner = defaultRunner
	return a
}

// World returns the main ECS World.
func (a *App) World() *world.World { return a.w }

// AddSystem registers a system in the named schedule, auto-creating it.
// Returns appface.Builder for plugin compatibility.
func (a *App) AddSystem(name string, sys scheduler.System) appface.Builder {
	s, ok := a.schedules[name]
	if !ok {
		s = scheduler.NewSchedule(name)
		a.schedules[name] = s
	}
	_ = s.AddSystem(sys) // errors only on duplicate name — safe to ignore
	return a
}

// AddSystems registers multiple systems in the named schedule.
func (a *App) AddSystems(name string, systems ...scheduler.System) appface.Builder {
	for _, sys := range systems {
		a.AddSystem(name, sys)
	}
	return a
}

// SetResource stores a singleton resource in the World (overwrites existing).
func (a *App) SetResource(value any) appface.Builder {
	world.SetResourceAny(a.w, value)
	return a
}

// InitResource stores the zero value of a resource type if not already set.
func (a *App) InitResource(value any) appface.Builder {
	world.InitResourceAny(a.w, value)
	return a
}

// AddPlugin adds p to the app. Build is called immediately; duplicates ignored.
func (a *App) AddPlugin(p appface.Plugin) appface.Builder {
	typeName := reflect.TypeOf(p).String()
	if !a.registry.register(typeName) {
		return a // already registered
	}
	p.Build(a)
	return a
}

// AddPlugins adds each plugin in group g in order.
func (a *App) AddPlugins(g appface.PluginGroup) appface.Builder {
	for _, p := range g.Plugins() {
		a.AddPlugin(p)
	}
	return a
}

// SetRunner replaces the default game-loop runner.
func (a *App) SetRunner(fn RunnerFn) *App {
	a.runner = fn
	return a
}

// SetRunMode sets RunLoop or RunOnce execution mode.
func (a *App) SetRunMode(mode RunMode) *App {
	a.runMode = mode
	return a
}

// Exit requests the main loop to stop after the current frame.
func (a *App) Exit() { a.shouldExit = true }

// SubApp returns the SubApp registered under label, or nil.
func (a *App) SubApp(label string) *SubApp { return a.subApps[label] }

// InsertSubApp registers sub under label.
func (a *App) InsertSubApp(label string, sub *SubApp) *App {
	a.subApps[label] = sub
	return a
}

// Run executes the full application lifecycle.
func (a *App) Run() error {
	if err := a.buildAllSchedules(); err != nil {
		return err
	}

	// Startup schedules (run once).
	for _, name := range startupOrder {
		if err := a.runSchedule(name); err != nil {
			return err
		}
	}

	if a.runMode == RunOnce {
		return a.update()
	}

	return a.runner(a)
}

// Update runs one frame: all per-frame schedules in order, then all SubApps.
func (a *App) Update() error {
	return a.update()
}

func (a *App) update() error {
	for _, name := range updateOrder {
		if err := a.runSchedule(name); err != nil {
			return err
		}
	}
	for _, sub := range a.subApps {
		if sub.extract != nil {
			sub.extract(a.w, sub.w)
		}
		if err := sub.runSchedules(a.executor); err != nil {
			return err
		}
	}
	a.w.ApplyDeferred()
	return nil
}

// runSchedule executes the schedule with the given name, if registered.
func (a *App) runSchedule(name string) error {
	s, ok := a.schedules[name]
	if !ok {
		return nil
	}
	return a.executor.Run(s, a.w)
}

// buildAllSchedules calls Build on every registered schedule.
func (a *App) buildAllSchedules() error {
	for _, s := range a.schedules {
		if err := s.Build(); err != nil {
			return err
		}
	}
	return nil
}

// defaultRunner runs the per-frame update continuously until Exit() is called.
func defaultRunner(a *App) error {
	for !a.shouldExit {
		if err := a.update(); err != nil {
			return err
		}
	}
	return nil
}

// pluginRegistry tracks registered plugin type names.
type pluginRegistry struct {
	seen map[string]bool
}

func newPluginRegistry() *pluginRegistry {
	return &pluginRegistry{seen: make(map[string]bool)}
}

// register returns true on first registration, false if already present.
func (r *pluginRegistry) register(typeName string) bool {
	if r.seen[typeName] {
		return false
	}
	r.seen[typeName] = true
	return true
}
