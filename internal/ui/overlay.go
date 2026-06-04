package ui

import (
	"fmt"
	"strings"

	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
	pkgdiag "github.com/neuengine/neu/pkg/diag"
	pkgmath "github.com/neuengine/neu/pkg/math"
)

// OverlayMetric is one diagnostic shown in the debug overlay: a label, the
// diagnostic path to read, and a printf format for its smoothed value.
type OverlayMetric struct {
	Label  string
	Path   pkgdiag.DiagnosticPath
	Format string // value format, e.g. "%.0f"; empty → "%.0f"
}

// DebugOverlay is the per-frame overlay render data (a World resource): the
// formatted text lines plus their laid-out glyphs. The render path draws
// Glyphs; tests inspect Lines. Empty until the overlay system runs.
type DebugOverlay struct {
	Lines  []string
	Glyphs []PositionedGlyph
}

// DefaultOverlayMetrics lists the engine's built-in metrics (T-6C04), labelled in
// uppercase since the bitmap font's covered set is uppercase + digits.
func DefaultOverlayMetrics() []OverlayMetric {
	return []OverlayMetric{
		{Label: "FPS", Path: "engine/fps", Format: "%.0f"},
		{Label: "FRAME MS", Path: "engine/frame_time_ms", Format: "%.1f"},
		{Label: "ENTITIES", Path: "engine/entity_count", Format: "%.0f"},
	}
}

// DebugOverlayPlugin renders selected diagnostics as UI text each Last schedule,
// using the bitmap font (T-6D04.2). It is opt-in (a debug aid, not in
// DefaultPlugins) and headless-safe: it produces the laid-out glyph data; a GPU
// backend draws it (like every other UI feature). It shares the DiagnosticsStore
// with the DiagnosticsPlugin (pass the same Store); a nil Store makes the system
// a no-op so the overlay degrades gracefully when diagnostics are absent.
type DebugOverlayPlugin struct {
	Store   *pkgdiag.DiagnosticsStore
	Atlas   *FontAtlas
	Metrics []OverlayMetric // nil → DefaultOverlayMetrics
	Origin  pkgmath.Vec2
	SizePx  uint32 // 0 → the font cell size (8)
}

// Build implements appface.Plugin.
func (p DebugOverlayPlugin) Build(b appface.Builder) {
	atlas := p.Atlas
	if atlas == nil {
		atlas = NewFontAtlas(256, 256)
	}
	size := p.SizePx
	if size == 0 {
		size = uint32(atlas.Font().CellSize())
	}
	metrics := p.Metrics
	if metrics == nil {
		metrics = DefaultOverlayMetrics()
	}

	overlay := &DebugOverlay{}
	b.SetResource(overlay)
	b.AddSystem(appface.Last, scheduler.NewFuncSystem("ui.DebugOverlay", func(_ *world.World) {
		updateOverlay(p.Store, atlas, size, p.Origin, metrics, overlay)
	}))
}

// updateOverlay reads each metric's smoothed value, formats a line, and lays the
// joined text out into glyphs. No-op (cleared overlay) when the store is absent.
func updateOverlay(store *pkgdiag.DiagnosticsStore, atlas *FontAtlas, size uint32, origin pkgmath.Vec2, metrics []OverlayMetric, out *DebugOverlay) {
	out.Lines = out.Lines[:0]
	out.Glyphs = out.Glyphs[:0]
	if store == nil {
		return
	}
	for _, m := range metrics {
		var v float64
		if d, ok := store.Get(m.Path); ok {
			v = d.SmoothedAverage()
		}
		format := m.Format
		if format == "" {
			format = "%.0f"
		}
		out.Lines = append(out.Lines, m.Label+": "+fmt.Sprintf(format, v))
	}
	out.Glyphs = LayoutText(atlas, 0, size, origin, strings.Join(out.Lines, "\n"))
}

var _ appface.Plugin = DebugOverlayPlugin{}
