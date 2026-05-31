package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// Baseline is the committed reference set of benchmark metrics keyed by canonical
// benchmark name. It is regenerated with `benchcompare -update`.
type Baseline struct {
	GoVersion string             `json:"go_version,omitempty"`
	Updated   string             `json:"updated,omitempty"`
	Results   map[string]Metrics `json:"results"`
}

// LoadBaseline reads and decodes a baseline JSON file.
func LoadBaseline(path string) (Baseline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Baseline{}, fmt.Errorf("benchcompare: read baseline: %w", err)
	}
	var b Baseline
	if err := json.Unmarshal(data, &b); err != nil {
		return Baseline{}, fmt.Errorf("benchcompare: parse baseline %s: %w", path, err)
	}
	if b.Results == nil {
		b.Results = map[string]Metrics{}
	}
	return b, nil
}

// baselineFrom builds a Baseline from parsed results (used by -update).
func baselineFrom(results []BenchResult, goVersion, updated string) Baseline {
	b := Baseline{GoVersion: goVersion, Updated: updated, Results: make(map[string]Metrics, len(results))}
	for _, r := range results {
		b.Results[r.Name] = r.Metrics
	}
	return b
}

// Save writes the baseline as indented JSON (deterministic key order via the map
// being marshalled by encoding/json, which sorts string keys).
func (b Baseline) Save(path string) error {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("benchcompare: encode baseline: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("benchcompare: write baseline: %w", err)
	}
	return nil
}

// names returns the baseline's benchmark names in sorted order.
func (b Baseline) names() []string {
	out := make([]string, 0, len(b.Results))
	for n := range b.Results {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
