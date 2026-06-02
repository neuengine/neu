// Package ui implements the UI layout solver, font atlas, hit-testing, and the
// render feature. The public Style/Node/Interaction components live in pkg/ui;
// this package holds the algorithms (which import render-internal types for the
// feature).
//
// Bootstrap: l2-ui-system-go Draft (Phase 6 Track D).
package ui

import (
	"slices"

	pkgmath "github.com/neuengine/neu/pkg/math"
	pkgui "github.com/neuengine/neu/pkg/ui"
)

// LayoutNode is the solver's view of a UI subtree: a Style plus children. After
// Solve, Rect holds the computed position and size (logical pixels). Dirty drives
// incremental relayout (INV-1): SolveIfDirty skips a fully-clean tree.
type LayoutNode struct {
	Children   []*LayoutNode
	solveCount int
	Style      pkgui.Style
	Rect       pkgui.LayoutRect
	Dirty      bool
}

// Viewport carries the root available size and the viewport dimensions used to
// resolve Vw/Vh values.
type Viewport struct{ Width, Height float32 }

// MarkDirty flags this node for relayout.
func (n *LayoutNode) MarkDirty() { n.Dirty = true }

// SolveCount reports how many times the node has been laid out (test hook).
func (n *LayoutNode) SolveCount() int { return n.solveCount }

// Solve lays out the whole tree from the root. The root's size resolves against
// the viewport (Auto ⇒ full viewport).
func Solve(root *LayoutNode, vp Viewport) {
	if root == nil {
		return
	}
	w := resolveOr(root.Style.Width, vp.Width, vp, vp.Width)
	h := resolveOr(root.Style.Height, vp.Height, vp, vp.Height)
	w = clamp(w, root.Style.MinWidth, root.Style.MaxWidth, vp.Width, vp)
	h = clamp(h, root.Style.MinHeight, root.Style.MaxHeight, vp.Height, vp)
	root.Rect = pkgui.LayoutRect{Position: pkgmath.Vec2{}, Size: pkgmath.Vec2{X: w, Y: h}}
	layoutChildren(root, vp)
}

// SolveIfDirty solves only when some node in the tree is dirty. It returns true
// if a solve ran. A fully-clean tree is skipped entirely (INV-1 zero-cost).
func SolveIfDirty(root *LayoutNode, vp Viewport) bool {
	if root == nil || !anyDirty(root) {
		return false
	}
	Solve(root, vp)
	clearDirty(root)
	return true
}

func anyDirty(n *LayoutNode) bool {
	return n.Dirty || slices.ContainsFunc(n.Children, anyDirty)
}

func clearDirty(n *LayoutNode) {
	n.Dirty = false
	for _, c := range n.Children {
		clearDirty(c)
	}
}

// layoutChildren positions n's children inside its content box using a
// single-line flexbox along the main axis.
func layoutChildren(n *LayoutNode, vp Viewport) {
	n.solveCount++
	if n.Style.Display == pkgui.DisplayNone || len(n.Children) == 0 {
		return
	}

	pl, _ := n.Style.Padding.Left.Resolve(n.Rect.Size.X, vp.Width, vp.Height)
	pr, _ := n.Style.Padding.Right.Resolve(n.Rect.Size.X, vp.Width, vp.Height)
	pt, _ := n.Style.Padding.Top.Resolve(n.Rect.Size.Y, vp.Width, vp.Height)
	pb, _ := n.Style.Padding.Bottom.Resolve(n.Rect.Size.Y, vp.Width, vp.Height)

	contentX := n.Rect.Position.X + pl
	contentY := n.Rect.Position.Y + pt
	contentW := n.Rect.Size.X - pl - pr
	contentH := n.Rect.Size.Y - pt - pb

	dir := n.Style.FlexDirection
	row := dir.IsRow()
	mainSize := contentH
	crossSize := contentW
	if row {
		mainSize, crossSize = contentW, contentH
	}

	// Base main-axis size per child (flex-basis or explicit main size; Auto ⇒ 0).
	bases := make([]float32, len(n.Children))
	var sumBase float32
	for i, c := range n.Children {
		bases[i] = childBaseMain(c, row, mainSize, vp)
		sumBase += bases[i]
	}
	gap := n.Style.Gap * float32(len(n.Children)-1)
	free := mainSize - sumBase - gap

	// Distribute free space via grow (free > 0) or shrink (free < 0).
	dist := distribute(n.Children, bases, free)

	// Main-axis offsets per justify-content.
	used := sumBase + gap
	for _, d := range dist {
		used += d
	}
	offset, spacing := justify(n.Style.JustifyContent, mainSize, used, len(n.Children), n.Style.Gap)

	order := make([]int, len(n.Children))
	for i := range order {
		order[i] = i
		if dir.IsReverse() {
			order[i] = len(n.Children) - 1 - i
		}
	}

	pos := offset
	for _, idx := range order {
		c := n.Children[idx]
		mainLen := bases[idx] + dist[idx]
		crossLen := childCross(c, row, crossSize, vp)

		// Cross-axis alignment.
		crossOff := alignCross(alignFor(n, c), crossSize, crossLen)

		if row {
			c.Rect = pkgui.LayoutRect{
				Position: pkgmath.Vec2{X: contentX + pos, Y: contentY + crossOff},
				Size:     pkgmath.Vec2{X: mainLen, Y: crossLen},
			}
		} else {
			c.Rect = pkgui.LayoutRect{
				Position: pkgmath.Vec2{X: contentX + crossOff, Y: contentY + pos},
				Size:     pkgmath.Vec2{X: crossLen, Y: mainLen},
			}
		}
		pos += mainLen + spacing
		layoutChildren(c, vp)
	}
}

// childBaseMain resolves a child's base size along the main axis.
func childBaseMain(c *LayoutNode, row bool, parentMain float32, vp Viewport) float32 {
	basis := c.Style.FlexBasis
	if basis.Kind != pkgui.ValAuto {
		if v, ok := basis.Resolve(parentMain, vp.Width, vp.Height); ok {
			return v
		}
	}
	main := c.Style.Height
	if row {
		main = c.Style.Width
	}
	v, ok := main.Resolve(parentMain, vp.Width, vp.Height)
	if !ok {
		return 0 // Auto base; grow supplies size
	}
	return v
}

// childCross resolves a child's cross-axis size (Auto ⇒ stretch to parent).
func childCross(c *LayoutNode, row bool, parentCross float32, vp Viewport) float32 {
	cross := c.Style.Width
	if row {
		cross = c.Style.Height
	}
	if v, ok := cross.Resolve(parentCross, vp.Width, vp.Height); ok {
		return v
	}
	return parentCross // Auto cross ⇒ stretch
}

// distribute returns the extra main-axis length each child gets from free space.
func distribute(children []*LayoutNode, bases []float32, free float32) []float32 {
	out := make([]float32, len(children))
	if free > 0 {
		var totalGrow float32
		for _, c := range children {
			totalGrow += c.Style.FlexGrow
		}
		if totalGrow > 0 {
			for i, c := range children {
				out[i] = free * c.Style.FlexGrow / totalGrow
			}
		}
	} else if free < 0 {
		var totalShrink float32
		for _, c := range children {
			totalShrink += c.Style.FlexShrink
		}
		if totalShrink > 0 {
			for i, c := range children {
				out[i] = free * c.Style.FlexShrink / totalShrink
				if bases[i]+out[i] < 0 {
					out[i] = -bases[i] // never shrink below zero
				}
			}
		}
	}
	return out
}

// justify computes the leading offset and inter-item spacing for the main axis.
func justify(j pkgui.JustifyContent, mainSize, used float32, count int, gap float32) (offset, spacing float32) {
	remaining := mainSize - used
	spacing = gap
	switch j {
	case pkgui.JustifyCenter:
		offset = remaining / 2
	case pkgui.JustifyFlexEnd:
		offset = remaining
	case pkgui.JustifySpaceBetween:
		if count > 1 {
			spacing = gap + remaining/float32(count-1)
		}
	case pkgui.JustifySpaceAround:
		if count > 0 {
			unit := remaining / float32(count)
			offset = unit / 2
			spacing = gap + unit
		}
	case pkgui.JustifySpaceEvenly:
		if count > 0 {
			unit := remaining / float32(count+1)
			offset = unit
			spacing = gap + unit
		}
	default: // JustifyFlexStart
	}
	return offset, spacing
}

// alignCross computes the cross-axis offset for an item of length itemLen.
func alignCross(a pkgui.AlignItems, crossSize, itemLen float32) float32 {
	switch a {
	case pkgui.AlignCenter:
		return (crossSize - itemLen) / 2
	case pkgui.AlignFlexEnd:
		return crossSize - itemLen
	default: // AlignFlexStart / AlignStretch (stretch handled by childCross)
		return 0
	}
}

// alignFor returns the effective cross-axis alignment for a child (the parent's
// AlignItems; per-child AlignSelf is a future refinement).
func alignFor(parent *LayoutNode, _ *LayoutNode) pkgui.AlignItems {
	return parent.Style.AlignItems
}

// resolveOr resolves v against parent, returning fallback for Auto.
func resolveOr(v pkgui.Val, parent float32, vp Viewport, fallback float32) float32 {
	if r, ok := v.Resolve(parent, vp.Width, vp.Height); ok {
		return r
	}
	return fallback
}

// clamp applies min/max Val bounds to a resolved size.
func clamp(size float32, min, max pkgui.Val, parent float32, vp Viewport) float32 {
	if mn, ok := min.Resolve(parent, vp.Width, vp.Height); ok && size < mn {
		size = mn
	}
	if mx, ok := max.Resolve(parent, vp.Width, vp.Height); ok && size > mx {
		size = mx
	}
	return size
}
