//go:build !headless

package platform

import (
	"testing"

	pkgplatform "github.com/neuengine/neu/pkg/platform"
)

// TestDefaultCaps exercises every OS branch of defaultCaps (the test host only
// drives one branch through NewPlatformProfile, so call it directly here).
func TestDefaultCaps(t *testing.T) {
	t.Parallel()
	cases := []struct {
		os       pkgplatform.PlatformOS
		mustHave pkgplatform.PlatformCaps
		mustLack pkgplatform.PlatformCaps
	}{
		{pkgplatform.OSWindows, pkgplatform.HasGPU | pkgplatform.HasMouse | pkgplatform.HasMultiWindow, 0},
		{pkgplatform.OSLinux, pkgplatform.HasGPU | pkgplatform.HasFileSystem, 0},
		{pkgplatform.OSMacOS, pkgplatform.HasGPU | pkgplatform.HasClipboard, 0},
		{pkgplatform.OSWeb, pkgplatform.HasGPU | pkgplatform.HasKeyboard, pkgplatform.HasFileSystem | pkgplatform.HasMultiWindow},
		{pkgplatform.OSAndroid, pkgplatform.HasTouch | pkgplatform.HasVibration, pkgplatform.HasMouse | pkgplatform.HasMultiWindow},
		{pkgplatform.OSIOS, pkgplatform.HasTouch | pkgplatform.HasSpatialAudio, pkgplatform.HasMouse},
		{pkgplatform.OSUnknown, pkgplatform.HasFileSystem, pkgplatform.HasGPU},
	}
	for _, c := range cases {
		got := defaultCaps(c.os)
		if c.mustHave != 0 && !got.Has(c.mustHave) {
			t.Errorf("defaultCaps(%v) = %s; missing %s", c.os, got, c.mustHave)
		}
		if c.mustLack != 0 && got&c.mustLack != 0 {
			t.Errorf("defaultCaps(%v) = %s; should lack %s", c.os, got, c.mustLack)
		}
	}
}

// TestDetectOSNoPanic ensures detectOS resolves on the test host without panic.
func TestDetectOSNoPanic(t *testing.T) {
	t.Parallel()
	_ = detectOS()
	_ = detectArch()
}
