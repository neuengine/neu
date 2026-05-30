package ui

import (
	"testing"

	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/pkg/ecs"
	pkgmath "github.com/neuengine/neu/pkg/math"
	pkgui "github.com/neuengine/neu/pkg/ui"
)

const eps = 0.01

func rectEq(t *testing.T, name string, got pkgui.LayoutRect, x, y, w, h float32) {
	t.Helper()
	if abs(got.Position.X-x) > eps || abs(got.Position.Y-y) > eps ||
		abs(got.Size.X-w) > eps || abs(got.Size.Y-h) > eps {
		t.Errorf("%s = pos(%.1f,%.1f) size(%.1f,%.1f), want pos(%.1f,%.1f) size(%.1f,%.1f)",
			name, got.Position.X, got.Position.Y, got.Size.X, got.Size.Y, x, y, w, h)
	}
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func grow(n float32) pkgui.Style { return pkgui.Style{FlexGrow: n} }

func TestSolveRowGrow(t *testing.T) {
	t.Parallel()
	root := &LayoutNode{
		Style: pkgui.Style{FlexDirection: pkgui.Row, Width: pkgui.Px(200), Height: pkgui.Px(100)},
		Children: []*LayoutNode{
			{Style: grow(1)},
			{Style: grow(1)},
		},
	}
	Solve(root, Viewport{Width: 200, Height: 100})
	rectEq(t, "child0", root.Children[0].Rect, 0, 0, 100, 100)
	rectEq(t, "child1", root.Children[1].Rect, 100, 0, 100, 100)
}

func TestSolveColumnJustifyCenter(t *testing.T) {
	t.Parallel()
	root := &LayoutNode{
		Style: pkgui.Style{
			FlexDirection: pkgui.Column, JustifyContent: pkgui.JustifyCenter,
			Width: pkgui.Px(100), Height: pkgui.Px(200),
		},
		Children: []*LayoutNode{
			{Style: pkgui.Style{Width: pkgui.Px(100), Height: pkgui.Px(40)}},
		},
	}
	Solve(root, Viewport{Width: 100, Height: 200})
	// free = 200-40 = 160; center offset = 80.
	rectEq(t, "centered", root.Children[0].Rect, 0, 80, 100, 40)
}

func TestSolveAlignCenterAndPaddingGap(t *testing.T) {
	t.Parallel()
	// Row, align-center cross, padding 10, gap 20, two grow children.
	root := &LayoutNode{
		Style: pkgui.Style{
			FlexDirection: pkgui.Row, AlignItems: pkgui.AlignCenter,
			Width: pkgui.Px(240), Height: pkgui.Px(120),
			Padding: pkgui.PxRect(10, 10, 10, 10), Gap: 20,
		},
		Children: []*LayoutNode{
			{Style: pkgui.Style{FlexGrow: 1, Height: pkgui.Px(40)}},
			{Style: pkgui.Style{FlexGrow: 1, Height: pkgui.Px(40)}},
		},
	}
	Solve(root, Viewport{Width: 240, Height: 120})
	// content = 220×100 at (10,10). free main = 220-0-20(gap)=200 → 100 each.
	// cross height fixed 40, align-center within 100 → offset (100-40)/2=30 → y=10+30=40.
	rectEq(t, "c0", root.Children[0].Rect, 10, 40, 100, 40)
	rectEq(t, "c1", root.Children[1].Rect, 130, 40, 100, 40) // 10 + 100 + 20 gap = 130
}

func TestSolveNested(t *testing.T) {
	t.Parallel()
	root := &LayoutNode{
		Style: pkgui.Style{FlexDirection: pkgui.Column, Width: pkgui.Px(100), Height: pkgui.Px(100)},
		Children: []*LayoutNode{
			{
				Style: pkgui.Style{FlexGrow: 1, FlexDirection: pkgui.Row},
				Children: []*LayoutNode{
					{Style: grow(1)},
					{Style: grow(1)},
				},
			},
		},
	}
	Solve(root, Viewport{Width: 100, Height: 100})
	rectEq(t, "child", root.Children[0].Rect, 0, 0, 100, 100)
	gc := root.Children[0].Children
	rectEq(t, "grandchild0", gc[0].Rect, 0, 0, 50, 100)
	rectEq(t, "grandchild1", gc[1].Rect, 50, 0, 50, 100)
}

func TestDirtyGate(t *testing.T) {
	t.Parallel()
	root := &LayoutNode{
		Style:    pkgui.Style{Width: pkgui.Px(100), Height: pkgui.Px(100)},
		Children: []*LayoutNode{{Style: grow(1)}},
	}
	vp := Viewport{Width: 100, Height: 100}
	Solve(root, vp)
	base := root.Children[0].SolveCount()

	// Clean tree: SolveIfDirty must skip (INV-1 zero-cost), no re-solve.
	if SolveIfDirty(root, vp) {
		t.Error("clean tree should be skipped")
	}
	if root.Children[0].SolveCount() != base {
		t.Error("clean tree re-solved despite no dirt")
	}

	// Dirty a node → SolveIfDirty runs and increments solve counts.
	root.Children[0].MarkDirty()
	if !SolveIfDirty(root, vp) {
		t.Error("dirty tree should solve")
	}
	if root.Children[0].SolveCount() <= base {
		t.Error("dirty solve did not re-lay-out")
	}
}

func TestSolveShrink(t *testing.T) {
	t.Parallel()
	// Row, width 100, two children each basis 80 + shrink 1 → overflow shrinks both.
	root := &LayoutNode{
		Style: pkgui.Style{FlexDirection: pkgui.Row, Width: pkgui.Px(100), Height: pkgui.Px(20)},
		Children: []*LayoutNode{
			{Style: pkgui.Style{FlexBasis: pkgui.Px(80), FlexShrink: 1}},
			{Style: pkgui.Style{FlexBasis: pkgui.Px(80), FlexShrink: 1}},
		},
	}
	Solve(root, Viewport{Width: 100, Height: 20})
	// free = 100-160 = -60; each shrinks 30 → 50 wide.
	rectEq(t, "shrink0", root.Children[0].Rect, 0, 0, 50, 20)
	rectEq(t, "shrink1", root.Children[1].Rect, 50, 0, 50, 20)
}

func TestSolveSpaceBetweenAndReverse(t *testing.T) {
	t.Parallel()
	// Row reverse, space-between, two fixed-width children.
	root := &LayoutNode{
		Style: pkgui.Style{
			FlexDirection: pkgui.RowReverse, JustifyContent: pkgui.JustifySpaceBetween,
			Width: pkgui.Px(200), Height: pkgui.Px(20),
		},
		Children: []*LayoutNode{
			{Style: pkgui.Style{Width: pkgui.Px(50), Height: pkgui.Px(20)}},
			{Style: pkgui.Style{Width: pkgui.Px(50), Height: pkgui.Px(20)}},
		},
	}
	Solve(root, Viewport{Width: 200, Height: 20})
	// used = 100; remaining 100; spacing between the two = 100. Reverse order:
	// child[1] placed first at 0, child[0] at 50+100=150.
	rectEq(t, "rev-first", root.Children[1].Rect, 0, 0, 50, 20)
	rectEq(t, "rev-second", root.Children[0].Rect, 150, 0, 50, 20)
}

func TestFontAtlasCacheINV4(t *testing.T) {
	t.Parallel()
	a := NewFontAtlas(256, 256)
	const font, size = 1, 16

	a.Glyph(font, size, 'A')
	a.Glyph(font, size, 'A') // cache hit — must NOT rasterize again
	if a.Rasterized() != 1 {
		t.Errorf("same glyph rasterized %d times, want 1 (INV-4)", a.Rasterized())
	}
	// Distinct size → distinct cache entry.
	a.Glyph(font, 32, 'A')
	if a.Rasterized() != 2 || a.Len() != 2 {
		t.Errorf("distinct size: rasterized=%d len=%d, want 2/2", a.Rasterized(), a.Len())
	}
	// Space is narrower than a letter.
	if w, _ := glyphBox(30, ' '); w >= 30 {
		t.Error("space glyph should be narrower than its size")
	}
}

func ent(i uint32) ecs.Entity { return entity.FromID(entity.NewEntityID(i, 0)) }

func TestHitTest(t *testing.T) {
	t.Parallel()
	r := func(x, y, w, h float32) pkgui.LayoutRect {
		return pkgui.LayoutRect{Position: pkgmath.Vec2{X: x, Y: y}, Size: pkgmath.Vec2{X: w, Y: h}}
	}
	// Two overlapping nodes; the later one (entity 2) is on top.
	targets := []HitTarget{
		{Entity: ent(1), Rect: r(0, 0, 100, 100), Filter: pkgui.MouseStop},
		{Entity: ent(2), Rect: r(0, 0, 100, 100), Filter: pkgui.MouseStop},
	}
	if got := HitTest(targets, pkgmath.Vec2{X: 50, Y: 50}); !got.Hit || got.Entity != ent(2) {
		t.Errorf("top-most hit = %+v, want entity 2", got)
	}
	// Outside everything.
	if HitTest(targets, pkgmath.Vec2{X: 200, Y: 200}).Hit {
		t.Error("point outside all rects should not hit")
	}
	// Top node MouseIgnore → falls through to the node beneath.
	targets[1].Filter = pkgui.MouseIgnore
	if got := HitTest(targets, pkgmath.Vec2{X: 50, Y: 50}); !got.Hit || got.Entity != ent(1) {
		t.Errorf("MouseIgnore top → fall through to entity 1, got %+v", got)
	}
}

func TestInteractionFor(t *testing.T) {
	t.Parallel()
	if InteractionFor(true, true) != pkgui.InteractionPressed {
		t.Error("hit+down = Pressed")
	}
	if InteractionFor(true, false) != pkgui.InteractionHovered {
		t.Error("hit = Hovered")
	}
	if InteractionFor(false, true) != pkgui.InteractionNone {
		t.Error("no hit = None")
	}
}

func TestUiFeatureZSortAndDrain(t *testing.T) {
	t.Parallel()
	f := NewUiFeature()
	f.SetRects([]ExtractedRect{
		{Z: 5}, {Z: 1}, {Z: 3},
	})
	f.Prepare(nil) // sorts ascending by Z
	// After sort the feature still holds 3 rects; Draw counts them.
	f.Draw(nil, nil)
	if f.LastDrawn() != 3 {
		t.Errorf("LastDrawn = %d, want 3", f.LastDrawn())
	}
	// Verify ascending z order post-sort.
	rects := []ExtractedRect{{Z: 5}, {Z: 1}, {Z: 3}}
	SortByZ(rects)
	if rects[0].Z != 1 || rects[1].Z != 3 || rects[2].Z != 5 {
		t.Errorf("SortByZ result = %v", rects)
	}
	f.Flush(nil)
	f.Draw(nil, nil)
	if f.LastDrawn() != 0 {
		t.Error("Flush should clear the rect list")
	}
	// No-op lifecycle hooks must not panic.
	f.Initialize(nil)
	f.Collect(nil)
	f.Extract(nil)
	f.PrepareEffectPermutations(nil)
}
