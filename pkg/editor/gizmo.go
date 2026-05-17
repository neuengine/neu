package editor

import neu "github.com/neuengine/neu/pkg/math"

// GizmoHit describes the result of a ray intersection with a gizmo.
type GizmoHit struct {
	Distance float32
	Handle   string // which gizmo sub-handle was hit (e.g. "axis.x", "ring.y")
}

// GizmoWriter is the drawing API passed to GizmoPlugin.Draw. Defined as an
// interface so pkg/editor stays implementation-free (INV-3).
type GizmoWriter interface {
	Line(from, to neu.Vec3, color neu.LinearRgba)
	Sphere(center neu.Vec3, radius float32, color neu.LinearRgba)
}
