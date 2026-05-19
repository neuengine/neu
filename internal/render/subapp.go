package render

import (
	"errors"

	"github.com/neuengine/neu/internal/ecs/world"
	gpu "github.com/neuengine/neu/pkg/render"
)

// ErrExtractReentry is returned by [RenderSubApp.RunFrame] if the extract
// stage is entered twice within one frame — a hard INV-4 violation
// (l1-render-core: "Extract runs exactly once per frame, before any pass").
var ErrExtractReentry = errors.New("render: extract ran more than once this frame")

// RenderSubApp owns an isolated render World and drives the four-stage
// per-frame schedule (Collect → Extract → Prepare → Draw, l1-render-core
// §4.9). The main App calls [RenderSubApp.RunFrame] after its own Update.
// Extract copies main-world data into the render world exactly once per
// frame and strictly before any render pass executes (INV-4).
//
// Bootstrap 0.1.0: Prepare is a drain-only placeholder (feature-driven
// resource prep lands in T-4A04); App-plugin wiring is a later integration
// (this type is internal/render-scoped per spec §Go Package).
type RenderSubApp struct {
	backend  gpu.RenderBackend
	server   *Server
	tracker  *ResourceTracker
	render   *world.World
	graph    *RenderGraph
	extracts extractRegistry

	features []RenderFeature
	views    []*RenderView
	data     *RenderDataHolder

	frame      uint64
	bound      bool
	extractRun bool    // per-frame INV-4 guard
	trace      []Stage // stages entered this frame (diagnostics/tests)
	passStage  Stage   // stage during which graph passes executed
}

// NewRenderSubApp returns a RenderSubApp fronting backend with a fresh
// render World, server, and resource tracker.
func NewRenderSubApp(backend gpu.RenderBackend) *RenderSubApp {
	return &RenderSubApp{
		backend: backend,
		server:  NewServer(backend),
		tracker: NewResourceTracker(backend),
		render:  world.NewWorld(),
		graph:   &RenderGraph{},
		data:    NewRenderDataHolder(),
	}
}

// RegisterFeature adds a render feature and calls its Initialize hook. The
// feature receives per-stage callbacks during RunFrame (l1-render-core §4.10).
func (s *RenderSubApp) RegisterFeature(f RenderFeature) {
	if f == nil {
		return
	}
	f.Initialize(s)
	s.features = append(s.features, f)
}

// AddView registers a RenderView; features' Draw is dispatched once per view
// each frame. Views are normally created by the camera system (T-4C).
func (s *RenderSubApp) AddView(v *RenderView) {
	if v != nil {
		s.views = append(s.views, v)
	}
}

// Data returns the shared struct-of-arrays render-object store.
func (s *RenderSubApp) Data() *RenderDataHolder { return s.data }

// RegisterExtract appends fn to the per-frame extract set (run in
// registration order during StageExtract). nil is ignored.
func (s *RenderSubApp) RegisterExtract(fn ExtractFn) { s.extracts.add(fn) }

// Graph returns the render graph; callers add passes before the first
// RunFrame (or whenever passes change — RunFrame rebuilds as needed).
func (s *RenderSubApp) Graph() *RenderGraph { return s.graph }

// RenderWorld returns the isolated render-side World (the extract target).
func (s *RenderSubApp) RenderWorld() *world.World { return s.render }

// Server returns the command-queue server (T-4A01).
func (s *RenderSubApp) Server() *Server { return s.server }

// Tracker returns the resource tracker (T-4A01).
func (s *RenderSubApp) Tracker() *ResourceTracker { return s.tracker }

// Frame returns the current frame counter (advanced at StageCollect).
func (s *RenderSubApp) Frame() uint64 { return s.frame }

// Trace returns the stages entered during the most recent RunFrame, in
// order. Diagnostics / tests only.
func (s *RenderSubApp) Trace() []Stage { return append([]Stage(nil), s.trace...) }

// PassStage returns the stage in which render passes executed last frame
// (StageDraw on a well-formed frame). Tests assert INV: passes run only
// in StageDraw.
func (s *RenderSubApp) PassStage() Stage { return s.passStage }

// RunFrame executes one render frame against the main World: Collect →
// Extract (once, INV-4) → Prepare → Draw. Must be called from the render
// goroutine (it binds the server's inline fast-path on first call).
// Returns ErrExtractReentry if the INV-4 guard trips, or any graph build
// error.
func (s *RenderSubApp) RunFrame(main *world.World) error {
	// --- StageCollect ---
	if !s.bound {
		s.server.Bind() // this goroutine owns the queue (l1-render-core §4.5)
		s.bound = true
	}
	s.frame++
	s.extractRun = false
	s.trace = s.trace[:0]
	s.trace = append(s.trace, StageCollect)
	for _, f := range s.features {
		f.Collect(&CollectContext{Frame: s.frame, Render: s.render})
	}

	// --- StageExtract (exactly once, before any pass) ---
	s.trace = append(s.trace, StageExtract)
	if s.extractRun {
		return ErrExtractReentry
	}
	s.extractRun = true
	s.extracts.run(main, s.render)
	for _, f := range s.features {
		f.Extract(&ExtractContext{Frame: s.frame, Main: main, Render: s.render, Data: s.data})
	}
	s.server.Drain() // flush any extract-queued resource init

	// --- StagePrepare ---
	s.trace = append(s.trace, StagePrepare)
	for _, f := range s.features {
		f.PrepareEffectPermutations(&PrepareContext{Frame: s.frame, Render: s.render, Data: s.data, Server: s.server})
	}
	for _, f := range s.features {
		f.Prepare(&PrepareContext{Frame: s.frame, Render: s.render, Data: s.data, Server: s.server})
	}
	s.server.Drain()

	// --- StageDraw (passes execute ONLY here) ---
	s.trace = append(s.trace, StageDraw)
	if err := s.graph.Build(s.tracker); err != nil {
		return err
	}
	s.passStage = StageDraw
	if err := s.graph.Execute(&PassContext{Frame: s.frame, Backend: s.backend}); err != nil {
		return err
	}
	for _, v := range s.views {
		for _, f := range s.features {
			f.Draw(&DrawContext{Frame: s.frame, Backend: s.backend, Server: s.server, Graph: s.graph}, v)
		}
	}
	s.backend.Submit()
	s.backend.Present()
	for _, f := range s.features {
		f.Flush(&FlushContext{Frame: s.frame})
	}
	s.tracker.EndFrame(s.frame)
	s.server.Drain()
	return nil
}
