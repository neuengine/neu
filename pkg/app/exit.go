package app

import (
	"fmt"

	"github.com/neuengine/neu/internal/ecs/event"
	"github.com/neuengine/neu/internal/ecs/world"
)

// AppExit is the event a system raises to ask the App's runner to stop after the
// current frame. A system only receives a *world.World, so it cannot call the
// App-only [App.Exit]; sending AppExit — via [RequestExit] — is the system-facing
// equivalent that the runner drains into the same shutdown path.
//
// Code mirrors a process exit status: the zero value is a graceful shutdown and
// any non-zero Code surfaces as an [*ExitError] from [App.Run].
type AppExit struct {
	Code uint8
}

// IsError reports whether the exit carries a failure code (non-zero).
func (e AppExit) IsError() bool { return e.Code != 0 }

// RequestExit sends an AppExit event on w, asking the App driving w to stop
// after the current frame. It is safe to call from any system. When w is not
// driven by an App (the AppExit bus was never registered) the call is a no-op,
// so systems remain usable against a bare World or test harness.
func RequestExit(w *world.World, code uint8) {
	if bus := event.Bus[AppExit](w); bus != nil {
		bus.Send(AppExit{Code: code})
	}
}

// ExitError reports that the App stopped because a system raised an AppExit
// carrying a non-zero code. [App.Run] returns it so the caller (e.g. main) can
// map the engine's exit to a process status.
type ExitError struct {
	Code uint8
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("app: exited with code %d", e.Code)
}

// drainExitEvents observes any AppExit raised during the frame and latches the
// shutdown flag (plus the first non-zero exit code). It runs at the tail of every
// frame so both the default and any custom runner honor a system-raised exit.
//
// The reader is created lazily on the first frame the bus exists, and its
// emptiness is checked before iterating so the steady-state path allocates
// nothing (C-004) — [event.EventReader.All] only builds its iterator closure on
// the shutdown frame.
func (a *App) drainExitEvents() {
	if a.exitReader == nil {
		if event.Bus[AppExit](a.w) == nil {
			return
		}
		a.exitReader = event.NewEventReader[AppExit](a.w)
	}
	if a.exitReader.IsEmpty() {
		return
	}
	for e := range a.exitReader.All() {
		a.shouldExit = true
		if e.Code != 0 && a.exitCode == 0 {
			a.exitCode = e.Code
		}
	}
}

// exitErr maps a latched non-zero exit code to an [*ExitError], or nil.
func (a *App) exitErr() error {
	if a.exitCode != 0 {
		return &ExitError{Code: a.exitCode}
	}
	return nil
}
