package ui

import (
	"github.com/neuengine/neu/pkg/asset"
	pkgmath "github.com/neuengine/neu/pkg/math"
	renderimage "github.com/neuengine/neu/pkg/render/image"
)

// TextAlignment is the horizontal alignment of wrapped text lines.
type TextAlignment uint8

const (
	TextAlignLeft TextAlignment = iota
	TextAlignCenter
	TextAlignRight
)

// TextWrapping controls line breaking.
type TextWrapping uint8

const (
	WordWrap TextWrapping = iota
	CharWrap
	NoWrap
)

// Font is a loaded typeface asset (glyph outlines + metrics).
type Font struct {
	Name      string
	PixelSize float32 // nominal rasterization size
}

// TextSection is a run of text with a single font/size/color.
type TextSection struct {
	Value    string
	Font     asset.Handle[Font]
	FontSize float32
	Color    pkgmath.LinearRgba
}

// Text renders one or more styled text sections within a node (L1 §4.6).
type Text struct {
	Sections   []TextSection
	Alignment  TextAlignment
	Wrapping   TextWrapping
	LineHeight float32
}

// PlainText builds a single-section Text with one font/size/color.
func PlainText(s string, font asset.Handle[Font], size float32, color pkgmath.LinearRgba) Text {
	return Text{Sections: []TextSection{{Value: s, Font: font, FontSize: size, Color: color}}}
}

// ImageMode controls how an image fills its node.
type ImageMode uint8

const (
	// ImageStretch fills the node, ignoring aspect ratio.
	ImageStretch ImageMode = iota
	// ImageFit preserves aspect ratio within the node.
	ImageFit
)

// ImageNode renders an image asset inside a node.
type ImageNode struct {
	Image asset.Handle[renderimage.Image]
	Mode  ImageMode
}
