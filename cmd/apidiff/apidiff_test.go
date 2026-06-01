package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleSrc = `package foo

type Widget struct {
	Name string
	size int
}

type widget struct{}

type Renderer = interface{ Render() string }

type Box[T any] struct{ v T }

const MaxItems = 100

var Default Widget

func New(name string) *Widget { return &Widget{Name: name} }

func (w *Widget) Render() string { return w.Name }

func (w *Widget) internal() {} // unexported method — excluded

func (b *Box[T]) Get() T { var z T; return z } // generic receiver

func unexported() {} // excluded
`

func symbolSet(t *testing.T, src string) map[string]Symbol {
	t.Helper()
	syms, err := extractFromSource("foo.go", src)
	if err != nil {
		t.Fatalf("extractFromSource: %v", err)
	}
	m := map[string]Symbol{}
	for _, s := range syms {
		m[s.Kind+" "+s.Name] = s
	}
	return m
}

func TestExtractFromSource(t *testing.T) {
	t.Parallel()
	got := symbolSet(t, sampleSrc)

	want := []string{"func New", "method Widget.Render", "method Box.Get", "type Widget", "type Renderer", "type Box", "const MaxItems", "var Default"}
	for _, w := range want {
		if _, ok := got[w]; !ok {
			t.Errorf("missing symbol %q (got %v)", w, keysOf(got))
		}
	}
	// Exclusions: unexported func, unexported method, method on unexported type,
	// the unexported type itself.
	for _, bad := range []string{"func unexported", "method Widget.internal", "type widget"} {
		if _, ok := got[bad]; ok {
			t.Errorf("symbol %q should be excluded", bad)
		}
	}
	// Signature captures the func type (rendered, normalized).
	if sig := got["func New"].Signature; !strings.Contains(sig, "func New(name string) *Widget") {
		t.Errorf("New signature = %q", sig)
	}
	// Alias renders with "=".
	if sig := got["type Renderer"].Signature; !strings.Contains(sig, "=") {
		t.Errorf("alias signature should contain '=': %q", sig)
	}
}

func keysOf(m map[string]Symbol) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestExtractParseError(t *testing.T) {
	t.Parallel()
	if _, err := extractFromSource("bad.go", "package foo\nfunc ("); err == nil {
		t.Error("malformed source should error")
	}
}

func TestDiff(t *testing.T) {
	t.Parallel()
	old := Snapshot{Packages: map[string][]Symbol{
		"pkg/x": {
			{Name: "F", Kind: "func", Signature: "func F()"},
			{Name: "G", Kind: "func", Signature: "func G(int)"},
			{Name: "T", Kind: "type", Signature: "type T struct{a int}"},
		},
		"pkg/gone": {{Name: "Old", Kind: "func", Signature: "func Old()"}},
	}}
	current := Snapshot{Packages: map[string][]Symbol{
		"pkg/x": {
			{Name: "F", Kind: "func", Signature: "func F() error"},       // changed
			{Name: "T", Kind: "type", Signature: "type T struct{a int}"}, // unchanged
			{Name: "H", Kind: "func", Signature: "func H()"},             // added
		},
		// pkg/gone removed entirely
	}}
	ch := Diff(old, current)

	if !ch.Breaking() {
		t.Fatal("expected breaking changes")
	}
	if names := changeNames(ch.Removed); names != "pkg/gone.Old,pkg/x.G" {
		t.Errorf("Removed = %s", names)
	}
	if names := changeNames(ch.Changed); names != "pkg/x.F" {
		t.Errorf("Changed = %s", names)
	}
	if names := changeNames(ch.Added); names != "pkg/x.H" {
		t.Errorf("Added = %s", names)
	}

	// Identical snapshots → no changes, not breaking.
	if Diff(current, current).Breaking() {
		t.Error("identical snapshots must not be breaking")
	}
	// Only additions → not breaking.
	addOnly := Diff(Snapshot{Packages: map[string][]Symbol{"p": {{Name: "A", Kind: "func", Signature: "func A()"}}}},
		Snapshot{Packages: map[string][]Symbol{"p": {{Name: "A", Kind: "func", Signature: "func A()"}, {Name: "B", Kind: "func", Signature: "func B()"}}}})
	if addOnly.Breaking() || len(addOnly.Added) != 1 {
		t.Errorf("add-only diff: breaking=%v added=%d", addOnly.Breaking(), len(addOnly.Added))
	}
}

func changeNames(cs []Change) string {
	parts := make([]string, len(cs))
	for i, c := range cs {
		parts[i] = c.Pkg + "." + c.Name
	}
	return strings.Join(parts, ",")
}

func TestSnapshotRoundTrip(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "api", "snapshot.json") // nested dir → Save must MkdirAll
	s := Snapshot{GoVersion: "go1.26.3", Packages: map[string][]Symbol{
		"pkg/x": {{Name: "F", Kind: "func", Signature: "func F()"}},
	}}
	if err := s.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := LoadSnapshot(path)
	if err != nil || got.GoVersion != "go1.26.3" || len(got.Packages["pkg/x"]) != 1 {
		t.Fatalf("reloaded = %+v, err %v", got, err)
	}
	// Missing → empty, not error.
	if empty, err := LoadSnapshot(filepath.Join(t.TempDir(), "absent.json")); err != nil || len(empty.Packages) != 0 {
		t.Errorf("missing snapshot: %+v err %v", empty, err)
	}
	// Malformed → error.
	bad := filepath.Join(t.TempDir(), "bad.json")
	_ = os.WriteFile(bad, []byte("{nope"), 0o644)
	if _, err := LoadSnapshot(bad); err == nil {
		t.Error("malformed snapshot should error")
	}
}

// mkPkg writes a single-file package under root/name.
func mkPkg(t *testing.T, root, name, src string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanPackagesSkips(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mkPkg(t, root, "good", "package good\nfunc Exported() {}\n")
	mkPkg(t, root, "_scaffold", "package scaffold\nfunc Skip() {}\n") // _ dir skipped
	// _test.go file is skipped even in a real package.
	_ = os.WriteFile(filepath.Join(root, "good", "good_test.go"), []byte("package good\nfunc TestX(t any) {}\n"), 0o644)

	pkgs, err := scanPackages(root)
	if err != nil {
		t.Fatalf("scanPackages: %v", err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package, got %d: %v", len(pkgs), pkgs)
	}
	for k, syms := range pkgs {
		if !strings.HasSuffix(k, "good") || len(syms) != 1 || syms[0].Name != "Exported" {
			t.Errorf("pkg %q syms = %+v", k, syms)
		}
	}
}

func TestRunEndToEnd(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	pkgRoot := filepath.Join(root, "pkg")
	mkPkg(t, pkgRoot, "x", "package x\nfunc Alpha() {}\nfunc Beta() int { return 0 }\n")
	snap := filepath.Join(root, "api", "snapshot.json")

	var out, errb bytes.Buffer
	// -update bootstraps the snapshot.
	if code := run([]string{"-pkgs", pkgRoot, "-snapshot", snap, "-update"}, &out, &errb); code != 0 {
		t.Fatalf("update exit = %d: %s", code, errb.String())
	}
	// Clean compare → exit 0.
	out.Reset()
	if code := run([]string{"-pkgs", pkgRoot, "-snapshot", snap}, &out, &errb); code != 0 {
		t.Fatalf("clean compare exit = %d: %s", code, out.String())
	}

	// Remove Beta → breaking → exit 1.
	mkPkg(t, pkgRoot, "x", "package x\nfunc Alpha() {}\n")
	out.Reset()
	if code := run([]string{"-pkgs", pkgRoot, "-snapshot", snap}, &out, &errb); code != 1 {
		t.Fatalf("breaking compare exit = %d, want 1\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "REMOVED") {
		t.Errorf("expected REMOVED in report:\n%s", out.String())
	}

	// JSON mode is valid + reports the removal.
	out.Reset()
	run([]string{"-pkgs", pkgRoot, "-snapshot", snap, "-json"}, &out, &errb)
	var got Changes
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("json output invalid: %v\n%s", err, out.String())
	}
	if len(got.Removed) != 1 || got.Removed[0].Name != "Beta" {
		t.Errorf("json Removed = %+v", got.Removed)
	}
}

func TestRunUsageErrors(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	// Missing pkgs dir → exit 2.
	if code := run([]string{"-pkgs", filepath.Join(t.TempDir(), "nope")}, &out, &errb); code != 2 {
		t.Errorf("missing pkgs exit = %d, want 2", code)
	}
	// Empty pkgs dir (no packages) → exit 2.
	if code := run([]string{"-pkgs", t.TempDir()}, &out, &errb); code != 2 {
		t.Errorf("empty pkgs exit = %d, want 2", code)
	}
	// Bad flag → exit 2.
	if code := run([]string{"-bogus"}, &out, &errb); code != 2 {
		t.Errorf("bad flag exit = %d, want 2", code)
	}
}
