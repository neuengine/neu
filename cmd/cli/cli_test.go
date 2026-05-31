package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// run dispatches argv through a fresh router, returning exit code + stdout/stderr.
func run(argv ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := buildRouter().Run(argv, &out, &errb)
	return code, out.String(), errb.String()
}

func TestHelpNoArg(t *testing.T) {
	t.Parallel()
	// INV-2: no-arg invocation prints a structured help menu.
	code, out, _ := run()
	if code != 0 {
		t.Errorf("no-arg exit = %d, want 0", code)
	}
	for _, want := range []string{"Commands:", "doctor", "plugin", "scaffold"} {
		if !strings.Contains(out, want) {
			t.Errorf("help missing %q; got:\n%s", want, out)
		}
	}
}

func TestUnknownCommand(t *testing.T) {
	t.Parallel()
	code, _, errs := run("bogus")
	if code != 2 || !strings.Contains(errs, "unknown command") {
		t.Errorf("unknown cmd: code=%d err=%q", code, errs)
	}
}

func TestDoctorJSON(t *testing.T) {
	t.Parallel()
	// INV-4: --json emits stable structured output.
	code, out, _ := run("doctor", "--json")
	if code != 0 {
		t.Fatalf("doctor exit = %d", code)
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("doctor --json not valid JSON: %v\n%s", err, out)
	}
	if got["engine_version"] != engineVersion {
		t.Errorf("engine_version = %q, want %q", got["engine_version"], engineVersion)
	}
	// Text mode contains no JSON braces.
	_, textOut, _ := run("doctor")
	if strings.Contains(textOut, "{") {
		t.Error("text-mode doctor should not emit JSON")
	}
}

func TestScaffoldOverwriteSafe(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "comp.go")

	// First write succeeds.
	if code, _, _ := run("scaffold", "component", path); code != 0 {
		t.Fatalf("scaffold exit = %d", code)
	}
	orig, _ := os.ReadFile(path)
	if len(orig) == 0 {
		t.Fatal("scaffold wrote nothing")
	}

	// INV-1: second write without --force skips (no clobber).
	code, out, _ := run("scaffold", "component", path)
	if code != 0 || !strings.Contains(out, "skip") {
		t.Errorf("re-scaffold should skip: code=%d out=%q", code, out)
	}
	again, _ := os.ReadFile(path)
	if string(again) != string(orig) {
		t.Error("INV-1: existing file was overwritten without --force")
	}

	// --dry-run writes nothing new.
	newPath := filepath.Join(dir, "sys.go")
	if code, _, _ := run("scaffold", "system", newPath, "--dry-run"); code != 0 {
		t.Fatalf("dry-run exit = %d", code)
	}
	if _, err := os.Stat(newPath); !os.IsNotExist(err) {
		t.Error("--dry-run should not create the file")
	}

	// Unknown target errors.
	if code, _, _ := run("scaffold", "widget", filepath.Join(dir, "x")); code != 1 {
		t.Errorf("unknown target exit = %d, want 1", code)
	}
}

func TestPluginValidate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	good := filepath.Join(dir, "plugin.toml")
	os.WriteFile(good, []byte(`[plugin]
id = "com.example.test"
version = "1.0.0"
mode = "in-process"

[compatibility]
engine_version = "^1.0.0"

[entry.in_process]
package_path = "github.com/example/test"
factory = "New"
`), 0o644)

	if code, out, _ := run("plugin", "validate", good); code != 0 || !strings.Contains(out, "com.example.test") {
		t.Errorf("validate good: code=%d out=%q", code, out)
	}

	// Invalid manifest (missing engine_version) → exit 1.
	bad := filepath.Join(dir, "bad.toml")
	os.WriteFile(bad, []byte("[plugin]\nid = \"a.b\"\nversion = \"1.0.0\"\nmode = \"in-process\"\n"), 0o644)
	if code, _, _ := run("plugin", "validate", bad); code != 1 {
		t.Errorf("validate bad exit = %d, want 1", code)
	}

	// Missing path → usage error.
	if code, _, _ := run("plugin", "validate"); code != 1 {
		t.Errorf("validate no-arg exit = %d, want 1", code)
	}
	// Unknown subcommand.
	if code, _, _ := run("plugin", "frob"); code != 1 {
		t.Errorf("unknown plugin subcommand exit = %d, want 1", code)
	}
}

func TestPluginList(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	pdir := filepath.Join(dir, "com.example.one")
	os.MkdirAll(pdir, 0o755)
	os.WriteFile(filepath.Join(pdir, "plugin.toml"), []byte(`[plugin]
id = "com.example.one"
version = "2.1.0"
mode = "in-process"

[compatibility]
engine_version = "^1.0.0"

[entry.in_process]
package_path = "x"
factory = "New"
`), 0o644)

	code, out, _ := run("plugin", "list", dir)
	if code != 0 || !strings.Contains(out, "com.example.one") || !strings.Contains(out, "2.1.0") {
		t.Errorf("plugin list: code=%d out=%q", code, out)
	}
	// Empty dir → "no plugins".
	if _, out, _ := run("plugin", "list", t.TempDir()); !strings.Contains(out, "no plugins") {
		t.Errorf("empty list out = %q", out)
	}
}

func TestRouterCommands(t *testing.T) {
	t.Parallel()
	if len(buildRouter().Commands()) != 3 {
		t.Errorf("expected 3 registered commands, got %v", buildRouter().Commands())
	}
}
