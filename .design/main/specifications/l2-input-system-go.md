# Input System — Go Implementation

**Version:** 0.2.0
**Status:** Stable
**Layer:** go
**Implements:** [input-system.md](l1-input-system.md)

## Overview

This specification defines the Go implementation of the input system described in the L1 concept spec. The input system provides a device-agnostic abstraction for keyboard, mouse, gamepad, and touch input. The core type is the generic `ButtonInput[T]` resource that tracks pressed/just-pressed/just-released state using map-based sets. Device types use `iota` enums. All types live in the `internal/input` package with a dependency on `internal/ecs` for resource registration and `internal/math` for Vec2.

## Related Specifications

- [input-system.md](l1-input-system.md) — L1 concept specification (parent)
- [l2-input-system-go-codes.md](l2-input-system-go-codes.md) — sibling L2: KeyCode/Gamepad/Touch reference type tables (extracted §Type Definitions)

## 1. Motivation

The Go implementation of the Input system provides a unified, cross-platform interface for all user input devices. It ensures:

- Device-agnostic button and axis tracking via generic `ButtonInput[T]` and `AxisInput[T]`.
- Frame-perfect state detection (Pressed, JustPressed, JustReleased).
- High-performance, low-allocation event dispatch for immediate input reactions.
- Integration with the ECS resource system for global input availability.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: Relies on generics for type-safe input mapping and `comparable` constraints on maps.
- **PreUpdate Schedule**: Input state is updated at the beginning of each frame before any user systems execute.
- **Coordinate System**: Mouse/Touch positions are window-relative pixels (top-left is 0,0).

## 3. Core Invariants

> [!NOTE]
> See [input-system.md §3](l1-input-system.md) for technology-agnostic invariants.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: State Tracking | `ButtonInput` uses maps to track per-frame state transitions accurately. |
| **INV-2**: Abstraction | Device-specific `KeyCode`, `MouseButton`, `GamepadButton` all use the same `ButtonInput` logic. |
| **INV-3**: Buffering | Per-frame states (`JustPressed`, `JustReleased`) are cleared at the start of `PreUpdate`. |
| **INV-4**: Read-Only Safety | Input resources are updated by the internal `update_input` system; user systems treat them as read-only. |
| **INV-5**: Multi-touch | `TouchInput` events include unique `ID`s for simultaneous pointer tracking. |

## Go Package

```
internal/input/
```

All types in this spec belong to package `input`. The package imports `internal/ecs` for World and resource registration and `internal/math` for vector types.

## Type Definitions

### ButtonInput[T]

```go
// ButtonInput is a generic resource that tracks the pressed/released state
// of buttons (keys, mouse buttons, gamepad buttons). T must be comparable
// for use as a map key.
type ButtonInput[T comparable] struct {
    pressed      map[T]bool // currently held down
    justPressed  map[T]bool // transitioned to pressed this frame
    justReleased map[T]bool // transitioned to released this frame
}

// NewButtonInput creates an empty ButtonInput with pre-allocated maps.
func NewButtonInput[T comparable]() *ButtonInput[T]

// Pressed reports whether the button is currently held down.
func (b *ButtonInput[T]) Pressed(button T) bool

// JustPressed reports whether the button transitioned from released to pressed
// this frame. True for exactly one frame per press.
func (b *ButtonInput[T]) JustPressed(button T) bool

// JustReleased reports whether the button transitioned from pressed to released
// this frame. True for exactly one frame per release.
func (b *ButtonInput[T]) JustReleased(button T) bool

// AnyPressed reports whether any button is currently held down.
func (b *ButtonInput[T]) AnyPressed() bool

// AnyJustPressed reports whether any button was just pressed this frame.
func (b *ButtonInput[T]) AnyJustPressed() bool

// GetPressed returns a slice of all currently pressed buttons.
func (b *ButtonInput[T]) GetPressed() []T

// GetJustPressed returns a slice of all buttons just pressed this frame.
func (b *ButtonInput[T]) GetJustPressed() []T

// GetJustReleased returns a slice of all buttons just released this frame.
func (b *ButtonInput[T]) GetJustReleased() []T

// Press records a button press. Adds to pressed and justPressed sets.
// If the button was already pressed, it is not added to justPressed.
func (b *ButtonInput[T]) Press(button T)

// Release records a button release. Removes from pressed, adds to justReleased.
// If the button was not pressed, it is not added to justReleased.
func (b *ButtonInput[T]) Release(button T)

// Clear resets justPressed and justReleased sets for the new frame.
// Called by the engine at the start of input processing, not by user systems.
func (b *ButtonInput[T]) Clear()

// Reset clears all state — pressed, justPressed, and justReleased.
func (b *ButtonInput[T]) Reset()
```

### AxisInput[T]

```go
// AxisInput is a generic resource for analog inputs (gamepad axes, mouse
// position). T identifies the axis. Values are typically normalized
// (-1.0 to 1.0 for sticks, 0.0 to 1.0 for triggers).
type AxisInput[T comparable] struct {
    values map[T]float64
}

// NewAxisInput creates an empty AxisInput resource.
func NewAxisInput[T comparable]() *AxisInput[T]

// Get returns the current value of the axis. Returns 0 if not present.
func (a *AxisInput[T]) Get(axis T) float64

// Set updates the current value of the axis.
func (a *AxisInput[T]) Set(axis T, value float64)

// Reset sets all axis values to 0.
func (a *AxisInput[T]) Reset()
```

### KeyCode

> **→ [l2-input-system-go-codes.md](l2-input-system-go-codes.md) §3** — full
> `KeyCode uint16` enum (alphanumerics, function keys, navigation, modifiers,
> punctuation, `KeyCodeCount` sentinel). Extracted as a reference table to keep
> this spec focused on input state logic.

### MouseButton

```go
// MouseButton represents mouse button identifiers.
type MouseButton uint8

const (
    MouseButtonLeft MouseButton = iota
    MouseButtonRight
    MouseButtonMiddle
    MouseButtonBack
    MouseButtonForward
)
```

### MouseMotion

```go
// MouseMotion represents mouse movement delta for the current frame.
// Stored as an event, not a resource.
type MouseMotion struct {
    Delta math.Vec2
}
```

### MouseWheel

```go
// MouseWheel represents mouse wheel scroll for the current frame.
type MouseWheel struct {
    X float64 // horizontal scroll
    Y float64 // vertical scroll
}
```

### CursorPosition

```go
// CursorPosition is a resource tracking the current mouse cursor position
// in window-relative pixel coordinates.
type CursorPosition struct {
    Position math.Vec2
}
```

### Gamepad & Touch Types

> **→ [l2-input-system-go-codes.md](l2-input-system-go-codes.md) §3** — full
> `GamepadButton`, `GamepadAxis`, `GamepadID`, `TouchPhase`, and `TouchInput`
> definitions. Extracted as reference tables alongside `KeyCode`.

### Input Events

```go
// KeyboardInput is an event sent when a key is pressed or released.
type KeyboardInput struct {
    Key     KeyCode
    Pressed bool
}

// MouseButtonInput is an event sent when a mouse button is pressed or released.
type MouseButtonInput struct {
    Button  MouseButton
    Pressed bool
}

// CursorMoved is an event sent when the cursor moves.
type CursorMoved struct {
    Position math.Vec2
}

// GamepadConnectionEvent is sent when a gamepad is connected or disconnected.
type GamepadConnectionEvent struct {
    ID        GamepadID
    Connected bool
}

// GamepadButtonInput is an event sent when a gamepad button is pressed or released.
type GamepadButtonInput struct {
    Gamepad GamepadID
    Button  GamepadButton
    Pressed bool
}

// GamepadAxisChanged is an event sent when a gamepad axis value changes.
type GamepadAxisChanged struct {
    Gamepad GamepadID
    Axis    GamepadAxis
    Value   float64
}
```

## Key Methods

### Input Update System (PreUpdate Schedule)

```
SYSTEM update_input(world, platformEvents):
  // Step 1: Clear per-frame state from previous frame
  keyboard = world.Resource(ButtonInput[KeyCode])
  keyboard.Clear()

  mouse = world.Resource(ButtonInput[MouseButton])
  mouse.Clear()

  gamepad = world.Resource(ButtonInput[GamepadButton])
  gamepad.Clear()

  // Step 2: Process platform events for this frame
  FOR EACH event IN platformEvents:
    MATCH event:
      KeyDown(key):
        keyboard.Press(key)
        world.SendEvent(KeyboardInput{Key: key, Pressed: true})

      KeyUp(key):
        keyboard.Release(key)
        world.SendEvent(KeyboardInput{Key: key, Pressed: false})

      MouseDown(button):
        mouse.Press(button)
        world.SendEvent(MouseButtonInput{Button: button, Pressed: true})

      MouseUp(button):
        mouse.Release(button)
        world.SendEvent(MouseButtonInput{Button: button, Pressed: false})

      MouseMove(x, y):
        world.Resource(CursorPosition).Position = Vec2{x, y}
        world.SendEvent(CursorMoved{Position: Vec2{x, y}})

      MouseMotionDelta(dx, dy):
        world.SendEvent(MouseMotion{Delta: Vec2{dx, dy}})

      MouseScroll(x, y):
        world.SendEvent(MouseWheel{X: x, Y: y})

      GamepadButton(id, button, pressed):
        IF pressed: gamepad.Press(button)
        ELSE: gamepad.Release(button)
        world.SendEvent(GamepadButtonInput{...})

      GamepadAxis(id, axis, value):
        world.Resource(AxisInput[GamepadAxis]).Set(axis, value)
        world.SendEvent(GamepadAxisChanged{...})

      Touch(phase, id, pos, force):
        world.SendEvent(TouchInput{...})
```

### Resource Registration

The following resources are registered in the World by `InputPlugin`:

```
ButtonInput[KeyCode]       — keyboard button state
ButtonInput[MouseButton]   — mouse button state
ButtonInput[GamepadButton] — gamepad button state
AxisInput[GamepadAxis]     — gamepad analog axes
CursorPosition             — current cursor position
```

### InputPlugin

```go
// InputPlugin registers all input resources and the input update system.
type InputPlugin struct{}

// Build inserts ButtonInput, AxisInput, and CursorPosition resources into
// the World and registers the input update system in PreUpdate.
func (p InputPlugin) Build(app *app.App)
```

## Performance Strategy

- **map[T]bool for sets**: Simple and allocation-free after initial growth. For typical input counts (< 10 simultaneously pressed keys), map overhead is negligible.
- **Clear resets maps in place**: `justPressed` and `justReleased` maps are cleared with `clear()` builtin (Go 1.21+), reusing allocated memory.
- **No per-key allocation**: `KeyCode`, `MouseButton`, `GamepadButton` are integer types — no heap allocation when used as map keys.
- **Single system per frame**: All input processing runs in one system in `PreUpdate`, minimizing overhead.
- **Event batching**: All platform events for a frame are processed in a single pass.

## Error Handling

- **Unknown key codes**: Platform events with unrecognized key codes are silently ignored. Log at debug level via `log/slog`.
- **Gamepad disconnection**: On disconnect, all buttons for that gamepad are released. The `ButtonInput[GamepadButton]` resource is shared across gamepads — per-gamepad tracking requires the GamepadID in events.
- **Double press in single frame**: If a key is pressed and released in the same frame, both `justPressed` and `justReleased` are true for that frame.
- **No platform backend**: If no window/platform backend is registered, the input system runs with no events (all inputs stay at default state).

## Testing Strategy

- **Unit tests**: Create `ButtonInput[KeyCode]`, call `Press(KeyA)`, verify `Pressed`, `JustPressed`. Call `Clear`, verify `JustPressed` is false but `Pressed` is still true. Call `Release(KeyA)`, verify `JustReleased`.
- **ButtonInput semantics**: Press + Release in same frame — verify both `JustPressed` and `JustReleased` are true.
- **AxisInput**: Set axis value, verify `Get` returns it. Reset, verify zero.
- **Clear vs Reset**: Verify `Clear` only clears per-frame state, `Reset` clears everything.
- **Multi-button**: Press 5 keys, verify `GetPressed` returns all 5. Verify `AnyPressed` is true.
- **Event generation**: Mock platform events, run input system, verify correct engine events are sent.
- **Integration**: Register InputPlugin in an App, simulate key press via platform event, verify `ButtonInput[KeyCode].JustPressed` in a test system.
- **Benchmarks**: `BenchmarkButtonInputPress`, `BenchmarkButtonInputClear` with 10 active keys — target zero allocations after warmup.

## 7. Drawbacks & Alternatives

- **Drawback**: Map-based state tracking can have overhead for a very large number of simultaneous inputs.
- **Alternative**: Fixed-size bitsets for button states.
- **Decision**: Maps are used for initial flexibility and generics support. Bitsets can be swapped in for performance-critical platforms if needed.

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
| 0.2.0 | 2026-05-15 | Extracted `KeyCode`/`Gamepad*`/`Touch*` type tables into `l2-input-system-go-codes.md` to clear SPEC_DECOMPOSE (531→364 lines); §Type Definitions now references the codes spec |
