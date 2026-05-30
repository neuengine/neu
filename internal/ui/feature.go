package ui

import (
	render "github.com/neuengine/neu/internal/render"
	pkgui "github.com/neuengine/neu/pkg/ui"
)

// ExtractedRect is a UI node's draw record in the render world: its computed
// rect plus its resolved z for ordering.
type ExtractedRect struct {
	Rect pkgui.LayoutRect
	Z    int32
}

// UiFeature composites the UI pass after all scene/2D features (INV-2), using an
// orthographic projection matching the viewport. Rects are drawn in z order
// (painter's order + ZIndex override). Headless: Draw counts what would be
// drawn (no GPU backend in CI); Flush clears the per-frame list.
type UiFeature struct {
	rects     []ExtractedRect
	lastDrawn int
}

// NewUiFeature returns an empty UI feature.
func NewUiFeature() *UiFeature { return &UiFeature{} }

// SetRects installs the z-sorted rects extracted for this frame.
func (f *UiFeature) SetRects(rects []ExtractedRect) { f.rects = rects }

// Initialize is a no-op for the UI feature.
func (f *UiFeature) Initialize(*render.RenderSubApp) {}

// Collect enumerates UI cameras (no-op in the headless model).
func (f *UiFeature) Collect(*render.CollectContext) {}

// Extract copies laid-out UI nodes into the render world (driven externally via
// SetRects in the headless model).
func (f *UiFeature) Extract(*render.ExtractContext) {}

// PrepareEffectPermutations is a no-op (UI uses a single quad shader).
func (f *UiFeature) PrepareEffectPermutations(*render.PrepareContext) {}

// Prepare sorts the extracted rects by z (painter's order + ZIndex).
func (f *UiFeature) Prepare(*render.PrepareContext) { SortByZ(f.rects) }

// Draw composites the UI quads. Headless: record the count that would be drawn.
func (f *UiFeature) Draw(_ *render.DrawContext, _ *render.RenderView) {
	f.lastDrawn = len(f.rects)
}

// Flush clears the per-frame rect list.
func (f *UiFeature) Flush(*render.FlushContext) { f.rects = f.rects[:0] }

// LastDrawn reports the rect count from the most recent Draw.
func (f *UiFeature) LastDrawn() int { return f.lastDrawn }

// SortByZ orders rects ascending by z (lower z drawn first, higher on top). It
// is a stable insertion sort so equal-z nodes keep painter's (traversal) order.
func SortByZ(rects []ExtractedRect) {
	for i := 1; i < len(rects); i++ {
		j := i
		for j > 0 && rects[j-1].Z > rects[j].Z {
			rects[j-1], rects[j] = rects[j], rects[j-1]
			j--
		}
	}
}

var _ render.RenderFeature = (*UiFeature)(nil)
