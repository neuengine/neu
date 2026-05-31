package plugin

import (
	"errors"
	"testing"
)

func TestParseVersionAndCompare(t *testing.T) {
	t.Parallel()
	v, err := ParseVersion("v1.4.2-rc1+build")
	if err != nil || v.Major != 1 || v.Minor != 4 || v.Patch != 2 {
		t.Fatalf("ParseVersion = %+v, %v", v, err)
	}
	if _, err := ParseVersion("1.x.0"); err == nil {
		t.Error("malformed version should error")
	}
	a, _ := ParseVersion("1.4.0")
	b, _ := ParseVersion("1.5.0")
	if a.Compare(b) != -1 || b.Compare(a) != 1 || a.Compare(a) != 0 {
		t.Error("Compare ordering wrong")
	}
}

func TestConstraintMatches(t *testing.T) {
	t.Parallel()
	cases := []struct {
		constraint string
		version    string
		want       bool
	}{
		{"^1.4.0", "1.4.0", true},
		{"^1.4.0", "1.9.9", true},
		{"^1.4.0", "2.0.0", false},
		{"^1.4.0", "1.3.0", false},
		{"^0.4.0", "0.4.9", true},
		{"^0.4.0", "0.5.0", false}, // caret on 0.x locks the minor
		{"~1.4.0", "1.4.9", true},
		{"~1.4.0", "1.5.0", false},
		{">=1.4.0, <1.7.0", "1.6.0", true},
		{">=1.4.0, <1.7.0", "1.7.0", false},
		{"1.4.0", "1.4.0", true}, // bare ⇒ exact
		{"1.4.0", "1.4.1", false},
		{"*", "9.9.9", true}, // any
	}
	for _, c := range cases {
		con, err := ParseConstraint(c.constraint)
		if err != nil {
			t.Fatalf("ParseConstraint(%q): %v", c.constraint, err)
		}
		v, _ := ParseVersion(c.version)
		if got := con.Matches(v); got != c.want {
			t.Errorf("%q matches %q = %v, want %v", c.constraint, c.version, got, c.want)
		}
	}
	if _, err := ParseConstraint(">=bad"); !errors.Is(err, ErrInvalidVersion) {
		t.Errorf("bad constraint err = %v", err)
	}
}

func TestCapabilitySetAndTiers(t *testing.T) {
	t.Parallel()
	s := NewCapabilitySet(CapWorldRead, "events.publish.assistant.*")
	if !s.Has(CapWorldRead) {
		t.Error("granted cap should be present")
	}
	if s.Has(CapNetworkOutbound) {
		t.Error("ungranted cap should be absent")
	}
	// Wildcard grant covers a specific topic.
	if !s.Has("events.publish.assistant.chunk") {
		t.Error("wildcard grant should cover specific topic")
	}
	// Tier defaults.
	if DefaultTier(CapWorldRead) != TierGranted {
		t.Error("world.read should be granted")
	}
	if DefaultTier(CapNetworkOutbound) != TierPrompt {
		t.Error("network.outbound should be prompt")
	}
	if DefaultTier(CapProcessSpawn) != TierAlwaysPrompt {
		t.Error("process.spawn should be always-prompt")
	}
	if DefaultTier("com.example.custom") != TierAlwaysPrompt {
		t.Error("unknown custom cap should default to always-prompt")
	}
	if DefaultTier("events.subscribe.foo") != TierPrompt {
		t.Error("topic cap should be prompt")
	}
}

const sampleManifest = `
# AI API plugin manifest
[plugin]
id          = "io.teratron.neuengine.aiapi"
version     = "0.1.0"
name        = "AI API Plugin"
mode        = "in-process"
authors     = ["Neu Engine Core Team"]
license     = "MIT"

[compatibility]
engine_version = "^0.1.0"
platforms      = ["linux-amd64", "darwin-arm64"]

[capabilities.required]
items = ["network.outbound", "metrics.publish"]

[entry.in_process]
package_path = "github.com/neuengine/neu/pkg/plugins/aiapi"
factory      = "New"
`

func TestManifestParseValidate(t *testing.T) {
	t.Parallel()
	m, err := ParseManifest([]byte(sampleManifest))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if m.ID != "io.teratron.neuengine.aiapi" || m.Mode != ModeInProcess {
		t.Errorf("parsed manifest = %+v", m)
	}
	if len(m.Authors) != 1 || len(m.Platforms) != 2 || len(m.RequiredCaps) != 2 {
		t.Errorf("arrays: authors=%v platforms=%v caps=%v", m.Authors, m.Platforms, m.RequiredCaps)
	}
	if m.Entry.Factory != "New" || m.Entry.PackagePath == "" {
		t.Errorf("entry = %+v", m.Entry)
	}
	if err := m.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
	// Compatible with 0.1.5, not with 0.2.0 (caret on 0.x).
	v015, _ := ParseVersion("0.1.5")
	v020, _ := ParseVersion("0.2.0")
	if ok, _ := m.CompatibleWith(v015); !ok {
		t.Error("should be compatible with 0.1.5")
	}
	if ok, _ := m.CompatibleWith(v020); ok {
		t.Error("should NOT be compatible with 0.2.0")
	}
}

func TestManifestValidateErrors(t *testing.T) {
	t.Parallel()
	// Missing id.
	if err := (Manifest{Version: "1.0.0", EngineVersion: "^1.0.0", Mode: ModeInProcess}).Validate(); !errors.Is(err, ErrManifestInvalid) {
		t.Errorf("missing id err = %v", err)
	}
	// In-process without entry.
	bad := Manifest{ID: "a.b", Version: "1.0.0", EngineVersion: "^1.0.0", Mode: ModeInProcess}
	if err := bad.Validate(); !errors.Is(err, ErrManifestInvalid) {
		t.Errorf("missing entry err = %v", err)
	}
}

func TestModeAndStateStrings(t *testing.T) {
	t.Parallel()
	if ModeInProcess.String() != "in-process" || ModeOutOfProcess.String() != "out-of-process" {
		t.Error("Mode.String")
	}
	if Mode(99).String() != "unknown" {
		t.Error("unknown Mode.String")
	}
	if m, ok := ParseMode("out-of-process"); !ok || m != ModeOutOfProcess {
		t.Error("ParseMode")
	}
	if _, ok := ParseMode("bogus"); ok {
		t.Error("bogus mode should not parse")
	}
	for s, want := range map[State]string{
		StateDiscovered: "Discovered", StateApproved: "Approved", StateLoading: "Loading",
		StateActive: "Active", StateFailed: "Failed", StateDisabled: "Disabled",
	} {
		if s.String() != want {
			t.Errorf("State(%d) = %q, want %q", s, s.String(), want)
		}
	}
	if State(99).String() != "Unknown" {
		t.Error("unknown State.String")
	}
}

func TestConstraintComparisonOps(t *testing.T) {
	t.Parallel()
	cases := []struct {
		constraint, version string
		want                bool
	}{
		{">1.0.0", "1.0.1", true},
		{">1.0.0", "1.0.0", false},
		{"<=1.5.0", "1.5.0", true},
		{"<=1.5.0", "1.6.0", false},
		{"=2.0.0", "2.0.0", true},
	}
	for _, c := range cases {
		con, err := ParseConstraint(c.constraint)
		if err != nil {
			t.Fatalf("ParseConstraint(%q): %v", c.constraint, err)
		}
		v, _ := ParseVersion(c.version)
		if got := con.Matches(v); got != c.want {
			t.Errorf("%q matches %q = %v, want %v", c.constraint, c.version, got, c.want)
		}
	}
}

func TestVersionString(t *testing.T) {
	t.Parallel()
	v, _ := ParseVersion("3.7.1")
	if v.String() != "3.7.1" {
		t.Errorf("Version.String = %q", v.String())
	}
}

func TestOutOfProcessManifestAndCapAdd(t *testing.T) {
	t.Parallel()
	oop := Manifest{
		ID: "com.example.oop", Version: "1.0.0", EngineVersion: "^1.0.0",
		Mode:  ModeOutOfProcess,
		Entry: EntrySpec{Binary: "bin/linux-amd64", Transport: "stdio"},
	}
	if err := oop.Validate(); err != nil {
		t.Errorf("OOP Validate: %v", err)
	}
	oop.Entry.Transport = ""
	if err := oop.Validate(); !errors.Is(err, ErrManifestInvalid) {
		t.Errorf("OOP no-transport err = %v", err)
	}
	bad := Manifest{EngineVersion: ">=x"}
	if _, err := bad.CompatibleWith(Version{Major: 1}); err == nil {
		t.Error("bad engine_version should error in CompatibleWith")
	}
	s := NewCapabilitySet()
	s.Add(CapCodegen)
	if !s.Has(CapCodegen) {
		t.Error("Add then Has")
	}
}
