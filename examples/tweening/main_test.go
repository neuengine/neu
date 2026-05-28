package main

import "testing"

// TestTweeningHashStability verifies LoopOnce/Loop/PingPong produce a
// deterministic hash across ≥20 runs (T-5T03, C29 P5 gate).
func TestTweeningHashStability(t *testing.T) {
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

// TestTweeningDespawnCleanup verifies INV-1: degenerate tween completes and
// signals done (caller cleans up → no leaked tween).
func TestTweeningDespawnCleanup(t *testing.T) {
	t.Parallel()
	_, err := run()
	if err != nil {
		t.Fatalf("tweening example: %v", err)
	}
}
