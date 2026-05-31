package main

import "testing"

// TestRunStable asserts the example is deterministic across >=20 runs — the
// invariant cmd/examplecheck relies on when comparing the printed hash to the
// committed golden.
func TestRunStable(t *testing.T) {
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
			t.Errorf("run #%d: hash = %d, want %d (non-deterministic)", i+1, got, first)
		}
	}
}
