package gametime

import "time"

// FixedTime drives deterministic fixed-timestep updates.
// It accumulates virtual-time deltas and consumes them in fixed-size steps.
// The overstep remainder is exposed for render-time sub-frame interpolation (INV-4).
type FixedTime struct {
	period      time.Duration
	accumulated time.Duration
	overstep    time.Duration
	elapsed     time.Duration
}

// NewFixedTime creates a FixedTime with the given period.
// Panics if period <= 0 (programming error).
func NewFixedTime(period time.Duration) FixedTime {
	if period <= 0 {
		panic("gametime: FixedTime period must be positive")
	}
	return FixedTime{period: period}
}

// Accumulate adds virtualDelta to the accumulator, clamped to DefaultMaxAccumulation
// to prevent death spirals when frames run long.
func (f *FixedTime) Accumulate(virtualDelta time.Duration) {
	f.accumulated += virtualDelta
	if f.accumulated > DefaultMaxAccumulation {
		f.accumulated = DefaultMaxAccumulation
	}
}

// Expend consumes one period from the accumulator.
// Returns true when a full step was available; false when accumulator < period.
// After all Expend calls return false, Overstep holds the leftover for interpolation.
func (f *FixedTime) Expend() bool {
	if f.accumulated < f.period {
		f.overstep = f.accumulated
		return false
	}
	f.accumulated -= f.period
	f.elapsed += f.period
	return true
}

// Period returns the fixed timestep duration.
func (f *FixedTime) Period() time.Duration { return f.period }

// PeriodSeconds returns Period as float64 seconds.
func (f *FixedTime) PeriodSeconds() float64 { return f.period.Seconds() }

// Accumulated returns the time currently awaiting consumption.
func (f *FixedTime) Accumulated() time.Duration { return f.accumulated }

// Overstep returns the leftover time after the last Expend sequence.
// Range: 0 to Period. Valid only after Expend has returned false.
func (f *FixedTime) Overstep() time.Duration { return f.overstep }

// OverstepFraction returns Overstep / Period (0.0 to 1.0) for render interpolation.
func (f *FixedTime) OverstepFraction() float64 {
	if f.period == 0 {
		return 0
	}
	return float64(f.overstep) / float64(f.period)
}

// Elapsed returns the total fixed-step time consumed.
func (f *FixedTime) Elapsed() time.Duration { return f.elapsed }

// SetPeriod changes the fixed timestep. Takes effect on the next Expend call.
// Panics if period <= 0.
func (f *FixedTime) SetPeriod(period time.Duration) {
	if period <= 0 {
		panic("gametime: FixedTime period must be positive")
	}
	f.period = period
}
