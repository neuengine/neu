# Input System Codes — Go Implementation

**Version:** 0.1.0
**Status:** Stable
**Layer:** go
**Implements:** [input-system.md](l1-input-system.md)

## Overview

Go enumeration and reference type tables for the input system: the full
`KeyCode` keyboard map, gamepad identifiers (`GamepadButton`, `GamepadAxis`,
`GamepadID`), and touch types (`TouchPhase`, `TouchInput`).

Extracted from [l2-input-system-go.md](l2-input-system-go.md) §Type Definitions
to keep that spec focused on the input *state machine* (the generic
`ButtonInput[T]` / `AxisInput[T]` resources, update systems, plugin wiring)
while this spec owns the large device-code reference tables. Both specs share
the same L1 parent.

## Related Specifications

- [input-system.md](l1-input-system.md) — L1 concept specification (parent)
- [l2-input-system-go.md](l2-input-system-go.md) — sibling L2: input state resources, systems, plugin

## 1. Constraints & Assumptions

- All code enums use `iota`-based `uint8`/`uint16` for compact map keys.
- `KeyCode` represents **physical** key positions (layout-independent).
- Enum ordering is append-only; new codes are added before the sentinel where
  one exists (`KeyCodeCount`) to keep existing values stable.

## 2. Invariant Compliance

| L1 Invariant (l1-input-system) | Go Realization |
| :--- | :--- |
| Device-agnostic abstraction | Distinct typed enums per device class; no raw platform codes leak upward |
| Stable identifiers across frames | `iota` values are positional and append-only; `KeyCodeCount` sentinel bounds the set |
| Multi-touch support | `TouchInput.ID` uniquely identifies each finger/pointer |

## 3. Type Definitions

### KeyCode

```go
// KeyCode represents physical keyboard key codes.
type KeyCode uint16

const (
    KeyA KeyCode = iota
    KeyB
    KeyC
    KeyD
    KeyE
    KeyF
    KeyG
    KeyH
    KeyI
    KeyJ
    KeyK
    KeyL
    KeyM
    KeyN
    KeyO
    KeyP
    KeyQ
    KeyR
    KeyS
    KeyT
    KeyU
    KeyV
    KeyW
    KeyX
    KeyY
    KeyZ

    KeyDigit0
    KeyDigit1
    KeyDigit2
    KeyDigit3
    KeyDigit4
    KeyDigit5
    KeyDigit6
    KeyDigit7
    KeyDigit8
    KeyDigit9

    KeyF1
    KeyF2
    KeyF3
    KeyF4
    KeyF5
    KeyF6
    KeyF7
    KeyF8
    KeyF9
    KeyF10
    KeyF11
    KeyF12

    KeyEscape
    KeyEnter
    KeySpace
    KeyBackspace
    KeyTab
    KeyDelete
    KeyInsert
    KeyHome
    KeyEnd
    KeyPageUp
    KeyPageDown

    KeyArrowUp
    KeyArrowDown
    KeyArrowLeft
    KeyArrowRight

    KeyShiftLeft
    KeyShiftRight
    KeyControlLeft
    KeyControlRight
    KeyAltLeft
    KeyAltRight
    KeySuperLeft
    KeySuperRight

    KeyMinus
    KeyEqual
    KeyBracketLeft
    KeyBracketRight
    KeyBackslash
    KeySemicolon
    KeyQuote
    KeyBackquote
    KeyComma
    KeyPeriod
    KeySlash
    KeyCapsLock
    KeyNumLock
    KeyScrollLock
    KeyPrintScreen
    KeyPause

    KeyCodeCount // sentinel — total number of key codes
)
```

### GamepadButton

```go
// GamepadButton represents gamepad button identifiers.
type GamepadButton uint8

const (
    GamepadButtonSouth GamepadButton = iota // A / Cross
    GamepadButtonEast                       // B / Circle
    GamepadButtonNorth                      // Y / Triangle
    GamepadButtonWest                       // X / Square
    GamepadButtonLeftBumper
    GamepadButtonRightBumper
    GamepadButtonLeftTrigger
    GamepadButtonRightTrigger
    GamepadButtonSelect
    GamepadButtonStart
    GamepadButtonLeftStick
    GamepadButtonRightStick
    GamepadButtonDPadUp
    GamepadButtonDPadDown
    GamepadButtonDPadLeft
    GamepadButtonDPadRight
)
```

### GamepadAxis

```go
// GamepadAxis represents analog gamepad axes.
type GamepadAxis uint8

const (
    GamepadAxisLeftStickX GamepadAxis = iota
    GamepadAxisLeftStickY
    GamepadAxisRightStickX
    GamepadAxisRightStickY
    GamepadAxisLeftTrigger
    GamepadAxisRightTrigger
)
```

### GamepadID

```go
// GamepadID identifies a connected gamepad.
type GamepadID uint8
```

### TouchPhase

```go
// TouchPhase represents the phase of a touch event.
type TouchPhase uint8

const (
    TouchPhaseStarted   TouchPhase = iota
    TouchPhaseMoved
    TouchPhaseEnded
    TouchPhaseCancelled
)
```

### TouchInput

```go
// TouchInput represents a single touch event with multi-touch support.
type TouchInput struct {
    Phase    TouchPhase
    ID       uint64    // unique identifier per finger/pointer
    Position math.Vec2 // screen position
    Force    float64   // pressure (0.0 to 1.0, if available)
}
```

## 4. Open Questions

<!-- TBD: Should KeyCode include a platform-native scancode passthrough field
     for IME / non-Latin layouts, or is physical-position-only sufficient? -->

## Canonical References

<!-- MANDATORY for Stable status. List authoritative source files that downstream agents
     MUST read before implementing this spec. Use relative paths from project root.
     Stub state — fill with concrete files when implementation begins (Phase 2). -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

<!-- Empty table = no canonical sources yet. Populate one row per authoritative file
     when implementation lands (Phase 2). Stable promotion requires ≥1 row. -->

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-15 | Initial draft: extracted `KeyCode`, `GamepadButton/Axis/ID`, `TouchPhase/TouchInput` from `l2-input-system-go.md` per `/magic-analyze` SPEC_DECOMPOSE |
