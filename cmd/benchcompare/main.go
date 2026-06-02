package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"runtime"
	"time"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run is the testable entry point. It returns the process exit code: 0 = OK,
// 1 = regression detected, 2 = usage/IO error.
func run(argv []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("benchcompare", flag.ContinueOnError)
	fs.SetOutput(stderr)
	baselinePath := fs.String("baseline", "bench/baseline.json", "path to the baseline JSON file")
	currentPath := fs.String("current", "", "benchmark output file to read (default: stdin)")
	threshold := fs.Float64("threshold", 5.0, "regression threshold (percent) for ns/op and B/op")
	update := fs.Bool("update", false, "write the parsed results as the new baseline and exit")
	asJSON := fs.Bool("json", false, "emit the comparison report as JSON")
	if err := fs.Parse(argv); err != nil {
		return 2
	}

	src := stdin
	if *currentPath != "" {
		f, err := os.Open(*currentPath)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "benchcompare: %v\n", err)
			return 2
		}
		defer func() { _ = f.Close() }()
		src = f
	}

	results, err := ParseBenchmarks(src)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "benchcompare: parse: %v\n", err)
		return 2
	}
	if len(results) == 0 {
		_, _ = fmt.Fprintln(stderr, "benchcompare: no benchmark results found in input")
		return 2
	}

	if *update {
		b := baselineFrom(results, runtime.Version(), time.Now().UTC().Format(time.RFC3339))
		if err := b.Save(*baselinePath); err != nil {
			_, _ = fmt.Fprintf(stderr, "benchcompare: %v\n", err)
			return 2
		}
		_, _ = fmt.Fprintf(stdout, "benchcompare: wrote baseline with %d benchmarks to %s\n", len(b.Results), *baselinePath)
		return 0
	}

	base, err := LoadBaseline(*baselinePath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "benchcompare: %v\n", err)
		return 2
	}
	res := Compare(base.Results, results, *threshold)

	if *asJSON {
		if err := writeJSON(stdout, res); err != nil {
			_, _ = fmt.Fprintf(stderr, "benchcompare: %v\n", err)
			return 2
		}
	} else {
		writeReport(stdout, base, res, *threshold)
	}
	if res.HasRegression() {
		return 1
	}
	return 0
}

// writeReport renders a human-readable comparison summary.
func writeReport(w io.Writer, base Baseline, res CompareResult, threshold float64) {
	_, _ = fmt.Fprintf(w, "benchcompare: %d baseline benchmarks, threshold ±%.1f%%\n", len(base.names()), threshold)
	for _, d := range res.Drifts {
		tag := "ok     "
		if d.Regression {
			tag = "REGRESS"
		}
		_, _ = fmt.Fprintf(w, "  %s %-40s %-10s %.4g → %.4g (%s)\n",
			tag, d.Name, d.Metric, d.Baseline, d.Current, formatPct(d.DeltaPct))
	}
	for _, n := range res.Added {
		_, _ = fmt.Fprintf(w, "  new     %s (not in baseline)\n", n)
	}
	for _, n := range res.Missing {
		_, _ = fmt.Fprintf(w, "  missing %s (in baseline, not run)\n", n)
	}
	if res.HasRegression() {
		_, _ = fmt.Fprintln(w, "benchcompare: REGRESSION detected")
	} else {
		_, _ = fmt.Fprintln(w, "benchcompare: OK")
	}
}

// formatPct renders a percentage delta, rendering a jump from a zero baseline as
// "+new" rather than the non-printable +Inf.
func formatPct(p float64) string {
	if math.IsInf(p, 1) {
		return "+new"
	}
	return fmt.Sprintf("%+.1f%%", p)
}

// driftJSON is the JSON view of a Drift. DeltaPct is a pointer so a non-finite
// delta (jump from a zero baseline) serialises as null instead of failing the
// encoder (encoding/json rejects Inf/NaN).
type driftJSON struct {
	Name       string   `json:"name"`
	Metric     string   `json:"metric"`
	Baseline   float64  `json:"baseline"`
	Current    float64  `json:"current"`
	DeltaPct   *float64 `json:"delta_pct"`
	Regression bool     `json:"regression"`
}

func writeJSON(w io.Writer, res CompareResult) error {
	view := struct {
		Drifts        []driftJSON `json:"drifts"`
		Added         []string    `json:"added"`
		Missing       []string    `json:"missing"`
		HasRegression bool        `json:"has_regression"`
	}{Added: res.Added, Missing: res.Missing, HasRegression: res.HasRegression()}
	for _, d := range res.Drifts {
		dj := driftJSON{Name: d.Name, Metric: d.Metric, Baseline: d.Baseline, Current: d.Current, Regression: d.Regression}
		if !math.IsInf(d.DeltaPct, 0) && !math.IsNaN(d.DeltaPct) {
			p := d.DeltaPct
			dj.DeltaPct = &p
		}
		view.Drifts = append(view.Drifts, dj)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(view)
}
