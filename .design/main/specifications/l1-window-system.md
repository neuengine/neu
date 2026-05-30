# Window System

**Version:** 0.1.0
**Status:** Stable
**Layer:** concept

## Overview

The window system manages operating-system windows as ECS entities. Each window is an entity with a `Window` component describing its properties (title, size, mode, vsync, cursor settings). A `PrimaryWindow` marker identifies the main window, created automatically by DefaultPlugins. Window events (resize, close, focus, cursor) flow through the standard event system. A platform abstraction layer (trait-based backend) makes the windowing backend pluggable across native desktop, web, and console targets.

## Related Specifications

- [input-system.md](l1-input-system.md) — Keyboard, mouse, and gamepad input routed per window
- [render-core.md](l1-render-core.md) — Each window owns a render surface / swapchain
- [event-system.md](l1-event-system.md) — Window lifecycle and interaction events
- [platform-system.md](l1-platform-system.md) — WindowBackend selection per platform, capability negotiation

## 1. Motivation

Entity-based windows unify single-window and multi-window applications under one model. A game that opens a second viewport or a debug inspector window simply spawns another entity with a `Window` component. Decoupling from any specific windowing library lets the engine run on platforms with very different window management semantics. Without this:

- User code would depend on platform-specific APIs (Win32, X11, Wayland, Cocoa).
- Multi-window support would require ad-hoc bookkeeping.
- Render surface creation would be tightly coupled to a single windowing library.

## 2. Constraints & Assumptions

- Window creation and destruction are deferred via the command queue, not executed immediately.
- Only one primary window exists at a time; closing it triggers application exit.
- All window state mutations go through the Window component — user code never calls platform APIs directly.
- Window creation and destruction happen on the main thread due to OS constraints; the ECS schedules these operations accordingly.
- The render core obtains its surface/swapchain from window entities; each window with a camera produces an independent render target.
- Input events carry a window entity ID so multi-window input is unambiguous.
- A headless mode removes the requirement for a window to exist.

## 3. Core Invariants

- **INV-1**: Exactly one PrimaryWindow exists at any time while the app is running.
- **INV-2**: Closing the primary window triggers AppExit.
- **INV-3**: Window creation/destruction is deferred via commands (not immediate).
- **INV-4**: All window mutations flow through the Window component — no direct platform API calls from user code.

## 4. Detailed Design

### 4.1 Window Component

The central component attached to every window entity:

```
Window {
    Title:              string
    Mode:               WindowMode
    Resolution:         WindowResolution  // physical_width, physical_height, scale_factor_override
    Position:           WindowPosition    // Automatic | At(x, y)
    Resizable:          bool
    Decorations:        bool              // title bar and borders
    Transparent:        bool              // compositing transparency
    Visible:            bool
    Focused:            bool              // read-only, driven by events
    PresentMode:        PresentMode
    Cursor:             CursorOptions
    ImeEnabled:         bool
    Canvas:             Option<string>    // web target element selector
}
```

Users mutate the Window component; the backend synchronizes platform state each frame.

### 4.2 WindowResolution

Tracks both logical and physical sizes. The scale factor (DPI) can come from the OS or be overridden. Logical size = physical size / scale factor.

### 4.3 Window Modes

```
WindowMode:
    Windowed              // Standard resizable/decorated window
    BorderlessFullscreen  // Borderless window matching monitor resolution
    SizedFullscreen       // Borderless window at custom resolution
    Fullscreen            // Exclusive fullscreen with mode switch
```

Mode is set via the `Window.Mode` field. Changing it triggers a platform-level mode switch on the next sync.

### 4.4 Present Mode

Controls vertical sync behavior for the render surface:

```
PresentMode:
    AutoVsync    // Engine picks best vsync mode for the platform
    AutoNoVsync  // Engine picks best non-vsync mode
    Fifo         // Traditional vsync (wait for vblank)
    Immediate    // No sync, may tear
    Mailbox      // Triple-buffered, low latency vsync
```

### 4.5 CursorOptions

```
CursorOptions {
    Visible:    bool
    GrabMode:   CursorGrabMode
    Icon:       CursorIcon
    HitTest:    bool              // whether the window receives pointer events
}

CursorGrabMode:
    None      // Cursor moves freely
    Confined  // Cursor cannot leave the window
    Locked    // Cursor is hidden and locked in place (FPS-style)

CursorIcon:
    Default | Pointer | Crosshair | Text | Wait | Help | Progress |
    Move | Grab | Grabbing | Custom(Handle<Image>)
```

Cursor state is part of the Window component and synchronized to the platform each frame.

### 4.6 PrimaryWindow Marker

A zero-sized marker component identifying the main application window:

- The `WindowPlugin` spawns the primary window on startup with default settings.
- Exactly one entity may hold PrimaryWindow at any time (INV-1).
- When the entity with PrimaryWindow is despawned or receives WindowCloseRequested, the engine sends an AppExit event (INV-2).
- User code can override properties by mutating the `Window` component on the marked entity.

### 4.7 Window Events

Platform events are translated into engine events each frame:

| Event | Payload |
| :--- | :--- |
| `WindowCreated` | window entity |
| `WindowResized` | window entity, new logical width/height |
| `WindowMoved` | window entity, new position |
| `WindowCloseRequested` | window entity |
| `WindowClosed` | window entity |
| `WindowFocused` | window entity, focused bool |
| `CursorEntered` | window entity |
| `CursorLeft` | window entity |
| `CursorMoved` | window entity, position in logical pixels |
| `FileDragAndDrop` | window entity, path list or HoveredFile/Dropped/Cancelled |
| `Ime` | window entity, composition text or commit string |
| `WindowScaleFactorChanged` | window entity, new scale factor |

A `WindowCloseRequested` event does not close the window by itself; user code must despawn the entity or suppress the event.

### 4.8 WindowPlugin

The plugin configures and spawns the primary window during app startup:

```
WindowPlugin {
    PrimaryWindow:       Option<Window>    // None = headless, no window
    ExitCondition:       ExitCondition     // OnPrimaryClosed | OnAllClosed | DontExit
    CloseWhenRequested:  bool              // auto-despawn on CloseRequested
}
```

### 4.9 Multi-Window Support

Additional windows are created by spawning a new entity with a Window component:

```
commands.Spawn(Window{ Title: "Inspector", Mode: Windowed, ... })
```

- Each window entity can have its own Camera (via `RenderTarget::Window(entity)`) for independent rendering.
- Closing a non-primary window despawns its entity but does not exit the app.
- Input events carry a window entity ID so systems know which window received the event.
- The render graph schedules render passes for each window independently.

### 4.10 Platform Abstraction

The windowing backend implements an interface:

```
WindowBackend interface {
    CreateWindow(entity Entity, settings WindowDescriptor) (RawWindowHandle, error)
    DestroyWindow(entity Entity) error
    ApplyChanges(entity Entity, diff WindowDiff) error
    PollEvents() []PlatformEvent
}
```

- The default backend wraps the native OS windowing API (equivalent to winit in Go).
- A web backend maps the window to an HTML canvas element.
- Custom backends can be registered for console platforms.
- A headless backend is provided for testing (no actual OS windows).
- Platform events are polled at the start of each frame, converted to engine events, and dispatched.

## 5. Open Questions

- Should secondary windows support their own independent ECS schedules or always share the main schedule?
- How should fullscreen toggle interact with saved window position/size restoration?
- What is the story for VR/XR headsets — are they modeled as windows or as a separate abstraction?
- Should window settings be hot-reloadable from a config file, or only settable via code?

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
| 0.1.0 | 2026-03-25 | Initial draft |
| — | — | Planned examples: `examples/window/` |
