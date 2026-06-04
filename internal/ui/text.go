package ui

import (
	pkgmath "github.com/neuengine/neu/pkg/math"
	pkgui "github.com/neuengine/neu/pkg/ui"
)

// PositionedGlyph is one laid-out glyph: its atlas placement + source bitmap
// (from FontAtlas.Glyph) and the screen-space rect a backend draws it into.
type PositionedGlyph struct {
	Glyph GlyphInfo
	Rect  pkgui.LayoutRect
	Rune  rune
}

// LayoutText lays s out into positioned glyphs via the font atlas, advancing a
// monospace pen from origin. A '\n' starts a new line (the pen drops by the line
// height and returns to origin.X); other control runes are skipped. Each glyph's
// rect is sizePx square at the pen position; the pen then advances by the glyph's
// (size-scaled) advance. The atlas caches glyphs, so repeated text is cheap (INV-4).
func LayoutText(atlas *FontAtlas, fontID uint64, sizePx uint32, origin pkgmath.Vec2, s string) []PositionedGlyph {
	if atlas == nil || sizePx == 0 {
		return nil
	}
	out := make([]PositionedGlyph, 0, len(s))
	lineH := float32(sizePx) + lineGap
	penX, penY := origin.X, origin.Y
	for _, r := range s {
		if r == '\n' {
			penX = origin.X
			penY += lineH
			continue
		}
		if r < ' ' {
			continue // skip other control runes
		}
		g, ok := atlas.Glyph(fontID, sizePx, r)
		if !ok {
			continue // atlas could not allocate — skip rather than overlap
		}
		out = append(out, PositionedGlyph{
			Glyph: g,
			Rune:  r,
			Rect: pkgui.LayoutRect{
				Position: pkgmath.Vec2{X: penX, Y: penY},
				Size:     pkgmath.Vec2{X: float32(sizePx), Y: float32(sizePx)},
			},
		})
		penX += g.Advance
	}
	return out
}

// lineGap is the vertical padding added between text lines.
const lineGap float32 = 2
