package hierarchy

import (
	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/query"
	"github.com/neuengine/neu/internal/ecs/world"
	neu "github.com/neuengine/neu/pkg/math"
)

// Transform represents the local position, rotation, and scale of an entity
// relative to its parent (or world origin for root entities).
// Mutate this component directly; the propagation system writes GlobalTransform.
type Transform struct {
	Translation neu.Vec3
	Rotation    neu.Quat
	Scale       neu.Vec3
}

// Required declares GlobalTransform as a required companion component.
// When Transform is added to an entity, GlobalTransform is auto-injected
// with an identity value if not already present.
func (Transform) Required() []component.Data {
	return []component.Data{{Value: GlobalTransform{}}}
}

// NewTransform returns a Transform at the origin with identity rotation and
// uniform scale of 1.
func NewTransform() Transform {
	return Transform{
		Rotation: neu.QuatIdentity(),
		Scale:    neu.Vec3{X: 1, Y: 1, Z: 1},
	}
}

// FromTranslation creates a Transform at the given position with identity
// rotation and uniform scale of 1.
func FromTranslation(t neu.Vec3) Transform {
	return Transform{
		Translation: t,
		Rotation:    neu.QuatIdentity(),
		Scale:       neu.Vec3{X: 1, Y: 1, Z: 1},
	}
}

// FromRotation creates a Transform at the origin with the given rotation
// and uniform scale of 1.
func FromRotation(r neu.Quat) Transform {
	return Transform{
		Rotation: r,
		Scale:    neu.Vec3{X: 1, Y: 1, Z: 1},
	}
}

// ToAffine3A converts this local transform into an Affine3A matrix using
// TRS decomposition: scale → rotate → translate.
func (t Transform) ToAffine3A() neu.Affine3A {
	return neu.FromTRS(t.Translation, t.Rotation, t.Scale)
}

// LookAt returns a Transform at eye that faces target using the given up vector.
func LookAt(eye, target, up neu.Vec3) Transform {
	m := neu.LookAt(eye, target, up)
	return Transform{
		Translation: eye,
		Rotation:    neu.QuatFromRotMat3(m.XAxis, m.YAxis, m.ZAxis),
		Scale:       neu.Vec3{X: 1, Y: 1, Z: 1},
	}
}

// GlobalTransform is the computed world-space affine transform.
// Read-only for user systems — written by the propagation system each frame.
// Added automatically when Transform is added (required component pattern).
type GlobalTransform struct {
	matrix neu.Affine3A
}

// NewGlobalTransform returns an identity GlobalTransform.
func NewGlobalTransform() GlobalTransform {
	return GlobalTransform{matrix: neu.Affine3AIdentity()}
}

// FromAffine3A wraps an existing affine matrix in a GlobalTransform.
func FromAffine3A(m neu.Affine3A) GlobalTransform {
	return GlobalTransform{matrix: m}
}

// Affine3A returns the underlying world-space affine matrix.
func (g GlobalTransform) Affine3A() neu.Affine3A { return g.matrix }

// Translation extracts the world-space translation vector.
func (g GlobalTransform) Translation() neu.Vec3 { return g.matrix.Translation }

// Right returns the local X-axis direction in world space.
func (g GlobalTransform) Right() neu.Vec3 { return g.matrix.XAxis }

// Up returns the local Y-axis direction in world space.
func (g GlobalTransform) Up() neu.Vec3 { return g.matrix.YAxis }

// Forward returns the local negative-Z-axis direction in world space.
func (g GlobalTransform) Forward() neu.Vec3 { return g.matrix.ZAxis.Neg() }

// Mul combines two GlobalTransforms: parent * child (apply child then parent).
func (g GlobalTransform) Mul(child GlobalTransform) GlobalTransform {
	return GlobalTransform{matrix: g.matrix.Mul(child.matrix)}
}

// MulTransform computes the world-space transform of a child entity:
// GlobalTransform(child) = GlobalTransform(parent) * Transform(child)
func (g GlobalTransform) MulTransform(local Transform) GlobalTransform {
	return GlobalTransform{matrix: g.matrix.Mul(local.ToAffine3A())}
}

// ── Propagation system ────────────────────────────────────────────────────────

// propagateTransforms is the system function registered in the PostUpdate
// schedule. It finds all root entities (Transform + GlobalTransform, no ChildOf)
// and propagates world-space transforms down through the hierarchy.
func propagateTransforms(w *world.World) {
	// Query root entities: have Transform + GlobalTransform, no ChildOf.
	q, err := query.NewQuery2[Transform, GlobalTransform](w, query.Without[ChildOf]{})
	if err != nil {
		return
	}
	for e, t := range q.All(w) {
		gt := GlobalTransform{matrix: t.A.ToAffine3A()}
		*t.B = gt
		propagateToChildren(w, e, gt)
	}
}

// propagateToChildren recursively updates GlobalTransform on every descendant
// of parent using the provided world-space parent transform.
func propagateToChildren(w *world.World, parent entity.Entity, parentGlobal GlobalTransform) {
	ch, ok := world.Get[Children](w, parent)
	if !ok {
		return
	}
	for _, child := range ch.entities {
		localTransform, ok := world.Get[Transform](w, child)
		if !ok {
			continue
		}
		childGlobal := parentGlobal.MulTransform(*localTransform)
		if gt, ok := world.Get[GlobalTransform](w, child); ok {
			*gt = childGlobal
		}
		propagateToChildren(w, child, childGlobal)
	}
}
