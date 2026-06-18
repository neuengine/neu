package predict

import (
	"reflect"
	"time"

	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/world"
	neumath "github.com/neuengine/neu/pkg/math"
)

// DefaultBlendDuration is the default misprediction correction blend-out time.
const DefaultBlendDuration = 100 * time.Millisecond

// CorrectionState is a component added to a predicted entity when a
// reconciliation misprediction is detected. It carries the visual displacement
// between the entity's previous display position and its corrected simulation
// position so that CorrectionSmoothingSystem can blend the correction out
// invisibly to the player (INV-1 — simulation is already at the correct
// position; only the rendered display lags).
//
// Remove CorrectionState once BlendRemaining reaches zero — the entity has
// fully converged to its simulation position.
type CorrectionState struct {
	// VisualOffset is the displacement to add to the entity's render transform
	// so it appears at its old position. Decays toward Vec3{} over BlendDuration.
	VisualOffset neumath.Vec3
	// RotationOffset is the residual rotation correction. Decays toward
	// QuatIdentity() over BlendDuration via Slerp.
	RotationOffset neumath.Quat
	// BlendRemaining is the time left in the current blend. Decremented each Run.
	BlendRemaining time.Duration
	// BlendDuration is the total blend time set at correction onset. Used to
	// compute the decay rate. Zero is treated as DefaultBlendDuration.
	BlendDuration time.Duration
}

// CorrectionSmoothingSystem runs in the Last schedule each frame. For every
// entity carrying a CorrectionState it decays VisualOffset and RotationOffset
// proportionally toward zero. When BlendRemaining reaches zero the component
// is removed — the correction is complete (INV-1: no teleport, blended).
type CorrectionSmoothingSystem struct {
	dt time.Duration // per-frame step; see NewCorrectionSmoothingSystem
}

// NewCorrectionSmoothingSystem creates a system that advances each correction
// by dt per Run call. A dt ≤ 0 defaults to 1/60 s (60 Hz fixed step).
func NewCorrectionSmoothingSystem(dt time.Duration) *CorrectionSmoothingSystem {
	if dt <= 0 {
		dt = time.Second / 60
	}
	return &CorrectionSmoothingSystem{dt: dt}
}

// Run decays all active CorrectionState components by one dt step and removes
// components whose BlendRemaining has expired.
func (s *CorrectionSmoothingSystem) Run(w *world.World) {
	corrID, ok := w.Components().Lookup(reflect.TypeFor[CorrectionState]())
	if !ok {
		return // no entity has ever had CorrectionState; nothing to do
	}

	var toRemove []entity.Entity
	w.Archetypes().Each(func(arch *world.Archetype) bool {
		if !arch.Has(corrID) {
			return true // skip archetypes that lack CorrectionState
		}
		for _, ent := range arch.Entities() {
			cs, present := world.Get[CorrectionState](w, ent)
			if !present {
				continue
			}
			remaining := cs.BlendRemaining - s.dt
			if remaining <= 0 {
				toRemove = append(toRemove, ent)
				continue
			}
			// Proportional decay: reduce offset by the fraction consumed this step.
			factor := float32(s.dt) / float32(cs.BlendRemaining)
			cs.VisualOffset = cs.VisualOffset.Lerp(neumath.Vec3{}, factor)
			cs.RotationOffset = cs.RotationOffset.Slerp(neumath.QuatIdentity(), factor)
			cs.BlendRemaining = remaining
		}
		return true
	})

	// Removal happens after iteration to avoid invalidating the archetype slice.
	for _, ent := range toRemove {
		_ = world.Remove[CorrectionState](w, ent)
	}
}
