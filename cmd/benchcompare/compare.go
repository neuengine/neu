package main

import (
	"math"
	"sort"
)

// Drift is one metric's change between baseline and current for a benchmark.
type Drift struct {
	Name       string
	Metric     string // "ns/op", "B/op", "allocs/op"
	Baseline   float64
	Current    float64
	DeltaPct   float64
	Regression bool
}

// CompareResult bundles every reportable metric change plus the set of
// benchmarks that appeared or disappeared relative to the baseline.
type CompareResult struct {
	Drifts  []Drift
	Missing []string // present in baseline, absent from current
	Added   []string // present in current, absent from baseline
}

// HasRegression reports whether any drift breached the policy.
func (c CompareResult) HasRegression() bool {
	for _, d := range c.Drifts {
		if d.Regression {
			return true
		}
	}
	return false
}

// pctChange returns the percentage change from base to cur. A change away from a
// zero baseline is +Inf (a categorical regression for allocation metrics).
func pctChange(base, cur float64) float64 {
	if base == 0 {
		if cur == 0 {
			return 0
		}
		return math.Inf(1)
	}
	return (cur - base) / base * 100
}

// Compare evaluates current results against the baseline. ns/op and B/op regress
// when they grow by more than thresholdPct; allocs/op regress on *any* increase
// (the engine targets 0-alloc hot paths — a 0→1 jump is a categorical break). A
// metric is reported when it regresses or moves more than thresholdPct either way.
func Compare(base map[string]Metrics, current []BenchResult, thresholdPct float64) CompareResult {
	var res CompareResult
	seen := make(map[string]bool, len(current))

	sorted := append([]BenchResult(nil), current...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	for _, cur := range sorted {
		seen[cur.Name] = true
		b, ok := base[cur.Name]
		if !ok {
			res.Added = append(res.Added, cur.Name)
			continue
		}
		metrics := []struct {
			label       string
			base, value float64
			allocs      bool
		}{
			{"ns/op", b.NsPerOp, cur.NsPerOp, false},
			{"B/op", b.BytesPerOp, cur.BytesPerOp, false},
			{"allocs/op", b.AllocsPerOp, cur.AllocsPerOp, true},
		}
		for _, m := range metrics {
			delta := pctChange(m.base, m.value)
			var regression bool
			if m.allocs {
				regression = m.value > m.base // any allocation increase
			} else {
				regression = delta > thresholdPct
			}
			if regression || math.Abs(delta) > thresholdPct {
				res.Drifts = append(res.Drifts, Drift{
					Name: cur.Name, Metric: m.label,
					Baseline: m.base, Current: m.value, DeltaPct: delta, Regression: regression,
				})
			}
		}
	}

	for name := range base {
		if !seen[name] {
			res.Missing = append(res.Missing, name)
		}
	}
	sort.Strings(res.Missing)
	return res
}
