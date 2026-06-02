package diag

import (
	"github.com/neuengine/neu/internal/ecs/gametime"
	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
	pkgdiag "github.com/neuengine/neu/pkg/diag"
)

// Built-in diagnostic paths the DiagnosticsPlugin collects each frame.
const (
	PathFPS         pkgdiag.DiagnosticPath = "engine/fps"
	PathFrameTimeMS pkgdiag.DiagnosticPath = "engine/frame_time_ms"
	PathEntityCount pkgdiag.DiagnosticPath = "engine/entity_count"
)

// DiagnosticsPlugin wires the DiagnosticsStore + the engine's built-in metrics
// (FPS, frame time, live entity count) into the App: it registers the metrics at
// Build and adds a collection system in the Last schedule, after gametime's
// First-schedule RealTime update. Collection stays zero-cost until a consumer
// registers a reader (INV-1) — e.g. the debug overlay (deferred to the UI wiring)
// or a test.
type DiagnosticsPlugin struct {
	// Store, when non-nil, is shared instead of creating a new one — letting the
	// app or a test own the store and register readers.
	Store *pkgdiag.DiagnosticsStore
}

// Build implements appface.Plugin.
func (p DiagnosticsPlugin) Build(app appface.Builder) {
	store := p.Store
	if store == nil {
		store = pkgdiag.NewDiagnosticsStore()
	}
	store.Register(pkgdiag.NewDiagnostic(PathFPS, "fps", 0))
	store.Register(pkgdiag.NewDiagnostic(PathFrameTimeMS, "ms", 0))
	store.Register(pkgdiag.NewDiagnostic(PathEntityCount, "count", 0))
	app.SetResource(store)

	app.AddSystem(appface.Last, scheduler.NewFuncSystem("diag.BuiltinMetrics", func(w *world.World) {
		collectBuiltins(w, store)
	}))
}

// collectBuiltins gathers this frame's built-in metrics from the World and pushes
// them through [recordFrame].
func collectBuiltins(w *world.World, store *pkgdiag.DiagnosticsStore) {
	recordFrame(store, frameDeltaSeconds(w), liveEntityCount(w))
}

// frameDeltaSeconds returns the wall-clock seconds since the previous frame, or 0
// when no RealTime resource is present (gametime not installed / first frame).
func frameDeltaSeconds(w *world.World) float64 {
	if rt, ok := world.Resource[gametime.RealTime](w); ok {
		return rt.DeltaSeconds()
	}
	return 0
}

// liveEntityCount sums the row counts of every archetype.
func liveEntityCount(w *world.World) int {
	var n int
	w.Archetypes().EachFrom(0, func(a *world.Archetype) bool {
		n += a.Len()
		return true
	})
	return n
}

// recordFrame pushes the built-in metrics for one frame. It honors the INV-1
// zero-cost gate (no reader ⇒ no work) and skips FPS on a zero delta (first frame
// or paused) to avoid a divide-by-zero.
func recordFrame(store *pkgdiag.DiagnosticsStore, dtSeconds float64, entityCount int) {
	if !store.HasAnyReader() {
		return
	}
	store.Push(PathFrameTimeMS, dtSeconds*1000)
	if dtSeconds > 0 {
		store.Push(PathFPS, 1.0/dtSeconds)
	}
	store.Push(PathEntityCount, float64(entityCount))
}
