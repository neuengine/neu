package window

import "github.com/neuengine/neu/pkg/ecs"

// RawWindowHandle is an opaque reference to a platform window/surface, handed to
// the render core for swapchain creation. Its payload is intentionally hidden so
// no platform shape leaks into the public type.
type RawWindowHandle struct{ id uint64 }

// NewRawWindowHandle wraps a backend-assigned id.
func NewRawWindowHandle(id uint64) RawWindowHandle { return RawWindowHandle{id: id} }

// ID returns the backend-assigned identifier.
func (h RawWindowHandle) ID() uint64 { return h.id }

// IsValid reports whether the handle refers to a created window (id != 0).
func (h RawWindowHandle) IsValid() bool { return h.id != 0 }

// WindowDescriptor is the immutable subset of Window needed to create a window.
type WindowDescriptor struct {
	Title       string
	Canvas      string
	Position    WindowPosition
	Resolution  WindowResolution
	Mode        WindowMode
	Decorations bool
	Transparent bool
	PresentMode PresentMode
}

// DescriptorFromWindow extracts the creation descriptor from a Window.
func DescriptorFromWindow(w Window) WindowDescriptor {
	return WindowDescriptor{
		Title:       w.Title,
		Resolution:  w.Resolution,
		Mode:        w.Mode,
		Position:    w.Position,
		Decorations: w.Decorations,
		Transparent: w.Transparent,
		PresentMode: w.PresentMode,
		Canvas:      w.Canvas,
	}
}

// WindowDiff carries only the fields that changed between two Window states; a
// nil pointer means "unchanged". The backend applies exactly the set fields,
// so an unchanged frame yields an empty diff and no ApplyChanges call (INV-4).
type WindowDiff struct {
	Title       *string
	Mode        *WindowMode
	Resolution  *WindowResolution
	Position    *WindowPosition
	Cursor      *CursorOptions
	PresentMode *PresentMode
	Visible     *bool
	Decorations *bool
}

// HasChanges reports whether any field changed.
func (d WindowDiff) HasChanges() bool {
	return d.Title != nil || d.Mode != nil || d.Resolution != nil || d.Position != nil ||
		d.Cursor != nil || d.PresentMode != nil || d.Visible != nil || d.Decorations != nil
}

// WindowBackend is the pluggable windowing implementation. All calls run on the
// main thread (OS constraint). The default backend wraps native OS windowing; a
// headless backend (internal/window) provides a window-free implementation for
// CI and dedicated servers.
type WindowBackend interface {
	CreateWindow(e ecs.Entity, d WindowDescriptor) (RawWindowHandle, error)
	DestroyWindow(e ecs.Entity) error
	ApplyChanges(e ecs.Entity, diff WindowDiff) error
	PollEvents() []PlatformEvent
}

// ExitCondition decides when the app exits in response to window closes (L1 §4.8).
type ExitCondition uint8

const (
	// OnPrimaryClosed exits when the primary window closes (default).
	OnPrimaryClosed ExitCondition = iota
	// OnAllClosed exits when the last window closes.
	OnAllClosed
	// DontExit never exits on window close.
	DontExit
)

// WindowPlugin configures and (in a full App build) spawns the primary window.
type WindowPlugin struct {
	PrimaryWindow      *Window // nil ⇒ headless, no primary window
	ExitCondition      ExitCondition
	CloseWhenRequested bool
}
