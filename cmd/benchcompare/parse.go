// Command benchcompare parses `go test -bench -benchmem` output and compares it
// against a committed baseline, failing on performance regression. It backs the
// engine's benchmark drift gate (Track L; consumed by the Track E CI gate).
//
// The parser is stdlib-only (C-003): the `testing` benchmark line format is
// stable and documented, so no external benchmark-parsing dependency is needed.
package main

import (
	"bufio"
	"io"
	"strconv"
	"strings"
)

// Metrics holds the per-operation measurements of a single benchmark.
type Metrics struct {
	NsPerOp     float64 `json:"ns_per_op"`
	BytesPerOp  float64 `json:"bytes_per_op"`
	AllocsPerOp float64 `json:"allocs_per_op"`
}

// BenchResult is one parsed benchmark line: its canonical name (GOMAXPROCS
// suffix stripped), iteration count, and metrics.
type BenchResult struct {
	Name       string
	Iterations int64
	Metrics
}

// ParseBenchmarks reads `go test -bench` output and returns the benchmark
// results, ignoring non-result lines (goos/pkg/PASS/ok/log output). A line is a
// result only if it starts with "Benchmark" and its second field is an integer
// iteration count, so stray "Benchmark..." log lines are skipped.
func ParseBenchmarks(r io.Reader) ([]BenchResult, error) {
	var out []BenchResult
	sc := bufio.NewScanner(r)
	// Benchmark lines can be long with many custom metrics; grow the buffer.
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "Benchmark") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue // need at least: name, iters, value, unit
		}
		iters, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue // second field is not an iteration count ⇒ not a result line
		}
		res := BenchResult{Name: stripProcSuffix(fields[0]), Iterations: iters}
		// Remaining fields are (value, unit) pairs.
		for i := 2; i+1 < len(fields); i += 2 {
			val, err := strconv.ParseFloat(fields[i], 64)
			if err != nil {
				continue
			}
			switch fields[i+1] {
			case "ns/op":
				res.NsPerOp = val
			case "B/op":
				res.BytesPerOp = val
			case "allocs/op":
				res.AllocsPerOp = val
			}
		}
		out = append(out, res)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// stripProcSuffix removes the trailing "-N" GOMAXPROCS suffix the testing
// framework appends (e.g. "BenchmarkLerp-8" ⇒ "BenchmarkLerp"), yielding a name
// stable across machines with different CPU counts.
func stripProcSuffix(name string) string {
	i := strings.LastIndexByte(name, '-')
	if i < 0 || i == len(name)-1 {
		return name
	}
	for _, c := range name[i+1:] {
		if c < '0' || c > '9' {
			return name
		}
	}
	return name[:i]
}
