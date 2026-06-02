package window

import (
	"log/slog"

	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/event"
	"github.com/neuengine/neu/internal/ecs/query"
	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app"
	"github.com/neuengine/neu/pkg/app/appface"
	pkgwindow "github.com/neuengine/neu/pkg/window"
)

// windowRuntime is the resource backing the window-sync system: the platform
// backend, the last-applied snapshot per window (for field diffing), primary-
// window tracking, and the exit policy.
type windowRuntime struct {
	backend        pkgwindow.WindowBackend
	applied        map[entity.Entity]pkgwindow.Window
	primary        PrimaryWindowRes
	exit           pkgwindow.ExitCondition
	closeOnRequest bool
}

// SyncPlugin wires the window subsystem into the App: it installs a backend
// (headless by default), registers the Window components and the PlatformEvent
// bus, optionally spawns the primary window, and adds the PreUpdate system that
// mirrors ECS window state to the backend, drains platform events onto the bus,
// and turns a qualifying close into an AppExit.
//
// It is intentionally not part of DefaultPlugins — windowing is opt-in so
// headless builds and CI stay window-free. Add it explicitly:
//
//	app.AddPlugin(window.SyncPlugin{Config: pkgwindow.WindowPlugin{PrimaryWindow: &win}})
type SyncPlugin struct {
	// Backend is the platform windowing implementation. A nil backend installs
	// a headless one (no OS windows), keeping CI deterministic.
	Backend pkgwindow.WindowBackend
	// Config carries the primary-window descriptor and the exit policy.
	Config pkgwindow.WindowPlugin
}

// Build implements appface.Plugin.
func (p SyncPlugin) Build(b appface.Builder) {
	w := b.World()

	world.RegisterComponent[pkgwindow.Window](w)
	world.RegisterComponent[pkgwindow.PrimaryWindow](w)
	event.RegisterEvent[pkgwindow.PlatformEvent](w)

	backend := p.Backend
	if backend == nil {
		backend = NewHeadlessWindowBackend()
	}
	rt := windowRuntime{
		backend:        backend,
		applied:        make(map[entity.Entity]pkgwindow.Window),
		exit:           p.Config.ExitCondition,
		closeOnRequest: p.Config.CloseWhenRequested,
	}
	if p.Config.PrimaryWindow != nil {
		e := w.Spawn(
			component.Data{Value: *p.Config.PrimaryWindow},
			component.Data{Value: pkgwindow.PrimaryWindow{}},
		)
		rt.primary.SetPrimary(e)
	}
	world.SetResource(w, rt)

	// Reuse one query across frames — its matched-archetype list grows lazily as
	// windows spawn, so the per-frame path stays allocation-free.
	q, _ := query.NewQuery1[pkgwindow.Window](w)
	b.AddSystem(appface.PreUpdate, scheduler.NewFuncSystem("window.Sync", func(w *world.World) {
		rt, ok := world.Resource[windowRuntime](w)
		if !ok {
			return
		}
		syncWindows(w, rt, q)
		pollWindowEvents(w, rt, q)
	}))
}

// syncWindows mirrors ECS window state to the backend: it creates a backend
// window for each new Window entity, applies field diffs for changed ones
// (DiffWindow yields an empty diff — and thus no ApplyChanges — when nothing
// changed, INV-4), and destroys backend windows whose entity was despawned.
func syncWindows(w *world.World, rt *windowRuntime, q *query.Query1[pkgwindow.Window]) {
	for e, win := range q.All(w) {
		prev, known := rt.applied[e]
		if !known {
			_, err := rt.backend.CreateWindow(e, pkgwindow.DescriptorFromWindow(*win))
			logBackendErr("create", e, err)
			rt.applied[e] = *win
			continue
		}
		if diff := DiffWindow(prev, *win); diff.HasChanges() {
			logBackendErr("apply", e, rt.backend.ApplyChanges(e, diff))
		}
		rt.applied[e] = *win
	}
	// Tear down backend windows whose entity no longer exists (despawned).
	for e := range rt.applied {
		if w.Contains(e) {
			continue
		}
		logBackendErr("destroy", e, rt.backend.DestroyWindow(e))
		delete(rt.applied, e)
		if rt.primary.IsPrimary(e) {
			rt.primary.Clear()
		}
	}
}

// pollWindowEvents drains this frame's platform events, mirrors each onto the
// PlatformEvent bus for downstream systems, folds event-driven state (focus,
// size, position, scale) back into the Window component, and converts a
// qualifying close into an AppExit.
func pollWindowEvents(w *world.World, rt *windowRuntime, q *query.Query1[pkgwindow.Window]) {
	events := rt.backend.PollEvents()
	if len(events) == 0 {
		return
	}
	bus := event.Bus[pkgwindow.PlatformEvent](w)
	for _, ev := range events {
		if bus != nil {
			bus.Send(ev)
		}
		foldEvent(w, rt, ev)
		if ev.Kind == pkgwindow.EventCloseRequested || ev.Kind == pkgwindow.EventClosed {
			handleClose(w, rt, q, ev)
		}
	}
}

// foldEvent applies an OS-driven event payload to its Window component and to
// the applied snapshot, so the resulting state is not diffed back into a
// redundant ApplyChanges next frame (the platform already reflects it). Events
// without a Window field to update (cursor motion, lifecycle) are ignored here.
func foldEvent(w *world.World, rt *windowRuntime, ev pkgwindow.PlatformEvent) {
	win, ok := world.Get[pkgwindow.Window](w, ev.Window)
	if !ok {
		return
	}
	switch ev.Kind {
	case pkgwindow.EventResized:
		win.Resolution.PhysicalWidth = ev.Width
		win.Resolution.PhysicalHeight = ev.Height
	case pkgwindow.EventMoved:
		win.Position = pkgwindow.WindowPosition{Mode: pkgwindow.PositionAt, X: ev.X, Y: ev.Y}
	case pkgwindow.EventFocused:
		win.Focused = ev.Focused
	case pkgwindow.EventScaleFactorChanged:
		win.Resolution.ScaleFactorOverride = ev.ScaleFactor
	default:
		return
	}
	if _, known := rt.applied[ev.Window]; known {
		rt.applied[ev.Window] = *win
	}
}

// handleClose evaluates the exit policy for a close event and asks the App to
// exit when it triggers (via app.RequestExit, which is a no-op off an App loop).
// CloseWhenRequested despawns the window on a close request; an actual close
// always despawns. Despawned windows are torn down at the backend by the next
// syncWindows pass.
func handleClose(w *world.World, rt *windowRuntime, q *query.Query1[pkgwindow.Window], ev pkgwindow.PlatformEvent) {
	primary := rt.primary.IsPrimary(ev.Window)
	remaining := max(q.Count(w)-1, 0)
	if pkgwindow.CausesAppExit(ev.Kind, rt.exit, primary, remaining) {
		app.RequestExit(w, 0)
	}
	if ev.Kind == pkgwindow.EventClosed || (ev.Kind == pkgwindow.EventCloseRequested && rt.closeOnRequest) {
		_ = w.Despawn(ev.Window) // already-dead is benign; the destroy pass cleans up
	}
}

// logBackendErr records a non-nil backend error without failing the frame — a
// windowing failure is operational, not a reason to halt the loop.
func logBackendErr(op string, e entity.Entity, err error) {
	if err != nil {
		slog.Error("window: backend "+op+" failed", "entity", e.ID(), "error", err)
	}
}
