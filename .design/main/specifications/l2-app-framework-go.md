# App Framework — Go Implementation

**Version:** 0.1.0
**Status:** Stable
**Layer:** go
**Implements:** [app-framework.md](l1-app-framework.md)

## Overview

This specification defines the Go implementation of the App framework described in the L1 concept spec. The `App` struct is the top-level entry point: it owns the main `World`, a schedule runner, and a plugin registry. It provides a builder-pattern API for configuring plugins, systems, and resources. Plugins are the primary extension mechanism. SubApps provide isolated execution contexts. The framework supports `context.Context` for graceful shutdown.

## Related Specifications

- [app-framework.md](l1-app-framework.md) — L1 concept specification (parent)

## 1. Motivation

The Go implementation of the App framework provides the glue that binds the ECS modules together into a usable engine. It ensures:

- Type-safe plugin registration and initialization.
- Predictable execution order of systems across multiple schedules.
- Resource isolation via SubApps.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: Uses `unique` for plugin identification and labels.
- **Single-threaded Builder**: App construction is not concurrent safe; `Run()` starts the execution environment.

## 3. Core Invariants

> [!NOTE]
> See [app-framework.md §3](l1-app-framework.md) for technology-agnostic invariants.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: Hierarchy consistency | The App ensures the main World is the parent of all SubApp Worlds. |
| **INV-2**: Idempotent Plugins | `PluginRegistry` uses `reflect.TypeOf` to prevent duplicate registration. |
| **INV-3**: Schedule existence | `AddSystem` auto-creates schedules if they don't exist. |
| **INV-4**: Graceful shutdown | `Run()` blocks on a `context.Context` and triggers `Cleanup`. |

## Go Package

```
internal/app/
```

All types in this spec belong to package `app`. The package imports `internal/ecs` for World, Schedule, System, and related types.

## Type Definitions

### App

```go
// App is the top-level engine entry point. It owns the main World, schedule
// configuration, plugin registry, and sub-applications.
type App struct {
    world       *ecs.World
    schedules   *ecs.Schedules
    plugins     *PluginRegistry
    subApps     map[string]*SubApp
    runner      RunnerFn
    runMode     RunMode
}

// NewApp creates a new App with an empty World, default schedules, and the
// default runner function.
func NewApp() *App
```

### App Builder Methods

```go
// AddPlugin adds a single plugin to the app. The plugin's Build method is
// called immediately. Duplicate plugins (same concrete type) are silently
// ignored (idempotent).
func (a *App) AddPlugin(plugin Plugin) *App

// AddPlugins adds a PluginGroup. Each plugin in the group is added in order.
func (a *App) AddPlugins(group PluginGroup) *App

// AddSystem registers a system in the named schedule.
func (a *App) AddSystem(schedule string, system ecs.System) *App

// AddSystems registers multiple systems in the named schedule.
func (a *App) AddSystems(schedule string, systems ...ecs.System) *App

// SetResource inserts a resource into the main World. If a resource of the
// same type already exists, it is overwritten.
func (a *App) SetResource(value any) *App

// InitResource inserts the zero value of a resource type if it does not
// already exist.
func (a *App) InitResource(value any) *App

// InitState registers a state type with the given initial value. Creates
// State[S] and NextState[S] resources and registers state transition schedules.
func (a *App) InitState(initial any) *App

// SetRunner replaces the default game loop runner function.
func (a *App) SetRunner(runner RunnerFn) *App

// SetRunMode sets the execution mode (RunLoop or RunOnce).
func (a *App) SetRunMode(mode RunMode) *App

// SubApp returns the SubApp registered under the given label, or nil.
func (a *App) SubApp(label string) *SubApp

// InsertSubApp registers a SubApp under the given label.
func (a *App) InsertSubApp(label string, sub *SubApp) *App

// World returns a reference to the main World.
func (a *App) World() *ecs.World

// Run executes the full application lifecycle: plugin finalization, startup
// schedules, main loop, and cleanup. Blocks until exit.
func (a *App) Run() error
```

### Plugin

```go
// Plugin is the primary extension interface. Every plugin must implement Build,
// which configures the App with systems, resources, and other plugins.
type Plugin interface {
    Build(app *App)
}

// PluginReady is an optional interface. If implemented, the framework polls
// Ready until it returns true before proceeding to the Finish phase.
type PluginReady interface {
    Ready(app *App) bool
}

// PluginFinish is an optional interface. Finish is called after all plugins
// have been built and are ready.
type PluginFinish interface {
    Finish(app *App)
}

// PluginCleanup is an optional interface. Cleanup is called on shutdown in
// reverse registration order.
type PluginCleanup interface {
    Cleanup(app *App)
}
```

### PluginFn

```go
// PluginFn adapts a plain function into a Plugin. Allows lightweight plugins
// without defining a named struct.
type PluginFn func(app *App)

// Build calls the wrapped function.
func (f PluginFn) Build(app *App)
```

### PluginGroup

```go
// PluginGroup is an ordered collection of plugins. Supports per-plugin
// enable/disable customization.
type PluginGroup interface {
    Plugins() []Plugin
}
```

### PluginGroupBuilder

```go
// PluginGroupBuilder provides a fluent API for customizing a PluginGroup.
type PluginGroupBuilder struct {
    plugins  []pluginEntry
    disabled map[string]bool // keyed by plugin type name
}

// Disable removes a plugin from the group by its type name.
// Returns the builder for chaining.
func (b *PluginGroupBuilder) Disable(typeName string) *PluginGroupBuilder

// Set replaces a plugin in the group with a configured instance of the
// same type.
func (b *PluginGroupBuilder) Set(plugin Plugin) *PluginGroupBuilder

// Plugins returns the final ordered list of enabled plugins.
func (b *PluginGroupBuilder) Plugins() []Plugin
```

### PluginRegistry

```go
// PluginRegistry tracks registered plugins by type name to enforce the
// idempotency invariant (no duplicate plugins).
type PluginRegistry struct {
    registered map[string]bool
}

// IsRegistered reports whether a plugin of the given type has been added.
func (r *PluginRegistry) IsRegistered(typeName string) bool

// Register marks a plugin type as registered. Returns false if already present.
func (r *PluginRegistry) Register(typeName string) bool
```

### SubApp

```go
// SubApp is an isolated execution context with its own World and schedules.
// It communicates with the main App through an extract function that copies
// data from the main World to the sub-app World once per frame.
type SubApp struct {
    world     *ecs.World
    schedules *ecs.Schedules
    extract   ExtractFn
}

// NewSubApp creates a SubApp with an empty World and the given extract function.
func NewSubApp(extract ExtractFn) *SubApp

// World returns a reference to the sub-app's World.
func (s *SubApp) World() *ecs.World
```

### ExtractFn

```go
// ExtractFn copies relevant data from the main World to the sub-app World.
// Runs once per frame before the sub-app's schedules execute.
type ExtractFn func(main *ecs.World, sub *ecs.World)
```

### RunMode

```go
// RunMode controls how the App executes.
type RunMode uint8

const (
    // RunLoop runs the main loop continuously until exit is requested.
    RunLoop RunMode = iota

    // RunOnce runs startup schedules, one frame, and cleanup. Useful for
    // CLI tools, testing, and batch processing.
    RunOnce
)
```

### RunnerFn

```go
// RunnerFn is a customizable game loop function. It receives the App and
// controls how and when the main schedule is executed. The default runner
// is a simple loop with context-based shutdown.
type RunnerFn func(app *App) error
```

### Schedule Labels (Constants)

```go
const (
    // Startup schedules — run once before the main loop.
    PreStartup  = "PreStartup"
    Startup     = "Startup"
    PostStartup = "PostStartup"

    // Per-frame schedules — run every frame in this order.
    First           = "First"
    PreUpdate       = "PreUpdate"
    StateTransition = "StateTransition"
    RunFixedMainLoop = "RunFixedMainLoop"
    Update          = "Update"
    PostUpdate      = "PostUpdate"
    Last            = "Last"

    // Fixed timestep schedules — run 0..N times per frame inside RunFixedMainLoop.
    FixedPreUpdate  = "FixedPreUpdate"
    FixedUpdate     = "FixedUpdate"
    FixedPostUpdate = "FixedPostUpdate"
)
```

### DefaultPlugins

```go
// DefaultPlugins is the standard PluginGroup for a full application.
type DefaultPlugins struct{}

// Plugins returns the default plugin set in initialization order.
func (d DefaultPlugins) Plugins() []Plugin
// Returns: LogPlugin, TimePlugin, InputPlugin, HierarchyPlugin,
//          StatePlugin, ScheduleRunnerPlugin
```

### MinimalPlugins

```go
// MinimalPlugins is a minimal PluginGroup for headless or test applications.
type MinimalPlugins struct{}

// Plugins returns the minimal plugin set.
func (m MinimalPlugins) Plugins() []Plugin
// Returns: LogPlugin, TimePlugin, ScheduleRunnerPlugin
```

## Key Methods

### Application Lifecycle

```
FUNCTION App.Run():
  // Phase 1: Build — already done during AddPlugin calls

  // Phase 2: Ready — poll until all PluginReady return true
  FOR EACH plugin WITH PluginReady interface:
    WHILE NOT plugin.Ready(app):
      WAIT (with cycle detection / max iterations)

  // Phase 3: Finish — all PluginFinish in registration order
  FOR EACH plugin WITH PluginFinish interface:
    plugin.Finish(app)

  // Phase 4: Startup schedules (run once)
  RunSchedule(PreStartup)
  RunSchedule(Startup)
  RunSchedule(PostStartup)

  // Phase 5: Main loop via runner
  err = app.runner(app)

  // Phase 6: Cleanup — all PluginCleanup in reverse order
  FOR EACH plugin WITH PluginCleanup interface (reversed):
    plugin.Cleanup(app)

  RETURN err
```

### Default Runner

```
FUNCTION DefaultRunner(app):
  ctx = context.Background()  // or app-provided context
  FOR NOT ctx.Done() AND NOT app.ShouldExit():
    app.Update()
  RETURN nil
```

### Per-Frame Update

```
FUNCTION App.Update():
  RunSchedule(First)
  RunSchedule(PreUpdate)
  RunSchedule(StateTransition)
  RunFixedMainLoop()
  RunSchedule(Update)
  RunSchedule(PostUpdate)
  RunSchedule(Last)

  FOR EACH subApp IN app.subApps:
    subApp.extract(app.world, subApp.world)
    subApp.RunSchedules()
```

### Plugin Deduplication

```
FUNCTION App.AddPlugin(plugin):
  typeName = reflect.TypeOf(plugin).String()
  IF plugins.IsRegistered(typeName):
    RETURN app  // idempotent no-op
  plugins.Register(typeName)
  plugin.Build(app)
  RETURN app
```

## Performance Strategy

- **Plugin registry uses map by type name**: O(1) lookup for deduplication. Plugin Build runs once.
- **Schedule runner**: Schedules are pre-compiled during startup (system ordering, dependency graph). Per-frame overhead is minimal.
- **SubApp extract**: Runs once per frame. Users control what data is copied — no automatic sync.
- **RunnerFn indirection**: Single function pointer call per frame — negligible overhead.
- **context.Context for shutdown**: Uses standard Go cancellation pattern. No polling overhead — select on `ctx.Done()` channel.

## Error Handling

- **Duplicate plugin**: Silent no-op (idempotent by design, INV-2).
- **Plugin Ready timeout**: If a plugin's `Ready` method never returns true, the framework detects the cycle after a configurable max iteration count and panics with a descriptive message.
- **Runner error**: `Run()` propagates the error returned by the runner function.
- **Missing schedule**: Adding a system to an unregistered schedule name auto-creates the schedule. This is not an error.
- **SubApp label conflict**: Inserting a SubApp with an existing label overwrites the previous one. Log a warning via `log/slog`.
- **Startup panic**: Panics during plugin Build or startup schedules propagate unrecovered. The caller of `Run()` should use `recover` if needed.

## Testing Strategy

- **Unit tests**: Create App, add plugins, verify plugin registry deduplication. Verify AddSystem places systems in correct schedules.
- **Plugin lifecycle**: Create test plugins implementing all optional interfaces (Ready, Finish, Cleanup). Verify call order: Build → Ready → Finish → [run] → Cleanup (reversed).
- **PluginGroup**: Create a group, disable one plugin, verify it is skipped.
- **RunOnce**: Create App with RunOnce mode, verify startup + one frame + cleanup execute.
- **SubApp**: Create App with SubApp, verify extract function is called each frame and SubApp schedules run.
- **Schedule ordering**: Verify per-frame schedules run in correct order (First → PreUpdate → ... → Last).
- **Startup schedules**: Verify PreStartup → Startup → PostStartup run exactly once before the main loop.
- **Integration**: Build a minimal App with TimePlugin and a counting system, run for 3 frames with RunOnce-like harness, verify frame count.
- **Benchmarks**: `BenchmarkAppUpdate` with 10 empty systems — measure per-frame overhead of the schedule runner.

## 7. Drawbacks & Alternatives

- **Drawback**: Plugin registration order matters for simple dependencies, which can be fragile.
- **Alternative**: Explicit dependency graph for plugins.
- **Decision**: Registration order is simpler and follows Bevy's successful model.

## Canonical References

<!-- MANDATORY for Stable status. List authoritative source files that downstream agents
     MUST read before implementing this spec. Use relative paths from project root.
     Stub state — fill with concrete files when implementation begins (Phase 1+). -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

<!-- Empty table = no canonical sources yet. Populate one row per authoritative file
     when implementation lands (Phase 1+). Stable promotion requires ≥1 row. -->

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-03-26 | Initial L2 draft |
