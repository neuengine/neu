package gametime

import (
	"hash/fnv"
	"testing"
	"time"
)

// TestFixedStepGolden verifies determinism of the fixed-timestep accumulator:
// two runs with identical virtual-time inputs must produce the same state hash
// across 600 fixed steps (T-2T02).
func TestFixedStepGolden(t *testing.T) {
	h1 := runFixedStepSim()
	h2 := runFixedStepSim()
	if h1 != h2 {
		t.Errorf("determinism violation: run1=%016x, run2=%016x", h1, h2)
	}
}

// runFixedStepSim drives FixedTime with a deterministic sequence of virtual-time
// deltas and returns an FNV-64 hash of the resulting (stepIndex, elapsed) pairs.
// No real clock is involved — the output is fully reproducible.
func runFixedStepSim() uint64 {
	const (
		targetSteps = 600
		frameCount  = targetSteps * 2 // more frames than steps (tests accumulation)
	)
	period := time.Second / 60
	ft := NewFixedTime(period)

	// Use a deterministic sequence of virtual deltas: alternating half-period and
	// one-and-a-half period to exercise overstep and multi-step per frame.
	deltas := [2]time.Duration{period / 2, period + period/2}

	h := fnv.New64a()
	buf := make([]byte, 8)
	step := 0

	for frame := 0; frame < frameCount && step < targetSteps; frame++ {
		delta := deltas[frame%2]
		ft.Accumulate(delta)

		for ft.Expend() {
			if step >= targetSteps {
				break
			}
			step++
			// Mix step index, elapsed ns, and accumulated ns into the hash.
			writeU64(buf, uint64(step))
			_, _ = h.Write(buf)
			writeU64(buf, uint64(ft.Elapsed()))
			_, _ = h.Write(buf)
			writeU64(buf, uint64(ft.Accumulated()))
			_, _ = h.Write(buf)
		}
	}
	return h.Sum64()
}

func writeU64(b []byte, v uint64) {
	b[0] = byte(v >> 56)
	b[1] = byte(v >> 48)
	b[2] = byte(v >> 40)
	b[3] = byte(v >> 32)
	b[4] = byte(v >> 24)
	b[5] = byte(v >> 16)
	b[6] = byte(v >> 8)
	b[7] = byte(v)
}
