package main

import "testing"

// TestAudioHashStability verifies that bus-graph routing + spatial + DESPAWN
// produce a deterministic hash across ≥20 runs (C29 P5 gate, T-5T01).
func TestAudioHashStability(t *testing.T) {
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

// TestAudioBusGraph verifies the bus DAG has no cycles and correct routing.
func TestAudioBusGraph(t *testing.T) {
	t.Parallel()
	_, err := run()
	if err != nil {
		t.Fatalf("audio example failed: %v", err)
	}
}
