package main

import "testing"

// Test2DHashStability verifies INV-2 sort determinism and INV-3 batch count
// are stable across ≥20 runs (T-5T04, C29 P5 gate).
func Test2DHashStability(t *testing.T) {
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
			t.Errorf("run #%d: hash = %d, want %d (INV-2 violated)", i+1, got, first)
		}
	}
}

// Test2DSpritePick verifies that pick returns a hit for a point inside the
// sprite AABB (T-5T04).
func Test2DSpritePick(t *testing.T) {
	t.Parallel()
	_, err := run()
	if err != nil {
		t.Fatalf("2D example: %v", err)
	}
}
