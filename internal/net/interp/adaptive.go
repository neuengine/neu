package interp

import "time"

// DefaultMinDelay is the minimum render delay enforced by AdaptiveDelay.
const DefaultMinDelay = 20 * time.Millisecond

// DefaultMaxDelay is the maximum render delay enforced by AdaptiveDelay.
const DefaultMaxDelay = 500 * time.Millisecond

// DefaultAdjustStep is the per-frame nudge applied by AdaptiveDelay.
const DefaultAdjustStep = time.Millisecond

// AdaptiveDelay is a simple proportional controller that nudges RenderDelay each
// frame to keep the number of snapshot entries ahead of renderTime near TargetFill.
// Increase on underfill (jitter); decrease on overfill (excess latency).
type AdaptiveDelay struct {
	// RenderDelay is the current controlled render-clock lag. Read by the plugin
	// each frame and forwarded to InterpolateSystem via SetRenderDelay.
	RenderDelay time.Duration
	// TargetFill is the desired number of entries ahead of renderTime.
	TargetFill int
	// MinDelay clamps RenderDelay from below.
	MinDelay time.Duration
	// MaxDelay clamps RenderDelay from above.
	MaxDelay time.Duration
	// AdjustStep is the per-frame ±delta applied when fill differs from TargetFill.
	AdjustStep time.Duration
}

// NewAdaptiveDelay initialises an AdaptiveDelay from cfg.
func NewAdaptiveDelay(cfg InterpolationConfig) *AdaptiveDelay {
	min := DefaultMinDelay
	if cfg.RenderDelay > 0 && cfg.RenderDelay/2 < min {
		min = cfg.RenderDelay / 2
	}
	return &AdaptiveDelay{
		RenderDelay: cfg.RenderDelay,
		TargetFill:  cfg.TargetFill,
		MinDelay:    min,
		MaxDelay:    DefaultMaxDelay,
		AdjustStep:  DefaultAdjustStep,
	}
}

// Adjust updates RenderDelay based on the current buffer fill relative to
// TargetFill. Call once per frame after inserting new snapshots.
func (a *AdaptiveDelay) Adjust(buf *SnapshotBuffer) {
	if buf.Len() == 0 {
		// Aggressive increase on empty buffer to build up headroom.
		a.RenderDelay += a.AdjustStep * 10
		if a.RenderDelay > a.MaxDelay {
			a.RenderDelay = a.MaxDelay
		}
		return
	}
	latest, _ := buf.Latest()
	renderTime := latest.Timestamp - a.RenderDelay

	// Count entries strictly ahead of renderTime (future relative to render clock).
	ahead := 0
	for _, e := range buf.ring {
		if e.Timestamp > renderTime {
			ahead++
		}
	}

	switch {
	case ahead < a.TargetFill:
		a.RenderDelay += a.AdjustStep
		if a.RenderDelay > a.MaxDelay {
			a.RenderDelay = a.MaxDelay
		}
	case ahead > a.TargetFill:
		a.RenderDelay -= a.AdjustStep
		if a.RenderDelay < a.MinDelay {
			a.RenderDelay = a.MinDelay
		}
	}
}
