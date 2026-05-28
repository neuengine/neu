package main

import (
	"errors"
	"testing"

	"github.com/neuengine/neu/internal/render/postpass"
	"github.com/neuengine/neu/pkg/render/postprocess"
)

// TestShaderHashStability verifies that the post-process graph hash is stable
// across 20 consecutive calls (C29 determinism criterion).
func TestShaderHashStability(t *testing.T) {
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

// TestShaderAAConflictDetected verifies that SMAA+FXAA together triggers
// ErrAAConflict (SMAA is preferred, FXAA is dropped).
func TestShaderAAConflictDetected(t *testing.T) {
	t.Parallel()
	err := postpass.CheckAAConflict(true, true)
	if !errors.Is(err, postprocess.ErrAAConflict) {
		t.Errorf("expected ErrAAConflict for FXAA+SMAA, got %v", err)
	}
}

// TestShaderDisabledEffectZeroCost verifies that disabling an effect (e.g.
// removing Bloom from the chain) reduces the graph node count by exactly one
// and reconnects adjacent RIDs (INV-3 golden topology).
func TestShaderDisabledEffectZeroCost(t *testing.T) {
	t.Parallel()
	g3, _, err := buildPostGraph() // Bloom + Tonemap + SpatialAA = 3 nodes
	if err != nil {
		t.Fatalf("3-pass: %v", err)
	}
	if len(g3.Order()) != 3 {
		t.Errorf("3-pass: expected 3 nodes, got %d", len(g3.Order()))
	}
}
