package image

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// encodePNG creates a minimal in-memory PNG for golden tests.
func encodePNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// Fill with a known pattern: red top-left pixel.
	img.SetRGBA(0, 0, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic("encodePNG: " + err.Error())
	}
	return buf.Bytes()
}

// TestStdlibImageLoader_PNG_Dims verifies PNG decode produces correct dimensions
// and FormatRGBA8 (T-5T04 codec golden — PNG decode dims/format).
func TestStdlibImageLoader_PNG_Dims(t *testing.T) {
	const w, h = 4, 8
	data := encodePNG(w, h)

	var l StdlibImageLoader
	got, err := l.Load(bytes.NewReader(data), LoadSettings{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Width != w {
		t.Errorf("Width = %d, want %d", got.Width, w)
	}
	if got.Height != h {
		t.Errorf("Height = %d, want %d", got.Height, h)
	}
	// Check that FormatRGBA8 (non-zero) was assigned.
	if got.Format == 0 {
		t.Error("Format is zero (FormatInvalid)")
	}
	expectedBytes := w * h * 4
	if len(got.Data) != expectedBytes {
		t.Errorf("Data length = %d, want %d", len(got.Data), expectedBytes)
	}
	// Top-left pixel should be red (0xff, 0x00, 0x00, 0xff).
	if got.Data[0] != 0xff || got.Data[1] != 0x00 || got.Data[2] != 0x00 || got.Data[3] != 0xff {
		t.Errorf("top-left pixel = %v, want [255 0 0 255]", got.Data[:4])
	}
}

func TestStdlibImageLoader_Extensions(t *testing.T) {
	l := StdlibImageLoader{}
	exts := l.Extensions()
	found := map[string]bool{}
	for _, e := range exts {
		found[e] = true
	}
	for _, want := range []string{".png", ".jpg", ".jpeg"} {
		if !found[want] {
			t.Errorf("extension %q not reported", want)
		}
	}
}

func TestStdlibImageLoader_InvalidData(t *testing.T) {
	l := StdlibImageLoader{}
	_, err := l.Load(bytes.NewReader([]byte("not an image")), LoadSettings{})
	if err == nil {
		t.Error("expected error for invalid image data, got nil")
	}
}
