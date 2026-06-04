package ui

import (
	"reflect"
	"testing"

	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
	pkgdiag "github.com/neuengine/neu/pkg/diag"
	pkgmath "github.com/neuengine/neu/pkg/math"
)

// overlayStore returns a store with the three engine metrics registered, read,
// and pushed to constant values (the EMA of a constant is exact, so the formatted
// lines are deterministic).
func overlayStore(t *testing.T) *pkgdiag.DiagnosticsStore {
	t.Helper()
	s := pkgdiag.NewDiagnosticsStore()
	for _, p := range []pkgdiag.DiagnosticPath{"engine/fps", "engine/frame_time_ms", "engine/entity_count"} {
		s.Register(pkgdiag.NewDiagnostic(p, "", 8))
		s.AddReader(p)
	}
	s.Push("engine/fps", 60)
	s.Push("engine/frame_time_ms", 16.7)
	s.Push("engine/entity_count", 3)
	return s
}

func TestDebugOverlayFormatsLines(t *testing.T) {
	t.Parallel()
	overlay := &DebugOverlay{}
	updateOverlay(overlayStore(t), NewFontAtlas(256, 256), 8, pkgmath.Vec2{}, DefaultOverlayMetrics(), overlay)

	want := []string{"FPS: 60", "FRAME MS: 16.7", "ENTITIES: 3"}
	if !reflect.DeepEqual(overlay.Lines, want) {
		t.Errorf("overlay lines = %v, want %v", overlay.Lines, want)
	}
	if len(overlay.Glyphs) == 0 {
		t.Error("overlay should have laid-out glyphs for the formatted text")
	}
}

func TestDebugOverlayNilStoreClears(t *testing.T) {
	t.Parallel()
	overlay := &DebugOverlay{Lines: []string{"stale"}, Glyphs: make([]PositionedGlyph, 3)}
	updateOverlay(nil, NewFontAtlas(256, 256), 8, pkgmath.Vec2{}, DefaultOverlayMetrics(), overlay)
	if len(overlay.Lines) != 0 || len(overlay.Glyphs) != 0 {
		t.Errorf("nil store should clear the overlay, got %d lines / %d glyphs", len(overlay.Lines), len(overlay.Glyphs))
	}
}

func TestDebugOverlayMissingMetricIsZero(t *testing.T) {
	t.Parallel()
	overlay := &DebugOverlay{}
	updateOverlay(pkgdiag.NewDiagnosticsStore(), NewFontAtlas(256, 256), 8, pkgmath.Vec2{}, DefaultOverlayMetrics(), overlay)
	if len(overlay.Lines) == 0 || overlay.Lines[0] != "FPS: 0" {
		t.Errorf("missing metric should read 0, got lines=%v", overlay.Lines)
	}
}

func TestDebugOverlayPluginBuild(t *testing.T) {
	t.Parallel()
	b := &recordingBuilder{w: world.NewWorld()}
	DebugOverlayPlugin{Store: overlayStore(t)}.Build(b)

	if b.schedule != appface.Last {
		t.Errorf("overlay system schedule = %q, want Last", b.schedule)
	}
	if b.system == nil || b.system.Name() != "ui.DebugOverlay" {
		t.Fatalf("registered system = %v, want ui.DebugOverlay", b.system)
	}
	// The registered system runs without panic and fills the published resource.
	b.system.Run(b.w)
	if op, ok := world.Resource[*DebugOverlay](b.w); !ok || len((*op).Lines) != 3 {
		t.Errorf("after Run, DebugOverlay resource should hold 3 lines (ok=%v)", ok)
	}
}
