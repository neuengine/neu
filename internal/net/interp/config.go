package interp

import "time"

// InterpolationConfig holds static parameters for the snapshot interpolation system.
// Pass it to InterpolationPlugin or NewInterpolateSystem.
type InterpolationConfig struct {
	// RenderDelay is the initial lag behind the latest snapshot. Default 100ms.
	RenderDelay time.Duration
	// BufferCapacity is the maximum number of snapshot entries to retain.
	// Default DefaultBufferCapacity.
	BufferCapacity int
	// TargetFill is the desired number of snapshots ahead of renderTime.
	// The adaptive controller steers toward this to absorb jitter. Default 3.
	TargetFill int
	// MaxExtrapolation caps the extrapolation factor when renderTime exceeds the
	// newest snapshot. Default DefaultMaxExtrapolation.
	MaxExtrapolation float32
	// Extrapolate enables bounded extrapolation past the newest snapshot.
	Extrapolate bool
}

// DefaultInterpolationConfig returns a config suitable for most use cases.
func DefaultInterpolationConfig() InterpolationConfig {
	return InterpolationConfig{
		RenderDelay:      100 * time.Millisecond,
		BufferCapacity:   DefaultBufferCapacity,
		TargetFill:       3,
		MaxExtrapolation: DefaultMaxExtrapolation,
	}
}
