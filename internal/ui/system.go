package ui

import (
	"sort"

	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/hierarchy"
	"github.com/neuengine/neu/internal/ecs/input"
	"github.com/neuengine/neu/internal/ecs/query"
	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
	"github.com/neuengine/neu/pkg/ecs"
	pkgui "github.com/neuengine/neu/pkg/ui"
)

// UIViewport is a resource holding the layout viewport in logical pixels. A
// window-sync layer can refresh it from the primary window each frame; it
// defaults to zero (degenerate layout) until set.
type UIViewport struct{ Width, Height float32 }

// laidNode pairs a UI-node entity with its freshly computed rect, handed from the
// layout pass to the interaction pass within one system tick (so the hit-test
// reads this frame's rects without re-querying components mid-mutation).
type laidNode struct {
	entity ecs.Entity
	rect   pkgui.LayoutRect
}

// focusState tracks the currently focused entity so a focus change can clear the
// previous holder's Focused marker without scanning the world.
type focusState struct {
	current ecs.Entity
	set     bool
}

// InteractionPlugin wires the runtime UI interaction stack into the App. Each
// PreUpdate the single `ui.Interaction` system (1) lays out the UI-node entity
// tree and writes every node's computed LayoutRect, then (2) hit-tests the cursor
// against those rects and updates each node's Interaction plus the Focused marker.
//
// Using one system (not two) guarantees layout precedes hit-test without relying
// on intra-schedule ordering. The plugin is opt-in — UI is application-specific,
// so it is NOT in DefaultPlugins. The font-atlas glyph upload (T-6D04.2) is gated
// on the x/image/font ADR (T-6X04) and is deliberately not part of this stack.
type InteractionPlugin struct {
	// Viewport seeds the UIViewport resource (logical pixels).
	Viewport UIViewport
}

// Build implements appface.Plugin.
func (p InteractionPlugin) Build(b appface.Builder) {
	w := b.World()
	world.RegisterComponent[pkgui.Style](w)
	world.RegisterComponent[pkgui.Node](w)
	world.RegisterComponent[pkgui.LayoutRect](w)
	world.RegisterComponent[pkgui.Interaction](w)
	world.RegisterComponent[pkgui.MouseFilter](w)
	world.RegisterComponent[pkgui.ZIndex](w)
	world.RegisterComponent[pkgui.Focused](w)
	world.SetResource(w, p.Viewport)

	styleQ, _ := query.NewQuery1[pkgui.Style](w)
	b.AddSystem(appface.PreUpdate, scheduler.NewFuncSystem("ui.Interaction", func(w *world.World) {
		updateInteraction(w, solveLayout(w, styleQ))
	}))
}

// solveLayout builds a LayoutNode tree from the Style-bearing entities (linked by
// the engine hierarchy, preserving child order), solves each root subtree against
// the UIViewport, writes the computed LayoutRect back onto every node, and returns
// the laid-out (entity, rect) pairs for the interaction pass.
func solveLayout(w *world.World, styleQ *query.Query1[pkgui.Style]) []laidNode {
	nodes := make(map[ecs.Entity]*LayoutNode)
	var order []ecs.Entity
	for e, st := range styleQ.All(w) {
		nodes[e] = &LayoutNode{Style: *st}
		order = append(order, e)
	}
	if len(nodes) == 0 {
		return nil
	}
	// Link children (ordered) — only those that are themselves UI nodes.
	for _, e := range order {
		ch, ok := world.Get[hierarchy.Children](w, e)
		if !ok {
			continue
		}
		ln := nodes[e]
		for _, c := range ch.Slice() {
			if cn, isUI := nodes[c]; isUI {
				ln.Children = append(ln.Children, cn)
			}
		}
	}
	vp := Viewport{}
	if r, ok := world.Resource[UIViewport](w); ok {
		vp = Viewport{Width: r.Width, Height: r.Height}
	}
	// Solve each root (an entity with no UI-node parent).
	for _, e := range order {
		if isRoot(w, e, nodes) {
			Solve(nodes[e], vp)
		}
	}
	// Write rects back (post-iteration → safe to mutate archetypes) and collect.
	laid := make([]laidNode, 0, len(order))
	for _, e := range order {
		rect := nodes[e].Rect
		_ = w.Insert(e, component.Data{Value: rect})
		laid = append(laid, laidNode{entity: e, rect: rect})
	}
	return laid
}

// isRoot reports whether e has no parent that is itself a UI node, so its subtree
// is solved as a standalone tree.
func isRoot(w *world.World, e ecs.Entity, nodes map[ecs.Entity]*LayoutNode) bool {
	co, ok := world.Get[hierarchy.ChildOf](w, e)
	if !ok {
		return true
	}
	_, parentIsUI := nodes[co.Parent]
	return !parentIsUI
}

// updateInteraction hit-tests the cursor against the laid-out nodes and updates
// each node's Interaction; a fresh left-button press moves keyboard Focus to the
// hit node. No-op without a CursorPosition resource or laid-out nodes.
func updateInteraction(w *world.World, laid []laidNode) {
	cp, ok := world.Resource[input.CursorPosition](w)
	if !ok || len(laid) == 0 {
		return
	}
	down, justPressed := mouseLeft(w)

	// Build hit targets with their effective Z (painter's order: ascending Z, so
	// HitTest's reverse walk returns the top-most node).
	type item struct {
		entity ecs.Entity
		z      int32
		target HitTarget
	}
	items := make([]item, len(laid))
	for i, ln := range laid {
		filter := pkgui.MouseStop
		if f, ok := world.Get[pkgui.MouseFilter](w, ln.entity); ok {
			filter = *f
		}
		var z int32
		if zi, ok := world.Get[pkgui.ZIndex](w, ln.entity); ok {
			z = zi.Effective()
		}
		items[i] = item{
			entity: ln.entity,
			z:      z,
			target: HitTarget{Entity: ln.entity, Rect: ln.rect, Filter: filter},
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].z < items[j].z })
	targets := make([]HitTarget, len(items))
	for i := range items {
		targets[i] = items[i].target
	}
	hit := HitTest(targets, cp.Position)

	// Update each node's Interaction state.
	for _, it := range items {
		isHit := hit.Hit && it.entity == hit.Entity
		_ = w.Insert(it.entity, component.Data{Value: InteractionFor(isHit, down)})
	}
	// Focus follows a fresh press on a node.
	if justPressed && hit.Hit {
		setFocus(w, hit.Entity)
	}
}

// mouseLeft returns the held + just-pressed state of the left mouse button.
func mouseLeft(w *world.World) (down, justPressed bool) {
	mbp, ok := world.Resource[*input.ButtonInput[input.MouseButton]](w)
	if !ok || *mbp == nil {
		return false, false
	}
	mb := *mbp
	return mb.Pressed(input.MouseButtonLeft), mb.JustPressed(input.MouseButtonLeft)
}

// setFocus makes target the sole holder of the Focused marker, clearing the
// previous holder (tracked in the focusState resource).
func setFocus(w *world.World, target ecs.Entity) {
	if cur, ok := world.Resource[focusState](w); ok && cur.set && cur.current != target {
		_ = world.Remove[pkgui.Focused](w, cur.current)
	}
	_ = w.Insert(target, component.Data{Value: pkgui.Focused{}})
	world.SetResource(w, focusState{current: target, set: true})
}

var _ appface.Plugin = InteractionPlugin{}
