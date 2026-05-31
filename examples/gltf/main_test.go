package main

import "testing"

// TestGltfFanOutHashStability verifies the glTF fan-out is deterministic and the
// GltfAssetLabel addressing is stable across ≥20 reloads (INV-4, C29 golden gate).
func TestGltfFanOutHashStability(t *testing.T) {
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
			t.Errorf("run #%d: hash = %d, want %d (INV-4 unstable)", i+1, got, first)
		}
	}
}
