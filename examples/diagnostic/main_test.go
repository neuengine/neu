package main

import "testing"

// TestDiagnosticHashStable verifies the diagnostics reader-gate, averages, and
// gizmo geometry are deterministic across ≥20 runs (T-6T06, C29 P6 gate).
func TestDiagnosticHashStable(t *testing.T) {
	t.Parallel()
	first, err := run()
	if err != nil {
		t.Fatalf("run(): %v", err)
	}
	for i := range 20 {
		got, err := run()
		if err != nil {
			t.Fatalf("run #%d: %v", i+1, err)
		}
		if got != first {
			t.Errorf("run #%d: hash = %d, want %d", i+1, got, first)
		}
	}
}
