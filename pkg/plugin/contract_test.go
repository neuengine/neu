package plugin_test

import (
	"errors"
	"testing"

	"github.com/neuengine/neu/pkg/plugin"
)

// T-6T01 plugin-SDK contract tests: the behavioural guarantees a Stable
// plugin-distribution SDK must hold for any third-party consumer — manifest
// parsing is panic-free on hostile input (native fuzzing), validation rejects
// every malformed shape, engine-version compatibility is enforced, and the
// capability model gates by tier + wildcard with auditable denial errors.

const validInProcTOML = `
[plugin]
id = "com.example.test"
version = "1.2.0"
name = "Test Plugin"
mode = "in-process"

[compatibility]
engine_version = "^1.0.0"

[capabilities.required]
items = ["network.outbound"]

[entry.in_process]
package_path = "com.example/test"
factory = "New"
`

const validOOPTOML = `
[plugin]
id = "com.example.oop"
version = "2.1.0"
mode = "out-of-process"

[compatibility]
engine_version = ">=1.0.0"

[entry.out_of_process]
binary = "./plugin"
transport = "stdio"
`

// --- manifest parsing: native fuzz (input validation, CLAUDE.md mandate) ------

// FuzzParseManifest asserts the untrusted-manifest parser never panics and that
// any successfully parsed manifest survives Validate + CompatibleWith without
// panicking (they may reject it — that is a clean error, not a crash).
func FuzzParseManifest(f *testing.F) {
	for _, seed := range []string{
		validInProcTOML, validOOPTOML, "", "# comment only\n",
		"[plugin]\nid = ", "no-equals-no-section",
		"[plugin]\nid = \"x.y\"\nmode = \"weird\"\n",
		"[capabilities.required]\nitems = [\"a\", \"b\", ]\n",
		"[entry.out_of_process]\nbinary = \"\"\ntransport = \"\"\n",
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		m, err := plugin.ParseManifest(data)
		if err != nil {
			return // a parse error is an acceptable outcome
		}
		// A parsed manifest must not crash the downstream contract surface.
		_ = m.Validate()
		if m.EngineVersion != "" {
			if _, cerr := m.CompatibleWith(plugin.Version{}); cerr != nil {
				_ = cerr // a constraint error is fine; a panic would fail the fuzz
			}
		}
	})
}

// --- manifest round-trip ------------------------------------------------------

func TestManifestParseRoundTrip(t *testing.T) {
	t.Parallel()
	m, err := plugin.ParseManifest([]byte(validInProcTOML))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("valid manifest failed Validate: %v", err)
	}
	if m.ID != "com.example.test" || m.Version != "1.2.0" {
		t.Errorf("id/version = %q/%q, want com.example.test/1.2.0", m.ID, m.Version)
	}
	if m.Mode != plugin.ModeInProcess {
		t.Errorf("mode = %v, want in-process", m.Mode)
	}
	if m.Entry.PackagePath != "com.example/test" || m.Entry.Factory != "New" {
		t.Errorf("entry = %+v, want package_path+factory set", m.Entry)
	}
	if len(m.RequiredCaps) != 1 || m.RequiredCaps[0] != plugin.CapNetworkOutbound {
		t.Errorf("required caps = %v, want [network.outbound]", m.RequiredCaps)
	}

	oop, err := plugin.ParseManifest([]byte(validOOPTOML))
	if err != nil || oop.Validate() != nil {
		t.Fatalf("OOP manifest parse/validate failed: %v / %v", err, oop.Validate())
	}
	if oop.Mode != plugin.ModeOutOfProcess || oop.Entry.Binary == "" || oop.Entry.Transport == "" {
		t.Errorf("OOP entry = %+v (mode %v), want binary+transport set", oop.Entry, oop.Mode)
	}
}

// --- validation contract ------------------------------------------------------

func validInProc() plugin.Manifest {
	return plugin.Manifest{
		ID:            "com.example.ok",
		Version:       "1.0.0",
		EngineVersion: "^1.0.0",
		Mode:          plugin.ModeInProcess,
		Entry:         plugin.EntrySpec{PackagePath: "p", Factory: "f"},
	}
}

func TestManifestValidationRejectsMalformed(t *testing.T) {
	t.Parallel()
	if err := validInProc().Validate(); err != nil {
		t.Fatalf("baseline valid manifest rejected: %v", err)
	}

	cases := map[string]func(*plugin.Manifest){
		"missing id":          func(m *plugin.Manifest) { m.ID = "" },
		"non-reverse-dns id":  func(m *plugin.Manifest) { m.ID = "noDotHere" },
		"unparseable version": func(m *plugin.Manifest) { m.Version = "not-a-version" },
		"missing engine ver":  func(m *plugin.Manifest) { m.EngineVersion = "" },
		"bad engine ver":      func(m *plugin.Manifest) { m.EngineVersion = "@@@" },
		"inproc no entry":     func(m *plugin.Manifest) { m.Entry = plugin.EntrySpec{} },
		"oop no entry": func(m *plugin.Manifest) {
			m.Mode = plugin.ModeOutOfProcess
			m.Entry = plugin.EntrySpec{}
		},
	}
	for name, mutate := range cases {
		m := validInProc()
		mutate(&m)
		err := m.Validate()
		if err == nil {
			t.Errorf("%s: Validate accepted a malformed manifest", name)
			continue
		}
		if !errors.Is(err, plugin.ErrManifestInvalid) {
			t.Errorf("%s: err = %v, want wrapped ErrManifestInvalid", name, err)
		}
	}
}

// --- engine-compatibility contract -------------------------------------------

func TestManifestCompatibility(t *testing.T) {
	t.Parallel()
	mustVer := func(s string) plugin.Version {
		v, err := plugin.ParseVersion(s)
		if err != nil {
			t.Fatalf("ParseVersion(%q): %v", s, err)
		}
		return v
	}
	m := validInProc() // engine_version "^1.0.0"

	for _, tc := range []struct {
		engine string
		want   bool
	}{
		{"1.0.0", true},
		{"1.9.3", true},
		{"2.0.0", false}, // caret pins the major
		{"0.9.0", false},
	} {
		got, err := m.CompatibleWith(mustVer(tc.engine))
		if err != nil {
			t.Fatalf("CompatibleWith(%s): %v", tc.engine, err)
		}
		if got != tc.want {
			t.Errorf("^1.0.0 admits %s = %v, want %v", tc.engine, got, tc.want)
		}
	}

	// A malformed constraint surfaces an error, not a false-negative.
	bad := validInProc()
	bad.EngineVersion = "@@@"
	if _, err := bad.CompatibleWith(mustVer("1.0.0")); err == nil {
		t.Error("CompatibleWith with a bad constraint should error")
	}
}

// --- capability enforcement contract -----------------------------------------

func TestCapabilityTiers(t *testing.T) {
	t.Parallel()
	cases := map[plugin.Capability]plugin.Tier{
		plugin.CapWorldRead:        plugin.TierGranted,
		plugin.CapMetricsPublish:   plugin.TierGranted,
		plugin.CapNetworkOutbound:  plugin.TierPrompt,
		plugin.CapFSWriteProject:   plugin.TierPrompt,
		plugin.CapProcessSpawn:     plugin.TierAlwaysPrompt,
		plugin.CapFSReadUser:       plugin.TierAlwaysPrompt,
		"events.publish.assistant": plugin.TierPrompt,       // topic-scoped → prompt
		"com.acme.custom":          plugin.TierAlwaysPrompt, // unknown → always-prompt (no silent grant)
	}
	for cap, want := range cases {
		if got := plugin.DefaultTier(cap); got != want {
			t.Errorf("DefaultTier(%q) = %v, want %v", cap, got, want)
		}
	}
}

func TestCapabilitySetGatingAndWildcard(t *testing.T) {
	t.Parallel()
	s := plugin.NewCapabilitySet(plugin.CapWorldRead, "events.publish.*")

	if !s.Has(plugin.CapWorldRead) {
		t.Error("exact granted capability should be Has-true")
	}
	if s.Has(plugin.CapNetworkOutbound) {
		t.Error("ungranted capability must be denied")
	}
	// Wildcard grant covers any matching specific topic.
	if !s.Has("events.publish.assistant.reply") {
		t.Error("wildcard grant should cover a matching specific topic")
	}
	if s.Has("events.subscribe.assistant") {
		t.Error("wildcard for publish must not cover subscribe")
	}

	s.Add(plugin.CapCodegen)
	if !s.Has(plugin.CapCodegen) {
		t.Error("Add should grant the capability")
	}
}

func TestCapabilityErrorWrapsSentinel(t *testing.T) {
	t.Parallel()
	err := plugin.CapabilityError{Plugin: "com.x.y", Capability: plugin.CapProcessSpawn}
	if !errors.Is(err, plugin.ErrCapabilityDenied) {
		t.Error("CapabilityError must unwrap to ErrCapabilityDenied (errors.Is)")
	}
	if got := err.Error(); got == "" {
		t.Error("CapabilityError.Error() should describe the plugin + capability")
	}
}
