package gametime

import "time"

// TimerMode controls whether a Timer fires once or repeats.
type TimerMode uint8

const (
	// TimerOnce fires once and stays finished.
	TimerOnce TimerMode = iota
	// TimerRepeating resets automatically and tracks completion count per Tick.
	TimerRepeating
)

// Timer is a countdown utility. It can be embedded in components or used as a resource.
type Timer struct {
	duration      time.Duration
	elapsed       time.Duration
	mode          TimerMode
	finished      bool
	timesFinished int
	paused        bool
}

// NewTimer creates a timer with the given duration and mode.
// A zero duration timer finishes on the first Tick call.
func NewTimer(duration time.Duration, mode TimerMode) Timer {
	return Timer{duration: duration, mode: mode}
}

// Tick advances the timer by delta. Updates finished state and completion count.
func (t *Timer) Tick(delta time.Duration) {
	if t.paused {
		t.timesFinished = 0
		t.finished = t.mode == TimerOnce && t.elapsed >= t.duration
		return
	}

	t.timesFinished = 0

	if t.mode == TimerOnce && t.finished {
		return // stays finished
	}

	t.elapsed += delta

	switch t.mode {
	case TimerOnce:
		if t.elapsed >= t.duration {
			t.elapsed = t.duration
			if !t.finished {
				t.timesFinished = 1
				t.finished = true
			}
		}
	case TimerRepeating:
		if t.duration > 0 {
			for t.elapsed >= t.duration {
				t.elapsed -= t.duration
				t.timesFinished++
			}
		}
		t.finished = t.timesFinished > 0
	}
}

// Finished reports whether the timer has completed.
// For TimerOnce, stays true after completion. For TimerRepeating, true only the frame it fires.
func (t *Timer) Finished() bool { return t.finished }

// JustFinished reports whether the timer completed during the last Tick call.
func (t *Timer) JustFinished() bool { return t.timesFinished > 0 }

// TimesFinished returns the number of completions during the last Tick call.
// Always 0 or 1 for TimerOnce; can be > 1 for TimerRepeating when delta > duration.
func (t *Timer) TimesFinished() int { return t.timesFinished }

// Fraction returns elapsed / duration (0.0 to 1.0). Returns 1.0 if finished.
func (t *Timer) Fraction() float64 {
	if t.duration == 0 {
		return 1
	}
	f := float64(t.elapsed) / float64(t.duration)
	if f > 1 {
		f = 1
	}
	return f
}

// FractionRemaining returns 1.0 - Fraction().
func (t *Timer) FractionRemaining() float64 { return 1 - t.Fraction() }

// Remaining returns the time left until completion.
func (t *Timer) Remaining() time.Duration {
	if t.elapsed >= t.duration {
		return 0
	}
	return t.duration - t.elapsed
}

// Reset restarts the timer from zero.
func (t *Timer) Reset() {
	t.elapsed = 0
	t.finished = false
	t.timesFinished = 0
}

// Pause pauses the timer. Tick calls have no effect while paused.
func (t *Timer) Pause() { t.paused = true }

// Unpause resumes the timer.
func (t *Timer) Unpause() { t.paused = false }

// IsPaused reports whether the timer is paused.
func (t *Timer) IsPaused() bool { return t.paused }

// Stopwatch counts elapsed time upward with pause/reset support.
type Stopwatch struct {
	elapsed time.Duration
	paused  bool
}

// NewStopwatch creates a running stopwatch starting at zero.
func NewStopwatch() Stopwatch { return Stopwatch{} }

// Tick advances the stopwatch by delta (no-op when paused).
func (s *Stopwatch) Tick(delta time.Duration) {
	if !s.paused {
		s.elapsed += delta
	}
}

// Elapsed returns the total elapsed time.
func (s *Stopwatch) Elapsed() time.Duration { return s.elapsed }

// ElapsedSeconds returns Elapsed as float64 seconds.
func (s *Stopwatch) ElapsedSeconds() float64 { return s.elapsed.Seconds() }

// Reset resets elapsed to zero.
func (s *Stopwatch) Reset() { s.elapsed = 0 }

// Pause pauses the stopwatch.
func (s *Stopwatch) Pause() { s.paused = true }

// Unpause resumes the stopwatch.
func (s *Stopwatch) Unpause() { s.paused = false }

// IsPaused reports whether the stopwatch is paused.
func (s *Stopwatch) IsPaused() bool { return s.paused }
