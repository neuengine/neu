// Package diag wires the diagnostic subsystem into the render pipeline: the
// GizmoFeature drains the immediate-mode gizmo buffer each frame. The store,
// ring buffer, gizmo API, slog filter, and profiling spans live in pkg/diag;
// this package holds the render-feature integration (which imports internal
// render types).
//
// Bootstrap: l2-diagnostic-system-go Draft (Phase 6 Track C).
package diag

import (
	render "github.com/neuengine/neu/internal/render"
	pkgdiag "github.com/neuengine/neu/pkg/diag"
)

// GizmoFeature draws the per-frame gizmo buffer after the scene and before UI
// (INV-2: pure visual, no world mutation). Headless: Draw issues no GPU calls;
// the buffer is reset in Flush so frame N's gizmos never persist into frame N+1.
type GizmoFeature struct {
	buf       *pkgdiag.GizmoBuffer
	lastLines int // line-vertex count that would have been drawn last frame
	lastTexts int // text-label count last frame
}

// NewGizmoFeature returns a feature that drains buf.
func NewGizmoFeature(buf *pkgdiag.GizmoBuffer) *GizmoFeature {
	return &GizmoFeature{buf: buf}
}

// Initialize is a no-op for gizmos.
func (f *GizmoFeature) Initialize(*render.RenderSubApp) {}

// Collect is a no-op: gizmo geometry is produced by main-world systems.
func (f *GizmoFeature) Collect(*render.CollectContext) {}

// Extract is a no-op in the headless buffer-sharing model.
func (f *GizmoFeature) Extract(*render.ExtractContext) {}

// PrepareEffectPermutations is a no-op (gizmos use a single line shader).
func (f *GizmoFeature) PrepareEffectPermutations(*render.PrepareContext) {}

// Prepare is a no-op: the buffer is already in line-list form.
func (f *GizmoFeature) Prepare(*render.PrepareContext) {}

// Draw records what would be drawn (headless has no GPU backend) — the line
// list and text labels accumulated this frame.
func (f *GizmoFeature) Draw(_ *render.DrawContext, _ *render.RenderView) {
	f.lastLines = len(f.buf.Lines())
	f.lastTexts = len(f.buf.Texts())
}

// Flush clears the buffer so the next frame starts empty (gizmos are one-frame).
func (f *GizmoFeature) Flush(*render.FlushContext) { f.buf.Reset() }

// LastLineVertexCount reports the line-vertex count from the most recent Draw.
func (f *GizmoFeature) LastLineVertexCount() int { return f.lastLines }

// LastTextCount reports the text-label count from the most recent Draw.
func (f *GizmoFeature) LastTextCount() int { return f.lastTexts }

var _ render.RenderFeature = (*GizmoFeature)(nil)
