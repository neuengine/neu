package image

import "errors"

// ErrAtlasOverlap is returned by TextureAtlasLayout.Add when the new region
// overlaps an existing one (INV-5).
var ErrAtlasOverlap = errors.New("atlas: overlapping region")

// AtlasRegion is a named axis-aligned rectangle within a texture atlas.
// Min and Max are texel coordinates (left-inclusive, right-exclusive).
type AtlasRegion struct {
	Name       string
	Min, Max   [2]uint32 // [x, y]
}

// TextureAtlasLayout is an immutable set of non-overlapping named regions.
// Build with repeated calls to Add; use as an asset handle in TextureAtlas.
type TextureAtlasLayout struct {
	regions []AtlasRegion
}

// Add inserts a named region [min, max) and returns ErrAtlasOverlap if it
// overlaps any existing region (INV-5). Regions are half-open: min inclusive,
// max exclusive.
func (l *TextureAtlasLayout) Add(name string, min, max [2]uint32) error {
	for _, r := range l.regions {
		if overlaps(r.Min, r.Max, min, max) {
			return ErrAtlasOverlap
		}
	}
	l.regions = append(l.regions, AtlasRegion{Name: name, Min: min, Max: max})
	return nil
}

// Regions returns a defensive copy of all registered regions.
func (l *TextureAtlasLayout) Regions() []AtlasRegion {
	out := make([]AtlasRegion, len(l.regions))
	copy(out, l.regions)
	return out
}

// overlaps reports whether two axis-aligned rectangles [amin,amax) and [bmin,bmax)
// share any texel. Uses the standard interval overlap test.
func overlaps(amin, amax, bmin, bmax [2]uint32) bool {
	return amin[0] < bmax[0] && amax[0] > bmin[0] &&
		amin[1] < bmax[1] && amax[1] > bmin[1]
}

// ─── DynamicAtlas (shelf-pack) ────────────────────────────────────────────────

// shelf tracks one horizontal band in the DynamicAtlas.
type shelf struct {
	y      uint32 // top edge of this shelf
	h      uint32 // height of this shelf (set by the tallest packed item)
	cursor uint32 // x cursor (next free column)
}

// DynamicAtlas is a run-time growable texture atlas that uses the shelf-pack
// algorithm. When full it doubles in size and re-packs all existing allocations.
type DynamicAtlas struct {
	width, height uint32
	shelves       []shelf
	allocs        []AtlasRegion // ordered history for re-pack
}

// NewDynamicAtlas creates an atlas of the given dimensions. Both must be ≥ 1.
func NewDynamicAtlas(width, height uint32) *DynamicAtlas {
	if width == 0 {
		width = 1
	}
	if height == 0 {
		height = 1
	}
	d := &DynamicAtlas{width: width, height: height}
	d.shelves = []shelf{{y: 0, h: 0, cursor: 0}}
	return d
}

// Alloc allocates a w×h region and returns the placed AtlasRegion + true.
// Returns (AtlasRegion{}, false) if the atlas is permanently full after doubling.
func (d *DynamicAtlas) Alloc(name string, w, h uint32) (AtlasRegion, bool) {
	if w == 0 || h == 0 {
		return AtlasRegion{}, false
	}
	// Try to fit in an existing shelf.
	if r, ok := d.tryAlloc(name, w, h); ok {
		return r, true
	}
	// Double the atlas and re-pack.
	d.grow()
	if r, ok := d.tryAlloc(name, w, h); ok {
		return r, true
	}
	return AtlasRegion{}, false
}

func (d *DynamicAtlas) tryAlloc(name string, w, h uint32) (AtlasRegion, bool) {
	// Try to place in an existing shelf with enough height and width.
	for i := range d.shelves {
		s := &d.shelves[i]
		// A shelf with h==0 is a new shelf (can grow to any height up to the
		// remaining vertical space).
		available := d.height - s.y
		if s.h > 0 && s.h < h {
			continue // shelf is shorter than the item
		}
		if s.h == 0 && available < h {
			continue // not enough vertical space for a new shelf
		}
		if d.width-s.cursor < w {
			continue // not enough horizontal space in this shelf
		}
		x := s.cursor
		y := s.y
		s.cursor += w
		if s.h == 0 {
			// First item on this shelf — set its height and open the next shelf slot.
			s.h = h
			nextY := s.y + h
			if nextY < d.height {
				d.shelves = append(d.shelves, shelf{y: nextY})
			}
		}
		r := AtlasRegion{Name: name, Min: [2]uint32{x, y}, Max: [2]uint32{x + w, y + h}}
		d.allocs = append(d.allocs, r)
		return r, true
	}
	return AtlasRegion{}, false
}

// grow doubles the atlas dimensions and re-packs existing allocations.
func (d *DynamicAtlas) grow() {
	d.width *= 2
	d.height *= 2
	// Reset shelf state and re-pack.
	old := d.allocs
	d.allocs = make([]AtlasRegion, 0, len(old))
	d.shelves = []shelf{{y: 0, h: 0, cursor: 0}}
	for _, a := range old {
		w := a.Max[0] - a.Min[0]
		h := a.Max[1] - a.Min[1]
		d.tryAlloc(a.Name, w, h) // best-effort; should always fit post-double
	}
}

// Width reports the current atlas width.
func (d *DynamicAtlas) Width() uint32 { return d.width }

// Height reports the current atlas height.
func (d *DynamicAtlas) Height() uint32 { return d.height }
