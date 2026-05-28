package mesh

import "github.com/neuengine/neu/pkg/asset"

// ─── Skinning / morph data components ────────────────────────────────────────
// These are ECS components attached alongside [SkinnedMesh]; the render extract
// reads them to build the per-draw skin matrix palette.

// JointMatrix holds the final bone transform palette for GPU upload.
// Element i is the pre-multiplied skin matrix for joint i (model→skin space).
// Populated by the animation system and consumed by the render extract.
type JointMatrix struct {
	Matrices []float32 // column-major Mat4 values, len = numJoints * 16
}

// MorphTarget describes one morph target (blend shape) stored as a separate Mesh asset.
// The base mesh and all targets must share the same vertex layout.
type MorphTarget struct {
	Name string
	Mesh asset.Handle[Mesh]
}

// MorphTargetList is the set of morph targets registered for a SkinnedMesh.
type MorphTargetList struct {
	Targets []MorphTarget
}
