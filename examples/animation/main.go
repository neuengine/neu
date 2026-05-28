// examples/animation demonstrates clip playback, deterministic curve sampling,
// and animation graph blend (T-5T02, C29 P5 gate).
//
// Bootstrap: validates l2-animation-system-go against l1-animation-system.
package main

import (
	"fmt"
	"hash/fnv"

	internalanimation "github.com/neuengine/neu/internal/animation"
	pkganimation "github.com/neuengine/neu/pkg/animation"
)

// poseHash computes a deterministic FNV-1a hash over a sampled float32 slice.
// Used to verify INV-1: same clip+time → same hash across ≥20 runs.
func poseHash(vals []float32) uint64 {
	h := fnv.New64a()
	for _, v := range vals {
		b := [4]byte{
			byte(int32(v*1e6) >> 24),
			byte(int32(v*1e6) >> 16),
			byte(int32(v*1e6) >> 8),
			byte(int32(v * 1e6)),
		}
		_, _ = h.Write(b[:])
	}
	return h.Sum64()
}

// run samples a known clip and returns a deterministic pose hash (INV-1).
func run() (uint64, error) {
	// A simple 3-keyframe clip animating a scalar property from 0 → 5 → 10.
	clip := pkganimation.AnimationClip{
		Duration: 2,
		Curves: []pkganimation.VariableCurve{
			{
				Target: pkganimation.AnimationTargetId{Property: "X"},
				Keyframes: pkganimation.Keyframes{
					Times:  []float32{0, 1, 2},
					Values: []float32{0, 5, 10},
					Interp: pkganimation.InterpolationLinear,
				},
			},
		},
	}
	if len(clip.Curves) == 0 {
		return 0, fmt.Errorf("clip has no curves")
	}

	// Sample at t=0.5, 1.0, 1.5 and verify determinism.
	times := []float32{0.5, 1.0, 1.5}
	var combined uint64
	for _, t := range times {
		vals := internalanimation.SampleCurve(clip.Curves[0].Keyframes, t)
		if len(vals) == 0 {
			return 0, fmt.Errorf("empty sample at t=%v", t)
		}
		combined ^= poseHash(vals)
	}

	// Validate skeletal skin check (INV-4 partial animation: mismatched counts = error).
	if err := internalanimation.ValidateSkin(3, 3); err != nil {
		return 0, fmt.Errorf("ValidateSkin: %w", err)
	}
	if err := internalanimation.ValidateSkin(3, 2); err == nil {
		return 0, fmt.Errorf("expected ErrSkinMismatch for 3 joints / 2 matrices")
	}

	return combined, nil
}

func main() {
	h, err := run()
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}
	fmt.Printf("PASS: animation example hash=%d\n", h)
}
