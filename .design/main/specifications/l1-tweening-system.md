# Tweening System

**Version:** 0.1.0
**Status:** Stable
**Layer:** concept

## Overview

The tweening system provides a data-driven way to smoothly animate numeric values over time using interpolation curves (easing functions). It is designed to run asynchronously alongside the main gameplay loop, enabling fluid UI transitions, camera movements, and procedural animations without custom state tracking in individual components.

## Related Specifications

- [time-system.md](l1-time-system.md) — Tweening requires time progression
- [component-system.md](l1-component-system.md) — Tweens modify component data

## 1. Motivation

Games heavily rely on animations that do not belong to complex skeletal animation systems. Fading opacity, sliding UI panels, camera shakes, or interpolating an entity's scale over 1.5 seconds require managing duration, easing, and current state. Doing this manually in game logic systems leads to bloated code and duplicate state tracking. A dedicated Tweening System centralizes this.

## 2. Constraints & Assumptions

- Tweens must be capable of interpolating scalar values (float), vectors (Vec2, Vec3), and colors.
- Tweens must not block execution; they run in the background.
- Tween execution is typically evaluated during a dedicated `Tweening` schedule/set, often just before `Update` or `UIUpdate`.

## 3. Core Invariants

- **INV-1**: A Tween cleanly cleans itself up upon completion (or destruction of the target entity) to prevent memory leaks.
- **INV-2**: Tweens support standard easing functions (Linear, EaseInQuad, EaseOutBounce, etc.).
- **INV-3**: Tweens correctly respect the time dimension they are attached to (Real or Virtual).

## 4. Detailed Design

### 4.1 Tween Component

A tween is represented as a component (or set of components) attached to an entity, defining what is animated and how.

```plaintext
Tween
  - TargetField:    string / Reflection Path // e.g. "Transform.Translation.X"
  - StartValue:     any
  - EndValue:       any
  - Duration:       float64
  - Elapsed:        float64
  - Easing:         func(float64) float64
  - LoopMode:       Once | Loop | PingPong
  - TimeDimension:  Virtual | Real
```

### 4.2 Interpolation Execution

The tweening system runs each frame, iterating over all active `Tween` components:

1. Determine `Delta` based on the tween's `TimeDimension`.
2. Advance `Elapsed += Delta`.
3. Calculate normalized progress `t = Elapsed / Duration`.
4. Apply the easing function `e = Easing(t)`.
5. Interpolate `CurrentValue = Lerp(StartValue, EndValue, e)`.
6. Apply `CurrentValue` to the `TargetField`.
7. Handle completion (Loop, PingPong, or Despawn the Tween).

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
| 0.1.0 | 2026-04-20 | Initial draft based on 3D Engine analysis |
| — | — | Planned examples: `examples/tweening/` |
