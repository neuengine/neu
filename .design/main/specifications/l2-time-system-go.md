# Time System — Go Implementation

**Version:** 0.1.0
**Status:** Stable
**Layer:** go
**Implements:** [time-system.md](l1-time-system.md)

## Overview

This specification defines the Go implementation of the time system described in the L1 concept spec. The time system provides three independent time dimensions — Real, Virtual, and Fixed — each stored as a separate ECS resource. It uses Go's standard library `time.Duration` and `time.Time` types for wall-clock interop while storing deltas and elapsed values as `time.Duration` for precision. The package is named `gametime` to avoid conflict with the Go standard library `time` package.

## Related Specifications

- [time-system.md](l1-time-system.md) — L1 concept specification (parent)

## 1. Motivation

The Go implementation of the Time system provides precise, multi-dimensional time tracking for gameplay, physics, and UI. It ensures:

- High-precision timing using `time.Duration` (int64 nanoseconds).
- Separation of Real, Virtual, and Fixed time scales for independent control.
- Deterministic fixed-timestep updates for physics and network sync.
- Convenient utilities like `Timer` and `Stopwatch` for common gameplay tasks.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: Relies on `time.Duration` and `time.Time` from the standard library.
- **Single Source**: Wall-clock time is sampled exactly once per frame at the start of the `First` schedule.
- **Accumulator Safety**: Fixed time accumulation is capped (default 1s) to prevent "death spirals."

## 3. Core Invariants

> [!NOTE]
> See [time-system.md §3](l1-time-system.md) for technology-agnostic invariants.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: Non-decreasing Real | `RealTime.elapsed` adds monotonic `time.Duration` each frame. |
| **INV-2**: Deterministic Fixed | `FixedTime.period` is constant; `Expend` only returns true for complete steps. |
| **INV-3**: Virtual Independence | `VirtualTime` maintains its own `elapsed` and `delta` state relative to real time. |
| **INV-4**: Interpolation Support | `FixedTime.overstep` stores the remainder for render-time sub-frame interpolation. |
| **INV-5**: Precision | All internal accumulation uses `int64` nanoseconds via `time.Duration`. |

## Go Package

```
internal/gametime/
```

All types in this spec belong to package `gametime`. The package imports `time` from the standard library and `internal/ecs` for resource registration.

## Type Definitions

### Time (Convenience Alias)

```go
// Time is the primary time resource. It provides the current frame's delta
// and total elapsed virtual time. This is a convenience type that systems
// use most frequently — it mirrors VirtualTime for gameplay logic.
type Time struct {
    delta       time.Duration // virtual time since last frame (0 when paused)
    elapsed     time.Duration // total virtual time since startup
    startupTime time.Time     // wall-clock time at engine startup
    frameCount  uint64        // total frames since startup
}

// Delta returns the virtual time elapsed since the last frame.
// Returns 0 when paused.
func (t *Time) Delta() time.Duration

// DeltaSeconds returns Delta as float64 seconds (convenience for math).
func (t *Time) DeltaSeconds() float64

// Elapsed returns the total virtual time since startup.
func (t *Time) Elapsed() time.Duration

// ElapsedSeconds returns Elapsed as float64 seconds.
func (t *Time) ElapsedSeconds() float64

// StartupTime returns the wall-clock time when the engine started.
func (t *Time) StartupTime() time.Time

// FrameCount returns the number of frames since startup.
func (t *Time) FrameCount() uint64
```

### RealTime

```go
// RealTime tracks unscaled wall-clock time. It never pauses, reverses,
// or scales. Used for UI animations, audio timing, and profiling.
type RealTime struct {
    delta       time.Duration // wall-clock time since last frame
    elapsed     time.Duration // total wall-clock time since startup
    startupTime time.Time     // absolute wall-clock time at startup
    lastInstant time.Time     // wall-clock time at end of previous frame
}

// Delta returns the wall-clock time elapsed since the last frame.
func (r *RealTime) Delta() time.Duration

// DeltaSeconds returns Delta as float64 seconds.
func (r *RealTime) DeltaSeconds() float64

// Elapsed returns the total wall-clock time since startup.
func (r *RealTime) Elapsed() time.Duration

// ElapsedSeconds returns Elapsed as float64 seconds.
func (r *RealTime) ElapsedSeconds() float64

// StartupTime returns the absolute wall-clock time at startup.
func (r *RealTime) StartupTime() time.Time
```

### VirtualTime

```go
// VirtualTime is pausable and scalable game time. Gameplay systems should
// read from VirtualTime (or the convenience Time resource, which mirrors it).
type VirtualTime struct {
    delta         time.Duration // virtual time since last frame (0 when paused)
    elapsed       time.Duration // total virtual time since startup
    paused        bool          // when true, delta is forced to 0
    relativeSpeed float64       // multiplier applied to real delta (default 1.0)
}

// Delta returns the virtual time elapsed since the last frame.
func (v *VirtualTime) Delta() time.Duration

// DeltaSeconds returns Delta as float64 seconds.
func (v *VirtualTime) DeltaSeconds() float64

// Elapsed returns the total virtual time since startup (excludes paused periods).
func (v *VirtualTime) Elapsed() time.Duration

// IsPaused reports whether virtual time is currently paused.
func (v *VirtualTime) IsPaused() bool

// RelativeSpeed returns the current time scale multiplier.
func (v *VirtualTime) RelativeSpeed() float64

// Pause pauses virtual time. Delta will be 0 until unpaused.
func (v *VirtualTime) Pause()

// Unpause resumes virtual time.
func (v *VirtualTime) Unpause()

// SetRelativeSpeed sets the time scale multiplier. A value of 2.0 means
// virtual time passes twice as fast as real time. Must be >= 0.
func (v *VirtualTime) SetRelativeSpeed(speed float64)
```

### FixedTime

```go
// FixedTime drives deterministic fixed-timestep updates. It accumulates
// virtual time and consumes it in fixed-size steps. The overstep fraction
// is available for render interpolation.
type FixedTime struct {
    period      time.Duration // fixed timestep (e.g., 16666666ns for 60 Hz)
    accumulated time.Duration // carried-over time from previous frame
    overstep    time.Duration // remainder after last fixed step
    elapsed     time.Duration // total fixed-step time
}

// NewFixedTime creates a FixedTime with the given timestep period.
func NewFixedTime(period time.Duration) FixedTime

// Period returns the fixed timestep duration.
func (f *FixedTime) Period() time.Duration

// PeriodSeconds returns Period as float64 seconds.
func (f *FixedTime) PeriodSeconds() float64

// Accumulated returns the currently accumulated time awaiting consumption.
func (f *FixedTime) Accumulated() time.Duration

// Overstep returns the time remaining after the last fixed step.
// Range: 0 to Period. Useful for render interpolation.
func (f *FixedTime) Overstep() time.Duration

// OverstepFraction returns Overstep / Period (range 0.0 to 1.0).
func (f *FixedTime) OverstepFraction() float64

// Elapsed returns the total fixed-step time.
func (f *FixedTime) Elapsed() time.Duration

// SetPeriod changes the fixed timestep. Takes effect next frame.
func (f *FixedTime) SetPeriod(period time.Duration)

// Expend consumes one fixed timestep from the accumulator.
// Returns false if accumulator < period (no step available).
func (f *FixedTime) Expend() bool
```

### Timer

```go
// TimerMode controls whether a timer fires once or repeats.
type TimerMode uint8

const (
    TimerOnce      TimerMode = iota // fires once, then stays finished
    TimerRepeating                  // resets automatically, tracks completion count
)

// Timer is a countdown utility. It can be used as a field within
// user components or as a standalone resource.
type Timer struct {
    duration       time.Duration
    elapsed        time.Duration
    mode           TimerMode
    finished       bool
    timesFinished  int  // for repeating timers: completions this tick
    paused         bool
}

// NewTimer creates a timer with the given duration and mode.
func NewTimer(duration time.Duration, mode TimerMode) Timer

// Tick advances the timer by delta. Updates finished state and
// completion count for repeating timers.
func (t *Timer) Tick(delta time.Duration)

// Finished reports whether the timer has completed (stays true for Once mode).
func (t *Timer) Finished() bool

// JustFinished reports whether the timer completed during the last Tick call.
func (t *Timer) JustFinished() bool

// TimesFinished returns the number of completions during the last Tick call
// (relevant for repeating timers when delta > duration).
func (t *Timer) TimesFinished() int

// Fraction returns elapsed / duration (0.0 to 1.0).
func (t *Timer) Fraction() float64

// FractionRemaining returns 1.0 - Fraction().
func (t *Timer) FractionRemaining() float64

// Remaining returns the time left until completion.
func (t *Timer) Remaining() time.Duration

// Reset restarts the timer from zero.
func (t *Timer) Reset()

// Pause pauses the timer. Tick calls have no effect while paused.
func (t *Timer) Pause()

// Unpause resumes the timer.
func (t *Timer) Unpause()

// IsPaused reports whether the timer is paused.
func (t *Timer) IsPaused() bool
```

### Stopwatch

```go
// Stopwatch is an elapsed-time counter. It counts up from zero with
// pause/reset support.
type Stopwatch struct {
    elapsed time.Duration
    paused  bool
}

// NewStopwatch creates a running stopwatch starting at zero.
func NewStopwatch() Stopwatch

// Tick advances the stopwatch by delta.
func (s *Stopwatch) Tick(delta time.Duration)

// Elapsed returns the total elapsed time.
func (s *Stopwatch) Elapsed() time.Duration

// ElapsedSeconds returns Elapsed as float64 seconds.
func (s *Stopwatch) ElapsedSeconds() float64

// Reset resets elapsed to zero.
func (s *Stopwatch) Reset()

// Pause pauses the stopwatch.
func (s *Stopwatch) Pause()

// Unpause resumes the stopwatch.
func (s *Stopwatch) Unpause()

// IsPaused reports whether the stopwatch is paused.
func (s *Stopwatch) IsPaused() bool
```

### MaxAccumulation

```go
// DefaultMaxAccumulation is the maximum time the FixedTime accumulator is
// allowed to build up. Prevents death spirals when frames take too long.
const DefaultMaxAccumulation = 1 * time.Second
```

## Key Methods

### Frame Time Update (First Schedule)

```
SYSTEM update_time(real *RealTime, virtual *VirtualTime, fixed *FixedTime, t *Time):
  // Step 1: Measure wall-clock delta
  now = time.Now()
  real_delta = now - real.lastInstant
  real.lastInstant = now
  real.delta = real_delta
  real.elapsed += real_delta

  // Step 2: Update virtual time
  IF NOT virtual.paused:
    virtual_delta = time.Duration(float64(real_delta) * virtual.relativeSpeed)
    virtual.delta = virtual_delta
    virtual.elapsed += virtual_delta
  ELSE:
    virtual.delta = 0

  // Step 3: Accumulate fixed time
  fixed.accumulated += virtual.delta
  IF fixed.accumulated > DefaultMaxAccumulation:
    fixed.accumulated = DefaultMaxAccumulation  // death spiral prevention

  // Step 4: Update convenience Time resource
  t.delta = virtual.delta
  t.elapsed = virtual.elapsed
  t.frameCount += 1
```

### Fixed Timestep Loop (RunFixedMainLoop Schedule)

```
SYSTEM run_fixed_main_loop(fixed *FixedTime, schedules):
  WHILE fixed.Expend():
    fixed.elapsed += fixed.period
    RunSchedule(FixedPreUpdate)
    RunSchedule(FixedUpdate)
    RunSchedule(FixedPostUpdate)

  fixed.overstep = fixed.accumulated  // remainder for interpolation
```

### Death Spiral Prevention

If the real frame delta is very large (e.g., breakpoint pause, window drag), the accumulator is clamped to `DefaultMaxAccumulation`. For a 60 Hz fixed step (period ~16.67ms), this means at most 60 fixed update iterations per frame.

### TimePlugin

```go
// TimePlugin registers all time resources and the time update system.
type TimePlugin struct{}

// Build inserts RealTime, VirtualTime, FixedTime, and Time resources into
// the World and registers the update_time system in the First schedule.
func (p TimePlugin) Build(app *app.App)
```

## Performance Strategy

- **`time.Duration` (int64 nanoseconds)**: Integer arithmetic for time accumulation — no floating-point drift over long sessions. Convert to `float64` only at the API boundary (`DeltaSeconds()`).
- **Single `time.Now()` call per frame**: Only one syscall to the monotonic clock per frame, in the First schedule.
- **Timer/Stopwatch are value types**: No heap allocation. Stored inline in components or resources.
- **Fixed timestep loop**: Simple subtraction loop with integer comparison. Zero allocation per iteration.
- **No channels or goroutines**: Time system is fully synchronous, runs on the main schedule thread.

## Error Handling

- **Negative relative speed**: `SetRelativeSpeed` clamps to 0 if a negative value is passed. Logs a warning via `log/slog`.
- **Zero period**: `NewFixedTime` with zero or negative period panics at initialization (programming error).
- **Timer with zero duration**: Immediately finishes on first `Tick`. Not an error — valid use case for "fire next frame."
- **Extremely large deltas**: Handled by death spiral prevention. The accumulator cap ensures bounded fixed-update iterations.
- **First frame delta**: On the very first frame, `lastInstant` is initialized to startup time. The first delta may be larger than typical — this is expected and handled by the accumulator cap.

## Testing Strategy

- **Unit tests**: Create RealTime, VirtualTime, FixedTime. Step through multiple frames with known deltas, verify elapsed and delta values.
- **Virtual time pause**: Pause VirtualTime, advance real time, verify virtual delta is 0 and elapsed does not change.
- **Virtual time scale**: Set relativeSpeed to 2.0, verify virtual delta is 2x real delta.
- **Fixed timestep**: Set period to 16ms, accumulate 50ms, verify `Expend()` returns true 3 times and overstep is ~2ms.
- **Death spiral**: Set accumulated to 5s, verify it clamps to `DefaultMaxAccumulation`.
- **Timer Once**: Create 1s timer, tick 0.5s (not finished), tick 0.6s (finished, just_finished).
- **Timer Repeating**: Create 100ms repeating timer, tick 350ms, verify `TimesFinished() == 3`.
- **Stopwatch**: Tick, verify elapsed. Pause, tick, verify elapsed unchanged. Reset, verify zero.
- **Integration**: Register TimePlugin in an App, run 3 frames, verify Time resource updates correctly.
- **Benchmarks**: `BenchmarkTimerTick`, `BenchmarkFixedTimeExpend` — target zero allocations.

## 7. Drawbacks & Alternatives

- **Drawback**: Integer-based nanosecond accumulation can overflow after ~292 years of continuous uptime.
- **Alternative**: Floating-point seconds.
- **Decision**: Nanoseconds are preferred for precision and to match the Go standard library patterns. Overflow is not a practical concern for game sessions.

## Canonical References

<!-- MANDATORY for Stable status. List authoritative source files that downstream agents
     MUST read before implementing this spec. Use relative paths from project root.
     Stub state — fill with concrete files when implementation begins (Phase 1+). -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

<!-- Empty table = no canonical sources yet. Populate one row per authoritative file
     when implementation lands (Phase 1+). Stable promotion requires ≥1 row. -->

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-03-26 | Initial L2 draft |
