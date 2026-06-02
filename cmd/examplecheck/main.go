package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// exampleRunner executes one example and returns its combined output + exec
// error. Injected into [execute] so the comparison logic is testable without
// spawning subprocesses; production wiring uses [runExample].
type exampleRunner func(name string) (string, error)

// config is the resolved CLI configuration.
type config struct {
	root        string
	goldensPath string
	changed     string
	update      bool
	list        bool
	timeout     time.Duration
}

// run parses flags and delegates to [execute] with the real (exec-backed) runner.
func run(argv []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("examplecheck", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("examples", "examples", "examples root directory")
	goldensPath := fs.String("goldens", "", "golden registry path (default: <examples>/goldens.json)")
	update := fs.Bool("update", false, "run examples and rewrite the golden registry")
	list := fs.Bool("list", false, "list discovered example names and exit")
	changed := fs.String("changed", "", "limit to examples changed since this git ref (selective build)")
	timeout := fs.Duration("timeout", 2*time.Minute, "per-example run timeout")
	if err := fs.Parse(argv); err != nil {
		return 2
	}
	cfg := config{root: *root, goldensPath: *goldensPath, changed: *changed, update: *update, list: *list, timeout: *timeout}
	if cfg.goldensPath == "" {
		cfg.goldensPath = filepath.Join(cfg.root, "goldens.json")
	}
	runner := func(name string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
		defer cancel()
		return runExample(ctx, cfg.root, name)
	}
	return execute(cfg, runner, stdout, stderr)
}

// execute discovers examples, optionally filters to git-changed ones, runs each
// through runner, and compares results to the golden registry. Exit codes:
// 0 = ok, 1 = failure/drift, 2 = usage/IO.
func execute(cfg config, runner exampleRunner, stdout, stderr io.Writer) int {
	names, err := discoverExamples(cfg.root)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "examplecheck: %v\n", err)
		return 2
	}
	if len(names) == 0 {
		_, _ = fmt.Fprintf(stderr, "examplecheck: no examples found in %s\n", cfg.root)
		return 2
	}

	if cfg.list {
		for _, n := range names {
			_, _ = fmt.Fprintln(stdout, n)
		}
		return 0
	}

	if cfg.changed != "" {
		paths, err := gitChangedPaths(context.Background(), cfg.changed)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "examplecheck: %v\n", err)
			return 2
		}
		names = examplesFromPaths(cfg.root, paths, names)
		if len(names) == 0 {
			_, _ = fmt.Fprintf(stdout, "examplecheck: no examples changed since %s\n", cfg.changed)
			return 0
		}
	}

	goldens, err := LoadGoldens(cfg.goldensPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "examplecheck: %v\n", err)
		return 2
	}
	recorded, failed, drifted := evaluate(names, goldens.Examples, runner, stdout)

	if cfg.update {
		g := Goldens{
			GoVersion: runtime.Version(),
			Updated:   time.Now().UTC().Format(time.RFC3339),
			Examples:  recorded,
		}
		if err := g.Save(cfg.goldensPath); err != nil {
			_, _ = fmt.Fprintf(stderr, "examplecheck: %v\n", err)
			return 2
		}
		_, _ = fmt.Fprintf(stdout, "examplecheck: wrote %d goldens to %s (%d failed)\n", len(recorded), cfg.goldensPath, failed)
		return 0
	}

	if failed > 0 || drifted > 0 {
		_, _ = fmt.Fprintf(stdout, "examplecheck: %d failed, %d drifted\n", failed, drifted)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "examplecheck: %d examples OK\n", len(names))
	return 0
}

// evaluate runs each example through runner, classifies it against the goldens,
// writes a per-example report line, and returns the recorded hashes (for -update)
// plus failure/drift counts. Pure with respect to the injected runner.
func evaluate(names []string, goldens map[string]uint64, runner exampleRunner, w io.Writer) (recorded map[string]uint64, failed, drifted int) {
	recorded = map[string]uint64{}
	for _, name := range names {
		out, runErr := runner(name)
		res := parseResult(out, runErr)
		golden, hasGolden := goldens[name]
		switch classify(res, golden, hasGolden) {
		case "fail":
			failed++
			_, _ = fmt.Fprintf(w, "FAIL    %s\n", name)
		case "drift":
			drifted++
			recorded[name] = res.Hash
			_, _ = fmt.Fprintf(w, "DRIFT   %s hash=%d, golden=%d\n", name, res.Hash, golden)
		case "new":
			recorded[name] = res.Hash
			_, _ = fmt.Fprintf(w, "new     %s hash=%d (no golden)\n", name, res.Hash)
		case "ok":
			recorded[name] = res.Hash
			_, _ = fmt.Fprintf(w, "ok      %s hash=%d\n", name, res.Hash)
		default: // smoke
			_, _ = fmt.Fprintf(w, "ok      %s (smoke)\n", name)
		}
	}
	return recorded, failed, drifted
}
