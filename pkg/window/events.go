package window

import "github.com/neuengine/neu/pkg/ecs"

// PlatformEventKind discriminates the window events the backend reports (L1 §4.7).
type PlatformEventKind uint8

const (
	EventCreated PlatformEventKind = iota
	EventResized
	EventMoved
	EventCloseRequested
	EventClosed
	EventFocused
	EventCursorEntered
	EventCursorLeft
	EventCursorMoved
	EventScaleFactorChanged
)

// String renders the event kind. Total switch.
func (k PlatformEventKind) String() string {
	switch k {
	case EventCreated:
		return "Created"
	case EventResized:
		return "Resized"
	case EventMoved:
		return "Moved"
	case EventCloseRequested:
		return "CloseRequested"
	case EventClosed:
		return "Closed"
	case EventFocused:
		return "Focused"
	case EventCursorEntered:
		return "CursorEntered"
	case EventCursorLeft:
		return "CursorLeft"
	case EventCursorMoved:
		return "CursorMoved"
	case EventScaleFactorChanged:
		return "ScaleFactorChanged"
	default:
		return "Unknown"
	}
}

// PlatformEvent is a backend-reported window event, carrying the originating
// window entity so multi-window input is unambiguous (L1 §4.7). Payload fields
// are interpreted per Kind; unused fields are zero.
type PlatformEvent struct {
	Kind          PlatformEventKind
	Window        ecs.Entity
	Width, Height uint32  // Resized
	X, Y          int     // Moved, CursorMoved (logical pixels)
	Focused       bool    // Focused
	ScaleFactor   float64 // ScaleFactorChanged
}

// CausesAppExit reports whether a close event on window e should trigger AppExit
// under the given exit condition. primary indicates whether e is the primary
// window; remaining is the window count after this close (for OnAllClosed).
func CausesAppExit(kind PlatformEventKind, cond ExitCondition, primary bool, remaining int) bool {
	if kind != EventCloseRequested && kind != EventClosed {
		return false
	}
	switch cond {
	case OnPrimaryClosed:
		return primary
	case OnAllClosed:
		return remaining == 0
	default: // DontExit
		return false
	}
}
