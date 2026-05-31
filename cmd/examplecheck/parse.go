// Command examplecheck discovers the engine's runnable examples, executes each,
// and verifies its output against a committed golden registry (Track I /
// T-6I01, T-6I02). It catches behavioural drift that the per-example ×20
// stability tests cannot: those prove an example is internally deterministic,
// while the committed golden hash proves the deterministic value did not change.
//
// Examples follow one of two conventions (see examples/README.md):
//   - hash examples print "PASS: <name> hash=<N>" → golden-checked for drift;
//   - smoke examples print "PASS ..." lines with no single hash → run-only check.
//
// Any "FAIL" line or non-zero exit fails the example.
package main

import (
	"strconv"
	"strings"
)

// Result is the parsed outcome of running one example.
type Result struct {
	OK      bool   // ran to completion with no FAIL line and a clean exit
	HasHash bool   // emitted a "hash=<N>" value (golden-checkable)
	Hash    uint64
}

// parseResult interprets an example's stdout plus the exec error. An example
// passes when it exited cleanly and printed no "FAIL"; a "hash=<digits>" token,
// when present, is captured for golden comparison.
func parseResult(stdout string, runErr error) Result {
	r := Result{OK: runErr == nil && !strings.Contains(stdout, "FAIL")}
	if h, ok := extractHash(stdout); ok {
		r.Hash = h
		r.HasHash = true
	}
	return r
}

// classify decides an example's outcome against its golden:
//   - "fail"  — did not run cleanly;
//   - "smoke" — ran OK but emitted no hash (run-only check);
//   - "new"   — emitted a hash with no committed golden yet;
//   - "drift" — emitted a hash that differs from the golden;
//   - "ok"    — emitted a hash matching the golden.
func classify(res Result, golden uint64, hasGolden bool) string {
	switch {
	case !res.OK:
		return "fail"
	case !res.HasHash:
		return "smoke"
	case !hasGolden:
		return "new"
	case res.Hash != golden:
		return "drift"
	default:
		return "ok"
	}
}

// extractHash returns the unsigned integer following the first "hash=" token.
func extractHash(s string) (uint64, bool) {
	_, after, found := strings.Cut(s, "hash=")
	if !found {
		return 0, false
	}
	end := 0
	for end < len(after) && after[end] >= '0' && after[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0, false
	}
	n, err := strconv.ParseUint(after[:end], 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}
