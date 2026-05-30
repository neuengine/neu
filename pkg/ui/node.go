package ui

import pkgmath "github.com/neuengine/neu/pkg/math"

// Node is the base UI marker. A node entity pairs it with a Style and receives a
// computed LayoutRect. Node itself renders nothing.
type Node struct{}

// LayoutRect is the solver's output for a node: position (top-left) and size in
// logical pixels, relative to the viewport.
type LayoutRect struct {
	Position pkgmath.Vec2
	Size     pkgmath.Vec2
}

// Contains reports whether point p (logical pixels) lies inside the rect.
func (r LayoutRect) Contains(p pkgmath.Vec2) bool {
	return p.X >= r.Position.X && p.X <= r.Position.X+r.Size.X &&
		p.Y >= r.Position.Y && p.Y <= r.Position.Y+r.Size.Y
}

// ZIndex overrides the default painter's-order draw order. UseGlobal selects the
// Global value (relative to all UI nodes) over Local (relative to siblings).
type ZIndex struct {
	Local     int32
	Global    int32
	UseGlobal bool
}

// Effective returns the z value to sort by.
func (z ZIndex) Effective() int32 {
	if z.UseGlobal {
		return z.Global
	}
	return z.Local
}

// BackgroundColor is a solid fill behind a node (visual, not layout).
type BackgroundColor struct{ Color pkgmath.LinearRgba }

// BorderColor colors the node's border (width from Style.Margin/Border).
type BorderColor struct{ Color pkgmath.LinearRgba }
