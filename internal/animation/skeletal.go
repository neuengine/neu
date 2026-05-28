package animation

// SkinData holds resolved runtime data for one skinned mesh.
type SkinData struct {
	// JointIDs are the entity IDs (as uint64) of the joint entities.
	JointIDs []uint64
	// InverseBindMatrices are 4×4 column-major matrices (16 float32 each).
	InverseBindMatrices [][16]float32
}

// JointTransform holds the computed local transform for one joint.
// Written by the animation system before hierarchy propagation (INV-2/3).
type JointTransform struct {
	Translation [3]float32
	Rotation    [4]float32 // quaternion [x, y, z, w]
	Scale       [3]float32
}

// ApplyJointTransform writes t into the given transform's local fields.
// Uses the generic representation so this package does not import pkg/hierarchy.
func ApplyJointTransform(out *JointTransform, t JointTransform) {
	*out = t
}

// ErrSkinMismatch is returned when a Skin's joint count and inverse-bind-matrix
// count differ (detected at load time).
type ErrSkinMismatch struct{ Joints, Matrices int }

func (e ErrSkinMismatch) Error() string {
	return "animation: skin joints / inverse-bind-matrix length mismatch"
}

// ValidateSkin checks that a Skin's joint and matrix counts match.
func ValidateSkin(joints int, matrices int) error {
	if joints != matrices {
		return ErrSkinMismatch{Joints: joints, Matrices: matrices}
	}
	return nil
}
