package input

import (
	"slices"
	"testing"

	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
	neu "github.com/neuengine/neu/pkg/math"
)

// ── ButtonInput ───────────────────────────────────────────────────────────────

func TestButtonInput_PressReleaseBasic(t *testing.T) {
	b := NewButtonInput[KeyCode]()

	b.Press(KeyA)
	if !b.Pressed(KeyA) {
		t.Error("Pressed(KeyA) should be true after Press")
	}
	if !b.JustPressed(KeyA) {
		t.Error("JustPressed(KeyA) should be true after Press")
	}
	if b.JustReleased(KeyA) {
		t.Error("JustReleased(KeyA) should be false after Press")
	}

	b.Release(KeyA)
	if b.Pressed(KeyA) {
		t.Error("Pressed(KeyA) should be false after Release")
	}
	if !b.JustReleased(KeyA) {
		t.Error("JustReleased(KeyA) should be true after Release")
	}
}

func TestButtonInput_Clear(t *testing.T) {
	b := NewButtonInput[KeyCode]()
	b.Press(KeyA)
	b.Release(KeyA)
	b.Press(KeyB) // held

	b.Clear()

	if b.JustPressed(KeyA) {
		t.Error("JustPressed should be cleared")
	}
	if b.JustReleased(KeyA) {
		t.Error("JustReleased should be cleared")
	}
	// Held key survives Clear
	if !b.Pressed(KeyB) {
		t.Error("Pressed should survive Clear")
	}
}

func TestButtonInput_Reset(t *testing.T) {
	b := NewButtonInput[KeyCode]()
	b.Press(KeyA)
	b.Press(KeyB)
	b.Reset()

	if b.AnyPressed() {
		t.Error("AnyPressed should be false after Reset")
	}
	if b.AnyJustPressed() {
		t.Error("AnyJustPressed should be false after Reset")
	}
}

func TestButtonInput_PressAlreadyDown(t *testing.T) {
	b := NewButtonInput[KeyCode]()
	b.Press(KeyA)
	b.Clear()     // simulate next frame
	b.Press(KeyA) // key held — already in pressed, not in justPressed

	if !b.Pressed(KeyA) {
		t.Error("Pressed should still be true")
	}
	if b.JustPressed(KeyA) {
		t.Error("JustPressed should be false when key was already held")
	}
}

func TestButtonInput_ReleasePressInSameFrame(t *testing.T) {
	b := NewButtonInput[KeyCode]()
	b.Press(KeyA)
	b.Release(KeyA)

	if !b.JustPressed(KeyA) {
		t.Error("JustPressed should be true (pressed this frame)")
	}
	if !b.JustReleased(KeyA) {
		t.Error("JustReleased should be true (released this frame)")
	}
}

func TestButtonInput_ReleaseNotPressed(t *testing.T) {
	b := NewButtonInput[KeyCode]()
	b.Release(KeyA) // never pressed — should be no-op
	if b.JustReleased(KeyA) {
		t.Error("JustReleased should be false when key was never pressed")
	}
}

func TestButtonInput_GetPressed(t *testing.T) {
	b := NewButtonInput[KeyCode]()
	b.Press(KeyA)
	b.Press(KeyB)
	b.Press(KeyC)

	got := b.GetPressed()
	if len(got) != 3 {
		t.Fatalf("GetPressed len = %d, want 3", len(got))
	}
	if !b.AnyPressed() {
		t.Error("AnyPressed should be true")
	}
}

func TestButtonInput_GetJustPressed(t *testing.T) {
	b := NewButtonInput[KeyCode]()
	b.Press(KeyA)
	b.Press(KeyB)

	got := b.GetJustPressed()
	if len(got) != 2 {
		t.Fatalf("GetJustPressed len = %d, want 2", len(got))
	}
	if !b.AnyJustPressed() {
		t.Error("AnyJustPressed should be true")
	}
}

func TestButtonInput_GetJustReleased(t *testing.T) {
	b := NewButtonInput[KeyCode]()
	b.Press(KeyA)
	b.Release(KeyA)

	got := b.GetJustReleased()
	if len(got) != 1 {
		t.Fatalf("GetJustReleased len = %d, want 1", len(got))
	}
}

func TestButtonInput_MouseButton(t *testing.T) {
	b := NewButtonInput[MouseButton]()
	b.Press(MouseButtonLeft)
	if !b.Pressed(MouseButtonLeft) {
		t.Error("mouse left should be pressed")
	}
	b.Clear()
	b.Release(MouseButtonLeft)
	if !b.JustReleased(MouseButtonLeft) {
		t.Error("mouse left should be just released")
	}
}

// ── AxisInput ─────────────────────────────────────────────────────────────────

func TestAxisInput_SetGet(t *testing.T) {
	a := NewAxisInput[GamepadAxis]()
	a.Set(GamepadAxisLeftStickX, 0.75)

	if got := a.Get(GamepadAxisLeftStickX); got != 0.75 {
		t.Errorf("Get = %v, want 0.75", got)
	}
	if got := a.Get(GamepadAxisRightStickY); got != 0 {
		t.Errorf("Get unset axis = %v, want 0", got)
	}
}

func TestAxisInput_Reset(t *testing.T) {
	a := NewAxisInput[GamepadAxis]()
	a.Set(GamepadAxisLeftStickX, 1.0)
	a.Set(GamepadAxisRightStickX, -0.5)
	a.Reset()

	if got := a.Get(GamepadAxisLeftStickX); got != 0 {
		t.Errorf("after Reset, axis = %v, want 0", got)
	}
}

// ── Types ─────────────────────────────────────────────────────────────────────

func TestKeyCodeCount(t *testing.T) {
	// Sanity: KeyCodeCount must be > 0 and consistent with the enum definition.
	if KeyCodeCount == 0 {
		t.Error("KeyCodeCount is 0")
	}
}

func TestMouseMotion(t *testing.T) {
	mm := MouseMotion{Delta: neu.Vec2{X: 3, Y: -2}}
	if mm.Delta.X != 3 {
		t.Errorf("MouseMotion.Delta.X = %v, want 3", mm.Delta.X)
	}
}

func TestCursorPosition(t *testing.T) {
	cp := CursorPosition{Position: neu.Vec2{X: 100, Y: 200}}
	if cp.Position.Y != 200 {
		t.Errorf("CursorPosition.Y = %v, want 200", cp.Position.Y)
	}
}

func TestTouchInput(t *testing.T) {
	ti := TouchInput{Phase: TouchPhaseStarted, ID: 42, Force: 0.8}
	if ti.Phase != TouchPhaseStarted {
		t.Error("TouchInput.Phase mismatch")
	}
	if ti.ID != 42 {
		t.Error("TouchInput.ID mismatch")
	}
}

// ── Plugin ────────────────────────────────────────────────────────────────────

type mockInputBuilder struct {
	w       *world.World
	systems []string
}

func (m *mockInputBuilder) World() *world.World { return m.w }
func (m *mockInputBuilder) AddSystem(sched string, sys scheduler.System) appface.Builder {
	m.systems = append(m.systems, sched+":"+sys.Name())
	return m
}
func (m *mockInputBuilder) AddSystems(sched string, systems ...scheduler.System) appface.Builder {
	for _, s := range systems {
		m.AddSystem(sched, s)
	}
	return m
}
func (m *mockInputBuilder) SetResource(v any) appface.Builder  { return m }
func (m *mockInputBuilder) InitResource(v any) appface.Builder { return m }
func (m *mockInputBuilder) AddPlugin(p appface.Plugin) appface.Builder {
	p.Build(m)
	return m
}
func (m *mockInputBuilder) AddPlugins(g appface.PluginGroup) appface.Builder { return m }

func TestInputPlugin_Build(t *testing.T) {
	w := world.NewWorld()
	mb := &mockInputBuilder{w: w}
	InputPlugin{}.Build(mb)

	// Verify all 5 resources are registered.
	if kb, ok := world.Resource[*ButtonInput[KeyCode]](w); !ok || *kb == nil {
		t.Error("ButtonInput[KeyCode] not registered")
	}
	if mb2, ok := world.Resource[*ButtonInput[MouseButton]](w); !ok || *mb2 == nil {
		t.Error("ButtonInput[MouseButton] not registered")
	}
	if gb, ok := world.Resource[*ButtonInput[GamepadButton]](w); !ok || *gb == nil {
		t.Error("ButtonInput[GamepadButton] not registered")
	}
	if ax, ok := world.Resource[*AxisInput[GamepadAxis]](w); !ok || *ax == nil {
		t.Error("AxisInput[GamepadAxis] not registered")
	}
	if _, ok := world.Resource[CursorPosition](w); !ok {
		t.Error("CursorPosition not registered")
	}

	// Verify the clear system is registered in PreUpdate.
	found := slices.Contains(mb.systems, appface.PreUpdate+":input.ClearInputState")
	if !found {
		t.Errorf("input.ClearInputState not registered in PreUpdate; systems = %v", mb.systems)
	}
}

func TestClearInputState_System(t *testing.T) {
	w := world.NewWorld()
	mb := &mockInputBuilder{w: w}
	InputPlugin{}.Build(mb)

	// Simulate a key press then run the clear system.
	kb, _ := world.Resource[*ButtonInput[KeyCode]](w)
	(*kb).Press(KeySpace)

	clearInputState(w)

	if (*kb).JustPressed(KeySpace) {
		t.Error("JustPressed should be false after clearInputState")
	}
	if !(*kb).Pressed(KeySpace) {
		t.Error("Pressed should survive clearInputState")
	}
}
