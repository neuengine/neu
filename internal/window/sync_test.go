package window

import (
	"errors"
	"strings"
	"testing"

	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/event"
	"github.com/neuengine/neu/internal/ecs/query"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app"
	pkgwindow "github.com/neuengine/neu/pkg/window"
)

// setup builds a world wired like SyncPlugin.Build (components + PlatformEvent
// and AppExit buses registered, runtime resource set) and returns the runtime
// pointer, the headless backend, and a reusable Window query.
func setup(t *testing.T, cfg pkgwindow.WindowPlugin) (*world.World, *windowRuntime, *HeadlessWindowBackend, *query.Query1[pkgwindow.Window]) {
	t.Helper()
	w := world.NewWorld()
	world.RegisterComponent[pkgwindow.Window](w)
	world.RegisterComponent[pkgwindow.PrimaryWindow](w)
	event.RegisterEvent[pkgwindow.PlatformEvent](w)
	event.RegisterEvent[app.AppExit](w) // so app.RequestExit is observable in-test

	be := NewHeadlessWindowBackend()
	rt := windowRuntime{
		backend:        be,
		applied:        map[entity.Entity]pkgwindow.Window{},
		exit:           cfg.ExitCondition,
		closeOnRequest: cfg.CloseWhenRequested,
	}
	if cfg.PrimaryWindow != nil {
		e := w.Spawn(
			component.Data{Value: *cfg.PrimaryWindow},
			component.Data{Value: pkgwindow.PrimaryWindow{}},
		)
		rt.primary.SetPrimary(e)
	}
	world.SetResource(w, rt)
	rtp, _ := world.Resource[windowRuntime](w)
	q, _ := query.NewQuery1[pkgwindow.Window](w)
	return w, rtp, be, q
}

func exitPending(w *world.World) bool {
	bus := event.Bus[app.AppExit](w)
	return bus != nil && bus.Len() > 0
}

// ── create / apply / destroy ──────────────────────────────────────────────────

func TestSyncCreatesWindow(t *testing.T) {
	primary := pkgwindow.DefaultWindow("main")
	w, rt, be, q := setup(t, pkgwindow.WindowPlugin{PrimaryWindow: &primary})

	syncWindows(w, rt, q)

	if be.ActiveCount() != 1 {
		t.Errorf("backend active windows = %d, want 1", be.ActiveCount())
	}
	if len(rt.applied) != 1 {
		t.Errorf("applied snapshot size = %d, want 1", len(rt.applied))
	}
}

func TestSyncSkipsUnchangedWindow(t *testing.T) {
	primary := pkgwindow.DefaultWindow("main")
	w, rt, be, q := setup(t, pkgwindow.WindowPlugin{PrimaryWindow: &primary})

	syncWindows(w, rt, q) // create
	callsAfterCreate := len(be.Calls())
	syncWindows(w, rt, q) // nothing changed

	if len(be.Calls()) != callsAfterCreate {
		t.Errorf("unchanged window triggered backend calls (%v) — INV-4 violated", be.Calls())
	}
}

func TestSyncAppliesDiffOnChange(t *testing.T) {
	primary := pkgwindow.DefaultWindow("main")
	w, rt, be, q := setup(t, pkgwindow.WindowPlugin{PrimaryWindow: &primary})
	syncWindows(w, rt, q) // create

	win, _ := world.Get[pkgwindow.Window](w, rt.primary.Entity)
	win.Title = "renamed"
	syncWindows(w, rt, q)

	var applied bool
	for _, c := range be.Calls() {
		if strings.HasPrefix(c, "Apply:") {
			applied = true
		}
	}
	if !applied {
		t.Errorf("a changed window did not produce an Apply call; calls=%v", be.Calls())
	}
}

func TestSyncDestroysDespawnedWindow(t *testing.T) {
	primary := pkgwindow.DefaultWindow("main")
	w, rt, be, q := setup(t, pkgwindow.WindowPlugin{PrimaryWindow: &primary})
	syncWindows(w, rt, q) // create
	e := rt.primary.Entity

	if err := w.Despawn(e); err != nil {
		t.Fatalf("despawn: %v", err)
	}
	syncWindows(w, rt, q)

	if be.ActiveCount() != 0 {
		t.Errorf("backend still has %d windows after despawn", be.ActiveCount())
	}
	if len(rt.applied) != 0 {
		t.Errorf("applied snapshot not cleared: %d entries", len(rt.applied))
	}
	if rt.primary.Set {
		t.Error("primary not cleared after its window was despawned")
	}
}

// ── poll / emit / fold ────────────────────────────────────────────────────────

func TestPollEmitsAndFoldsResize(t *testing.T) {
	primary := pkgwindow.DefaultWindow("main")
	w, rt, be, q := setup(t, pkgwindow.WindowPlugin{PrimaryWindow: &primary})
	syncWindows(w, rt, q)
	e := rt.primary.Entity

	be.ScriptEvents(pkgwindow.PlatformEvent{Kind: pkgwindow.EventResized, Window: e, Width: 800, Height: 600})
	pollWindowEvents(w, rt, q)

	if got := event.Bus[pkgwindow.PlatformEvent](w).Len(); got != 1 {
		t.Errorf("PlatformEvent bus Len = %d, want 1 (event must be mirrored)", got)
	}
	win, _ := world.Get[pkgwindow.Window](w, e)
	if win.Resolution.PhysicalWidth != 800 || win.Resolution.PhysicalHeight != 600 {
		t.Errorf("resize not folded: %dx%d", win.Resolution.PhysicalWidth, win.Resolution.PhysicalHeight)
	}
	// The folded geometry must not be re-diffed into an ApplyChanges next frame.
	callsBefore := len(be.Calls())
	syncWindows(w, rt, q)
	if len(be.Calls()) != callsBefore {
		t.Error("folded resize was re-applied to the backend (snapshot not kept in lock-step)")
	}
}

func TestPollFoldsFocusMoveScale(t *testing.T) {
	primary := pkgwindow.DefaultWindow("main")
	w, rt, _, q := setup(t, pkgwindow.WindowPlugin{PrimaryWindow: &primary})
	syncWindows(w, rt, q)
	e := rt.primary.Entity
	be := rt.backend.(*HeadlessWindowBackend)

	be.ScriptEvents(
		pkgwindow.PlatformEvent{Kind: pkgwindow.EventFocused, Window: e, Focused: true},
		pkgwindow.PlatformEvent{Kind: pkgwindow.EventMoved, Window: e, X: 10, Y: 20},
		pkgwindow.PlatformEvent{Kind: pkgwindow.EventScaleFactorChanged, Window: e, ScaleFactor: 2},
	)
	pollWindowEvents(w, rt, q)

	win, _ := world.Get[pkgwindow.Window](w, e)
	if !win.Focused {
		t.Error("focus not folded")
	}
	if win.Position.Mode != pkgwindow.PositionAt || win.Position.X != 10 || win.Position.Y != 20 {
		t.Errorf("move not folded: %+v", win.Position)
	}
	if win.Resolution.ScaleFactorOverride != 2 {
		t.Errorf("scale not folded: %v", win.Resolution.ScaleFactorOverride)
	}
}

func TestPollIgnoresUnknownTargetAndCursor(t *testing.T) {
	primary := pkgwindow.DefaultWindow("main")
	w, rt, be, q := setup(t, pkgwindow.WindowPlugin{PrimaryWindow: &primary})
	syncWindows(w, rt, q)

	// A cursor event (no Window field to fold) and an event for a dead entity
	// must be mirrored but cause no panic / no state change.
	be.ScriptEvents(
		pkgwindow.PlatformEvent{Kind: pkgwindow.EventCursorMoved, Window: rt.primary.Entity, X: 5, Y: 5},
		pkgwindow.PlatformEvent{Kind: pkgwindow.EventResized, Window: entity.Entity{}, Width: 1, Height: 1},
	)
	pollWindowEvents(w, rt, q)

	if got := event.Bus[pkgwindow.PlatformEvent](w).Len(); got != 2 {
		t.Errorf("bus Len = %d, want 2", got)
	}
}

func TestPollNoEventsIsNoOp(t *testing.T) {
	primary := pkgwindow.DefaultWindow("main")
	w, rt, _, q := setup(t, pkgwindow.WindowPlugin{PrimaryWindow: &primary})
	syncWindows(w, rt, q)
	pollWindowEvents(w, rt, q) // empty queue
	if exitPending(w) {
		t.Error("no events must not request exit")
	}
}

// ── exit policies ─────────────────────────────────────────────────────────────

func TestExitOnPrimaryClose(t *testing.T) {
	primary := pkgwindow.DefaultWindow("main")
	w, rt, be, q := setup(t, pkgwindow.WindowPlugin{PrimaryWindow: &primary, ExitCondition: pkgwindow.OnPrimaryClosed})
	syncWindows(w, rt, q)

	be.ScriptEvents(pkgwindow.PlatformEvent{Kind: pkgwindow.EventCloseRequested, Window: rt.primary.Entity})
	pollWindowEvents(w, rt, q)

	if !exitPending(w) {
		t.Error("closing the primary window under OnPrimaryClosed must request exit")
	}
}

func TestNoExitOnNonPrimaryClose(t *testing.T) {
	primary := pkgwindow.DefaultWindow("main")
	w, rt, be, q := setup(t, pkgwindow.WindowPlugin{PrimaryWindow: &primary, ExitCondition: pkgwindow.OnPrimaryClosed})
	secondary := w.Spawn(component.Data{Value: pkgwindow.DefaultWindow("aux")})
	syncWindows(w, rt, q)

	be.ScriptEvents(pkgwindow.PlatformEvent{Kind: pkgwindow.EventCloseRequested, Window: secondary})
	pollWindowEvents(w, rt, q)

	if exitPending(w) {
		t.Error("closing a non-primary window under OnPrimaryClosed must not request exit")
	}
}

func TestExitOnAllClosed(t *testing.T) {
	primary := pkgwindow.DefaultWindow("main")
	w, rt, be, q := setup(t, pkgwindow.WindowPlugin{PrimaryWindow: &primary, ExitCondition: pkgwindow.OnAllClosed})
	syncWindows(w, rt, q)

	// Closing the only (last) window ⇒ remaining 0 ⇒ exit.
	be.ScriptEvents(pkgwindow.PlatformEvent{Kind: pkgwindow.EventClosed, Window: rt.primary.Entity})
	pollWindowEvents(w, rt, q)

	if !exitPending(w) {
		t.Error("closing the last window under OnAllClosed must request exit")
	}
	if w.Contains(rt.primary.Entity) {
		t.Error("EventClosed must despawn the window entity")
	}
}

func TestOnAllClosedKeepsRunningWhileWindowsRemain(t *testing.T) {
	primary := pkgwindow.DefaultWindow("main")
	w, rt, be, q := setup(t, pkgwindow.WindowPlugin{PrimaryWindow: &primary, ExitCondition: pkgwindow.OnAllClosed})
	secondary := w.Spawn(component.Data{Value: pkgwindow.DefaultWindow("aux")})
	syncWindows(w, rt, q)

	be.ScriptEvents(pkgwindow.PlatformEvent{Kind: pkgwindow.EventClosed, Window: secondary})
	pollWindowEvents(w, rt, q)

	if exitPending(w) {
		t.Error("a window remained open — OnAllClosed must not exit yet")
	}
}

func TestDontExitOnClose(t *testing.T) {
	primary := pkgwindow.DefaultWindow("main")
	w, rt, be, q := setup(t, pkgwindow.WindowPlugin{PrimaryWindow: &primary, ExitCondition: pkgwindow.DontExit})
	syncWindows(w, rt, q)

	be.ScriptEvents(pkgwindow.PlatformEvent{Kind: pkgwindow.EventCloseRequested, Window: rt.primary.Entity})
	pollWindowEvents(w, rt, q)

	if exitPending(w) {
		t.Error("DontExit must never request exit on close")
	}
}

func TestCloseWhenRequestedDespawns(t *testing.T) {
	primary := pkgwindow.DefaultWindow("main")
	w, rt, be, q := setup(t, pkgwindow.WindowPlugin{
		PrimaryWindow:      &primary,
		ExitCondition:      pkgwindow.DontExit,
		CloseWhenRequested: true,
	})
	syncWindows(w, rt, q)
	e := rt.primary.Entity

	be.ScriptEvents(pkgwindow.PlatformEvent{Kind: pkgwindow.EventCloseRequested, Window: e})
	pollWindowEvents(w, rt, q)

	if w.Contains(e) {
		t.Error("CloseWhenRequested must despawn the window on a close request")
	}
	// The despawn is torn down at the backend on the next sync pass.
	syncWindows(w, rt, q)
	if be.ActiveCount() != 0 {
		t.Errorf("backend still has %d windows after close-requested despawn", be.ActiveCount())
	}
}

// ── backend error handling ────────────────────────────────────────────────────

type failBackend struct{}

func (failBackend) CreateWindow(entity.Entity, pkgwindow.WindowDescriptor) (pkgwindow.RawWindowHandle, error) {
	return pkgwindow.RawWindowHandle{}, errors.New("create boom")
}
func (failBackend) DestroyWindow(entity.Entity) error { return errors.New("destroy boom") }
func (failBackend) ApplyChanges(entity.Entity, pkgwindow.WindowDiff) error {
	return errors.New("apply boom")
}
func (failBackend) PollEvents() []pkgwindow.PlatformEvent { return nil }

var _ pkgwindow.WindowBackend = failBackend{}

func TestSyncToleratesBackendErrors(t *testing.T) {
	w := world.NewWorld()
	world.RegisterComponent[pkgwindow.Window](w)
	rt := &windowRuntime{backend: failBackend{}, applied: map[entity.Entity]pkgwindow.Window{}}
	e := w.Spawn(component.Data{Value: pkgwindow.DefaultWindow("x")})
	q, _ := query.NewQuery1[pkgwindow.Window](w)

	syncWindows(w, rt, q) // create errors — logged, snapshot still recorded
	if _, ok := rt.applied[e]; !ok {
		t.Error("snapshot must be recorded even when CreateWindow errors")
	}

	win, _ := world.Get[pkgwindow.Window](w, e)
	win.Title = "y"
	syncWindows(w, rt, q) // apply errors — logged, frame survives

	if err := w.Despawn(e); err != nil {
		t.Fatalf("despawn: %v", err)
	}
	syncWindows(w, rt, q) // destroy errors — logged, snapshot still cleared
	if len(rt.applied) != 0 {
		t.Error("snapshot must be cleared after despawn even when DestroyWindow errors")
	}
}

// ── full App integration ──────────────────────────────────────────────────────

func TestSyncPluginExitStopsAppLoop(t *testing.T) {
	be := NewHeadlessWindowBackend()
	primary := pkgwindow.DefaultWindow("main")
	a := app.NewApp()
	a.AddPlugin(SyncPlugin{
		Backend: be,
		Config:  pkgwindow.WindowPlugin{PrimaryWindow: &primary, ExitCondition: pkgwindow.OnPrimaryClosed},
	})

	// The primary window was spawned during Build; script its close.
	q, _ := query.NewQuery1[pkgwindow.Window](a.World())
	e, _, err := q.Single(a.World())
	if err != nil {
		t.Fatalf("primary window not spawned: %v", err)
	}
	be.ScriptEvents(pkgwindow.PlatformEvent{Kind: pkgwindow.EventCloseRequested, Window: e})

	frames := 0
	a.SetRunner(func(ap *app.App) error {
		for frames < 50 && !ap.ShouldExit() {
			frames++
			if err := ap.Update(); err != nil {
				return err
			}
		}
		return nil
	})
	if err := a.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !a.ShouldExit() {
		t.Error("a window close did not stop the App loop")
	}
	if frames > 2 {
		t.Errorf("loop took %d frames to honor the close, want ≤ 2", frames)
	}
	if be.ActiveCount() != 1 {
		t.Errorf("backend window count = %d, want 1 (created once at first sync)", be.ActiveCount())
	}
}

func TestSyncPluginDefaultsToHeadlessBackend(t *testing.T) {
	primary := pkgwindow.DefaultWindow("main")
	a := app.NewApp()
	a.SetRunMode(app.RunOnce)
	a.AddPlugin(SyncPlugin{Config: pkgwindow.WindowPlugin{PrimaryWindow: &primary}})
	if err := a.Run(); err != nil {
		t.Fatalf("Run with default (headless) backend: %v", err)
	}
	rt, ok := world.Resource[windowRuntime](a.World())
	if !ok {
		t.Fatal("windowRuntime resource not set")
	}
	if _, isHeadless := rt.backend.(*HeadlessWindowBackend); !isHeadless {
		t.Errorf("nil Backend should default to *HeadlessWindowBackend, got %T", rt.backend)
	}
}
