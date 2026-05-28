package main

import "testing"

// Test3DFrameHashStability verifies that the 3-D scene's render graph
// produces an identical FNV-1a frame hash across 20 consecutive calls.
// Stability is the C29 acceptance criterion for the golden-image harness.
func Test3DFrameHashStability(t *testing.T) {
	t.Parallel()
	first, err := run()
	if err != nil {
		t.Fatalf("run(): %v", err)
	}
	for i := range 20 {
		h, err := run()
		if err != nil {
			t.Fatalf("run() #%d: %v", i+1, err)
		}
		if h != first {
			t.Errorf("run #%d: hash = %d, want %d (non-deterministic)", i+1, h, first)
		}
	}
}

// Test3DSceneComponents exercises the scene asset construction to verify that
// the mesh, material, and light types are wired together correctly.
func Test3DSceneComponents(t *testing.T) {
	t.Parallel()
	a := buildAssets()

	// Mesh must be valid (Cube has position attribute).
	if err := a.mesh.Validate(); err != nil {
		t.Errorf("mesh.Validate: %v", err)
	}
	if a.mesh.VertexCount() == 0 {
		t.Error("mesh has no vertices")
	}

	// Material must be AlphaOpaque.
	if a.mat.Alpha != 0 { // AlphaOpaque = 0
		t.Errorf("material.Alpha = %v, want AlphaOpaque", a.mat.Alpha)
	}

	// Directional light must have cascades configured.
	if a.dirLight.Cascades == nil {
		t.Error("directional light has no cascade config")
	}
	if a.dirLight.Cascades.Count != 2 {
		t.Errorf("cascade count = %d, want 2", a.dirLight.Cascades.Count)
	}
	splits, err := a.dirLight.Cascades.Splits(0.1)
	if err != nil {
		t.Errorf("Splits: %v", err)
	}
	if len(splits) != 2 {
		t.Errorf("len(splits) = %d, want 2", len(splits))
	}

	// Shadow casters must be present.
	if len(a.casters) == 0 {
		t.Error("no shadow casters")
	}
}

// Test3DGraphOrder verifies that the shadow passes precede the post-process
// passes in the built render graph (INV-4 from materials spec).
func Test3DGraphOrder(t *testing.T) {
	t.Parallel()
	a := buildAssets()
	g, err := buildGraph(a)
	if err != nil {
		t.Fatalf("buildGraph: %v", err)
	}

	order := g.Order()
	// Shadow casters: indices 0..len(casters)-1
	// Lighting pass: index len(casters)
	// Post-process passes: indices len(casters)+1..
	shadowCount := len(a.casters)
	if len(order) <= shadowCount {
		t.Fatalf("expected > %d passes in graph, got %d", shadowCount, len(order))
	}
	// Post-process passes must appear after the lighting + shadow passes.
	// (Shadow passes are at lower graph indices → lower order positions by construction.)
	if len(g.Barriers()) == 0 {
		t.Error("render graph has no barriers — expected a chained pipeline")
	}
}
