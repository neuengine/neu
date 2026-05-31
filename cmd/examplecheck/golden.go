package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Goldens is the committed registry of expected example hashes, keyed by example
// directory name. Smoke-only examples (no hash) are intentionally absent.
type Goldens struct {
	GoVersion string            `json:"go_version,omitempty"`
	Updated   string            `json:"updated,omitempty"`
	Examples  map[string]uint64 `json:"examples"`
}

// LoadGoldens reads the golden registry. A missing file yields an empty registry
// (so a first run can be bootstrapped with -update) rather than an error.
func LoadGoldens(path string) (Goldens, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Goldens{Examples: map[string]uint64{}}, nil
		}
		return Goldens{}, fmt.Errorf("examplecheck: read goldens: %w", err)
	}
	var g Goldens
	if err := json.Unmarshal(data, &g); err != nil {
		return Goldens{}, fmt.Errorf("examplecheck: parse goldens %s: %w", path, err)
	}
	if g.Examples == nil {
		g.Examples = map[string]uint64{}
	}
	return g, nil
}

// Save writes the registry as indented JSON (encoding/json sorts string keys, so
// the output is deterministic).
func (g Goldens) Save(path string) error {
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return fmt.Errorf("examplecheck: encode goldens: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("examplecheck: write goldens: %w", err)
	}
	return nil
}
