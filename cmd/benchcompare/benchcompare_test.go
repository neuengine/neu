package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleOutput = `goos: windows
goarch: amd64
pkg: github.com/neuengine/neu/pkg/math
cpu: AMD Ryzen
BenchmarkLerpVec3-8   	100000000	        10.50 ns/op	       0 B/op	       0 allocs/op
BenchmarkSpecKey-8    	 5000000	       240.0 ns/op	      64 B/op	       2 allocs/op
BenchmarkNoMem-16     	 1000000	      1234 ns/op
Benchmark log noise here, not a result
PASS
ok  	github.com/neuengine/neu/pkg/math	3.5s
`

func TestParseBenchmarks(t *testing.T) {
	t.Parallel()
	got, err := ParseBenchmarks(strings.NewReader(sampleOutput))
	if err != nil {
		t.Fatalf("ParseBenchmarks: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("parsed %d results, want 3: %+v", len(got), got)
	}
	byName := map[string]BenchResult{}
	for _, r := range got {
		byName[r.Name] = r
	}
	lerp, ok := byName["BenchmarkLerpVec3"] // proc suffix stripped
	if !ok || lerp.NsPerOp != 10.5 || lerp.BytesPerOp != 0 || lerp.AllocsPerOp != 0 {
		t.Errorf("LerpVec3 = %+v", lerp)
	}
	spec := byName["BenchmarkSpecKey"]
	if spec.NsPerOp != 240 || spec.BytesPerOp != 64 || spec.AllocsPerOp != 2 {
		t.Errorf("SpecKey = %+v", spec)
	}
	// A result line without -benchmem columns parses with zero mem metrics.
	noMem := byName["BenchmarkNoMem"]
	if noMem.NsPerOp != 1234 || noMem.BytesPerOp != 0 {
		t.Errorf("NoMem = %+v", noMem)
	}
}

func TestStripProcSuffix(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"BenchmarkFoo-8":         "BenchmarkFoo",
		"BenchmarkFoo-128":       "BenchmarkFoo",
		"BenchmarkFoo/case-1-8":  "BenchmarkFoo/case-1", // only the trailing -N is stripped
		"BenchmarkNoSuffix":      "BenchmarkNoSuffix",
		"BenchmarkTrailingDash-": "BenchmarkTrailingDash-",
	}
	for in, want := range cases {
		if got := stripProcSuffix(in); got != want {
			t.Errorf("stripProcSuffix(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCompare(t *testing.T) {
	t.Parallel()
	base := map[string]Metrics{
		"BenchmarkLerp":    {NsPerOp: 10, BytesPerOp: 0, AllocsPerOp: 0},
		"BenchmarkSpec":    {NsPerOp: 240, BytesPerOp: 64, AllocsPerOp: 2},
		"BenchmarkStable":  {NsPerOp: 100, BytesPerOp: 0, AllocsPerOp: 0},
		"BenchmarkRemoved": {NsPerOp: 5, BytesPerOp: 0, AllocsPerOp: 0},
	}
	current := []BenchResult{
		{Name: "BenchmarkLerp", Metrics: Metrics{NsPerOp: 12}},                                  // +20% ns ⇒ regress
		{Name: "BenchmarkSpec", Metrics: Metrics{NsPerOp: 240, BytesPerOp: 64, AllocsPerOp: 3}}, // allocs 2→3 ⇒ regress
		{Name: "BenchmarkStable", Metrics: Metrics{NsPerOp: 102}},                               // +2% ⇒ within threshold, not reported
		{Name: "BenchmarkNew", Metrics: Metrics{NsPerOp: 50}},                                   // added
	}
	res := Compare(base, current, 5.0)

	if !res.HasRegression() {
		t.Fatal("expected a regression")
	}
	// Find the ns regression on Lerp and the allocs regression on Spec.
	var lerpNs, specAllocs bool
	for _, d := range res.Drifts {
		if d.Name == "BenchmarkLerp" && d.Metric == "ns/op" && d.Regression {
			lerpNs = true
		}
		if d.Name == "BenchmarkSpec" && d.Metric == "allocs/op" && d.Regression {
			specAllocs = true
		}
		if d.Name == "BenchmarkStable" {
			t.Errorf("Stable (+2%%) should be within threshold, got drift %+v", d)
		}
	}
	if !lerpNs {
		t.Error("missing ns/op regression on BenchmarkLerp")
	}
	if !specAllocs {
		t.Error("missing allocs/op regression on BenchmarkSpec (2→3)")
	}
	if len(res.Added) != 1 || res.Added[0] != "BenchmarkNew" {
		t.Errorf("Added = %v, want [BenchmarkNew]", res.Added)
	}
	if len(res.Missing) != 1 || res.Missing[0] != "BenchmarkRemoved" {
		t.Errorf("Missing = %v, want [BenchmarkRemoved]", res.Missing)
	}
}

func TestCompareAllocFromZeroAndImprovement(t *testing.T) {
	t.Parallel()
	base := map[string]Metrics{
		"BenchmarkZeroAlloc": {NsPerOp: 10, AllocsPerOp: 0},
		"BenchmarkImproved":  {NsPerOp: 200, AllocsPerOp: 5},
	}
	current := []BenchResult{
		{Name: "BenchmarkZeroAlloc", Metrics: Metrics{NsPerOp: 10, AllocsPerOp: 1}}, // 0→1 categorical regress
		{Name: "BenchmarkImproved", Metrics: Metrics{NsPerOp: 100, AllocsPerOp: 0}}, // big improvement
	}
	res := Compare(base, current, 5.0)
	if !res.HasRegression() {
		t.Fatal("0→1 allocation must be a regression")
	}
	// The improvement is reported but not a regression.
	var improvedReported, improvedRegressed bool
	for _, d := range res.Drifts {
		if d.Name == "BenchmarkImproved" && d.Metric == "ns/op" {
			improvedReported = true
			improvedRegressed = d.Regression
		}
	}
	if !improvedReported {
		t.Error("expected the -50% ns improvement to be reported")
	}
	if improvedRegressed {
		t.Error("an improvement must not be flagged as a regression")
	}
}

func TestBaselineRoundTrip(t *testing.T) {
	t.Parallel()
	results := []BenchResult{
		{Name: "BenchmarkA", Metrics: Metrics{NsPerOp: 10, BytesPerOp: 8, AllocsPerOp: 1}},
		{Name: "BenchmarkB", Metrics: Metrics{NsPerOp: 20}},
	}
	path := filepath.Join(t.TempDir(), "baseline.json")
	b := baselineFrom(results, "go1.26.3", "2026-05-31T00:00:00Z")
	if err := b.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := LoadBaseline(path)
	if err != nil {
		t.Fatalf("LoadBaseline: %v", err)
	}
	if got.GoVersion != "go1.26.3" || len(got.Results) != 2 {
		t.Fatalf("reloaded baseline = %+v", got)
	}
	if got.Results["BenchmarkA"] != (Metrics{NsPerOp: 10, BytesPerOp: 8, AllocsPerOp: 1}) {
		t.Errorf("BenchmarkA metrics = %+v", got.Results["BenchmarkA"])
	}
	if names := got.names(); len(names) != 2 || names[0] != "BenchmarkA" {
		t.Errorf("names() = %v, want sorted [A B]", names)
	}
}

func TestLoadBaselineErrors(t *testing.T) {
	t.Parallel()
	if _, err := LoadBaseline(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Error("missing baseline should error")
	}
	bad := filepath.Join(t.TempDir(), "bad.json")
	_ = os.WriteFile(bad, []byte("{not json"), 0o644)
	if _, err := LoadBaseline(bad); err == nil {
		t.Error("malformed baseline should error")
	}
}

// runCLI dispatches run() with the given stdin text, returning exit code + stdout.
func runCLI(t *testing.T, stdin string, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(argv, strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

func TestRunUpdateThenCompareOK(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "baseline.json")

	// -update writes the baseline from the sample output.
	if code, out, _ := runCLI(t, sampleOutput, "-update", "-baseline", path); code != 0 || !strings.Contains(out, "wrote baseline") {
		t.Fatalf("update: code=%d out=%q", code, out)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("baseline not written: %v", err)
	}
	// Comparing the same output against the just-written baseline ⇒ OK (exit 0).
	code, out, _ := runCLI(t, sampleOutput, "-baseline", path)
	if code != 0 {
		t.Fatalf("self-compare should pass: code=%d out=%q", code, out)
	}
}

func TestRunRegressionExit1(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "baseline.json")
	if code, _, _ := runCLI(t, sampleOutput, "-update", "-baseline", path); code != 0 {
		t.Fatalf("update exit = %d", code)
	}
	// A slower LerpVec3 (10.5 → 30 ns) is a regression ⇒ exit 1.
	regressed := strings.Replace(sampleOutput, "10.50 ns/op", "30.0 ns/op", 1)
	code, out, _ := runCLI(t, regressed, "-baseline", path)
	if code != 1 || !strings.Contains(out, "REGRESS") {
		t.Errorf("regression: code=%d out=%q", code, out)
	}
}

func TestRunJSON(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "baseline.json")
	runCLI(t, sampleOutput, "-update", "-baseline", path)
	// Add an allocation to force a drift, then request JSON.
	regressed := strings.Replace(sampleOutput, "2 allocs/op", "4 allocs/op", 1)
	code, out, _ := runCLI(t, regressed, "-baseline", path, "-json")
	if code != 1 {
		t.Fatalf("expected regression exit 1, got %d", code)
	}
	var view struct {
		Drifts        []map[string]any `json:"drifts"`
		HasRegression bool             `json:"has_regression"`
	}
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if !view.HasRegression || len(view.Drifts) == 0 {
		t.Errorf("JSON view = %+v", view)
	}
}

func TestRunUsageErrors(t *testing.T) {
	t.Parallel()
	// Empty input ⇒ exit 2.
	if code, _, _ := runCLI(t, "no benchmarks here\n"); code != 2 {
		t.Errorf("empty input exit = %d, want 2", code)
	}
	// Missing baseline file ⇒ exit 2.
	if code, _, _ := runCLI(t, sampleOutput, "-baseline", filepath.Join(t.TempDir(), "absent.json")); code != 2 {
		t.Errorf("missing baseline exit = %d, want 2", code)
	}
	// Unknown flag ⇒ exit 2.
	if code, _, _ := runCLI(t, sampleOutput, "-nonsense"); code != 2 {
		t.Errorf("bad flag exit = %d, want 2", code)
	}
}
