package platform

import "testing"

func TestCapsHasWithWithout(t *testing.T) {
	t.Parallel()
	c := HasGPU | HasKeyboard | HasMouse
	if !c.Has(HasGPU) || !c.Has(HasKeyboard|HasMouse) {
		t.Error("Has should report set bits (single and combined)")
	}
	if c.Has(HasTouch) {
		t.Error("Has should be false for an unset bit")
	}
	// Disjoint flags must not alias.
	if HasGPU == HasTouch || HasMultiWindow == HasSpatialAudio {
		t.Error("capability bits must be distinct")
	}
	added := c.With(HasTouch)
	if !added.Has(HasTouch) || !added.Has(HasGPU) {
		t.Error("With should add a bit without dropping existing ones")
	}
	removed := added.Without(HasGPU)
	if removed.Has(HasGPU) || !removed.Has(HasTouch) {
		t.Error("Without should clear only the named bit")
	}
}

func TestCapsString(t *testing.T) {
	t.Parallel()
	if got := PlatformCaps(0).String(); got != "none" {
		t.Errorf("empty caps String = %q, want none", got)
	}
	got := (HasGPU | HasMouse).String()
	if got != "GPU|Mouse" {
		t.Errorf("caps String = %q, want GPU|Mouse", got)
	}
}

func TestEnumStringsTotal(t *testing.T) {
	t.Parallel()
	osCases := map[PlatformOS]string{
		OSWindows: "Windows", OSLinux: "Linux", OSMacOS: "macOS",
		OSAndroid: "Android", OSIOS: "iOS", OSWeb: "Web", OSUnknown: "Unknown",
	}
	for o, want := range osCases {
		if got := o.String(); got != want {
			t.Errorf("PlatformOS(%d) = %q, want %q", o, got, want)
		}
	}
	archCases := map[PlatformArch]string{
		ArchAMD64: "amd64", ArchARM64: "arm64", ArchWASM: "wasm", ArchUnknown: "unknown",
	}
	for a, want := range archCases {
		if got := a.String(); got != want {
			t.Errorf("PlatformArch(%d) = %q, want %q", a, got, want)
		}
	}
	tierCases := map[PlatformTier]string{
		Tier1: "Tier1", Tier2: "Tier2", Tier3: "Tier3", TierUnknown: "TierUnknown",
	}
	for tr, want := range tierCases {
		if got := tr.String(); got != want {
			t.Errorf("PlatformTier(%d) = %q, want %q", tr, got, want)
		}
	}
}
