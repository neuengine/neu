package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// T-6T03 CLI golden-output integration tests. The existing cli_test.go covers
// behaviour (exit codes + Contains); these lock the EXACT user-facing output of
// the implemented commands via the stable --json contract (INV-4) and the exact
// scaffold templates, so a reword / format / JSON-shape change is a visible
// regression. The install/enable/disable/info/remove subcommands the spec
// enumerates are not implemented (the plugin install flow is deferred); their
// absence is asserted (INV-3: the CLI advertises no phantom command).

const goldenManifest = `[plugin]
id = "com.example.golden"
version = "1.2.3"
mode = "in-process"

[compatibility]
engine_version = "^1.0.0"

[entry.in_process]
package_path = "x"
factory = "New"
`

// TestHelpJSONGolden locks the command catalog + descriptions (sorted, INV-2/4).
func TestHelpJSONGolden(t *testing.T) {
	t.Parallel()
	code, out, _ := run("help", "--json")
	if code != 0 {
		t.Fatalf("help --json exit = %d", code)
	}
	var got struct {
		Commands []struct{ Name, Help string }
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("help --json is not valid JSON: %v\n%s", err, out)
	}
	want := []struct{ Name, Help string }{
		{"doctor", "report environment, version, and workspace health"},
		{"plugin", "manage plugins: validate, list"},
		{"scaffold", "generate a stub (component|system|plugin) — overwrite-safe"},
	}
	if !reflect.DeepEqual(got.Commands, want) {
		t.Errorf("command catalog drifted:\n got: %+v\nwant: %+v", got.Commands, want)
	}
}

// TestScaffoldTemplatesGolden locks the exact generated stub content — the stubs
// are the CLI's user-facing code contract.
func TestScaffoldTemplatesGolden(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"component": "package game\n\n// TODO: component fields (pure data).\ntype Component struct{}\n",
		"system":    "package game\n\n// TODO: system logic over a Query.\nfunc System() {}\n",
		"plugin":    "[plugin]\nid = \"com.example.plugin\"\nversion = \"0.1.0\"\nmode = \"in-process\"\n\n[compatibility]\nengine_version = \"^0.1.0\"\n",
	}
	for target, want := range cases {
		path := filepath.Join(t.TempDir(), "out")
		if code, _, _ := run("scaffold", target, path); code != 0 {
			t.Fatalf("scaffold %s exit != 0", target)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read scaffold %s: %v", target, err)
		}
		if string(got) != want {
			t.Errorf("scaffold %s template drifted:\n got: %q\nwant: %q", target, got, want)
		}
	}
}

// TestDoctorJSONShapeGolden locks the doctor JSON key set (values are
// environment-dynamic, so only the engine_version is pinned).
func TestDoctorJSONShapeGolden(t *testing.T) {
	t.Parallel()
	_, out, _ := run("doctor", "--json")
	var m map[string]string
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("doctor --json invalid: %v\n%s", err, out)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if want := []string{"arch", "engine_version", "go_version", "os"}; !reflect.DeepEqual(keys, want) {
		t.Errorf("doctor JSON keys = %v, want %v", keys, want)
	}
	if m["engine_version"] != engineVersion {
		t.Errorf("engine_version = %q, want %q", m["engine_version"], engineVersion)
	}
}

// TestPluginValidateJSONGolden locks the validate JSON for a known manifest.
func TestPluginValidateJSONGolden(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "plugin.toml")
	if err := os.WriteFile(path, []byte(goldenManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ := run("plugin", "validate", path, "--json")
	if code != 0 {
		t.Fatalf("validate exit = %d", code)
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("validate --json invalid: %v\n%s", err, out)
	}
	want := map[string]string{"id": "com.example.golden", "version": "1.2.3", "mode": "in-process"}
	if !reflect.DeepEqual(m, want) {
		t.Errorf("validate JSON = %v, want %v", m, want)
	}
}

// TestPluginListJSONGolden locks the list JSON for a known plugin directory.
func TestPluginListJSONGolden(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	pdir := filepath.Join(root, "com.example.golden")
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "plugin.toml"), []byte(goldenManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ := run("plugin", "list", root, "--json")
	if code != 0 {
		t.Fatalf("list exit = %d", code)
	}
	var got struct {
		Plugins []struct{ ID, Version, Mode string }
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("list --json invalid: %v\n%s", err, out)
	}
	want := []struct{ ID, Version, Mode string }{{"com.example.golden", "1.2.3", "in-process"}}
	if !reflect.DeepEqual(got.Plugins, want) {
		t.Errorf("plugin list JSON = %+v, want %+v", got.Plugins, want)
	}
}

// TestUnimplementedSubcommandsAbsent documents (and locks) that the install-flow
// subcommands the spec enumerates are not registered and `plugin <sub>` for them
// errors rather than being silently handled (INV-3: no phantom commands).
func TestUnimplementedSubcommandsAbsent(t *testing.T) {
	t.Parallel()
	registered := make(map[string]bool)
	for _, c := range buildRouter().Commands() {
		registered[c] = true
	}
	for _, phantom := range []string{"install", "enable", "disable", "info", "remove"} {
		if registered[phantom] {
			t.Errorf("%q is a registered top-level command but the install flow is unimplemented", phantom)
		}
		if code, _, _ := run("plugin", phantom); code != 1 {
			t.Errorf("`plugin %s` exit = %d, want 1 (unimplemented subcommand)", phantom, code)
		}
	}
}
