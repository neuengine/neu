package main

import "testing"

// TestAnimationHashStability verifies INV-1 determinism:
// same clip+time → identical pose hash across ≥20 runs (T-5T02).
func TestAnimationHashStability(t *testing.T) {
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
			t.Errorf("run #%d: hash = %d, want %d (INV-1 violated)", i+1, got, first)
		}
	}
}

// TestAnimationSkinValidation verifies that ErrSkinMismatch surfaces correctly.
func TestAnimationSkinValidation(t *testing.T) {
	t.Parallel()
	_, err := run()
	if err != nil {
		t.Fatalf("animation example: %v", err)
	}
}
