package render

import (
	"slices"
	"testing"

	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/render/postprocess"
)

// ─── Render-world isolation (INV-4) ──────────────────────────────────────────
//
// These tests verify that mutations applied in the main world after the Extract
// phase do not affect the in-flight render-world snapshot (l2-post-processing-go
// INV-4: settings read during Extract into render-world structs).

// testSettings is a main-world resource that holds a PostProcessStack.
type testSettings struct {
	Stack postprocess.PostProcessStack
}

// TestRenderWorldIsolation_PostProcessStack verifies that mutating the main
// world's PostProcessStack after RunFrame does not alter the extracted
// render-world snapshot. PostProcessStack is a value type; its copy on Extract
// is completely independent.
func TestRenderWorldIsolation_PostProcessStack(t *testing.T) {
	t.Parallel()
	main := world.NewWorld()

	// Main world settings: Bloom + Tonemapping enabled.
	var stack postprocess.PostProcessStack
	stack.Enable(postprocess.SlotBloom).Enable(postprocess.SlotTonemapping)
	world.SetResource(main, testSettings{Stack: stack})

	sub := NewRenderSubApp(nopBackend{})
	sub.RegisterExtract(func(m, r *world.World) {
		src, ok := world.Resource[testSettings](m)
		if !ok {
			return
		}
		// Value-copy: the render world owns an independent copy (INV-4).
		world.SetResource(r, testSettings{Stack: src.Stack})
	})

	if err := sub.RunFrame(main); err != nil {
		t.Fatalf("RunFrame: %v", err)
	}

	// Mutate main world AFTER extract.
	src, _ := world.Resource[testSettings](main)
	src.Stack.Enable(postprocess.SlotSpatialAA) // add a new effect
	src.Stack.Disable(postprocess.SlotBloom)    // remove an existing effect

	// The render world's snapshot must not reflect those mutations.
	snap, ok := world.Resource[testSettings](sub.RenderWorld())
	if !ok {
		t.Fatal("render world missing extracted snapshot")
	}
	if snap.Stack.IsEnabled(postprocess.SlotSpatialAA) {
		t.Error("INV-4 violated: SpatialAA appeared in render-world snapshot after main-world mutation")
	}
	if !snap.Stack.IsEnabled(postprocess.SlotBloom) {
		t.Error("INV-4 violated: Bloom was removed from render-world snapshot by main-world mutation")
	}
}

// TestRenderWorldIsolation_SliceResource extends the existing TestExtractIsolation
// by explicitly testing the slice-backing-array isolation: a resource with a
// []float32 field must not share its backing array with the render world.
func TestRenderWorldIsolation_SliceResource(t *testing.T) {
	t.Parallel()
	main := world.NewWorld()
	world.SetResource(main, sceneData{Positions: []float32{1, 2, 3}})

	sub := NewRenderSubApp(nopBackend{})
	sub.RegisterExtract(func(m, r *world.World) {
		src, _ := world.Resource[sceneData](m)
		world.SetResource(r, renderSnap{Positions: slices.Clone(src.Positions)})
	})

	if err := sub.RunFrame(main); err != nil {
		t.Fatalf("RunFrame: %v", err)
	}

	// Mutate main world.
	src, _ := world.Resource[sceneData](main)
	src.Positions[0] = 999

	snap, ok := world.Resource[renderSnap](sub.RenderWorld())
	if !ok {
		t.Fatal("render world missing snapshot")
	}
	if snap.Positions[0] != 1 {
		t.Errorf("isolation broken: snapshot[0] = %v, want 1", snap.Positions[0])
	}
}

// TestRenderWorldIsolation_MultiFrame verifies that isolation holds across
// multiple consecutive frames — each frame's extract is independent.
func TestRenderWorldIsolation_MultiFrame(t *testing.T) {
	t.Parallel()
	main := world.NewWorld()

	type frameTag struct{ N int }
	world.SetResource(main, frameTag{N: 1})

	sub := NewRenderSubApp(nopBackend{})
	sub.RegisterExtract(func(m, r *world.World) {
		src, _ := world.Resource[frameTag](m)
		world.SetResource(r, frameTag{N: src.N})
	})

	for frame := range 3 {
		// Advance main-world frame counter before each extract.
		world.SetResource(main, frameTag{N: frame + 1})
		if err := sub.RunFrame(main); err != nil {
			t.Fatalf("frame %d RunFrame: %v", frame, err)
		}
		// After each RunFrame the render world holds the snapshot from THIS frame.
		snap, _ := world.Resource[frameTag](sub.RenderWorld())
		if snap.N != frame+1 {
			t.Errorf("frame %d: render snapshot N = %d, want %d", frame, snap.N, frame+1)
		}
	}
}
