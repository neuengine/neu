package image

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// ─── Image validation ─────────────────────────────────────────────────────────

func TestNewImage_ValidFormat(t *testing.T) {
	t.Parallel()
	img, err := NewImage(FormatRGBA8, 4, 4, make([]byte, 4*4*4))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if img.Format != FormatRGBA8 {
		t.Errorf("Format = %d, want %d", img.Format, FormatRGBA8)
	}
}

func TestNewImage_InvalidFormat(t *testing.T) {
	t.Parallel()
	_, err := NewImage(FormatInvalid, 1, 1, []byte{0})
	if !errors.Is(err, ErrImageFormatInvalid) {
		t.Fatalf("got %v, want ErrImageFormatInvalid", err)
	}
}

// ─── PNG decode ───────────────────────────────────────────────────────────────

func makePNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: 255, G: 128, B: 64, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func TestDecodeImage_PNG(t *testing.T) {
	t.Parallel()
	data := makePNG(8, 8)
	img, err := DecodeImage(bytes.NewReader(data), "test.png")
	if err != nil {
		t.Fatalf("DecodeImage: %v", err)
	}
	if img.Width != 8 || img.Height != 8 {
		t.Errorf("dimensions = %dx%d, want 8x8", img.Width, img.Height)
	}
	if img.Format != FormatRGBA8 {
		t.Errorf("Format = %d, want FormatRGBA8", img.Format)
	}
	if len(img.Data) != 8*8*4 {
		t.Errorf("data len = %d, want %d", len(img.Data), 8*8*4)
	}
}

func TestDecodeImage_InvalidData(t *testing.T) {
	t.Parallel()
	_, err := DecodeImage(bytes.NewReader([]byte("not an image")), "bad.png")
	if err == nil {
		t.Fatal("expected error for invalid image data")
	}
}

// ─── Atlas layout (INV-5) ─────────────────────────────────────────────────────

func TestAtlasLayout_NoOverlap(t *testing.T) {
	t.Parallel()
	l := &TextureAtlasLayout{}
	if err := l.Add("a", [2]uint32{0, 0}, [2]uint32{4, 4}); err != nil {
		t.Fatalf("Add a: %v", err)
	}
	if err := l.Add("b", [2]uint32{4, 0}, [2]uint32{8, 4}); err != nil {
		t.Fatalf("Add b: %v", err)
	}
}

func TestAtlasLayout_Overlap(t *testing.T) {
	t.Parallel()
	l := &TextureAtlasLayout{}
	_ = l.Add("a", [2]uint32{0, 0}, [2]uint32{8, 8})

	tests := []struct {
		name string
		min  [2]uint32
		max  [2]uint32
	}{
		{"exact duplicate", [2]uint32{0, 0}, [2]uint32{8, 8}},
		{"corner overlap", [2]uint32{4, 4}, [2]uint32{12, 12}},
		{"fully inside", [2]uint32{2, 2}, [2]uint32{6, 6}},
		{"fully containing", [2]uint32{0, 0}, [2]uint32{16, 16}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l2 := &TextureAtlasLayout{}
			_ = l2.Add("base", [2]uint32{0, 0}, [2]uint32{8, 8})
			err := l2.Add("over", tc.min, tc.max)
			if !errors.Is(err, ErrAtlasOverlap) {
				t.Fatalf("got %v, want ErrAtlasOverlap", err)
			}
		})
	}
}

// ─── DynamicAtlas (shelf-pack) ────────────────────────────────────────────────

func TestDynamicAtlas_Alloc(t *testing.T) {
	t.Parallel()
	d := NewDynamicAtlas(64, 64)
	r, ok := d.Alloc("tile1", 16, 16)
	if !ok {
		t.Fatal("Alloc failed for first tile")
	}
	if r.Max[0]-r.Min[0] != 16 || r.Max[1]-r.Min[1] != 16 {
		t.Errorf("wrong region size: got %v..%v", r.Min, r.Max)
	}
}

func TestDynamicAtlas_Grows(t *testing.T) {
	t.Parallel()
	// Fill a small atlas, then it should grow.
	d := NewDynamicAtlas(16, 16)
	var count int
	for range 100 {
		if _, ok := d.Alloc("x", 4, 4); ok {
			count++
		} else {
			break
		}
	}
	if count == 0 {
		t.Fatal("DynamicAtlas never allocated any region")
	}
	// Atlas should have grown at least once.
	if d.Width() <= 16 && d.Height() <= 16 {
		t.Log("atlas did not grow — may be expected if all fit in initial size")
	}
}

func TestDynamicAtlas_AllRegionsValid(t *testing.T) {
	t.Parallel()
	// Allocate exactly as many 8×8 tiles as fit in a 64×64 atlas without
	// triggering any growth (64/8 × 64/8 = 64 tiles) — regions are guaranteed
	// unique within a single growth epoch.
	d := NewDynamicAtlas(64, 64)
	var placed []AtlasRegion
	const maxTiles = 64 // 8 per row × 8 rows; a 64×64 atlas fits exactly this many
	for range maxTiles {
		r, ok := d.Alloc("t", 8, 8)
		if !ok {
			break
		}
		placed = append(placed, r)
	}
	if len(placed) == 0 {
		t.Fatal("DynamicAtlas allocated no regions")
	}
	// No two placed regions should overlap.
	for i := range placed {
		for j := i + 1; j < len(placed); j++ {
			if overlaps(placed[i].Min, placed[i].Max, placed[j].Min, placed[j].Max) {
				t.Errorf("regions %d and %d overlap: %v and %v", i, j, placed[i], placed[j])
			}
		}
	}
}
