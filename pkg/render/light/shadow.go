package light

import (
	"errors"
	"math"
)

// ErrCascadeCoverage is returned when a CascadeShadowConfig's split distances
// do not span from the camera's near plane to MaxDistance (INV-3).
var ErrCascadeCoverage = errors.New("shadow: cascades do not cover near→max range")

// SplitMode selects how cascade far-plane distances are computed.
type SplitMode uint8

const (
	// SplitLogarithmic derives splits via near*(MaxDistance/near)^(i/Count)
	// for i in [1, Count]. The final split equals MaxDistance by construction.
	SplitLogarithmic SplitMode = iota
	// SplitManual reads split distances from CascadeShadowConfig.ManualSplits.
	SplitManual
)

// CubeShadow is the shadow descriptor for a PointLight (L1 §4.6: 6-face cube map).
// MapSize is the resolution of each face in pixels.
type CubeShadow struct{ MapSize uint32 }

// FaceCount returns 6 — a point-light cube shadow always has 6 faces (L1 §4.6).
func (CubeShadow) FaceCount() int { return 6 }

// SingleShadow is the shadow descriptor for a SpotLight (L1 §4.6: 1 shadow map).
type SingleShadow struct{ MapSize uint32 }

// MapCount returns 1 — a spot-light shadow always uses a single map (L1 §4.6).
func (SingleShadow) MapCount() int { return 1 }

// CascadeShadowConfig controls cascaded shadow maps for a DirectionalLight (L1 §4.6).
// Count cascades each cover a sub-frustum from near to MaxDistance.
//
// Note: the struct field for manual split distances is ManualSplits (not Splits) to
// avoid a name conflict with the Splits() method.
type CascadeShadowConfig struct {
	ManualSplits []float32
	Overlap      float32
	MapSize      uint32
	MaxDistance  float32
	Count        uint8
	SplitMode    SplitMode
}

// Splits derives the per-cascade far-plane distances for this config (INV-3).
// It returns Count values spanning from near to MaxDistance.
//
// Logarithmic: split_i = near*(MaxDistance/near)^(i/Count), i=1..Count.
// The final value equals MaxDistance by construction — no coverage error possible.
//
// Manual: returns a copy of ManualSplits. Returns ErrCascadeCoverage when
// len(ManualSplits) < Count or ManualSplits[Count-1] < MaxDistance.
func (c *CascadeShadowConfig) Splits(near float32) ([]float32, error) {
	count := max(1, min(4, int(c.Count)))

	out := make([]float32, count)
	switch c.SplitMode {
	case SplitManual:
		if len(c.ManualSplits) < count {
			return nil, ErrCascadeCoverage
		}
		copy(out, c.ManualSplits[:count])
		// INV-3: last split must reach MaxDistance.
		if out[count-1] < c.MaxDistance {
			return nil, ErrCascadeCoverage
		}
	default: // SplitLogarithmic
		ratio := float64(c.MaxDistance) / float64(near)
		for i := range count {
			t := float64(i+1) / float64(count)
			out[i] = float32(float64(near) * math.Pow(ratio, t))
		}
		// out[count-1] == MaxDistance by construction; INV-3 satisfied.
	}
	return out, nil
}
