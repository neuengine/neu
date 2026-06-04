package ui

import "testing"

func anyBitSet(rows [8]uint8) bool {
	for _, b := range rows {
		if b != 0 {
			return true
		}
	}
	return false
}

func TestParseGlyphBits(t *testing.T) {
	t.Parallel()
	got := parseGlyph([8]string{
		"########", // 0xFF
		"##......", // 0xC0
		"......##", // 0x03
		"...##...", // 0x18
		"........", // 0x00
		"#.......", // 0x80
		".......#", // 0x01
		"#######.", // 0xFE
	})
	want := [8]uint8{0xFF, 0xC0, 0x03, 0x18, 0x00, 0x80, 0x01, 0xFE}
	if got != want {
		t.Errorf("parseGlyph bits = %v, want %v", got, want)
	}
}

func TestBitmapFontGlyphLookup(t *testing.T) {
	t.Parallel()
	f := DefaultBitmapFont()

	// A defined letter is present and has lit pixels.
	if g, ok := f.Glyph('A'); !ok || !anyBitSet(g) {
		t.Errorf("Glyph('A') ok=%v bits-set=%v, want true/true", ok, anyBitSet(g))
	}
	// Space is defined but blank.
	if g, ok := f.Glyph(' '); !ok || anyBitSet(g) {
		t.Errorf("Glyph(' ') ok=%v bits-set=%v, want true/false (blank)", ok, anyBitSet(g))
	}
	// An undefined rune falls back to the missing-glyph box (drawable, ok=false).
	if g, ok := f.Glyph('☃'); ok || !anyBitSet(g) {
		t.Errorf("Glyph(undefined) ok=%v bits-set=%v, want false/true (fallback box)", ok, anyBitSet(g))
	}
}

func TestBitmapFontAdvance(t *testing.T) {
	t.Parallel()
	f := DefaultBitmapFont()
	if f.CellSize() != 8 {
		t.Errorf("CellSize = %d, want 8", f.CellSize())
	}
	// Monospace: every printable rune advances one cell.
	for _, r := range []rune{'A', '0', ' ', '.', '#'} {
		if a := f.Advance(r); a != 8 {
			t.Errorf("Advance(%q) = %d, want 8", r, a)
		}
	}
	// Control runes advance nothing.
	for _, r := range []rune{'\n', '\r', '\t'} {
		if a := f.Advance(r); a != 0 {
			t.Errorf("Advance(%q) = %d, want 0", r, a)
		}
	}
}

// TestBitmapFontCoverage asserts the documented set (digits + uppercase) is all
// defined and non-blank — a missing or accidentally-blank glyph would render as
// nothing, which the structural check catches.
func TestBitmapFontCoverage(t *testing.T) {
	t.Parallel()
	f := DefaultBitmapFont()
	var runes []rune
	for r := '0'; r <= '9'; r++ {
		runes = append(runes, r)
	}
	for r := 'A'; r <= 'Z'; r++ {
		runes = append(runes, r)
	}
	for _, r := range runes {
		g, ok := f.Glyph(r)
		if !ok {
			t.Errorf("glyph %q is not defined", r)
			continue
		}
		if !anyBitSet(g) {
			t.Errorf("glyph %q is blank (no lit pixels)", r)
		}
	}
}

func TestFontAtlasUsesBitmapFont(t *testing.T) {
	t.Parallel()
	a := NewFontAtlas(256, 256)
	if a.Font() == nil {
		t.Fatal("FontAtlas must have a backing font")
	}

	// A printable glyph carries the real source bitmap + a full-cell advance.
	g, ok := a.Glyph(1, 16, 'A')
	if !ok {
		t.Fatal("Glyph('A') failed to allocate")
	}
	if !anyBitSet(g.Bitmap) {
		t.Error("cached glyph 'A' has an empty bitmap")
	}
	if g.Advance != 16 {
		t.Errorf("Advance('A', 16px) = %v, want 16 (monospace = render size)", g.Advance)
	}

	// Space is monospace too (full-cell advance) but its bitmap is blank.
	sp, _ := a.Glyph(1, 16, ' ')
	if sp.Advance != 16 || anyBitSet(sp.Bitmap) {
		t.Errorf("space advance/bits = %v/%v, want 16/blank", sp.Advance, anyBitSet(sp.Bitmap))
	}

	// A newline advances nothing.
	if nl, _ := a.Glyph(1, 16, '\n'); nl.Advance != 0 {
		t.Errorf("newline advance = %v, want 0", nl.Advance)
	}
}
