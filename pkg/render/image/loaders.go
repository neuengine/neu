package image

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
)

// decodeStdlib decodes PNG or JPEG via the stdlib image package.
// Returns an RGBA8 Image; non-decodable formats wrap the underlying error.
func decodeStdlib(r io.Reader, hint string) (*Image, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("image decode %s: %w", hint, err)
	}
	bounds := img.Bounds()
	w := uint32(bounds.Dx())
	h := uint32(bounds.Dy())
	data := make([]byte, w*h*4)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.At(x, y)
			r, g, b, a := c.RGBA()
			i := (uint32(y-bounds.Min.Y)*w + uint32(x-bounds.Min.X)) * 4
			data[i+0] = byte(r >> 8)
			data[i+1] = byte(g >> 8)
			data[i+2] = byte(b >> 8)
			data[i+3] = byte(a >> 8)
		}
	}
	return &Image{Format: FormatRGBA8, Width: w, Height: h, MipLevels: 1, Data: data}, nil
}
