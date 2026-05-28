// Package image provides GPU-ready texture assets: decoded pixel data,
// sampler descriptors, mip metadata, and texture atlases.
//
// Bootstrap: l2-mesh-and-image-go Draft (C29 P4 gate open).
package image

import (
	"errors"
	"io"
)

// ImageFormat enumerates GPU-compatible texel formats.
type ImageFormat uint8

const (
	FormatInvalid  ImageFormat = iota
	FormatRGBA8                // 4 bytes/texel, sRGB or linear
	FormatRGBA16F              // 8 bytes/texel, HDR
	FormatRG11B10F             // 4 bytes/texel, packed HDR
	FormatDepth32F             // 4 bytes/texel, depth attachment
	FormatBC7                  // GPU block-compression, 8 bits/texel average
	FormatASTC4x4              // ASTC 4×4, 8 bits/texel average
)

// FilterMode describes texture sampling filter.
type FilterMode uint8

const (
	FilterLinear FilterMode = iota
	FilterNearest
)

// WrapMode describes texture coordinate wrapping.
type WrapMode uint8

const (
	WrapRepeat WrapMode = iota
	WrapClamp
	WrapMirror
)

// SamplerDesc describes GPU sampler state.
type SamplerDesc struct {
	MinFilter  FilterMode
	MagFilter  FilterMode
	WrapU      WrapMode
	WrapV      WrapMode
	MipLevels  uint32 // 0 means auto (all mips)
	Anisotropy uint8  // 0 = disabled
}

// Image holds decoded pixel data and GPU format metadata.
// Immutable once created — a mutation produces a new Image with a new AssetID.
type Image struct {
	Format    ImageFormat
	Width     uint32
	Height    uint32
	MipLevels uint32
	Data      []byte
	Sampler   SamplerDesc
}

// ErrImageFormatInvalid is returned when FormatInvalid is given to NewImage.
var ErrImageFormatInvalid = errors.New("image: invalid GPU format")

// NewImage validates the format and constructs an Image.
func NewImage(f ImageFormat, w, h uint32, data []byte) (*Image, error) {
	if f == FormatInvalid {
		return nil, ErrImageFormatInvalid
	}
	return &Image{Format: f, Width: w, Height: h, MipLevels: 1, Data: data}, nil
}

// DecodeImage decodes PNG or JPEG bytes into an RGBA8 Image.
// hint is a filename or MIME type used only for error messages.
// Runs on the caller's goroutine; callers wrapping this for async use should
// dispatch via the asset system's IOPool.
func DecodeImage(r io.Reader, hint string) (*Image, error) {
	return decodeStdlib(r, hint)
}
