package ui

import (
	"testing"

	pkgmath "github.com/neuengine/neu/pkg/math"
)

func TestLayoutTextMonospaceAdvance(t *testing.T) {
	t.Parallel()
	a := NewFontAtlas(256, 256)
	gs := LayoutText(a, 0, 8, pkgmath.Vec2{}, "AB")
	if len(gs) != 2 {
		t.Fatalf("LayoutText(\"AB\") = %d glyphs, want 2", len(gs))
	}
	if gs[0].Rune != 'A' || gs[0].Rect.Position.X != 0 {
		t.Errorf("glyph0 = %q@%v, want A@0", gs[0].Rune, gs[0].Rect.Position.X)
	}
	if gs[1].Rune != 'B' || gs[1].Rect.Position.X != 8 {
		t.Errorf("glyph1 = %q@%v, want B@8 (one cell advance)", gs[1].Rune, gs[1].Rect.Position.X)
	}
}

func TestLayoutTextNewline(t *testing.T) {
	t.Parallel()
	a := NewFontAtlas(256, 256)
	gs := LayoutText(a, 0, 8, pkgmath.Vec2{X: 5, Y: 5}, "A\nB")
	if len(gs) != 2 {
		t.Fatalf("len = %d, want 2", len(gs))
	}
	// B starts a new line: x back to origin.X, y down by sizePx + lineGap.
	wantY := float32(5) + 8 + lineGap
	if gs[1].Rect.Position.X != 5 || gs[1].Rect.Position.Y != wantY {
		t.Errorf("B position = %v, want {5 %v}", gs[1].Rect.Position, wantY)
	}
}

func TestLayoutTextControlAndEdges(t *testing.T) {
	t.Parallel()
	a := NewFontAtlas(256, 256)

	// A tab between letters is skipped (no glyph, no advance): B lands at one cell.
	gs := LayoutText(a, 0, 8, pkgmath.Vec2{}, "A\tB")
	if len(gs) != 2 || gs[1].Rect.Position.X != 8 {
		t.Errorf("\"A\\tB\" = %d glyphs, B@%v; want 2, B@8", len(gs), gs[1].Rect.Position.X)
	}
	// Empty string → no glyphs.
	if g := LayoutText(a, 0, 8, pkgmath.Vec2{}, ""); len(g) != 0 {
		t.Errorf("empty string = %d glyphs, want 0", len(g))
	}
	// Defensive: nil atlas or zero size → nil.
	if LayoutText(nil, 0, 8, pkgmath.Vec2{}, "A") != nil {
		t.Error("nil atlas should return nil")
	}
	if LayoutText(a, 0, 0, pkgmath.Vec2{}, "A") != nil {
		t.Error("zero size should return nil")
	}
}
