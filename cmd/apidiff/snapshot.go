package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Snapshot is the committed record of the exported API surface, keyed by
// package directory (slash path relative to the module root).
type Snapshot struct {
	Packages  map[string][]Symbol `json:"packages"`
	GoVersion string              `json:"go_version,omitempty"`
	Updated   string              `json:"updated,omitempty"`
}

// LoadSnapshot reads the committed snapshot. A missing file yields an empty
// snapshot (so a first run can bootstrap with -update) rather than an error.
func LoadSnapshot(path string) (Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Snapshot{Packages: map[string][]Symbol{}}, nil
		}
		return Snapshot{}, fmt.Errorf("apidiff: read snapshot: %w", err)
	}
	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return Snapshot{}, fmt.Errorf("apidiff: parse snapshot %s: %w", path, err)
	}
	if s.Packages == nil {
		s.Packages = map[string][]Symbol{}
	}
	return s, nil
}

// Save writes the snapshot as indented JSON, creating the parent directory.
func (s Snapshot) Save(path string) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("apidiff: mkdir %s: %w", dir, err)
		}
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("apidiff: encode snapshot: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("apidiff: write snapshot: %w", err)
	}
	return nil
}

// scanPackages walks root recursively and extracts the exported symbols of every
// package directory, keyed by slash path. Scaffold/hidden/testdata dirs and
// _test.go files are skipped.
func scanPackages(root string) (map[string][]Symbol, error) {
	pkgs := map[string][]Symbol{}
	err := filepath.WalkDir(root, func(path string, de fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if de.IsDir() {
			name := de.Name()
			if path != root && (strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") || name == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		syms, err := extractFromSource(path, string(data))
		if err != nil {
			return fmt.Errorf("apidiff: %s: %w", path, err)
		}
		if len(syms) == 0 {
			return nil
		}
		key := filepath.ToSlash(filepath.Dir(path))
		pkgs[key] = append(pkgs[key], syms...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	for k := range pkgs {
		sortSymbols(pkgs[k])
	}
	return pkgs, nil
}

// sortSymbols orders symbols deterministically by (kind, name, signature).
func sortSymbols(syms []Symbol) {
	sort.Slice(syms, func(i, j int) bool {
		if syms[i].Kind != syms[j].Kind {
			return syms[i].Kind < syms[j].Kind
		}
		if syms[i].Name != syms[j].Name {
			return syms[i].Name < syms[j].Name
		}
		return syms[i].Signature < syms[j].Signature
	})
}
