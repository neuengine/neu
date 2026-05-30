//go:build headless

package platform

import pkgplatform "github.com/neuengine/neu/pkg/platform"

// NewPlatformProfile builds the profile for a headless build (-tags headless):
// no window, no GPU, no audio — suitable for dedicated servers and CI (INV-4).
// The OS/arch are still detected, but the capability set explicitly excludes
// HasGPU, HasMultiWindow, and HasSpatialAudio. The headless platform plugin
// wires the existing render nopBackend / audio HeadlessBackend in this build.
func NewPlatformProfile() pkgplatform.PlatformProfile {
	os := detectOS()
	arch := detectArch()
	return pkgplatform.PlatformProfile{
		OS:   os,
		Arch: arch,
		Tier: tierFor(os, arch),
		// Server capabilities only: filesystem + keyboard (CLI). No GPU, no
		// window, no spatial audio.
		Capabilities: pkgplatform.HasFileSystem | pkgplatform.HasKeyboard,
	}
}
