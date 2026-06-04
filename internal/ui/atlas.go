package ui

import renderimage "github.com/neuengine/neu/pkg/render/image"

// glyphKey identifies a rasterized glyph by font, pixel size, and rune (INV-4:
// the same triple reuses the cached atlas region).
type glyphKey struct {
	fontID uint64
	sizePx uint32
	r      rune
}

// GlyphInfo is a cached glyph's atlas placement, horizontal advance, and the
// source 8x8 bitmap a backend uploads into the region (zero-value for a glyph
// with no pixels, e.g. space).
type GlyphInfo struct {
	Region  renderimage.AtlasRegion
	Advance float32
	Bitmap  [8]uint8
}

// FontAtlas caches rasterized glyphs in a shelf-packed DynamicAtlas (reused from
// render-core). A cache miss rasterizes the glyph once and packs it; repeated
// requests for the same (font, size, rune) return the cached region (INV-4).
//
// Bootstrap: a real TTF rasterizer (x/image/font) is gated on an ADR; until then
// glyph metrics are synthesized from the pixel size so the cache mechanics and
// the atlas packing are exercised end-to-end.
type FontAtlas struct {
	atlas      *renderimage.DynamicAtlas
	font       *BitmapFont
	glyphs     map[glyphKey]GlyphInfo
	rasterized int // cache-miss count (INV-4 verification hook)
}

// NewFontAtlas creates an atlas of the given initial dimensions, backed by the
// built-in stdlib bitmap font (the x/image/font ADR's zero-dep choice).
func NewFontAtlas(width, height uint32) *FontAtlas {
	return &FontAtlas{
		atlas:  renderimage.NewDynamicAtlas(width, height),
		font:   DefaultBitmapFont(),
		glyphs: make(map[glyphKey]GlyphInfo),
	}
}

// Font returns the atlas's backing bitmap font.
func (a *FontAtlas) Font() *BitmapFont { return a.font }

// Glyph returns the cached glyph for (fontID, sizePx, r). On a miss it resolves
// the source bitmap from the backing font, packs an atlas region, caches it, and
// counts the rasterization. The bool is false only if the atlas could not
// allocate space for the glyph.
func (a *FontAtlas) Glyph(fontID uint64, sizePx uint32, r rune) (GlyphInfo, bool) {
	key := glyphKey{fontID: fontID, sizePx: sizePx, r: r}
	if g, ok := a.glyphs[key]; ok {
		return g, true // cache hit — no rasterization (INV-4)
	}
	// Cache miss: resolve the source bitmap + pack a region for it.
	bitmap, _ := a.font.Glyph(r)
	w, h := glyphBox(sizePx, r)
	region, ok := a.atlas.Alloc(glyphName(key), w, h)
	if !ok {
		return GlyphInfo{}, false
	}
	g := GlyphInfo{Region: region, Advance: a.advance(sizePx, r), Bitmap: bitmap}
	a.glyphs[key] = g
	a.rasterized++
	return g, true
}

// advance scales the font's source-pixel advance for the rune to the requested
// render size (monospace: a printable rune advances one cell, a control rune
// advances nothing).
func (a *FontAtlas) advance(sizePx uint32, r rune) float32 {
	cell := a.font.CellSize()
	if cell == 0 {
		return 0
	}
	return float32(a.font.Advance(r)) / float32(cell) * float32(sizePx)
}

// Rasterized returns the number of cache-miss rasterizations performed.
func (a *FontAtlas) Rasterized() int { return a.rasterized }

// Len returns the number of cached glyphs.
func (a *FontAtlas) Len() int { return len(a.glyphs) }

// AtlasSize returns the backing atlas dimensions (grows as glyphs are added).
func (a *FontAtlas) AtlasSize() (w, h uint32) {
	return a.atlas.Width(), a.atlas.Height()
}

// glyphBox synthesizes a glyph's pixel box. A space is narrow; other runes are
// roughly square at the pixel size.
func glyphBox(sizePx uint32, r rune) (w, h uint32) {
	if r == ' ' {
		return sizePx / 3, sizePx
	}
	return sizePx, sizePx
}

// glyphName builds a stable atlas-region name for a glyph key.
func glyphName(k glyphKey) string {
	return string(rune(k.r)) + "#" + itoa(uint64(k.fontID)) + "@" + itoa(uint64(k.sizePx))
}

// itoa renders a uint64 without importing strconv (kept dependency-light).
func itoa(v uint64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
