package main

import "testing"

// TestUILayoutHashStable verifies the flexbox solver is deterministic: the
// computed LayoutRect hash is identical across ≥20 runs (T-6T06, C29 P6 gate).
func TestUILayoutHashStable(t *testing.T) {
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
			t.Errorf("run #%d: hash = %d, want %d (non-deterministic layout)", i+1, got, first)
		}
	}
}
