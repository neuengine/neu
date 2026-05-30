package ui

import (
	"slices"

	"github.com/neuengine/neu/pkg/ecs"
	pkgmath "github.com/neuengine/neu/pkg/math"
	pkgui "github.com/neuengine/neu/pkg/ui"
)

// HitTarget is a node's interaction surface for pointer hit-testing.
type HitTarget struct {
	Entity ecs.Entity
	Rect   pkgui.LayoutRect
	Filter pkgui.MouseFilter
}

// HitResult is the outcome of a pointer hit-test.
type HitResult struct {
	Entity ecs.Entity
	Hit    bool
}

// HitTest finds the top-most node containing p. Targets are in render order
// (later = drawn on top), so it walks them in reverse (L1 §4.4 / §4.10).
// MouseIgnore nodes are skipped; the first MouseStop or MousePass node that
// contains p is returned. The caller propagates events for MousePass.
func HitTest(targets []HitTarget, p pkgmath.Vec2) HitResult {
	for _, t := range slices.Backward(targets) {
		if t.Filter == pkgui.MouseIgnore {
			continue
		}
		if t.Rect.Contains(p) {
			return HitResult{Entity: t.Entity, Hit: true}
		}
	}
	return HitResult{}
}

// InteractionFor derives the Interaction state from a hit plus pointer-down
// status (the PreUpdate state-machine transition, L1 §4.4).
func InteractionFor(hit, pointerDown bool) pkgui.Interaction {
	switch {
	case hit && pointerDown:
		return pkgui.InteractionPressed
	case hit:
		return pkgui.InteractionHovered
	default:
		return pkgui.InteractionNone
	}
}
