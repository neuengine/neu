package render

import (
	"slices"
	"testing"

	"github.com/neuengine/neu/internal/ecs/world"
)

// recordingFeature logs which stage hooks fired, in order.
type recordingFeature struct {
	log       *[]string
	initedSub *RenderSubApp
}

func (f *recordingFeature) Initialize(s *RenderSubApp) {
	f.initedSub = s
	*f.log = append(*f.log, "Init")
}
func (f *recordingFeature) Collect(*CollectContext) { *f.log = append(*f.log, "Collect") }
func (f *recordingFeature) Extract(*ExtractContext) { *f.log = append(*f.log, "Extract") }
func (f *recordingFeature) PrepareEffectPermutations(*PrepareContext) {
	*f.log = append(*f.log, "PrepEff")
}
func (f *recordingFeature) Prepare(*PrepareContext)        { *f.log = append(*f.log, "Prepare") }
func (f *recordingFeature) Draw(*DrawContext, *RenderView) { *f.log = append(*f.log, "Draw") }
func (f *recordingFeature) Flush(*FlushContext)            { *f.log = append(*f.log, "Flush") }

// TestFeatureDispatch: RegisterFeature calls Initialize; RunFrame dispatches
// every stage hook in pipeline order; Draw fires once per registered view.
func TestFeatureDispatch(t *testing.T) {
	sub := NewRenderSubApp(nopBackend{})
	var log []string
	feat := &recordingFeature{log: &log}
	sub.RegisterFeature(feat)

	if feat.initedSub != sub {
		t.Fatal("RegisterFeature did not call Initialize with the subapp")
	}
	sub.AddView(&RenderView{}) // one view → one Draw

	if err := sub.RunFrame(world.NewWorld()); err != nil {
		t.Fatalf("RunFrame: %v", err)
	}

	want := []string{"Init", "Collect", "Extract", "PrepEff", "Prepare", "Draw", "Flush"}
	if !slices.Equal(log, want) {
		t.Fatalf("dispatch order = %v, want %v", log, want)
	}
}

// TestFeatureDrawPerView: Draw is invoked once per registered view per frame.
func TestFeatureDrawPerView(t *testing.T) {
	sub := NewRenderSubApp(nopBackend{})
	var log []string
	sub.RegisterFeature(&recordingFeature{log: &log})
	sub.AddView(&RenderView{})
	sub.AddView(&RenderView{})

	if err := sub.RunFrame(world.NewWorld()); err != nil {
		t.Fatalf("RunFrame: %v", err)
	}
	draws := 0
	for _, s := range log {
		if s == "Draw" {
			draws++
		}
	}
	if draws != 2 {
		t.Fatalf("Draw fired %d times, want 2 (one per view)", draws)
	}
}
