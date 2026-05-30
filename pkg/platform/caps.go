// Package platform defines the cross-platform capability model: an immutable
// PlatformProfile resource (OS, architecture, support tier) plus a branchless
// PlatformCaps bitfield systems query for runtime feature negotiation. The
// concrete per-OS profile construction and the wiring plugin live in
// internal/platform behind //go:build tags; this package is pure data + the
// PlatformPlugin interface, compiled unconditionally on every target.
//
// Bootstrap: l2-platform-system-go Draft (Phase 6 Track G, C29 gate open).
package platform

import "strings"

// PlatformCaps is a bitfield of available platform features. Has is a single
// mask-and-compare, so capability gates in hot systems are branchless and
// allocation-free.
type PlatformCaps uint32

const (
	// HasGPU indicates hardware-accelerated rendering is available.
	HasGPU PlatformCaps = 1 << iota
	// HasTouch indicates touch input is available.
	HasTouch
	// HasGamepad indicates gamepad input is available.
	HasGamepad
	// HasKeyboard indicates a physical keyboard is available.
	HasKeyboard
	// HasMouse indicates a mouse or trackpad is available.
	HasMouse
	// HasFileSystem indicates local filesystem access (false for sandboxed web).
	HasFileSystem
	// HasMultiWindow indicates multiple OS windows are supported.
	HasMultiWindow
	// HasClipboard indicates system clipboard access.
	HasClipboard
	// HasVibration indicates haptic feedback is available.
	HasVibration
	// HasSpatialAudio indicates 3D audio positioning is available.
	HasSpatialAudio
)

// Has reports whether all bits in f are set in c.
func (c PlatformCaps) Has(f PlatformCaps) bool { return c&f == f }

// With returns c with the bits in f added (immutable; returns a new value).
func (c PlatformCaps) With(f PlatformCaps) PlatformCaps { return c | f }

// Without returns c with the bits in f cleared.
func (c PlatformCaps) Without(f PlatformCaps) PlatformCaps { return c &^ f }

// capName pairs a single-bit capability with its label for String().
var capNames = []struct {
	bit  PlatformCaps
	name string
}{
	{HasGPU, "GPU"},
	{HasTouch, "Touch"},
	{HasGamepad, "Gamepad"},
	{HasKeyboard, "Keyboard"},
	{HasMouse, "Mouse"},
	{HasFileSystem, "FileSystem"},
	{HasMultiWindow, "MultiWindow"},
	{HasClipboard, "Clipboard"},
	{HasVibration, "Vibration"},
	{HasSpatialAudio, "SpatialAudio"},
}

// String lists the set capabilities, e.g. "GPU|Keyboard|Mouse".
func (c PlatformCaps) String() string {
	if c == 0 {
		return "none"
	}
	var b strings.Builder
	for _, cn := range capNames {
		if c.Has(cn.bit) {
			if b.Len() > 0 {
				b.WriteByte('|')
			}
			b.WriteString(cn.name)
		}
	}
	return b.String()
}
