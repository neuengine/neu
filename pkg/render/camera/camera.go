// Package camera provides camera components, projection matrices, three-layer
// visibility (Visibility/InheritedVisibility/ViewVisibility), and frustum
// extraction for the render pipeline.
//
// Bootstrap: l2-camera-and-visibility-go Draft (C29 P4 gate open).
package camera

import (
	"github.com/neuengine/neu/pkg/asset"
	renderimage "github.com/neuengine/neu/pkg/render/image"

	pkgmath "github.com/neuengine/neu/pkg/math"
	"github.com/neuengine/neu/pkg/ecs"
)

// TargetKind discriminates the render destination of a Camera.
type TargetKind uint8

const (
	TargetWindow    TargetKind = iota // output to a window surface
	TargetTexture                     // render-to-texture (RTT)
	TargetOffScreen                   // off-screen framebuffer (headless/testing)
)

// RenderTarget describes where a Camera writes its output.
type RenderTarget struct {
	Kind   TargetKind
	Window ecs.Entity                            // valid when Kind == TargetWindow
	Image  asset.Handle[renderimage.Image]       // valid when Kind == TargetTexture
}

// Viewport specifies the normalised sub-rectangle [0,1]² of the render target
// that the camera occupies. Nil (absent) means the full target.
type Viewport struct {
	// Min and Max are normalised (0..1) coordinates; Y increases downward.
	MinX, MinY, MaxX, MaxY float32
}

// ClearMode controls how the camera's target is cleared before rendering.
type ClearMode uint8

const (
	ClearCustom  ClearMode = iota // clear with ClearColorConfig.Color
	ClearNone                     // do not clear (overdraw)
	ClearInherit                  // inherit from the previous camera or pass
)

// ClearColorConfig specifies the clear behaviour for a camera target.
type ClearColorConfig struct {
	Mode  ClearMode
	Color pkgmath.LinearRgba
}

// Camera is a pure-data ECS component that describes how and where to render a view.
// Order determines draw sequence; lower values render first.
// Multiple active cameras produce multiple render passes (split-screen, RTT, minimaps).
type Camera struct {
	Target   RenderTarget
	Viewport *Viewport         // nil = full target
	Clear    ClearColorConfig
	HDR      bool
	Order    int32              // lower renders first; equal Order → EntityID tiebreak (INV-4)
	IsActive bool
}
