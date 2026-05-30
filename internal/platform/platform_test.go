package platform

import (
	"testing"

	"github.com/neuengine/neu/pkg/app/appface"
	pkgplatform "github.com/neuengine/neu/pkg/platform"
)

func TestNewPlatformProfile(t *testing.T) {
	t.Parallel()
	p := NewPlatformProfile()
	// OS/arch are detected from the test host; both must resolve to a known value.
	if p.OS == pkgplatform.OSUnknown {
		t.Errorf("OS should be detected, got Unknown")
	}
	if p.Arch == pkgplatform.ArchUnknown {
		t.Errorf("Arch should be detected, got Unknown")
	}
	if p.Tier == pkgplatform.TierUnknown {
		t.Errorf("Tier should be assigned, got TierUnknown")
	}
	// The default (non-headless) build on a desktop host has a GPU + filesystem.
	// (This test runs under !headless, so defaultCaps applies.)
	if p.OS == pkgplatform.OSWindows || p.OS == pkgplatform.OSLinux || p.OS == pkgplatform.OSMacOS {
		if !p.Capabilities.Has(pkgplatform.HasFileSystem) {
			t.Errorf("desktop profile should have HasFileSystem; caps=%s", p.Capabilities)
		}
	}
}

func TestOSArchMapping(t *testing.T) {
	t.Parallel()
	osCases := map[string]pkgplatform.PlatformOS{
		"windows": pkgplatform.OSWindows,
		"linux":   pkgplatform.OSLinux,
		"android": pkgplatform.OSAndroid,
		"darwin":  pkgplatform.OSMacOS,
		"ios":     pkgplatform.OSIOS,
		"js":      pkgplatform.OSWeb,
		"wasip1":  pkgplatform.OSWeb,
		"plan9":   pkgplatform.OSUnknown,
	}
	for goos, want := range osCases {
		if got := osFromGOOS(goos); got != want {
			t.Errorf("osFromGOOS(%q) = %v, want %v", goos, got, want)
		}
	}
	archCases := map[string]pkgplatform.PlatformArch{
		"amd64": pkgplatform.ArchAMD64,
		"arm64": pkgplatform.ArchARM64,
		"wasm":  pkgplatform.ArchWASM,
		"mips":  pkgplatform.ArchUnknown,
	}
	for goarch, want := range archCases {
		if got := archFromGOARCH(goarch); got != want {
			t.Errorf("archFromGOARCH(%q) = %v, want %v", goarch, got, want)
		}
	}
}

func TestDetectMappings(t *testing.T) {
	t.Parallel()
	// detectArch must return a defined enum on the host.
	if got := detectArch(); got > pkgplatform.ArchWASM {
		t.Errorf("detectArch out of range: %d", got)
	}
	// tierFor mappings.
	cases := []struct {
		os   pkgplatform.PlatformOS
		arch pkgplatform.PlatformArch
		want pkgplatform.PlatformTier
	}{
		{pkgplatform.OSWindows, pkgplatform.ArchAMD64, pkgplatform.Tier1},
		{pkgplatform.OSLinux, pkgplatform.ArchAMD64, pkgplatform.Tier1},
		{pkgplatform.OSLinux, pkgplatform.ArchARM64, pkgplatform.Tier3},
		{pkgplatform.OSWeb, pkgplatform.ArchWASM, pkgplatform.Tier2},
		{pkgplatform.OSUnknown, pkgplatform.ArchUnknown, pkgplatform.TierUnknown},
	}
	for _, c := range cases {
		if got := tierFor(c.os, c.arch); got != c.want {
			t.Errorf("tierFor(%v,%v) = %v, want %v", c.os, c.arch, got, c.want)
		}
	}
}

// stubBuilder embeds appface.Builder (nil) and overrides only SetResource —
// the single method Plugin.Build calls. Any other call would panic, proving
// Build touches nothing else.
type stubBuilder struct {
	appface.Builder
	resources []any
}

func (s *stubBuilder) SetResource(v any) appface.Builder {
	s.resources = append(s.resources, v)
	return s
}

func TestPluginInsertsProfile(t *testing.T) {
	t.Parallel()
	sb := &stubBuilder{}
	New().Build(sb)
	if len(sb.resources) != 1 {
		t.Fatalf("Build set %d resources, want 1", len(sb.resources))
	}
	if _, ok := sb.resources[0].(pkgplatform.PlatformProfile); !ok {
		t.Errorf("inserted resource is %T, want PlatformProfile", sb.resources[0])
	}
}
