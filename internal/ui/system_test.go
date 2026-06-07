package ui

import (
	"testing"

	"github.com/neuengine/neu/internal/ecs/command"
	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/hierarchy"
	"github.com/neuengine/neu/internal/ecs/input"
	"github.com/neuengine/neu/internal/ecs/query"
	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
	"github.com/neuengine/neu/pkg/ecs"
	pkgmath "github.com/neuengine/neu/pkg/math"
	pkgui "github.com/neuengine/neu/pkg/ui"
)

// uiWorld builds a World with the UI + hierarchy components and input resources
// registered, plus a UIViewport — the minimum the interaction system needs.
func uiWorld(vw, vh float32) *world.World {
	w := world.NewWorld()
	world.RegisterComponent[pkgui.Style](w)
	world.RegisterComponent[pkgui.Node](w)
	world.RegisterComponent[pkgui.LayoutRect](w)
	world.RegisterComponent[pkgui.Interaction](w)
	world.RegisterComponent[pkgui.MouseFilter](w)
	world.RegisterComponent[pkgui.ZIndex](w)
	world.RegisterComponent[pkgui.Focused](w)
	world.RegisterComponent[hierarchy.ChildOf](w)
	world.RegisterComponent[hierarchy.Children](w)
	world.SetResource(w, input.NewButtonInput[input.MouseButton]())
	world.SetResource(w, input.CursorPosition{})
	world.SetResource(w, UIViewport{Width: vw, Height: vh})
	return w
}

func spawnNode(w *world.World, style pkgui.Style) ecs.Entity {
	return w.Spawn(component.Data{Value: style}, component.Data{Value: pkgui.Node{}})
}

func styleQuery(w *world.World) *query.Query1[pkgui.Style] {
	q, _ := query.NewQuery1[pkgui.Style](w)
	return q
}

func cursorAt(w *world.World, x, y float32) {
	world.SetResource(w, input.CursorPosition{Position: pkgmath.Vec2{X: x, Y: y}})
}

func pressLeft(w *world.World) {
	mbp, _ := world.Resource[*input.ButtonInput[input.MouseButton]](w)
	(*mbp).Press(input.MouseButtonLeft)
}

// --- layout ------------------------------------------------------------------

func TestSolveLayoutWritesRect(t *testing.T) {
	t.Parallel()
	w := uiWorld(200, 100)
	e := spawnNode(w, pkgui.Style{Width: pkgui.Px(200), Height: pkgui.Px(100)})

	laid := solveLayout(w, styleQuery(w))
	if len(laid) != 1 {
		t.Fatalf("laid = %d nodes, want 1", len(laid))
	}
	rect, ok := world.Get[pkgui.LayoutRect](w, e)
	if !ok {
		t.Fatal("root never received a LayoutRect component")
	}
	if rect.Size != (pkgmath.Vec2{X: 200, Y: 100}) {
		t.Errorf("root rect size = %v, want {200 100}", rect.Size)
	}
}

func TestSolveLayoutHierarchyChildren(t *testing.T) {
	t.Parallel()
	w := uiWorld(200, 100)
	parent := spawnNode(w, pkgui.Style{FlexDirection: pkgui.Row, Width: pkgui.Px(200), Height: pkgui.Px(100)})
	c1 := spawnNode(w, pkgui.Style{FlexGrow: 1})
	c2 := spawnNode(w, pkgui.Style{FlexGrow: 1})

	buf := command.NewCommandBuffer(w.Entities(), 0)
	cmds := command.NewCommands(buf)
	hierarchy.AddChild(cmds, parent, c1)
	hierarchy.AddChild(cmds, parent, c2)
	buf.Apply(w)

	solveLayout(w, styleQuery(w))

	r1, _ := world.Get[pkgui.LayoutRect](w, c1)
	r2, _ := world.Get[pkgui.LayoutRect](w, c2)
	// Two grow children split the 200px row evenly; together they span it.
	if r1.Size.X <= 0 || r2.Size.X <= 0 {
		t.Fatalf("child widths = %v, %v, want both > 0", r1.Size.X, r2.Size.X)
	}
	if got := r1.Size.X + r2.Size.X; got != 200 {
		t.Errorf("child widths sum = %v, want 200", got)
	}
	// The second child is offset to the right of the first (row order preserved).
	if r2.Position.X <= r1.Position.X {
		t.Errorf("c2.X (%v) should be right of c1.X (%v)", r2.Position.X, r1.Position.X)
	}
}

// --- interaction -------------------------------------------------------------

func TestInteractionHoverPressMiss(t *testing.T) {
	t.Parallel()
	w := uiWorld(100, 100)
	e := spawnNode(w, pkgui.Style{Width: pkgui.Px(100), Height: pkgui.Px(100)})
	q := styleQuery(w)

	// Hover (cursor inside, no press).
	cursorAt(w, 50, 50)
	updateInteraction(w, solveLayout(w, q))
	if got := interactionOf(t, w, e); got != pkgui.InteractionHovered {
		t.Errorf("hover: Interaction = %v, want Hovered", got)
	}

	// Press (cursor inside, left down) → Pressed + Focused.
	pressLeft(w)
	updateInteraction(w, solveLayout(w, q))
	if got := interactionOf(t, w, e); got != pkgui.InteractionPressed {
		t.Errorf("press: Interaction = %v, want Pressed", got)
	}
	if _, ok := world.Get[pkgui.Focused](w, e); !ok {
		t.Error("press should focus the hit node")
	}

	// Miss (cursor outside) → None.
	resetMouse(w)
	cursorAt(w, 500, 500)
	updateInteraction(w, solveLayout(w, q))
	if got := interactionOf(t, w, e); got != pkgui.InteractionNone {
		t.Errorf("miss: Interaction = %v, want None", got)
	}
}

func TestInteractionMouseIgnoreSkipped(t *testing.T) {
	t.Parallel()
	w := uiWorld(100, 100)
	e := spawnNode(w, pkgui.Style{Width: pkgui.Px(100), Height: pkgui.Px(100)})
	if err := w.Insert(e, component.Data{Value: pkgui.MouseIgnore}); err != nil {
		t.Fatalf("insert MouseFilter: %v", err)
	}
	cursorAt(w, 50, 50)
	updateInteraction(w, solveLayout(w, styleQuery(w)))
	if got := interactionOf(t, w, e); got != pkgui.InteractionNone {
		t.Errorf("MouseIgnore node Interaction = %v, want None (not hit)", got)
	}
}

func TestUpdateInteractionNoCursorOrEmpty(t *testing.T) {
	t.Parallel()
	// Empty laid set → no-op (no panic).
	w := uiWorld(100, 100)
	updateInteraction(w, nil)

	// No CursorPosition resource → no-op even with nodes laid.
	w2 := world.NewWorld()
	world.RegisterComponent[pkgui.Style](w2)
	world.RegisterComponent[pkgui.LayoutRect](w2)
	world.RegisterComponent[pkgui.Interaction](w2)
	world.SetResource(w2, UIViewport{Width: 100, Height: 100})
	e := w2.Spawn(component.Data{Value: pkgui.Style{Width: pkgui.Px(50), Height: pkgui.Px(50)}})
	q, _ := query.NewQuery1[pkgui.Style](w2)
	updateInteraction(w2, solveLayout(w2, q))
	if _, ok := world.Get[pkgui.Interaction](w2, e); ok {
		t.Error("no CursorPosition → Interaction must not be written")
	}
}

// --- focus -------------------------------------------------------------------

func TestSetFocusTransfers(t *testing.T) {
	t.Parallel()
	w := uiWorld(100, 100)
	a := spawnNode(w, pkgui.Style{})
	b := spawnNode(w, pkgui.Style{})

	setFocus(w, a)
	if _, ok := world.Get[pkgui.Focused](w, a); !ok {
		t.Fatal("a should be focused after setFocus(a)")
	}

	setFocus(w, b)
	if _, ok := world.Get[pkgui.Focused](w, b); !ok {
		t.Error("b should be focused after setFocus(b)")
	}
	if _, ok := world.Get[pkgui.Focused](w, a); ok {
		t.Error("a's Focused marker should be cleared when focus moves to b")
	}
}

// --- plugin wiring -----------------------------------------------------------

// recordingBuilder is a minimal appface.Builder backed by a real World (so
// RegisterComponent + query construction work) that records the system the
// plugin registers and the schedule it targets.
type recordingBuilder struct {
	system   scheduler.System
	w        *world.World
	schedule string
}

func (b *recordingBuilder) World() *world.World { return b.w }
func (b *recordingBuilder) AddSystem(s string, sys scheduler.System) appface.Builder {
	b.schedule, b.system = s, sys
	return b
}
func (b *recordingBuilder) AddSystems(s string, sys ...scheduler.System) appface.Builder {
	for _, ss := range sys {
		b.AddSystem(s, ss)
	}
	return b
}
func (b *recordingBuilder) SetResource(v any) appface.Builder              { world.SetResourceAny(b.w, v); return b }
func (b *recordingBuilder) InitResource(any) appface.Builder               { return b }
func (b *recordingBuilder) AddPlugin(appface.Plugin) appface.Builder       { return b }
func (b *recordingBuilder) AddPlugins(appface.PluginGroup) appface.Builder { return b }

func TestInteractionPluginBuild(t *testing.T) {
	t.Parallel()
	b := &recordingBuilder{w: world.NewWorld()}
	InteractionPlugin{Viewport: UIViewport{Width: 320, Height: 240}}.Build(b)

	if b.schedule != appface.PreUpdate {
		t.Errorf("system schedule = %q, want PreUpdate", b.schedule)
	}
	if b.system == nil || b.system.Name() != "ui.Interaction" {
		t.Fatalf("registered system = %v, want ui.Interaction", b.system)
	}
	if vp, ok := world.Resource[UIViewport](b.w); !ok || vp.Width != 320 || vp.Height != 240 {
		t.Errorf("UIViewport resource = %+v (ok=%v), want 320x240", vp, ok)
	}
	// The registered system runs without panic on an empty (but configured) world.
	b.system.Run(b.w)
}

// --- helpers -----------------------------------------------------------------

func interactionOf(t *testing.T, w *world.World, e ecs.Entity) pkgui.Interaction {
	t.Helper()
	i, ok := world.Get[pkgui.Interaction](w, e)
	if !ok {
		t.Fatalf("entity %v has no Interaction component", e)
	}
	return *i
}

func resetMouse(w *world.World) {
	mbp, _ := world.Resource[*input.ButtonInput[input.MouseButton]](w)
	(*mbp).Release(input.MouseButtonLeft)
}
