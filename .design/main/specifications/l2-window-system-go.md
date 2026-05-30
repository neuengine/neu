# Window System — Go Implementation

**Version:** 0.1.0
**Status:** Stable
**Layer:** go
**Implements:** [l1-window-system.md](l1-window-system.md)

## Overview

Go-level design for OS windows as ECS entities. A window is an entity carrying a
`Window` component (title, mode, resolution, present mode, cursor); a `PrimaryWindow`
zero-sized marker tags the main window spawned by `WindowPlugin`. Mutations to the
`Window` component are diffed via change-detection ticks and applied to a pluggable
`WindowBackend` once per frame on the main thread. Create/destroy flow through the
command queue (deferred). Platform events are polled at frame start and translated
into engine events. A headless backend (no OS windows) keeps CI green; the real
backend is chosen by the platform plugin.

## Related Specifications

- [l1-window-system.md](l1-window-system.md) — L1 concept specification (parent)
- [l2-platform-system-go.md](l2-platform-system-go.md) — `PlatformPlugin` registers the `WindowBackend` per platform
- [l2-app-framework-go.md](l2-app-framework-go.md) — `App`, `Plugin`, startup levels, `AppExit` event
- [l2-render-core-go.md](l2-render-core-go.md) — each window supplies a render surface / swapchain (`RawWindowHandle`)
- [l2-event-system-go.md](l2-event-system-go.md) — window lifecycle/interaction events
- [l2-change-detection-go.md](l2-change-detection-go.md) — `Mut`/`Ref` ticks drive the per-frame `WindowDiff`
- [l2-command-system-go.md](l2-command-system-go.md) — deferred window create/destroy
- [l2-task-system-go.md](l2-task-system-go.md) — `MainThreadExecutor` runs backend calls under the OS main-thread constraint

## 1. Motivation

Modeling windows as entities unifies single- and multi-window apps: a debug inspector
is just another spawned `Window`. Binding the L1 concept to Go means the only new
machinery is (1) a `WindowBackend` interface the platform layer implements, and (2) a
diff-driven sync system that reuses the engine's existing change-detection and command
infrastructure rather than inventing window-specific bookkeeping.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: structs as components; `unsafe.Pointer`-free `RawWindowHandle` (an opaque typed handle).
- **C-003**: the headless backend is dep-free stdlib; native desktop backends are platform plugins (their windowing dep is justified per platform, not in core).
- Window create/destroy are issued as `Command`s (INV-3) and applied at the command flush point — never inline in a system.
- All backend calls (`CreateWindow`, `ApplyChanges`, `DestroyWindow`, `PollEvents`) run on the `MainThreadExecutor` (OS main-thread constraint).
- User code mutates only the `Window` component (INV-4); the sync system computes the `WindowDiff` and calls the backend — user code never touches the backend.
- Exactly one entity holds `PrimaryWindow` (INV-1); the `WindowPlugin` spawns it (or none, for headless).
- `WindowCloseRequested` is advisory: it does not despawn; `CloseWhenRequested`/`ExitCondition` policy decides.

## 3. Core Invariants

> [!NOTE]
> See [l1-window-system.md §3](l1-window-system.md) for the technology-agnostic
> invariants (INV-1…INV-4). Go-specific compliance is tabulated in §4.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: exactly one `PrimaryWindow` at any time | `PrimaryWindow` is a zero-sized marker; a startup-validated system asserts the count is 1 (a `PrimaryWindowRes` holds the entity for O(1) lookup). Spawning a second primary is a developer error (`ErrMultiplePrimary`). |
| **INV-2**: closing the primary triggers `AppExit` | When the `PrimaryWindow` entity is despawned **or** emits `WindowCloseRequested` under `ExitCondition==OnPrimaryClosed`, the sync system writes an `AppExit` event (reusing l2-app-framework-go). |
| **INV-3**: create/destroy deferred via commands | `SpawnWindow`/`DespawnWindow` enqueue `Command`s; the backend `CreateWindow`/`DestroyWindow` run at the command flush, on the main thread — never inline. |
| **INV-4**: all mutations flow through the `Window` component | The only public surface is the `Window` component. A `windowSyncSystem` reads `Mut[Window]` change ticks, builds a `WindowDiff` of changed fields, and calls `backend.ApplyChanges`. No exported backend method is reachable from game code. |

## Go Package

```
pkg/window/
  window.go        // Window component, WindowResolution, WindowPosition
  mode.go          // WindowMode, PresentMode enums
  cursor.go        // CursorOptions, CursorGrabMode, CursorIcon
  markers.go       // PrimaryWindow (zero-sized marker)
  events.go        // WindowCreated/Resized/Moved/CloseRequested/Closed/Focused, Cursor*, Ime, ...
  plugin.go        // WindowPlugin, ExitCondition; spawns the primary window
  backend.go       // WindowBackend interface, RawWindowHandle, WindowDescriptor, WindowDiff, PlatformEvent
internal/window/
  sync.go          // windowSyncSystem: Window diff → ApplyChanges (main thread)
  poll.go          // PollEvents → engine events translation each frame
  headless.go      // HeadlessWindowBackend: no OS windows, deterministic event queue (CI)
  primary.go       // PrimaryWindowRes + INV-1 assertion system
```

## Type Definitions

```go
type WindowMode uint8
const (Windowed WindowMode = iota; BorderlessFullscreen; SizedFullscreen; Fullscreen)

type PresentMode uint8
const (AutoVsync PresentMode = iota; AutoNoVsync; Fifo; Immediate; Mailbox)

type CursorGrabMode uint8
const (GrabNone CursorGrabMode = iota; GrabConfined; GrabLocked)

type WindowResolution struct {
    PhysicalWidth, PhysicalHeight uint32
    ScaleFactorOverride           float64 // 0 ⇒ use OS scale
}
func (r WindowResolution) Logical() (w, h float64) // physical / scale

type CursorOptions struct {
    Visible  bool
    GrabMode CursorGrabMode
    Icon     CursorIcon
    HitTest  bool
}

type Window struct {
    Title       string
    Mode        WindowMode
    Resolution  WindowResolution
    Position    WindowPosition // Automatic | At(x,y)
    Resizable   bool
    Decorations bool
    Transparent bool
    Visible     bool
    Focused     bool // read-only; driven by events
    PresentMode PresentMode
    Cursor      CursorOptions
    ImeEnabled  bool
    Canvas      string // web target element selector; "" ⇒ native
}

type PrimaryWindow struct{} // zero-sized marker

type ExitCondition uint8
const (OnPrimaryClosed ExitCondition = iota; OnAllClosed; DontExit)

type WindowPlugin struct {
    PrimaryWindow      *Window // nil ⇒ headless, no window
    ExitCondition      ExitCondition
    CloseWhenRequested bool
}

// Opaque handle bridged to render-core surface creation.
type RawWindowHandle struct{ /* platform-tagged payload */ }

type WindowBackend interface {
    CreateWindow(e ecs.EntityID, d WindowDescriptor) (RawWindowHandle, error)
    DestroyWindow(e ecs.EntityID) error
    ApplyChanges(e ecs.EntityID, diff WindowDiff) error
    PollEvents() []PlatformEvent
}
```

## Key Methods

```go
// Deferred lifecycle (INV-3) — enqueue commands.
func SpawnWindow(c *ecs.Commands, w Window) ecs.EntityID
func DespawnWindow(c *ecs.Commands, e ecs.EntityID)

// Sync system (main thread): diff changed Window fields → backend.
func windowSyncSystem(/* Query[Mut[Window]], Res[WindowBackend], Res[PrimaryWindowRes], EventWriter[AppExit] */)

// Frame-start poll: PlatformEvent → engine events.
func windowPollSystem(/* Res[WindowBackend], EventWriters... */)

// WindowPlugin wires the primary window + systems.
func (p WindowPlugin) Build(app *app.App)
```

## Performance Strategy

- **Diff-driven sync**: `windowSyncSystem` iterates only entities whose `Window` changed this tick (change-detection filter); an unchanged window costs nothing.
- **0-alloc event translation**: `PollEvents` fills a reused `[]PlatformEvent` from a pooled buffer; translation writes directly into the engine `EventWriter` ring (no per-event heap allocation).
- **Main-thread batching**: all backend calls for a frame are submitted to the `MainThreadExecutor` in one batch at the command-flush point, minimizing cross-thread handoffs.
- **Headless fast path**: the headless backend's `ApplyChanges`/`PollEvents` are near-no-ops returning from a pre-seeded queue — CI windowing is allocation-light and deterministic.

## Error Handling

| Condition | Behavior |
| :--- | :--- |
| Spawning a second `PrimaryWindow` | `ErrMultiplePrimary` from the INV-1 assertion system at flush |
| `CreateWindow` backend failure | wrapped error (`fmt.Errorf("window: %w", err)`); the entity's `Window` is marked failed; no partial window |
| `ApplyChanges` on a destroyed window | no-op + `Warning` (the entity despawn already removed the backend handle) |
| Backend call off the main thread | guarded by `MainThreadExecutor`; a misuse panics in debug builds |
| Closing primary under `OnPrimaryClosed` | `AppExit` written (INV-2); under `DontExit`, only `WindowClosed` is emitted |

## Testing Strategy

- **INV-1**: spawning two primaries ⇒ `ErrMultiplePrimary`; despawning the primary clears `PrimaryWindowRes`.
- **INV-2**: with `OnPrimaryClosed`, a `WindowCloseRequested` on the primary ⇒ exactly one `AppExit`; with `DontExit`, none.
- **INV-3**: `SpawnWindow`/`DespawnWindow` produce no backend call until the command flush; assert ordering via the headless backend's call log.
- **INV-4**: mutating `Window.Title`/`Mode` ⇒ a `WindowDiff` with only those fields set; an unchanged frame produces an empty diff (no `ApplyChanges` call).
- **Headless determinism**: a scripted `PlatformEvent` queue replays to identical engine events across ≥20 runs (mirrors the Phase 5 example hash pattern).
- **Multi-window**: spawning a non-primary window and closing it despawns only that entity and emits no `AppExit`.

## 7. Drawbacks & Alternatives

- **Drawback**: diffing the whole `Window` component each change is coarser than per-field setters.
  **Alternative**: individual setter commands per field.
  **Decision**: a single component honoring INV-4 + change-detection ticks keeps the API minimal and the diff is cheap (one struct compare). Kept.
- **Drawback**: a `RawWindowHandle` opaque payload leaks a little platform shape into a public type.
  **Alternative**: keep handles entirely inside the backend.
  **Decision**: render-core needs the handle to create a surface; an opaque typed wrapper (no exported fields) is the minimal bridge. Kept.
- **Drawback**: main-thread batching can stall if a backend call blocks.
  **Alternative**: async window ops.
  **Decision**: OS windowing APIs mandate the main thread; batching at the flush point bounds the stall to once per frame. Kept.

## Canonical References

<!-- All paths verified on disk; validated by examples/window (hash ×20, T-6T06)
     + pkg/window 89.3% / internal/window 82.6% cov. C29 P6 gate closed. The full
     ECS sync system + a native backend land at App integration. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |
| [WINDOW] | `pkg/window/window.go` | `Window` component, `WindowMode`/`PresentMode`/`Cursor` enums, `WindowResolution.Logical`, `PrimaryWindow` marker. |
| [BACKEND] | `pkg/window/backend.go` | `WindowBackend` iface, `RawWindowHandle`, `WindowDescriptor`, `WindowDiff` (INV-4), `ExitCondition`, `WindowPlugin`. |
| [EVENTS] | `pkg/window/events.go` | `PlatformEvent` + `CausesAppExit` close→exit decision (INV-2). |
| [HEADLESS] | `internal/window/headless.go` | `HeadlessWindowBackend`: deterministic call-log + scripted event queue (CI). |
| [DIFF] | `internal/window/diff.go` | `DiffWindow` pure field-diff (INV-4; Focused excluded as read-only). |
| [PRIMARY] | `internal/window/primary.go` | `PrimaryWindowRes` + `CheckSinglePrimary` (INV-1). |
| [EXAMPLE] | `examples/window/main.go` | Create→diff/apply→event-replay→close→destroy, hash-stable ×20 (T-6T06). |
| [TEST] | `pkg/window/window_test.go`, `internal/window/window_test.go` | Logical scaling, diff, exit matrix, headless lifecycle, INV-1 primary. |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-28 | Initial L2 draft — Go translation of l1-window-system v0.1.0. `Window` component + `PrimaryWindow` marker, diff-driven `windowSyncSystem` reusing change-detection ticks, deferred create/destroy via commands, `MainThreadExecutor`-bound backend calls, `WindowBackend` interface with a headless CI backend, `RawWindowHandle` bridge to render-core surfaces, `ExitCondition` policy. Authored ahead of Phase 6 (`/magic.spec`). Draft — L1 parent Draft + no implementation yet. |
