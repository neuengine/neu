// Package postprocess defines the per-camera post-process pipeline:
// sentinel errors, canonical EffectSlot order, and per-effect settings.
// See internal/render/postpass for the graph builder that assembles the chain.
package postprocess

import "errors"

var (
	// ErrPostOrder is returned when a color-altering (HDR) effect appears
	// after the tonemapping pass — violates INV-1.
	ErrPostOrder = errors.New("postprocess: color-altering effect after tonemapping")
	// ErrAAConflict is returned when incompatible AA settings are both enabled
	// on the same camera (e.g. FXAA + SMAA). SMAA is preferred; FXAA is dropped.
	ErrAAConflict = errors.New("postprocess: conflicting anti-aliasing settings")
)

// EffectSlot is the canonical index of a post-processing effect.
// The declaration order is the execution order — it is invariant (INV-2).
type EffectSlot uint8

const (
	SlotSSAO                EffectSlot = iota // screen-space ambient occlusion (HDR)
	SlotSSR                                   // screen-space reflections (HDR)
	SlotBloom                                 // bloom (HDR)
	SlotDepthOfField                          // depth of field (HDR)
	SlotMotionBlur                            // motion blur (HDR)
	SlotChromaticAberration                   // chromatic aberration (HDR)
	SlotFilmGrain                             // film grain (HDR)
	SlotColorGrading                          // color grading / LUT (HDR)
	SlotTonemapping                           // HDR → LDR boundary (INV-1)
	SlotSpatialAA                             // FXAA / SMAA (LDR)
	slotCount
)

// IsHDR reports whether s operates in linear HDR space (i.e. executes before
// the tonemapping boundary). Spatial AA runs LDR and is not HDR.
func (s EffectSlot) IsHDR() bool { return s < SlotTonemapping }

// String returns the slot name for diagnostics and graph pass names.
func (s EffectSlot) String() string {
	switch s {
	case SlotSSAO:
		return "SSAO"
	case SlotSSR:
		return "SSR"
	case SlotBloom:
		return "Bloom"
	case SlotDepthOfField:
		return "DepthOfField"
	case SlotMotionBlur:
		return "MotionBlur"
	case SlotChromaticAberration:
		return "ChromaticAberration"
	case SlotFilmGrain:
		return "FilmGrain"
	case SlotColorGrading:
		return "ColorGrading"
	case SlotTonemapping:
		return "Tonemapping"
	case SlotSpatialAA:
		return "SpatialAA"
	default:
		return "EffectSlot(?)"
	}
}

// PostProcessStack records which effects are enabled for a camera (INV-2/3).
// Presence of an enabled slot plus a corresponding settings component on the
// camera entity activates the effect's graph node in BuildPostChain.
type PostProcessStack struct {
	enabled [slotCount]bool
}

// Enable marks slot as active and returns the stack for chaining.
func (s *PostProcessStack) Enable(slot EffectSlot) *PostProcessStack {
	if slot < slotCount {
		s.enabled[slot] = true
	}
	return s
}

// Disable marks slot as inactive.
func (s *PostProcessStack) Disable(slot EffectSlot) {
	if slot < slotCount {
		s.enabled[slot] = false
	}
}

// IsEnabled reports whether slot is active.
func (s PostProcessStack) IsEnabled(slot EffectSlot) bool {
	return slot < slotCount && s.enabled[slot]
}

// EnabledSlots returns the active slots in canonical (enum) order (INV-2).
func (s PostProcessStack) EnabledSlots() []EffectSlot {
	out := make([]EffectSlot, 0, int(slotCount))
	for i := range int(slotCount) {
		if s.enabled[i] {
			out = append(out, EffectSlot(i))
		}
	}
	return out
}
