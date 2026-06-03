package plugin

import (
	"errors"
	"slices"
	"testing"
	"testing/fstest"

	pkgplugin "github.com/neuengine/neu/pkg/plugin"
)

func mustVersion(t *testing.T, s string) pkgplugin.Version {
	t.Helper()
	v, err := pkgplugin.ParseVersion(s)
	if err != nil {
		t.Fatalf("ParseVersion(%q): %v", s, err)
	}
	return v
}

// manifestTOML builds a valid in-process plugin.toml with the given id, engine
// constraint, and required caps.
func manifestTOML(id, engine string, reqCaps string) string {
	return "[plugin]\n" +
		"id = \"" + id + "\"\n" +
		"version = \"0.1.0\"\n" +
		"mode = \"in-process\"\n" +
		"[compatibility]\n" +
		"engine_version = \"" + engine + "\"\n" +
		"[entry.in_process]\n" +
		"package_path = \"github.com/example/p\"\n" +
		"factory = \"New\"\n" +
		"[capabilities.required]\n" +
		"items = [" + reqCaps + "]\n"
}

// ── DirSource discovery ───────────────────────────────────────────────────────

func TestDirSourceDiscoversManifests(t *testing.T) {
	fsys := fstest.MapFS{
		"a/plugin.toml": {Data: []byte(manifestTOML("com.example.a", "^1.0.0", `"world.read"`))},
		"b/plugin.toml": {Data: []byte(manifestTOML("com.example.b", "^1.0.0", ""))},
		"b/readme.txt":  {Data: []byte("ignored")},
	}
	src := DirSource{SourceName: "test", FS: fsys}
	if src.Name() != "test" {
		t.Errorf("Name() = %q, want test", src.Name())
	}
	found, errs := src.Discover()
	if len(errs) != 0 {
		t.Fatalf("unexpected discover errors: %v", errs)
	}
	if len(found) != 2 {
		t.Fatalf("found %d plugins, want 2", len(found))
	}
	for _, dp := range found {
		if dp.Source != "test" {
			t.Errorf("source = %q, want test", dp.Source)
		}
		if dp.Dir != "a" && dp.Dir != "b" {
			t.Errorf("unexpected dir %q", dp.Dir)
		}
	}
}

func TestDirSourceCollectsBadManifest(t *testing.T) {
	fsys := fstest.MapFS{
		"good/plugin.toml": {Data: []byte(manifestTOML("com.example.good", "^1.0.0", ""))},
		"bad/plugin.toml":  {Data: []byte("this line has no equals and is not a section")},
	}
	found, errs := DirSource{SourceName: "s", FS: fsys}.Discover()
	if len(found) != 1 {
		t.Errorf("found %d, want 1 (good only)", len(found))
	}
	if len(errs) != 1 {
		t.Fatalf("errs = %d, want 1 (the bad manifest)", len(errs))
	}
}

// ── grant resolution ──────────────────────────────────────────────────────────

func reqManifest(id string, caps ...pkgplugin.Capability) pkgplugin.Manifest {
	return pkgplugin.Manifest{ID: pkgplugin.PluginID(id), RequiredCaps: caps}
}

func TestResolveGrantsAutoAndPrompt(t *testing.T) {
	store := NewMemoryGrantStore()
	prompts := 0
	prompt := func(pkgplugin.PluginID, pkgplugin.Capability) bool { prompts++; return true }

	// world.read = TierGranted (auto), world.commands = TierPrompt (prompt once).
	man := reqManifest("com.x.p", pkgplugin.CapWorldRead, pkgplugin.CapWorldCommands)
	granted, ok := ResolveGrants(man, store, prompt)
	if !ok {
		t.Fatal("all required caps should be granted")
	}
	if !granted.Has(pkgplugin.CapWorldRead) || !granted.Has(pkgplugin.CapWorldCommands) {
		t.Error("both caps should be granted")
	}
	if prompts != 1 {
		t.Errorf("prompted %d times, want 1 (only the TierPrompt cap)", prompts)
	}

	// Second resolution reuses the persisted TierPrompt decision — no prompt.
	prompts = 0
	if _, ok := ResolveGrants(man, store, prompt); !ok || prompts != 0 {
		t.Errorf("persisted grant should skip the prompt; prompts=%d ok=%v", prompts, ok)
	}
}

func TestResolveGrantsDeniedPrompt(t *testing.T) {
	man := reqManifest("com.x.p", pkgplugin.CapWorldCommands) // TierPrompt
	deny := func(pkgplugin.PluginID, pkgplugin.Capability) bool { return false }
	granted, ok := ResolveGrants(man, NewMemoryGrantStore(), deny)
	if ok {
		t.Error("a denied required capability must make allOK false")
	}
	if granted.Has(pkgplugin.CapWorldCommands) {
		t.Error("a denied capability must not be granted")
	}
}

func TestResolveGrantsAlwaysPromptNotPersisted(t *testing.T) {
	store := NewMemoryGrantStore()
	man := reqManifest("com.x.p", pkgplugin.CapProcessSpawn) // TierAlwaysPrompt
	allow := func(pkgplugin.PluginID, pkgplugin.Capability) bool { return true }

	if _, ok := ResolveGrants(man, store, allow); !ok {
		t.Fatal("allowed always-prompt cap should be granted")
	}
	if _, persisted := store.Granted("com.x.p"); persisted {
		t.Error("TierAlwaysPrompt caps must never be persisted")
	}
}

func TestResolveGrantsNilPromptDenies(t *testing.T) {
	man := reqManifest("com.x.p", pkgplugin.CapWorldCommands)
	if _, ok := ResolveGrants(man, nil, nil); ok {
		t.Error("a nil prompt must deny prompted capabilities")
	}
}

func TestGrantStoreJSONRoundTrip(t *testing.T) {
	s := NewMemoryGrantStore()
	_ = s.Save("com.x.p", pkgplugin.NewCapabilitySet(pkgplugin.CapWorldCommands, pkgplugin.CapFSReadProject))
	data, err := MarshalGrants(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s2, err := UnmarshalGrants(data)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	g, ok := s2.Granted("com.x.p")
	if !ok || !g.Has(pkgplugin.CapWorldCommands) || !g.Has(pkgplugin.CapFSReadProject) {
		t.Errorf("round-trip lost grants: %v (ok=%v)", g, ok)
	}
}

// ── DiscoverAndRegister orchestration ─────────────────────────────────────────

func TestDiscoverAndRegister(t *testing.T) {
	fsys := fstest.MapFS{
		"ok/plugin.toml":       {Data: []byte(manifestTOML("com.example.ok", "^1.0.0", `"world.read"`))},
		"deny/plugin.toml":     {Data: []byte(manifestTOML("com.example.deny", "^1.0.0", `"world.commands"`))},
		"incompat/plugin.toml": {Data: []byte(manifestTOML("com.example.incompat", "^2.0.0", ""))},
	}
	m := NewManager(mustVersion(t, "1.0.0"))
	// Deny every prompt → the TierPrompt plugin is skipped; world.read is auto.
	deny := func(pkgplugin.PluginID, pkgplugin.Capability) bool { return false }

	rep := DiscoverAndRegister(m, []Source{DirSource{SourceName: "s", FS: fsys}}, NewMemoryGrantStore(), deny)

	if !slices.Contains(rep.Registered, "com.example.ok") {
		t.Errorf("com.example.ok should be registered; got %v", rep.Registered)
	}
	if _, skipped := rep.Skipped["com.example.deny"]; !skipped {
		t.Error("com.example.deny should be skipped (capability denied)")
	}
	if err := rep.Skipped["com.example.incompat"]; !errors.Is(err, pkgplugin.ErrEngineIncompatible) {
		t.Errorf("com.example.incompat skip reason = %v, want ErrEngineIncompatible", err)
	}
	if st, _ := m.State("com.example.ok"); st != pkgplugin.StateApproved {
		t.Errorf("registered plugin state = %v, want Approved", st)
	}
}

func TestDiscoverAndRegisterDuplicateSkipped(t *testing.T) {
	toml := manifestTOML("com.example.dup", "^1.0.0", `"world.read"`)
	fsys := fstest.MapFS{
		"a/plugin.toml": {Data: []byte(toml)},
		"b/plugin.toml": {Data: []byte(toml)}, // same id in two dirs
	}
	m := NewManager(mustVersion(t, "1.0.0"))
	rep := DiscoverAndRegister(m, []Source{DirSource{SourceName: "s", FS: fsys}}, NewMemoryGrantStore(), nil)

	if len(rep.Registered) != 1 {
		t.Errorf("registered %d, want 1 (duplicate id rejected)", len(rep.Registered))
	}
	if err := rep.Skipped["com.example.dup"]; !errors.Is(err, pkgplugin.ErrDuplicateID) {
		t.Errorf("duplicate skip reason = %v, want ErrDuplicateID", err)
	}
}
