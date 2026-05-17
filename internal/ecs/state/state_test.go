package state

import (
	"testing"

	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
)

// ── test state type ───────────────────────────────────────────────────────────

type GameState uint8

const (
	GameStateMenu    GameState = iota
	GameStatePlaying GameState = iota
	GameStatePaused  GameState = iota
)

// ── mock builder ──────────────────────────────────────────────────────────────

type mockBuilder struct {
	w       *world.World
	systems []string
}

func (m *mockBuilder) World() *world.World { return m.w }
func (m *mockBuilder) AddSystem(sched string, sys scheduler.System) appface.Builder {
	m.systems = append(m.systems, sched+":"+sys.Name())
	return m
}
func (m *mockBuilder) AddSystems(sched string, systems ...scheduler.System) appface.Builder {
	for _, s := range systems {
		m.AddSystem(sched, s)
	}
	return m
}
func (m *mockBuilder) SetResource(v any) appface.Builder  { return m }
func (m *mockBuilder) InitResource(v any) appface.Builder { return m }
func (m *mockBuilder) AddPlugin(p appface.Plugin) appface.Builder {
	p.Build(m)
	return m
}
func (m *mockBuilder) AddPlugins(g appface.PluginGroup) appface.Builder { return m }

// ── State[S] ──────────────────────────────────────────────────────────────────

func TestState_Current(t *testing.T) {
	s := State[GameState]{current: GameStatePlaying}
	if s.Current() != GameStatePlaying {
		t.Errorf("Current = %v, want Playing", s.Current())
	}
}

// ── NextState[S] ──────────────────────────────────────────────────────────────

func TestNextState_SetGetClear(t *testing.T) {
	var n NextState[GameState]

	if n.IsPending() {
		t.Error("should not be pending initially")
	}
	if n.Get() != nil {
		t.Error("Get should return nil initially")
	}

	n.Set(GameStatePlaying)
	if !n.IsPending() {
		t.Error("should be pending after Set")
	}
	if n.Get() == nil || *n.Get() != GameStatePlaying {
		t.Errorf("Get = %v, want Playing", n.Get())
	}

	n.Clear()
	if n.IsPending() {
		t.Error("should not be pending after Clear")
	}
}

// ── TransitionTo ──────────────────────────────────────────────────────────────

func TestTransitionTo(t *testing.T) {
	w := world.NewWorld()
	world.SetResource(w, NextState[GameState]{})

	TransitionTo(w, GameStatePlaying)

	ns, _ := world.Resource[NextState[GameState]](w)
	if !ns.IsPending() || *ns.Get() != GameStatePlaying {
		t.Error("TransitionTo did not set NextState")
	}
}

func TestTransitionTo_NoResource(t *testing.T) {
	// Must not panic when NextState resource is not registered.
	w := world.NewWorld()
	TransitionTo(w, GameStatePlaying)
}

// ── applyTransition ───────────────────────────────────────────────────────────

func TestApplyTransition_Basic(t *testing.T) {
	w := world.NewWorld()
	world.SetResource(w, State[GameState]{current: GameStateMenu})
	world.SetResource(w, NextState[GameState]{})
	world.SetResource(w, LastTransition[GameState]{})

	TransitionTo(w, GameStatePlaying)
	applyTransition[GameState](w)

	st, _ := world.Resource[State[GameState]](w)
	if st.Current() != GameStatePlaying {
		t.Errorf("State after transition = %v, want Playing", st.Current())
	}

	lt, _ := world.Resource[LastTransition[GameState]](w)
	if !lt.Changed {
		t.Error("LastTransition.Changed should be true")
	}
	if lt.From != GameStateMenu || lt.To != GameStatePlaying {
		t.Errorf("LastTransition = {%v→%v}, want {Menu→Playing}", lt.From, lt.To)
	}

	// NextState must be cleared.
	ns, _ := world.Resource[NextState[GameState]](w)
	if ns.IsPending() {
		t.Error("NextState should be cleared after transition")
	}
}

func TestApplyTransition_SameStateNoOp(t *testing.T) {
	w := world.NewWorld()
	world.SetResource(w, State[GameState]{current: GameStatePlaying})
	world.SetResource(w, NextState[GameState]{})
	world.SetResource(w, LastTransition[GameState]{})

	TransitionTo(w, GameStatePlaying) // transition to same state
	applyTransition[GameState](w)

	lt, _ := world.Resource[LastTransition[GameState]](w)
	if lt.Changed {
		t.Error("LastTransition.Changed should be false for same-state transition")
	}
}

func TestApplyTransition_NoPending(t *testing.T) {
	w := world.NewWorld()
	world.SetResource(w, State[GameState]{current: GameStateMenu})
	world.SetResource(w, NextState[GameState]{})
	world.SetResource(w, LastTransition[GameState]{Changed: true})

	applyTransition[GameState](w) // nothing pending

	lt, _ := world.Resource[LastTransition[GameState]](w)
	if lt.Changed {
		t.Error("LastTransition.Changed should be reset to false on frame with no transition")
	}
}

func TestApplyTransition_NoStateResource(t *testing.T) {
	// Must not panic when State resource is missing.
	w := world.NewWorld()
	world.SetResource(w, NextState[GameState]{})
	ns, _ := world.Resource[NextState[GameState]](w)
	ns.Set(GameStatePlaying)
	applyTransition[GameState](w) // no State[S] registered
}

// ── InitState ─────────────────────────────────────────────────────────────────

func TestInitState_RegistersResources(t *testing.T) {
	w := world.NewWorld()
	mb := &mockBuilder{w: w}

	InitState(mb, GameStateMenu)

	if !world.ContainsResource[State[GameState]](w) {
		t.Error("State[GameState] not registered")
	}
	if !world.ContainsResource[NextState[GameState]](w) {
		t.Error("NextState[GameState] not registered")
	}
	if !world.ContainsResource[LastTransition[GameState]](w) {
		t.Error("LastTransition[GameState] not registered")
	}

	// Must have registered a transition system.
	if len(mb.systems) == 0 {
		t.Error("no system registered by InitState")
	}
}

func TestInitState_Idempotent(t *testing.T) {
	w := world.NewWorld()
	mb := &mockBuilder{w: w}

	InitState(mb, GameStateMenu)
	count1 := len(mb.systems)
	InitState(mb, GameStatePlaying) // second call must be no-op
	if len(mb.systems) != count1 {
		t.Error("InitState called twice registered extra systems")
	}
}

// ── Schedule labels ───────────────────────────────────────────────────────────

func TestOnEnter_Label(t *testing.T) {
	label := OnEnter(GameStatePlaying)
	if label == "" {
		t.Error("OnEnter returned empty label")
	}
	// Different values must produce different labels.
	if OnEnter(GameStateMenu) == OnEnter(GameStatePlaying) {
		t.Error("OnEnter labels must differ for different values")
	}
}

func TestOnExit_Label(t *testing.T) {
	label := OnExit(GameStatePlaying)
	if label == "" {
		t.Error("OnExit returned empty label")
	}
	if OnExit(GameStateMenu) == OnExit(GameStatePlaying) {
		t.Error("OnExit labels must differ for different values")
	}
}

func TestOnTransition_Label(t *testing.T) {
	label := OnTransition[GameState]()
	if label == "" {
		t.Error("OnTransition returned empty label")
	}
}

func TestOnEnterExitDistinct(t *testing.T) {
	if OnEnter(GameStatePlaying) == OnExit(GameStatePlaying) {
		t.Error("OnEnter and OnExit labels must differ for the same value")
	}
}

// ── Run conditions ────────────────────────────────────────────────────────────

func TestInState(t *testing.T) {
	w := world.NewWorld()
	world.SetResource(w, State[GameState]{current: GameStatePlaying})

	cond := InState(GameStatePlaying)
	if !cond(w) {
		t.Error("InState should return true when state matches")
	}
	cond2 := InState(GameStateMenu)
	if cond2(w) {
		t.Error("InState should return false when state doesn't match")
	}
}

func TestInState_NoResource(t *testing.T) {
	w := world.NewWorld()
	cond := InState(GameStatePlaying)
	if cond(w) {
		t.Error("InState should return false when resource is absent")
	}
}

func TestStateChanged(t *testing.T) {
	w := world.NewWorld()
	world.SetResource(w, LastTransition[GameState]{Changed: true})

	cond := StateChanged[GameState]()
	if !cond(w) {
		t.Error("StateChanged should return true when LastTransition.Changed")
	}
}

func TestStateChanged_NoTransition(t *testing.T) {
	w := world.NewWorld()
	world.SetResource(w, LastTransition[GameState]{Changed: false})

	if StateChanged[GameState]()(w) {
		t.Error("StateChanged should return false when not changed")
	}
}

func TestStateChanged_NoResource(t *testing.T) {
	w := world.NewWorld()
	if StateChanged[GameState]()(w) {
		t.Error("StateChanged should return false when resource absent")
	}
}

func TestStateExists(t *testing.T) {
	w := world.NewWorld()
	cond := StateExists[GameState]()
	if cond(w) {
		t.Error("StateExists should be false before registration")
	}
	world.SetResource(w, State[GameState]{})
	if !cond(w) {
		t.Error("StateExists should be true after registration")
	}
}

// ── DespawnOnExit ─────────────────────────────────────────────────────────────

func TestDespawnOnExit_ComponentValue(t *testing.T) {
	d := DespawnOnExit[GameState]{State: GameStateMenu}
	if d.State != GameStateMenu {
		t.Error("DespawnOnExit.State mismatch")
	}
}

// ── StatePlugin ───────────────────────────────────────────────────────────────

func TestStatePlugin_Build(t *testing.T) {
	w := world.NewWorld()
	mb := &mockBuilder{w: w}
	StatePlugin{}.Build(mb) // must not panic
}

// ── TransitionSystemFor integration ──────────────────────────────────────────

func TestTransitionSystemFor_RoundTrip(t *testing.T) {
	w := world.NewWorld()
	world.SetResource(w, State[GameState]{current: GameStateMenu})
	world.SetResource(w, NextState[GameState]{})
	world.SetResource(w, LastTransition[GameState]{})

	sys := TransitionSystemFor[GameState]("test.Transition")
	if sys.Name() != "test.Transition" {
		t.Errorf("system name = %q, want %q", sys.Name(), "test.Transition")
	}

	// Simulate queuing and applying a transition via the system.
	TransitionTo(w, GameStatePlaying)
	sys.Run(w) // applies the transition

	st, _ := world.Resource[State[GameState]](w)
	if st.Current() != GameStatePlaying {
		t.Errorf("after sys.Run, state = %v, want Playing", st.Current())
	}
}
