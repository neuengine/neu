package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the testable entry point. Exit codes: 0 = ok (or only additions),
// 1 = breaking change, 2 = usage/IO error.
func run(argv []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("apidiff", flag.ContinueOnError)
	fs.SetOutput(stderr)
	pkgs := fs.String("pkgs", "pkg", "root directory of the public API to snapshot")
	snapPath := fs.String("snapshot", "api/snapshot.json", "committed API snapshot path")
	update := fs.Bool("update", false, "rewrite the snapshot from the current API and exit")
	asJSON := fs.Bool("json", false, "emit the diff as JSON")
	if err := fs.Parse(argv); err != nil {
		return 2
	}

	current, err := scanPackages(*pkgs)
	if err != nil {
		fmt.Fprintf(stderr, "apidiff: %v\n", err)
		return 2
	}
	if len(current) == 0 {
		fmt.Fprintf(stderr, "apidiff: no packages found under %s\n", *pkgs)
		return 2
	}

	if *update {
		snap := Snapshot{GoVersion: runtime.Version(), Updated: time.Now().UTC().Format(time.RFC3339), Packages: current}
		if err := snap.Save(*snapPath); err != nil {
			fmt.Fprintf(stderr, "apidiff: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "apidiff: wrote snapshot of %d packages to %s\n", len(current), *snapPath)
		return 0
	}

	old, err := LoadSnapshot(*snapPath)
	if err != nil {
		fmt.Fprintf(stderr, "apidiff: %v\n", err)
		return 2
	}
	changes := Diff(old, Snapshot{Packages: current})

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(changes); err != nil {
			fmt.Fprintf(stderr, "apidiff: %v\n", err)
			return 2
		}
	} else {
		writeReport(stdout, changes)
	}
	if changes.Breaking() {
		return 1
	}
	return 0
}

// writeReport renders a human-readable API diff.
func writeReport(w io.Writer, c Changes) {
	for _, ch := range c.Removed {
		fmt.Fprintf(w, "REMOVED %s %s %s\n", ch.Pkg, ch.Kind, ch.Name)
	}
	for _, ch := range c.Changed {
		fmt.Fprintf(w, "CHANGED %s %s %s\n        - %s\n        + %s\n", ch.Pkg, ch.Kind, ch.Name, ch.Old, ch.New)
	}
	for _, ch := range c.Added {
		fmt.Fprintf(w, "added   %s %s %s\n", ch.Pkg, ch.Kind, ch.Name)
	}
	switch {
	case c.Breaking():
		fmt.Fprintf(w, "apidiff: BREAKING — %d removed, %d changed, %d added\n", len(c.Removed), len(c.Changed), len(c.Added))
	case len(c.Added) > 0:
		fmt.Fprintf(w, "apidiff: OK — %d additions (backward-compatible)\n", len(c.Added))
	default:
		fmt.Fprintln(w, "apidiff: OK — no API changes")
	}
}
