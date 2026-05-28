package main

import "testing"

// TestCameraHashStability verifies that multi-camera ordering produces an
// identical hash across 20 consecutive calls (C29 determinism criterion).
func TestCameraHashStability(t *testing.T) {
	t.Parallel()
	first, err := run()
	if err != nil {
		t.Fatalf("run(): %v", err)
	}
	for i := range 20 {
		h, err := run()
		if err != nil {
			t.Fatalf("run #%d: %v", i+1, err)
		}
		if h != first {
			t.Errorf("run #%d: hash = %d, want %d", i+1, h, first)
		}
	}
}

// TestCameraWorldSetup verifies basic camera world construction and ordering.
func TestCameraWorldSetup(t *testing.T) {
	t.Parallel()
	_, err := run()
	if err != nil {
		t.Errorf("run: %v", err)
	}
}
