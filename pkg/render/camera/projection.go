package camera

import (
	"errors"

	pkgmath "github.com/neuengine/neu/pkg/math"
)

// ScalingMode controls how an orthographic projection adapts to viewport size.
type ScalingMode uint8

const (
	// ScalingWindowSize scales 1:1 with window pixels — no aspect correction.
	ScalingWindowSize ScalingMode = iota
	// ScalingFixedVertical keeps the vertical extent constant; width adapts.
	ScalingFixedVertical
	// ScalingFixedHorizontal keeps the horizontal extent constant; height adapts.
	ScalingFixedHorizontal
)

// Sentinel errors for projection validation.
var (
	// ErrInvalidNearPlane is returned by PerspectiveProjection.Matrix when near ≤ 0.
	ErrInvalidNearPlane = errors.New("camera: perspective near plane must be > 0")
	// ErrDegenerateOrtho is returned by OrthographicProjection.Matrix when the
	// viewing area has zero width or height.
	ErrDegenerateOrtho = errors.New("camera: orthographic area is degenerate")
)

// PerspectiveProjection describes a standard perspective (pinhole) camera.
type PerspectiveProjection struct {
	FovY   float32 // vertical field-of-view in radians
	Aspect float32 // width / height; callers set from viewport dimensions
	Near   float32 // must be > 0 (INV-5)
	Far    float32
}

// Matrix returns the column-major projection matrix for this perspective camera.
// Returns ErrInvalidNearPlane if Near ≤ 0; the camera is skipped rather than
// producing a degenerate matrix (INV-5).
func (p PerspectiveProjection) Matrix() (pkgmath.Mat4, error) {
	if p.Near <= 0 {
		return pkgmath.Mat4{}, ErrInvalidNearPlane
	}
	return pkgmath.Mat4Perspective(p.FovY, p.Aspect, p.Near, p.Far), nil
}

// OrthographicProjection describes a parallel (orthographic) camera.
// Area covers the visible world-space rectangle. ScalingMode controls how it
// adapts when the viewport changes size.
type OrthographicProjection struct {
	Left, Right float32 // horizontal extent in world units
	Bottom, Top float32 // vertical extent in world units
	Near, Far   float32
	Scaling     ScalingMode
}

// Matrix returns the column-major orthographic projection matrix.
// Returns ErrDegenerateOrtho when the visible area has zero or inverted extent.
func (p OrthographicProjection) Matrix() (pkgmath.Mat4, error) {
	if p.Right-p.Left == 0 || p.Top-p.Bottom == 0 {
		return pkgmath.Mat4{}, ErrDegenerateOrtho
	}
	return pkgmath.Mat4Orthographic(p.Left, p.Right, p.Bottom, p.Top, p.Near, p.Far), nil
}
