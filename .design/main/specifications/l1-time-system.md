# Time System

**Version:** 0.2.0
**Status:** Stable
**Layer:** concept

## Overview

The time system provides three independent time dimensions — Real, Virtual, and Fixed — each stored as a separate Resource in the World. Real time tracks wall-clock time. Virtual time is pausable and scalable, used for gameplay logic. Fixed time drives deterministic timestep updates. The system also provides Timer and Stopwatch utility types for common temporal patterns.

## Related Specifications

- [world-system.md](l1-world-system.md) — Time dimensions are World resources
- [system-scheduling.md](l1-system-scheduling.md) — Fixed timestep drives the FixedUpdate schedule
- [app-framework.md](l1-app-framework.md) — Time updates occur in the First schedule before other systems

## 1. Motivation

Games require multiple notions of time running simultaneously:

- **UI and audio** need real wall-clock time that never pauses.
- **Gameplay** needs virtual time that can be paused, slowed, or sped up.
- **Physics and networking** need deterministic fixed timesteps for reproducible simulation.

Without a structured time system, developers mix `time.Now()` calls throughout their code, leading to inconsistent behavior when pausing, frame spikes, and non-deterministic physics. A first-class time system solves all of these.

## 2. Constraints & Assumptions

- Time resources are updated exactly once per frame, at the start of the frame (in the First schedule).
- Fixed timestep accumulation is bounded — if the frame takes too long, accumulated time is clamped to prevent spiral-of-death (unbounded FixedUpdate iterations).
- All time values use `float64` seconds internally for precision over long play sessions.
- The time system has zero external dependencies (C24).

## 3. Core Invariants

- **INV-1**: Real time never pauses, reverses, or scales. It always reflects wall-clock elapsed time.
- **INV-2**: Virtual time advances by zero when paused, regardless of real elapsed time.
- **INV-3**: Fixed timestep is constant — every FixedUpdate sees the same delta.
- **INV-4**: Fixed update accumulation is clamped to `MaxAccumulation` (default: 1 second) to prevent death spirals.
- **INV-5**: Time resources are read-only to all systems except the engine's internal time update system.

## 4. Detailed Design

### 4.1 Time Dimensions

Each dimension is a separate resource type in the World:

```
Resource: TimeReal
  - Elapsed       float64   // total wall-clock time since startup
  - Delta         float64   // wall-clock time since last frame
  - StartInstant  time.Time // absolute wall-clock time at startup

Resource: TimeVirtual
  - Elapsed       float64   // total virtual time (paused periods excluded)
  - Delta         float64   // virtual time since last frame (0 when paused)
  - Paused        bool
  - RelativeSpeed float64   // multiplier applied to real delta (default 1.0)

Resource: TimeFixed
  - Elapsed       float64   // total fixed-step time
  - Delta         float64   // fixed timestep value (e.g., 1/60)
  - Accumulator   float64   // carried-over time from previous frame
  - OverstepFraction float64 // fraction of a timestep remaining after last step
```

### 4.2 Frame Update Flow

At the start of each frame (in the First schedule):

```
1. Measure wall-clock delta since last frame
2. Update TimeReal: delta = measured, elapsed += delta
3. Update TimeVirtual:
   if not paused:
     virtual_delta = real_delta * relative_speed
     elapsed += virtual_delta
     delta = virtual_delta
   else:
     delta = 0
4. Update TimeFixed:
   accumulator += virtual_delta
   accumulator = min(accumulator, max_accumulation)
```

### 4.3 Fixed Timestep Loop

The RunFixedMainLoop schedule (see [app-framework.md](l1-app-framework.md)) consumes accumulated time:

```
while time_fixed.accumulator >= time_fixed.delta:
    time_fixed.accumulator -= time_fixed.delta
    time_fixed.elapsed += time_fixed.delta
    run FixedPreUpdate schedule
    run FixedUpdate schedule
    run FixedPostUpdate schedule

time_fixed.overstep_fraction = time_fixed.accumulator / time_fixed.delta
```

The overstep fraction (range 0.0 to 1.0) is available for interpolation between the last fixed state and the current state, enabling smooth rendering at variable frame rates.

### 4.4 Timer Utility

```
Timer
  - Duration      float64
  - Elapsed       float64
  - Repeating     bool
  - Finished      bool
  - TimesFinished int       // for repeating timers, count of completions this tick

Methods:
  Tick(delta float64)       // advance by delta
  JustFinished() bool       // true if finished this tick
  Fraction() float64        // elapsed / duration (0.0 to 1.0)
  FractionRemaining() float64
  Reset()
  Pause() / Unpause()
```

### 4.5 Stopwatch Utility

```
Stopwatch
  - Elapsed  float64
  - Paused   bool

Methods:
  Tick(delta float64)
  Reset()
  Pause() / Unpause()
```

### 4.6 Death Spiral Prevention

If the real frame delta exceeds `MaxAccumulation` (default 1.0 second), the accumulator is clamped. This means FixedUpdate runs at most `MaxAccumulation / FixedDelta` times per frame. For a 60 Hz fixed step, that is 60 iterations maximum, regardless of how long the frame actually took.

### 4.7 Virtual Time Controls

Systems can request time manipulation through the TimeVirtual resource:

```
TimeVirtual.Pause()
TimeVirtual.Unpause()
TimeVirtual.SetRelativeSpeed(speed float64)
```

Pausing virtual time automatically pauses fixed timestep accumulation since fixed time derives from virtual time.

### 4.8 Deferred Execution

Systems frequently need to schedule logic for the near future without managing their own stateful timers. The Time system provides two primitives for this:

- **FrameDelay(n, fn)**: Executes `fn` after exactly `n` frame updates. Useful for next-frame initialization, deferred cleanup, or visual pop-in prevention.
- **TimerOnce(duration, fn)**: Executes `fn` after `duration` virtual time has elapsed. Obeys virtual time pause/scale rules.

Deferred callbacks are executed by the engine host/mediator at a specific synchronization point in the frame (typically during the First or Last schedule).

## 5. Open Questions

- Should there be a per-entity time scale (e.g., slow-motion for individual characters)?
- Should Timer and Stopwatch be components, or are they only useful as fields within user components?
- Is `float64` sufficient for elapsed time in sessions lasting many hours, or should a monotonic integer tick be the canonical representation?

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
| 0.1.0 | 2026-03-25 | Initial draft |
| 0.2.0 | 2026-04-20 | Added deferred execution primitives (FrameDelay, TimerOnce) based on 3D Engine analysis |
| — | — | Planned examples: `examples/time/` |
