// Package sprite provides 2D sprite components and camera helpers for the
// render pipeline. The Sprite2DFeature (internal/render/sprite2d) implements
// the RenderFeature interface that processes these components each frame.
//
// Bootstrap: l2-2d-rendering-go Draft (C29 P5 gate open).
package sprite

import (
	"github.com/neuengine/neu/pkg/asset"
	pkgmath "github.com/neuengine/neu/pkg/math"
	renderimage "github.com/neuengine/neu/pkg/render/image"
)

// Rect2D describes a 2D axis-aligned rectangle.
// Min is the top-left corner; Max is the bottom-right corner (in pixels or
// world units depending on context).
type Rect2D struct {
	MinX, MinY float32
	MaxX, MaxY float32
}

// Anchor defines where the sprite's origin is relative to its visual bounds.
type Anchor uint8

const (
	AnchorCenter      Anchor = iota
	AnchorTopLeft            // origin at top-left corner
	AnchorTopRight
	AnchorBottomLeft
	AnchorBottomRight
	AnchorCustom      // AnchorVec field is used
)

// Sprite is an ECS component that renders a 2D image attached to an entity.
// Sprites require a Transform component for world placement.
// A Sprite without a valid Image handle produces no draw call (INV-1).
type Sprite struct {
	Image      asset.Handle[renderimage.Image]
	Color      pkgmath.LinearRgba // multiplicative tint; default {1,1,1,1}
	FlipX      bool
	FlipY      bool
	Anchor     Anchor
	AnchorVec  pkgmath.Vec2    // used when Anchor == AnchorCustom
	CustomSize *pkgmath.Vec2   // overrides image dimensions when non-nil
	Rect       *Rect2D         // sub-region (atlas frame); nil = whole image
}

// ScaleMode controls how the center or edges of a 9-slice are scaled.
type ScaleMode uint8

const (
	ScaleModeStretch ScaleMode = iota
	ScaleModeTile
)

// TextureSlicer enables 9-slice (nine-patch) rendering for a Sprite.
// Attached alongside a Sprite, it causes the extraction system to emit nine
// quads instead of one, keeping corners fixed while edges/center scale.
type TextureSlicer struct {
	Border         pkgmath.Vec4 // top, bottom, left, right pixel widths
	CenterScale    ScaleMode
	MaxCornerScale float32 // cap on corner enlargement relative to native size
}

// SpriteMesh references a custom Mesh for non-rectangular 2D shapes.
// It passes through the same batching and sorting logic as Sprite but uses
// the mesh geometry instead of a generated quad.
type SpriteMesh struct {
	// MeshPath is the asset path of the Mesh to use. Resolved at load time.
	MeshPath string
}
