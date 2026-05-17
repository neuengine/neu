package app

// T-2T03: App lifecycle integration test.
// Boots DefaultPlugins, runs 100 ticks, exercises state-transition,
// input-injection, and hierarchy-mutation round-trip.

import (
	"testing"

	"github.com/neuengine/neu/internal/ecs/command"
	"github.com/neuengine/neu/internal/ecs/gametime"
	"github.com/neuengine/neu/internal/ecs/hierarchy"
	"github.com/neuengine/neu/internal/ecs/input"
	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/state"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
)

// ── Test state type ───────────────────────────────────────────────────────────

type intPhase uint8

const (
	intPhaseMenu intPhase = iota
	intPhaseGame
)

// ── TestDefaultPlugins_Integration_100Ticks ───────────────────────────────────

func TestDefaultPlugins_Integration_100Ticks(t *testing.T) {
	a := NewApp()
	a.AddPlugins(DefaultPlugins{})

	// Register a two-state state machine.
	state.InitState(a, intPhaseMenu)

	// ── Observability counters ────────────────────────────────────────────────

	var (
		startupFired     bool
		hierarchySpawned bool
		inputInjected    bool
		transitionFired  bool
		updateFrames     int
	)

	// ── Startup: spawn a parent+child hierarchy pair ──────────────────────────

	a.AddSystem(appface.Startup, scheduler.NewFuncSystem("integ.spawn", func(w *world.World) {
		buf := command.AcquireBuffer(w.Entities())
		cmds := command.NewCommands(buf)

		parent := cmds.Spawn()
		child := cmds.Spawn()
		hierarchy.AddChild(cmds, parent, child)

		buf.Apply(w)
		command.ReleaseBuffer(buf)

		hierarchySpawned = true
		startupFired = true
	}))

	// ── PreUpdate: inject input + trigger transition on frame 10 ─────────────

	a.AddSystem(appface.PreUpdate, scheduler.NewFuncSystem("integ.input", func(w *world.World) {
		updateFrames++

		if updateFrames == 10 {
			// Inject a key press.
			kb, ok := world.Resource[*input.ButtonInput[input.KeyCode]](w)
			if ok {
				(*kb).Press(input.KeySpace)
				inputInjected = true
			}

			// Queue a state transition.
			state.TransitionTo(w, intPhaseGame)
		}

		if updateFrames >= 100 {
			a.Exit()
		}
	}))

	// ── Update: detect state transition ──────────────────────────────────────

	a.AddSystem(appface.Update, scheduler.NewFuncSystem("integ.detect", func(w *world.World) {
		if state.StateChanged[intPhase]()(w) {
			transitionFired = true
		}
	}))

	if err := a.Run(); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	// ── Assertions ────────────────────────────────────────────────────────────

	if !startupFired {
		t.Error("Startup system did not run")
	}
	if !hierarchySpawned {
		t.Error("hierarchy pair was not spawned")
	}
	if !inputInjected {
		t.Error("input was not injected")
	}
	if !transitionFired {
		t.Error("state transition was not detected")
	}
	if updateFrames < 100 {
		t.Errorf("only %d frames ran, want ≥ 100", updateFrames)
	}

	// Final state must be intPhaseGame.
	w := a.World()
	st, ok := world.Resource[state.State[intPhase]](w)
	if !ok {
		t.Fatal("State[intPhase] resource missing")
	}
	if st.Current() != intPhaseGame {
		t.Errorf("final state = %v, want intPhaseGame", st.Current())
	}
}

// ── TestDefaultPlugins_TimeResourcesPresent ───────────────────────────────────

// Verifies that TimePlugin registers all four time resources.
func TestDefaultPlugins_TimeResourcesPresent(t *testing.T) {
	a := NewApp()
	a.SetRunMode(RunOnce)
	a.AddPlugins(DefaultPlugins{})
	if err := a.Run(); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	w := a.World()
	if _, ok := world.Resource[gametime.RealTime](w); !ok {
		t.Error("RealTime not registered")
	}
	if _, ok := world.Resource[gametime.VirtualTime](w); !ok {
		t.Error("VirtualTime not registered")
	}
	if _, ok := world.Resource[gametime.FixedTime](w); !ok {
		t.Error("FixedTime not registered")
	}
	if _, ok := world.Resource[gametime.Time](w); !ok {
		t.Error("Time not registered")
	}
}

// ── TestDefaultPlugins_InputResourcesPresent ──────────────────────────────────

func TestDefaultPlugins_InputResourcesPresent(t *testing.T) {
	a := NewApp()
	a.SetRunMode(RunOnce)
	a.AddPlugins(DefaultPlugins{})
	if err := a.Run(); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	w := a.World()
	if _, ok := world.Resource[*input.ButtonInput[input.KeyCode]](w); !ok {
		t.Error("ButtonInput[KeyCode] not registered")
	}
	if _, ok := world.Resource[*input.ButtonInput[input.MouseButton]](w); !ok {
		t.Error("ButtonInput[MouseButton] not registered")
	}
	if _, ok := world.Resource[input.CursorPosition](w); !ok {
		t.Error("CursorPosition not registered")
	}
}

// ── TestDefaultPlugins_AllSchedulesFire ──────────────────────────────────────

// Verifies every standard schedule fires at least once in a RunOnce App.
func TestDefaultPlugins_AllSchedulesFire(t *testing.T) {
	fired := map[string]bool{}
	schedules := []string{
		appface.PreStartup, appface.Startup, appface.PostStartup,
		appface.First, appface.PreUpdate, appface.StateTransition,
		appface.Update, appface.PostUpdate, appface.Last,
	}

	a := NewApp()
	a.SetRunMode(RunOnce)
	a.AddPlugins(DefaultPlugins{})

	for _, name := range schedules {
		n := name
		a.AddSystem(n, scheduler.NewFuncSystem("probe."+n, func(_ *world.World) {
			fired[n] = true
		}))
	}

	if err := a.Run(); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	for _, name := range schedules {
		if !fired[name] {
			t.Errorf("schedule %q did not fire", name)
		}
	}
}
