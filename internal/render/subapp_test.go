package render

import (
	"slices"
	"testing"

	"github.com/neuengine/neu/internal/ecs/world"
	gpu "github.com/neuengine/neu/pkg/render"
)

type sceneData struct{ Positions []float32 }  // main-world source
type renderSnap struct{ Positions []float32 } // render-world copy

// TestExtractIsolation: an ExtractFn deep-copies main-world data into the
// render world; mutating the main source AFTER RunFrame must not change the
// render snapshot (l1-render-core INV-4 / §2 "no shared mutable state").
func TestExtractIsolation(t *testing.T) {
	main := world.NewWorld()
	world.SetResource(main, sceneData{Positions: []float32{1, 2, 3}})

	sub := NewRenderSubApp(nopBackend{})
	sub.RegisterExtract(func(m, r *world.World) {
		src, ok := world.Resource[sceneData](m)
		if !ok {
			t.Error("extract: main sceneData missing")
			return
		}
		// Deep copy — never share the backing array with the main world.
		world.SetResource(r, renderSnap{Positions: slices.Clone(src.Positions)})
	})

	if err := sub.RunFrame(main); err != nil {
		t.Fatalf("RunFrame: %v", err)
	}

	// Mutate the main source after the frame's snapshot was taken.
	src, _ := world.Resource[sceneData](main)
	src.Positions[0] = 999
	src.Positions = append(src.Positions, 4)

	snap, ok := world.Resource[renderSnap](sub.RenderWorld())
	if !ok {
		t.Fatal("render world missing snapshot after extract")
	}
	want := []float32{1, 2, 3}
	if !slices.Equal(snap.Positions, want) {
		t.Fatalf("snapshot mutated through shared state: got %v, want %v (isolation broken)",
			snap.Positions, want)
	}
}

// TestFrameGuard: Extract runs exactly once per frame, stages are ordered
// Collect→Extract→Prepare→Draw, and render passes execute only in StageDraw
// (l1-render-core INV-4, §4.9).
func TestFrameGuard(t *testing.T) {
	sub := NewRenderSubApp(nopBackend{})

	var extractCount int
	sub.RegisterExtract(func(_, _ *world.World) { extractCount++ })

	var rec []string
	sub.Graph().AddPass(&fakePass{name: "P", recorder: &rec})

	const frames = 3
	for f := 1; f <= frames; f++ {
		if err := sub.RunFrame(world.NewWorld()); err != nil {
			t.Fatalf("frame %d RunFrame: %v", f, err)
		}
		// Exactly one extract per frame.
		if extractCount != f {
			t.Fatalf("frame %d: extractCount = %d, want %d (INV-4)", f, extractCount, f)
		}
		// Stage order.
		tr := sub.Trace()
		want := []Stage{StageCollect, StageExtract, StagePrepare, StageDraw}
		if !slices.Equal(tr, want) {
			t.Fatalf("frame %d: trace = %v, want %v", f, tr, want)
		}
		ie := slices.Index(tr, StageExtract)
		ip := slices.Index(tr, StagePrepare)
		id := slices.Index(tr, StageDraw)
		if ie >= ip || ip >= id {
			t.Fatalf("frame %d: bad order Extract@%d Prepare@%d Draw@%d", f, ie, ip, id)
		}
		// Passes executed, and only in StageDraw.
		if sub.PassStage() != StageDraw {
			t.Fatalf("frame %d: passStage = %v, want Draw", f, sub.PassStage())
		}
	}
	if len(rec) != frames || rec[0] != "P" {
		t.Fatalf("pass executed %d times, want %d (once/frame in Draw)", len(rec), frames)
	}
	if sub.Frame() != frames {
		t.Fatalf("Frame() = %d, want %d", sub.Frame(), frames)
	}
}

// TestRunFrame_GraphCycleSurfaces: a cyclic render graph fails the frame at
// StageDraw (ErrRenderGraphCycle propagates out of RunFrame).
func TestRunFrame_GraphCycleSurfaces(t *testing.T) {
	sub := NewRenderSubApp(nopBackend{})
	g := sub.Graph()
	g.AddPass(&fakePass{name: "A", in: []gpu.RID{tex(1)}, out: []gpu.RID{tex(2)}})
	g.AddPass(&fakePass{name: "B", in: []gpu.RID{tex(2)}, out: []gpu.RID{tex(1)}})
	if err := sub.RunFrame(world.NewWorld()); err == nil {
		t.Fatal("cyclic graph did not surface an error from RunFrame")
	}
}
