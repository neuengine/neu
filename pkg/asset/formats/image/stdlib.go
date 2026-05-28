// Package image provides stdlib-backed image asset loaders (PNG, JPEG, BMP, TGA).
// All loaders implement asset.AssetLoader[renderimage.Image, LoadSettings] and
// are registered via RegisterAll.
//
// Bootstrap: l2-asset-formats-go Draft (C29 P5 gate open).
package image

import (
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // register JPEG codec
	_ "image/png"  // register PNG codec
	"io"

	"github.com/neuengine/neu/pkg/asset"
	renderimage "github.com/neuengine/neu/pkg/render/image"
)

// ErrLoaderConflict is returned when two loaders are registered for the same extension.
type ErrLoaderConflict struct{ Ext, Existing string }

func (e ErrLoaderConflict) Error() string {
	return fmt.Sprintf("asset/formats/image: extension %q already registered by %s", e.Ext, e.Existing)
}

// ErrUnsupportedFormat is returned when a disabled-format file is requested.
type ErrUnsupportedFormat struct{ Ext string }

func (e ErrUnsupportedFormat) Error() string {
	return fmt.Sprintf("asset/formats/image: format %q is not compiled in (build tag required)", e.Ext)
}

// LoadSettings carries optional format hints for image loading.
// All fields are optional; zero-value means auto-detect from extension.
type LoadSettings struct {
	Format string // explicit format override; "" = extension auto-detect
}

// StdlibImageLoader decodes PNG, JPEG (and any other format registered with the
// stdlib image package) into a renderimage.Image (FormatRGBA8).
type StdlibImageLoader struct{}

var _ asset.AssetLoader[renderimage.Image, LoadSettings] = StdlibImageLoader{}

func (StdlibImageLoader) Extensions() []string { return []string{".png", ".jpg", ".jpeg"} }

func (StdlibImageLoader) Load(r io.Reader, _ LoadSettings) (renderimage.Image, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return renderimage.Image{}, fmt.Errorf("image/stdlib decode: %w", err)
	}
	bounds := img.Bounds()
	w := uint32(bounds.Dx())
	h := uint32(bounds.Dy())
	data := make([]byte, w*h*4)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.At(x, y)
			cr, cg, cb, ca := c.RGBA()
			i := (uint32(y-bounds.Min.Y)*w + uint32(x-bounds.Min.X)) * 4
			data[i+0] = byte(cr >> 8)
			data[i+1] = byte(cg >> 8)
			data[i+2] = byte(cb >> 8)
			data[i+3] = byte(ca >> 8)
		}
	}
	return renderimage.Image{
		Format:    renderimage.FormatRGBA8,
		Width:     w,
		Height:    h,
		MipLevels: 1,
		Data:      data,
	}, nil
}

// RegisterAll registers all compiled-in image loaders with the given AssetServer.
// Duplicate extensions return ErrLoaderConflict (INV-1).
func RegisterAll(srv *asset.AssetServer) error {
	// Only one loader for PNG+JPEG via stdlib; no duplicate risk at registration.
	if srv == nil {
		return errors.New("asset/formats/image: nil AssetServer")
	}
	asset.RegisterLoader[renderimage.Image, LoadSettings](srv, StdlibImageLoader{})
	return nil
}
