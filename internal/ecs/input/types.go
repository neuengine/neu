package input

import neu "github.com/neuengine/neu/pkg/math"

// ── Keyboard ──────────────────────────────────────────────────────────────────

// KeyCode represents a physical keyboard key (layout-independent).
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

// ── Mouse ─────────────────────────────────────────────────────────────────────

// MouseButton represents mouse button identifiers.
type MouseButton uint8

const (
	MouseButtonLeft MouseButton = iota
	MouseButtonRight
	MouseButtonMiddle
	MouseButtonBack
	MouseButtonForward
)

// MouseMotion is an event carrying the mouse movement delta for the current frame.
type MouseMotion struct {
	Delta neu.Vec2
}

// MouseWheel is an event carrying scroll amounts for the current frame.
type MouseWheel struct {
	X float64
	Y float64
}

// CursorPosition is a resource tracking the current mouse cursor position
// in window-relative pixel coordinates (top-left is 0,0).
type CursorPosition struct {
	Position neu.Vec2
}

// ── Gamepad ───────────────────────────────────────────────────────────────────

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

// GamepadID identifies a connected gamepad.
type GamepadID uint8

// ── Touch ─────────────────────────────────────────────────────────────────────

// TouchPhase represents the lifecycle phase of a touch event.
type TouchPhase uint8

const (
	TouchPhaseStarted TouchPhase = iota
	TouchPhaseMoved
	TouchPhaseEnded
	TouchPhaseCancelled
)

// TouchInput represents a single touch event. ID uniquely identifies each
// simultaneous finger/pointer contact.
type TouchInput struct {
	Phase    TouchPhase
	ID       uint64
	Position neu.Vec2
	Force    float64
}

// ── Input events (sent via event bus, not resources) ──────────────────────────

// KeyboardInput is an event sent when a key is pressed or released.
type KeyboardInput struct {
	Key     KeyCode
	Pressed bool
}

// MouseButtonInput is an event sent when a mouse button changes state.
type MouseButtonInput struct {
	Button  MouseButton
	Pressed bool
}

// CursorMoved is an event sent when the cursor moves.
type CursorMoved struct {
	Position neu.Vec2
}

// GamepadConnectionEvent is sent when a gamepad is connected or disconnected.
type GamepadConnectionEvent struct {
	ID        GamepadID
	Connected bool
}

// GamepadButtonInput is an event sent when a gamepad button changes state.
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
