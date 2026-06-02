package diag

import pkgmath "github.com/neuengine/neu/pkg/math"

// circleSegments is the line-segment resolution for curved gizmos.
const circleSegments = 24

// GizmoVertex is one endpoint of a gizmo line segment. Gizmo geometry is a flat
// line list: vertices come in consecutive pairs.
type GizmoVertex struct {
	Position pkgmath.Vec3
	Color    pkgmath.LinearRgba
}

// GizmoText is a world-space text label queued for the gizmo pass.
type GizmoText struct {
	Text     string
	Color    pkgmath.LinearRgba
	Position pkgmath.Vec3
}

// Gizmos is the immediate-mode debug-drawing API (L1 §4.6). Calls append to a
// per-frame buffer and never mutate the world (INV-2). A dedicated render pass
// draws the buffer after the scene and before UI; geometry is discarded each
// frame.
type Gizmos interface {
	Line(start, end pkgmath.Vec3, color pkgmath.LinearRgba)
	Ray(origin, dir pkgmath.Vec3, color pkgmath.LinearRgba)
	Arrow(start, end pkgmath.Vec3, color pkgmath.LinearRgba)
	Box(center, halfExtents pkgmath.Vec3, color pkgmath.LinearRgba)
	Circle(center, normal pkgmath.Vec3, radius float32, color pkgmath.LinearRgba)
	Sphere(center pkgmath.Vec3, radius float32, color pkgmath.LinearRgba)
	Grid(origin, normal pkgmath.Vec3, cell float32, count int, color pkgmath.LinearRgba)
	Text(pos pkgmath.Vec3, text string, color pkgmath.LinearRgba)
}

// GizmoBuffer is the default Gizmos implementation: a reusable line list plus a
// text list. Reset rewinds in place (0-alloc reuse across frames, mirroring the
// render-core pooled-buffer pattern).
type GizmoBuffer struct {
	lines []GizmoVertex
	texts []GizmoText
}

// NewGizmoBuffer returns an empty buffer.
func NewGizmoBuffer() *GizmoBuffer { return &GizmoBuffer{} }

// Line appends a single segment.
func (b *GizmoBuffer) Line(start, end pkgmath.Vec3, color pkgmath.LinearRgba) {
	b.lines = append(b.lines, GizmoVertex{start, color}, GizmoVertex{end, color})
}

// Ray draws a segment from origin along dir.
func (b *GizmoBuffer) Ray(origin, dir pkgmath.Vec3, color pkgmath.LinearRgba) {
	b.Line(origin, origin.Add(dir), color)
}

// Arrow draws a segment plus a small two-segment arrowhead at the end.
func (b *GizmoBuffer) Arrow(start, end pkgmath.Vec3, color pkgmath.LinearRgba) {
	b.Line(start, end, color)
	dir := end.Sub(start)
	length := dir.Length()
	if length == 0 {
		return
	}
	n := dir.Mul(1 / length)
	u, v := basis(n)
	head := length * 0.15
	back := end.Sub(n.Mul(head))
	b.Line(end, back.Add(u.Mul(head*0.5)), color)
	b.Line(end, back.Add(v.Mul(head*0.5)), color)
}

// Box draws the 12 edges of an axis-aligned box.
func (b *GizmoBuffer) Box(center, halfExtents pkgmath.Vec3, color pkgmath.LinearRgba) {
	hx, hy, hz := halfExtents.X, halfExtents.Y, halfExtents.Z
	// 8 corners.
	c := [8]pkgmath.Vec3{
		center.Add(pkgmath.Vec3{X: -hx, Y: -hy, Z: -hz}),
		center.Add(pkgmath.Vec3{X: hx, Y: -hy, Z: -hz}),
		center.Add(pkgmath.Vec3{X: hx, Y: hy, Z: -hz}),
		center.Add(pkgmath.Vec3{X: -hx, Y: hy, Z: -hz}),
		center.Add(pkgmath.Vec3{X: -hx, Y: -hy, Z: hz}),
		center.Add(pkgmath.Vec3{X: hx, Y: -hy, Z: hz}),
		center.Add(pkgmath.Vec3{X: hx, Y: hy, Z: hz}),
		center.Add(pkgmath.Vec3{X: -hx, Y: hy, Z: hz}),
	}
	edges := [12][2]int{
		{0, 1}, {1, 2}, {2, 3}, {3, 0}, // bottom
		{4, 5}, {5, 6}, {6, 7}, {7, 4}, // top
		{0, 4}, {1, 5}, {2, 6}, {3, 7}, // verticals
	}
	for _, e := range edges {
		b.Line(c[e[0]], c[e[1]], color)
	}
}

// Circle draws a circle of the given radius in the plane defined by normal.
func (b *GizmoBuffer) Circle(center, normal pkgmath.Vec3, radius float32, color pkgmath.LinearRgba) {
	u, v := basis(normal.Normalize())
	var prev pkgmath.Vec3
	for i := 0; i <= circleSegments; i++ {
		ang := float32(i) / circleSegments * 2 * pi
		p := center.Add(u.Mul(radius * cos32(ang))).Add(v.Mul(radius * sin32(ang)))
		if i > 0 {
			b.Line(prev, p, color)
		}
		prev = p
	}
}

// Sphere approximates a sphere with three orthogonal great circles.
func (b *GizmoBuffer) Sphere(center pkgmath.Vec3, radius float32, color pkgmath.LinearRgba) {
	b.Circle(center, pkgmath.Vec3{X: 1}, radius, color)
	b.Circle(center, pkgmath.Vec3{Y: 1}, radius, color)
	b.Circle(center, pkgmath.Vec3{Z: 1}, radius, color)
}

// Grid draws a count×count line grid centered at origin in the plane of normal.
func (b *GizmoBuffer) Grid(origin, normal pkgmath.Vec3, cell float32, count int, color pkgmath.LinearRgba) {
	if count <= 0 {
		return
	}
	u, v := basis(normal.Normalize())
	half := float32(count) * cell / 2
	for i := 0; i <= count; i++ {
		off := -half + float32(i)*cell
		// Lines parallel to v.
		b.Line(origin.Add(u.Mul(off)).Add(v.Mul(-half)), origin.Add(u.Mul(off)).Add(v.Mul(half)), color)
		// Lines parallel to u.
		b.Line(origin.Add(v.Mul(off)).Add(u.Mul(-half)), origin.Add(v.Mul(off)).Add(u.Mul(half)), color)
	}
}

// Text queues a world-space text label.
func (b *GizmoBuffer) Text(pos pkgmath.Vec3, text string, color pkgmath.LinearRgba) {
	b.texts = append(b.texts, GizmoText{Position: pos, Text: text, Color: color})
}

// Lines returns the accumulated line-list vertices (pairs).
func (b *GizmoBuffer) Lines() []GizmoVertex { return b.lines }

// Texts returns the accumulated text labels.
func (b *GizmoBuffer) Texts() []GizmoText { return b.texts }

// Reset rewinds the buffer for the next frame without releasing capacity.
func (b *GizmoBuffer) Reset() {
	b.lines = b.lines[:0]
	b.texts = b.texts[:0]
}

// Ensure GizmoBuffer satisfies the Gizmos contract.
var _ Gizmos = (*GizmoBuffer)(nil)

// basis returns two unit vectors orthogonal to n and to each other.
func basis(n pkgmath.Vec3) (u, v pkgmath.Vec3) {
	// Pick a reference axis not parallel to n.
	ref := pkgmath.Vec3{X: 1}
	if abs32(n.X) > 0.9 {
		ref = pkgmath.Vec3{Y: 1}
	}
	u = n.Cross(ref).Normalize()
	v = n.Cross(u).Normalize()
	return u, v
}
