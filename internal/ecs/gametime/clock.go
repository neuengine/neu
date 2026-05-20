// Package gametime provides real, virtual, and fixed-timestep time resources.
// Package name avoids collision with the Go standard library "time" package.
package gametime

import "time"

// DefaultMaxAccumulation caps the FixedTime accumulator to prevent death spirals
// (e.g., when a frame takes much longer than usual — debugger pause, window drag).
const DefaultMaxAccumulation = time.Second

// Time is the primary per-frame time resource for gameplay logic.
// It mirrors VirtualTime: it is pausable and scales with relativeSpeed.
type Time struct {
	startupTime time.Time
	delta       time.Duration
	elapsed     time.Duration
	frameCount  uint64
}

// Delta returns the virtual time elapsed since the last frame (0 when paused).
func (t *Time) Delta() time.Duration { return t.delta }

// DeltaSeconds returns Delta as float64 seconds.
func (t *Time) DeltaSeconds() float64 { return t.delta.Seconds() }

// Elapsed returns the total virtual time since startup.
func (t *Time) Elapsed() time.Duration { return t.elapsed }

// ElapsedSeconds returns Elapsed as float64 seconds.
func (t *Time) ElapsedSeconds() float64 { return t.elapsed.Seconds() }

// StartupTime returns the wall-clock time when the engine started.
func (t *Time) StartupTime() time.Time { return t.startupTime }

// FrameCount returns the number of frames since startup.
func (t *Time) FrameCount() uint64 { return t.frameCount }

// RealTime tracks unscaled wall-clock time. It never pauses, reverses, or scales.
// Used for UI animations, audio, and profiling.
type RealTime struct {
	startupTime time.Time
	lastInstant time.Time
	delta       time.Duration
	elapsed     time.Duration
}

// newRealTime creates a RealTime anchored to now.
func newRealTime(now time.Time) RealTime {
	return RealTime{startupTime: now, lastInstant: now}
}

// Advance samples wall-clock time and updates delta/elapsed.
func (r *RealTime) Advance(now time.Time) {
	r.delta = now.Sub(r.lastInstant)
	r.elapsed += r.delta
	r.lastInstant = now
}

// Delta returns the wall-clock time elapsed since the last frame.
func (r *RealTime) Delta() time.Duration { return r.delta }

// DeltaSeconds returns Delta as float64 seconds.
func (r *RealTime) DeltaSeconds() float64 { return r.delta.Seconds() }

// Elapsed returns the total wall-clock time since startup.
func (r *RealTime) Elapsed() time.Duration { return r.elapsed }

// ElapsedSeconds returns Elapsed as float64 seconds.
func (r *RealTime) ElapsedSeconds() float64 { return r.elapsed.Seconds() }

// StartupTime returns the absolute wall-clock time at startup.
func (r *RealTime) StartupTime() time.Time { return r.startupTime }

// VirtualTime is pausable and scalable game time.
// Gameplay systems read VirtualTime (or the convenience Time resource).
type VirtualTime struct {
	delta         time.Duration
	elapsed       time.Duration
	paused        bool
	relativeSpeed float64
}

// newVirtualTime creates a running VirtualTime at 1× speed.
func newVirtualTime() VirtualTime { return VirtualTime{relativeSpeed: 1} }

// Advance updates delta and elapsed from the real-time delta.
func (v *VirtualTime) Advance(realDelta time.Duration) {
	if v.paused {
		v.delta = 0
		return
	}
	v.delta = time.Duration(float64(realDelta) * v.relativeSpeed)
	v.elapsed += v.delta
}

// Delta returns the virtual time elapsed since the last frame (0 when paused).
func (v *VirtualTime) Delta() time.Duration { return v.delta }

// DeltaSeconds returns Delta as float64 seconds.
func (v *VirtualTime) DeltaSeconds() float64 { return v.delta.Seconds() }

// Elapsed returns the total virtual time since startup (excludes paused periods).
func (v *VirtualTime) Elapsed() time.Duration { return v.elapsed }

// IsPaused reports whether virtual time is currently paused.
func (v *VirtualTime) IsPaused() bool { return v.paused }

// RelativeSpeed returns the current time scale multiplier.
func (v *VirtualTime) RelativeSpeed() float64 { return v.relativeSpeed }

// Pause pauses virtual time. Delta will be 0 until unpaused.
func (v *VirtualTime) Pause() { v.paused = true }

// Unpause resumes virtual time.
func (v *VirtualTime) Unpause() { v.paused = false }

// SetRelativeSpeed sets the time scale multiplier. Negative values are clamped to 0.
func (v *VirtualTime) SetRelativeSpeed(speed float64) {
	if speed < 0 {
		speed = 0
	}
	v.relativeSpeed = speed
}
