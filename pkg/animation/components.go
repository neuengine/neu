package animation

import "github.com/neuengine/neu/pkg/asset"

// AnimationNodeIndex is an opaque index into an AnimationGraph node list.
type AnimationNodeIndex uint32

// ActiveAnimation describes one currently-playing clip layer within an AnimationPlayer.
type ActiveAnimation struct {
	Clip        asset.Handle[AnimationClip]
	Elapsed     float32
	Speed       float32 // negative = reverse; default 1.0
	Repeat      RepeatMode
	BlendWeight float32 // 0..1 blend contribution for layered blending
	Paused      bool
}

// AnimationPlayer is an ECS component that drives playback on an entity.
// Multiple ActiveAnimations support layered blending (e.g. upper-body attack
// over lower-body run). The slice is ordered by AnimationNodeIndex for
// deterministic evaluation (INV-1).
type AnimationPlayer struct {
	// Active holds all currently blended animation layers.
	// It is maintained as a sorted slice (by index) by the animation system
	// so evaluation order is deterministic and independent of insertion order.
	Active []indexedAnimation
}

// indexedAnimation pairs a node index with its playback state.
type indexedAnimation struct {
	Index AnimationNodeIndex
	Anim  ActiveAnimation
}

// Joint marks an entity as a skeleton joint. The Index maps into the parent
// Skin's joint array and inverse-bind-matrix list.
type Joint struct {
	Index uint16
}

// Skin is attached to the mesh entity and holds the joint hierarchy and
// inverse-bind matrices needed for GPU skinning.
type Skin struct {
	Joints              []joint // entity IDs of joint entities
	InverseBindMatrices [][16]float32
}

// joint is an entity ID stored as uint64 (avoids importing the entity package).
// The animation system resolves it to entity.EntityID at runtime.
type joint struct{ ID uint64 }

// MorphWeights holds per-morph-target blend weights in [0, 1].
// The animation system writes to this component; the mesh system reads it at
// render time to interpolate vertex offsets.
type MorphWeights struct {
	Weights []float32
}
