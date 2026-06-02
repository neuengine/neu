// Package window models OS windows as ECS entities: a Window component carries
// each window's properties, a PrimaryWindow marker tags the main window, and a
// pluggable WindowBackend applies changes on the main thread. Window create and
// destroy are deferred through the command queue; a headless backend keeps CI
// window-free.
//
// Bootstrap: l2-window-system-go Draft (Phase 6 Track B, C29 gate open).
package window

// WindowMode selects the window's display mode.
type WindowMode uint8

const (
	// Windowed is a standard resizable/decorated window.
	Windowed WindowMode = iota
	// BorderlessFullscreen is a borderless window matching monitor resolution.
	BorderlessFullscreen
	// SizedFullscreen is a borderless window at a custom resolution.
	SizedFullscreen
	// Fullscreen is exclusive fullscreen with a mode switch.
	Fullscreen
)

// PresentMode controls vertical-sync behavior for the render surface.
type PresentMode uint8

const (
	// AutoVsync lets the engine pick the best vsync mode for the platform.
	AutoVsync PresentMode = iota
	// AutoNoVsync lets the engine pick the best non-vsync mode.
	AutoNoVsync
	// Fifo is traditional vsync (wait for vblank).
	Fifo
	// Immediate presents without sync (may tear).
	Immediate
	// Mailbox is triple-buffered low-latency vsync.
	Mailbox
)

// PositionMode selects automatic placement versus an explicit position.
type PositionMode uint8

const (
	// PositionAutomatic lets the OS place the window.
	PositionAutomatic PositionMode = iota
	// PositionAt places the window at explicit coordinates.
	PositionAt
)

// WindowPosition is the requested window placement.
type WindowPosition struct {
	Mode PositionMode
	X, Y int
}

// WindowResolution tracks physical size and DPI scale. Logical size is physical
// size divided by the scale factor.
type WindowResolution struct {
	PhysicalWidth, PhysicalHeight uint32
	ScaleFactorOverride           float64 // 0 ⇒ use the OS scale (assumed 1.0 here)
}

// Logical returns the logical (scaled) width and height in logical pixels.
func (r WindowResolution) Logical() (w, h float64) {
	scale := r.ScaleFactorOverride
	if scale <= 0 {
		scale = 1
	}
	return float64(r.PhysicalWidth) / scale, float64(r.PhysicalHeight) / scale
}

// CursorGrabMode controls cursor confinement.
type CursorGrabMode uint8

const (
	// GrabNone lets the cursor move freely.
	GrabNone CursorGrabMode = iota
	// GrabConfined keeps the cursor inside the window.
	GrabConfined
	// GrabLocked hides and locks the cursor in place (FPS-style).
	GrabLocked
)

// CursorIcon names a standard cursor shape.
type CursorIcon uint8

const (
	CursorDefault CursorIcon = iota
	CursorPointer
	CursorCrosshair
	CursorText
	CursorWait
	CursorGrab
	CursorGrabbing
)

// CursorOptions is the cursor state carried on the Window component.
type CursorOptions struct {
	Visible  bool
	GrabMode CursorGrabMode
	Icon     CursorIcon
	HitTest  bool // whether the window receives pointer events
}

// Window is the component attached to every window entity (L1 §4.1). Users
// mutate it; the backend synchronizes platform state from a per-frame diff.
type Window struct {
	Title       string
	Canvas      string
	Position    WindowPosition
	Resolution  WindowResolution
	Cursor      CursorOptions
	Transparent bool
	Decorations bool
	Visible     bool
	Focused     bool
	PresentMode PresentMode
	Resizable   bool
	ImeEnabled  bool
	Mode        WindowMode
}

// DefaultWindow returns a sensible windowed default (1280×720, decorated, visible).
func DefaultWindow(title string) Window {
	return Window{
		Title:       title,
		Mode:        Windowed,
		Resolution:  WindowResolution{PhysicalWidth: 1280, PhysicalHeight: 720},
		Position:    WindowPosition{Mode: PositionAutomatic},
		Resizable:   true,
		Decorations: true,
		Visible:     true,
		PresentMode: AutoVsync,
		Cursor:      CursorOptions{Visible: true, HitTest: true},
	}
}

// PrimaryWindow is a zero-sized marker identifying the main application window.
// Exactly one entity holds it while the app runs (INV-1).
type PrimaryWindow struct{}
