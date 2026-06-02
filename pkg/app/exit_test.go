package app

import (
	"errors"
	"testing"

	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
)

// ── AppExit value semantics ───────────────────────────────────────────────────

func TestAppExit_IsError(t *testing.T) {
	if (AppExit{}).IsError() {
		t.Error("zero-value AppExit must be a graceful (non-error) exit")
	}
	if !(AppExit{Code: 1}).IsError() {
		t.Error("non-zero Code must be an error exit")
	}
}

func TestExitError_Message(t *testing.T) {
	e := &ExitError{Code: 7}
	if got := e.Error(); got == "" {
		t.Error("ExitError.Error() must not be empty")
	}
}

// ── ShouldExit / Exit ─────────────────────────────────────────────────────────

func TestApp_ShouldExit(t *testing.T) {
	a := NewApp()
	if a.ShouldExit() {
		t.Error("a fresh App must not report ShouldExit")
	}
	a.Exit()
	if !a.ShouldExit() {
		t.Error("Exit() must latch ShouldExit")
	}
}

// ── RequestExit stops the default runner ──────────────────────────────────────

func TestRequestExit_StopsLoop(t *testing.T) {
	a := NewApp()
	frames := 0
	a.AddSystem(appface.Update, scheduler.NewFuncSystem("counter", func(w *world.World) {
		frames++
		if frames >= 3 {
			RequestExit(w, 0)
		}
	}))
	if err := a.Run(); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if frames != 3 {
		t.Errorf("frames = %d, want 3 (loop must stop the frame AppExit is raised)", frames)
	}
}

// RequestExit must reach the runner even when DefaultPlugins' EventsPlugin
// rotates the bus each frame (the realistic configuration).
func TestRequestExit_WithDefaultPlugins(t *testing.T) {
	a := NewApp()
	a.AddPlugins(DefaultPlugins{})
	frames := 0
	a.AddSystem(appface.Update, scheduler.NewFuncSystem("counter", func(w *world.World) {
		frames++
		if frames >= 2 {
			RequestExit(w, 0)
		}
	}))
	if err := a.Run(); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if frames != 2 {
		t.Errorf("frames = %d, want 2 (SwapAll must not drop the exit before the drain)", frames)
	}
}

// ── Exit code propagation ─────────────────────────────────────────────────────

func TestRequestExit_GracefulNoError(t *testing.T) {
	a := NewApp()
	a.AddSystem(appface.Update, scheduler.NewFuncSystem("done", func(w *world.World) {
		RequestExit(w, 0)
	}))
	if err := a.Run(); err != nil {
		t.Errorf("graceful exit (code 0) must return nil, got %v", err)
	}
}

func TestRequestExit_ErrorCodePropagates(t *testing.T) {
	a := NewApp()
	a.AddSystem(appface.Update, scheduler.NewFuncSystem("boom", func(w *world.World) {
		RequestExit(w, 2)
	}))
	err := a.Run()
	var ee *ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("Run error = %v, want *ExitError", err)
	}
	if ee.Code != 2 {
		t.Errorf("exit code = %d, want 2", ee.Code)
	}
}

// The first non-zero code wins when several exits are raised.
func TestRequestExit_FirstErrorCodeWins(t *testing.T) {
	a := NewApp()
	a.AddSystem(appface.Update, scheduler.NewFuncSystem("multi", func(w *world.World) {
		RequestExit(w, 3)
		RequestExit(w, 9)
	}))
	err := a.Run()
	var ee *ExitError
	if !errors.As(err, &ee) || ee.Code != 3 {
		t.Fatalf("err = %v, want *ExitError{Code:3}", err)
	}
}

// ── RunOnce path ──────────────────────────────────────────────────────────────

func TestRequestExit_RunOnceCodePropagates(t *testing.T) {
	a := NewApp()
	a.SetRunMode(RunOnce)
	a.AddSystem(appface.Update, scheduler.NewFuncSystem("once", func(w *world.World) {
		RequestExit(w, 5)
	}))
	err := a.Run()
	var ee *ExitError
	if !errors.As(err, &ee) || ee.Code != 5 {
		t.Fatalf("err = %v, want *ExitError{Code:5}", err)
	}
}

// ── No bus → no-op (bare World) ───────────────────────────────────────────────

func TestRequestExit_NoBusIsNoOp(t *testing.T) {
	w := world.NewWorld()
	// Must not panic when no App has registered the AppExit bus.
	RequestExit(w, 1)
}

// drainExitEvents must no-op (no panic, no latch) when the bus is absent, e.g.
// when Update is driven directly on a World the App never registered against.
func TestDrainExitEvents_NoBusNoOp(t *testing.T) {
	a := &App{w: world.NewWorld()} // bypass NewApp ⇒ no AppExit bus registered
	a.drainExitEvents()
	if a.ShouldExit() {
		t.Error("drain with no bus must not latch ShouldExit")
	}
}
