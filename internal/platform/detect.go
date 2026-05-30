// Package platform provides the build-tag-selected construction of the
// PlatformProfile and the plugin that inserts it. detect.go holds the shared
// runtime.GOOS/GOARCH mapping, compiled on every target (no build tags).
//
// Bootstrap: l2-platform-system-go Draft (Phase 6 Track G).
package platform

import (
	"runtime"

	pkgplatform "github.com/neuengine/neu/pkg/platform"
)

// detectOS maps the host's runtime.GOOS to a PlatformOS.
func detectOS() pkgplatform.PlatformOS { return osFromGOOS(runtime.GOOS) }

// detectArch maps the host's runtime.GOARCH to a PlatformArch.
func detectArch() pkgplatform.PlatformArch { return archFromGOARCH(runtime.GOARCH) }

// osFromGOOS is the pure GOOS→PlatformOS mapping (testable independently of the
// host). Modern Go reports iOS as GOOS=ios, so darwin always maps to macOS.
func osFromGOOS(goos string) pkgplatform.PlatformOS {
	switch goos {
	case "windows":
		return pkgplatform.OSWindows
	case "linux":
		return pkgplatform.OSLinux
	case "android":
		return pkgplatform.OSAndroid
	case "darwin":
		return pkgplatform.OSMacOS
	case "ios":
		return pkgplatform.OSIOS
	case "js", "wasip1":
		return pkgplatform.OSWeb
	default:
		return pkgplatform.OSUnknown
	}
}

// archFromGOARCH is the pure GOARCH→PlatformArch mapping.
func archFromGOARCH(goarch string) pkgplatform.PlatformArch {
	switch goarch {
	case "amd64":
		return pkgplatform.ArchAMD64
	case "arm64":
		return pkgplatform.ArchARM64
	case "wasm":
		return pkgplatform.ArchWASM
	default:
		return pkgplatform.ArchUnknown
	}
}

// tierFor returns the support tier for an OS/arch pair (L1 §4.1).
func tierFor(os pkgplatform.PlatformOS, arch pkgplatform.PlatformArch) pkgplatform.PlatformTier {
	switch os {
	case pkgplatform.OSWindows, pkgplatform.OSMacOS:
		return pkgplatform.Tier1
	case pkgplatform.OSLinux:
		if arch == pkgplatform.ArchAMD64 {
			return pkgplatform.Tier1
		}
		return pkgplatform.Tier3 // Linux arm64 etc.
	case pkgplatform.OSWeb, pkgplatform.OSAndroid, pkgplatform.OSIOS:
		return pkgplatform.Tier2
	default:
		return pkgplatform.TierUnknown
	}
}
