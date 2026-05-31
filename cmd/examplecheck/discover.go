package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// discoverExamples returns the names of every runnable example directly under
// root — a subdirectory containing a main.go, excluding scaffold/hidden dirs
// (those whose name starts with "_" or "."). The result is sorted.
func discoverExamples(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("examplecheck: read examples dir: %w", err)
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, name, "main.go")); err != nil {
			continue // not a runnable example
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

// examplesFromPaths maps a set of changed file paths (as emitted by
// `git diff --name-only`) to the set of example names they touch, intersected
// with the known examples. Used for selective CI builds (T-6I02). Pure so it is
// unit-testable without invoking git.
func examplesFromPaths(root string, paths, known []string) []string {
	knownSet := make(map[string]bool, len(known))
	for _, k := range known {
		knownSet[k] = true
	}
	prefix := filepath.ToSlash(root) + "/"
	hit := map[string]bool{}
	for _, p := range paths {
		p = filepath.ToSlash(strings.TrimSpace(p))
		rest, ok := strings.CutPrefix(p, prefix)
		if !ok {
			continue
		}
		name, _, _ := strings.Cut(rest, "/") // examples/<name>/...
		if knownSet[name] {
			hit[name] = true
		}
	}
	out := make([]string, 0, len(hit))
	for n := range hit {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
