//go:build !headless

package platform

import pkgplatform "github.com/neuengine/neu/pkg/platform"

// NewPlatformProfile builds the profile for a normal (windowed) build. The OS
// and architecture are detected at runtime; capabilities follow from the OS
// class. A per-OS file with a more specific build tag (e.g. //go:build android)
// may override this in the future — the INV-5 extension path — without touching
// engine core.
func NewPlatformProfile() pkgplatform.PlatformProfile {
	os := detectOS()
	arch := detectArch()
	return pkgplatform.PlatformProfile{
		OS:           os,
		Arch:         arch,
		Tier:         tierFor(os, arch),
		Capabilities: defaultCaps(os),
	}
}

// defaultCaps returns the capability set for an OS class in a windowed build.
func defaultCaps(os pkgplatform.PlatformOS) pkgplatform.PlatformCaps {
	switch os {
	case pkgplatform.OSWindows, pkgplatform.OSLinux, pkgplatform.OSMacOS:
		return pkgplatform.HasGPU | pkgplatform.HasKeyboard | pkgplatform.HasMouse |
			pkgplatform.HasGamepad | pkgplatform.HasFileSystem | pkgplatform.HasMultiWindow |
			pkgplatform.HasClipboard | pkgplatform.HasSpatialAudio
	case pkgplatform.OSWeb:
		// Sandboxed: no local filesystem, single canvas (no multi-window).
		return pkgplatform.HasGPU | pkgplatform.HasKeyboard | pkgplatform.HasMouse |
			pkgplatform.HasGamepad | pkgplatform.HasClipboard
	case pkgplatform.OSAndroid, pkgplatform.OSIOS:
		// Touch-primary, no mouse, single window, haptics.
		return pkgplatform.HasGPU | pkgplatform.HasTouch | pkgplatform.HasFileSystem |
			pkgplatform.HasVibration | pkgplatform.HasSpatialAudio
	default:
		return pkgplatform.HasFileSystem
	}
}
