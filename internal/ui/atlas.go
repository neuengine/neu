package ui

import renderimage "github.com/neuengine/neu/pkg/render/image"

// glyphKey identifies a rasterized glyph by font, pixel size, and rune (INV-4:
// the same triple reuses the cached atlas region).
type glyphKey struct {
	fontID uint64
	sizePx uint32
	r      rune
}

// GlyphInfo is a cached glyph's atlas placement plus its horizontal advance.
type GlyphInfo struct {
	Region  renderimage.AtlasRegion
	Advance float32
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
	glyphs     map[glyphKey]GlyphInfo
	rasterized int // cache-miss count (INV-4 verification hook)
}

// NewFontAtlas creates an atlas of the given initial dimensions.
func NewFontAtlas(width, height uint32) *FontAtlas {
	return &FontAtlas{
		atlas:  renderimage.NewDynamicAtlas(width, height),
		glyphs: make(map[glyphKey]GlyphInfo),
	}
}

// Glyph returns the cached glyph for (fontID, sizePx, r). On a miss it
// synthesizes metrics, packs a region, caches it, and counts the rasterization.
// The bool is false only if the atlas could not allocate space for the glyph.
func (a *FontAtlas) Glyph(fontID uint64, sizePx uint32, r rune) (GlyphInfo, bool) {
	key := glyphKey{fontID: fontID, sizePx: sizePx, r: r}
	if g, ok := a.glyphs[key]; ok {
		return g, true // cache hit — no rasterization (INV-4)
	}
	// Cache miss: rasterize (synthesized) + pack.
	w, h := glyphBox(sizePx, r)
	region, ok := a.atlas.Alloc(glyphName(key), w, h)
	if !ok {
		return GlyphInfo{}, false
	}
	g := GlyphInfo{Region: region, Advance: float32(sizePx) * 0.6}
	a.glyphs[key] = g
	a.rasterized++
	return g, true
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
