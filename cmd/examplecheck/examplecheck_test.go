package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseResult(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		out     string
		runErr  error
		wantOK  bool
		wantHas bool
		wantH   uint64
	}{
		{"hash pass", "PASS: window hash=123\n", nil, true, true, 123},
		{"smoke pass", "PASS add/get\nPASS drop\n", nil, true, false, 0},
		{"fail line", "PASS x\nFAIL: boom\n", nil, false, false, 0},
		{"run error", "PASS: x hash=9\n", errors.New("exit 1"), false, true, 9},
		{"hash no digits", "done hash=\n", nil, true, false, 0},
		{"hash non-numeric", "hash=abc\n", nil, true, false, 0},
	}
	for _, tc := range tests {
		got := parseResult(tc.out, tc.runErr)
		if got.OK != tc.wantOK || got.HasHash != tc.wantHas || got.Hash != tc.wantH {
			t.Errorf("%s: parseResult = %+v, want OK=%v Has=%v H=%d", tc.name, got, tc.wantOK, tc.wantHas, tc.wantH)
		}
	}
}

func TestClassify(t *testing.T) {
	t.Parallel()
	cases := []struct {
		res       Result
		golden    uint64
		hasGolden bool
		want      string
	}{
		{Result{OK: false}, 0, false, "fail"},
		{Result{OK: true, HasHash: false}, 0, false, "smoke"},
		{Result{OK: true, HasHash: true, Hash: 5}, 0, false, "new"},
		{Result{OK: true, HasHash: true, Hash: 5}, 9, true, "drift"},
		{Result{OK: true, HasHash: true, Hash: 5}, 5, true, "ok"},
	}
	for _, c := range cases {
		if got := classify(c.res, c.golden, c.hasGolden); got != c.want {
			t.Errorf("classify(%+v, %d, %v) = %q, want %q", c.res, c.golden, c.hasGolden, got, c.want)
		}
	}
}

func TestGoldensRoundTrip(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "goldens.json")
	g := Goldens{GoVersion: "go1.26.3", Examples: map[string]uint64{"window": 1, "ui": 2}}
	if err := g.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := LoadGoldens(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.GoVersion != "go1.26.3" || got.Examples["window"] != 1 || got.Examples["ui"] != 2 {
		t.Errorf("reloaded = %+v", got)
	}
	// Missing file → empty registry, not an error.
	empty, err := LoadGoldens(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil || len(empty.Examples) != 0 {
		t.Errorf("missing goldens: got %+v, err %v", empty, err)
	}
	// Malformed → error.
	bad := filepath.Join(t.TempDir(), "bad.json")
	_ = os.WriteFile(bad, []byte("{nope"), 0o644)
	if _, err := LoadGoldens(bad); err == nil {
		t.Error("malformed goldens should error")
	}
}

func TestDiscoverExamples(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mkExample := func(name string, withMain bool) {
		dir := filepath.Join(root, name)
		_ = os.MkdirAll(dir, 0o755)
		if withMain {
			_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644)
		}
	}
	mkExample("window", true)
	mkExample("2d", true)
	mkExample("_template", true) // scaffold — excluded
	mkExample(".hidden", true)   // hidden — excluded
	mkExample("notrunnable", false)
	_ = os.WriteFile(filepath.Join(root, "goldens.json"), []byte("{}"), 0o644) // file, not dir

	got, err := discoverExamples(root)
	if err != nil {
		t.Fatalf("discoverExamples: %v", err)
	}
	want := []string{"2d", "window"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("discoverExamples = %v, want %v", got, want)
	}

	if _, err := discoverExamples(filepath.Join(root, "nope")); err == nil {
		t.Error("missing root should error")
	}
}

func TestExamplesFromPaths(t *testing.T) {
	t.Parallel()
	known := []string{"window", "2d", "ui"}
	paths := []string{
		"examples/window/main.go",
		"examples/2d/main_test.go",
		"examples/unknown/main.go", // not in known → ignored
		"pkg/render/sprite.go",     // not under examples → ignored
		"examples/ui/", // trailing slash
	}
	got := examplesFromPaths("examples", paths, known)
	want := "2d,ui,window"
	if strings.Join(got, ",") != want {
		t.Errorf("examplesFromPaths = %v, want %s", got, want)
	}
	// No changed example paths → empty.
	if got := examplesFromPaths("examples", []string{"go.mod"}, known); len(got) != 0 {
		t.Errorf("unrelated paths should yield no examples, got %v", got)
	}
}

// fakeRunner returns canned output per example name.
func fakeRunner(outputs map[string]string, errs map[string]error) exampleRunner {
	return func(name string) (string, error) {
		return outputs[name], errs[name]
	}
}

func TestEvaluate(t *testing.T) {
	t.Parallel()
	names := []string{"ok1", "drifted", "newhash", "smoke", "broken"}
	goldens := map[string]uint64{"ok1": 100, "drifted": 200}
	outputs := map[string]string{
		"ok1":      "PASS: ok1 hash=100\n",
		"drifted":  "PASS: drifted hash=999\n", // golden 200 → drift
		"newhash":  "PASS: newhash hash=42\n",  // no golden → new
		"smoke":    "PASS ran fine\n",          // no hash → smoke
		"broken":   "FAIL: kaboom\n",           // fail
	}
	var buf bytes.Buffer
	recorded, failed, drifted := evaluate(names, goldens, fakeRunner(outputs, nil), &buf)

	if failed != 1 || drifted != 1 {
		t.Errorf("failed=%d drifted=%d, want 1/1", failed, drifted)
	}
	// recorded holds every hash-emitting example (ok + drift + new), not smoke/fail.
	if recorded["ok1"] != 100 || recorded["drifted"] != 999 || recorded["newhash"] != 42 {
		t.Errorf("recorded = %v", recorded)
	}
	if _, ok := recorded["smoke"]; ok {
		t.Error("smoke example must not be recorded")
	}
	if _, ok := recorded["broken"]; ok {
		t.Error("failed example must not be recorded")
	}
	out := buf.String()
	for _, want := range []string{"ok      ok1", "DRIFT   drifted", "new     newhash", "ok      smoke (smoke)", "FAIL    broken"} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q; got:\n%s", want, out)
		}
	}
}

// mkExamplesRoot creates a temp examples root with a main.go per name.
func mkExamplesRoot(t *testing.T, names ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, n := range names {
		dir := filepath.Join(root, n)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestExecuteCompareUpdateDrift(t *testing.T) {
	t.Parallel()
	root := mkExamplesRoot(t, "alpha", "beta")
	goldens := filepath.Join(root, "goldens.json")
	outputs := map[string]string{
		"alpha": "PASS: alpha hash=111\n",
		"beta":  "PASS beta smoke\n", // no hash
	}
	cfg := config{root: root, goldensPath: goldens, timeout: time.Minute}
	var buf bytes.Buffer

	// First compare with no goldens file: alpha is "new", beta smoke → exit 0.
	if code := execute(cfg, fakeRunner(outputs, nil), &buf, &buf); code != 0 {
		t.Fatalf("initial compare exit = %d\n%s", code, buf.String())
	}

	// -update writes the registry (alpha only).
	buf.Reset()
	cfgUpd := cfg
	cfgUpd.update = true
	if code := execute(cfgUpd, fakeRunner(outputs, nil), &buf, &buf); code != 0 {
		t.Fatalf("update exit = %d", code)
	}
	g, err := LoadGoldens(goldens)
	if err != nil || g.Examples["alpha"] != 111 {
		t.Fatalf("goldens after update = %+v, err %v", g, err)
	}

	// Re-compare against the written golden: alpha matches → exit 0.
	buf.Reset()
	if code := execute(cfg, fakeRunner(outputs, nil), &buf, &buf); code != 0 {
		t.Fatalf("matching compare exit = %d\n%s", code, buf.String())
	}

	// alpha now drifts (222 ≠ golden 111) → exit 1.
	buf.Reset()
	drift := map[string]string{"alpha": "PASS: alpha hash=222\n", "beta": "PASS beta smoke\n"}
	if code := execute(cfg, fakeRunner(drift, nil), &buf, &buf); code != 1 {
		t.Fatalf("drift compare exit = %d, want 1\n%s", code, buf.String())
	}
	if !strings.Contains(buf.String(), "DRIFT   alpha") {
		t.Errorf("drift report missing: %s", buf.String())
	}
}

func TestRunListAndUsage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, n := range []string{"alpha", "beta"} {
		dir := filepath.Join(root, n)
		_ = os.MkdirAll(dir, 0o755)
		_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644)
	}
	var out, errb bytes.Buffer

	// -list discovers without running.
	if code := run([]string{"-examples", root, "-list"}, &out, &errb); code != 0 {
		t.Fatalf("-list exit = %d", code)
	}
	if !strings.Contains(out.String(), "alpha") || !strings.Contains(out.String(), "beta") {
		t.Errorf("-list output: %q", out.String())
	}

	// Empty root → exit 2.
	if code := run([]string{"-examples", t.TempDir()}, &out, &errb); code != 2 {
		t.Errorf("empty root exit = %d, want 2", code)
	}
	// Missing root → exit 2.
	if code := run([]string{"-examples", filepath.Join(root, "nope")}, &out, &errb); code != 2 {
		t.Errorf("missing root exit = %d, want 2", code)
	}
	// Bad flag → exit 2.
	if code := run([]string{"-bogus"}, &out, &errb); code != 2 {
		t.Errorf("bad flag exit = %d, want 2", code)
	}
}

// TestRunChangedSelective exercises the -changed selective-build path. The temp
// examples live outside the repo tree, so no git-changed path maps onto them and
// the run reports "no examples changed" (exit 0) deterministically.
func TestRunChangedSelective(t *testing.T) {
	t.Parallel()
	root := mkExamplesRoot(t, "alpha")
	var out, errb bytes.Buffer
	code := run([]string{"-examples", root, "-changed", "HEAD"}, &out, &errb)
	if code != 0 {
		t.Fatalf("-changed exit = %d (err=%s)", code, errb.String())
	}
	if !strings.Contains(out.String(), "no examples changed") {
		t.Errorf("-changed output = %q", out.String())
	}
}
