# Tweening System — Go Implementation

**Version:** 0.1.0
**Status:** Stable
**Layer:** go
**Implements:** [l1-tweening-system.md](l1-tweening-system.md)

## Overview

Go-level design for the tweening system: a `Tween` component interpolates a
target field over time through an easing function, advancing on a dedicated
`Tweening` schedule. It supports scalars, vectors, and colors; respects the
Real or Virtual time dimension; and cleans itself up on completion or target
despawn so no state leaks. Components are public (`pkg/tween/`); the advance
system and reflection write-back are engine-private (`internal/tween/`).

## Related Specifications

- [l1-tweening-system.md](l1-tweening-system.md) — L1 concept specification (parent)
- [l2-time-system-go.md](l2-time-system-go.md) — delta from `VirtualTime`/`RealTime` per `TimeDimension`
- [l2-component-system-go.md](l2-component-system-go.md) — tweens mutate component data; removal hooks
- [l2-math-system-go.md](l2-math-system-go.md) — easing curves + `Lerp` for scalar/Vec2/Vec3/Color
- [l2-type-registry-go.md](l2-type-registry-go.md) — `TargetField` reflection path → typed setter

## 1. Motivation

Many animations — fading opacity, sliding panels, camera shakes, scale pulses —
do not warrant the full skeletal/graph animation system. Hand-rolling duration,
easing, and per-component state leads to bloated, duplicated game logic. A
dedicated `Tween` component centralizes this with self-managing lifetime, so a
gameplay system just attaches a tween and forgets it.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: generics for typed `Lerp[T]`; `iter.Seq` over active tweens.
- **C-003**: stdlib only; easing functions and `Lerp` reuse `pkg/math`.
- **C-005**: tween advance may run in parallel over independent target entities
  and MUST be `-race` clean (each tween writes only its own target field).
- **C-027**: tween advance is allocation-free in steady state (no per-frame heap).
- Tweens run on a dedicated `Tweening` schedule, ordered before `Update`/`UIUpdate`.
- Interpolation supports `float32`, `Vec2`, `Vec3`, and `Color` (extensible via the registry).

## 3. Core Invariants

> [!NOTE]
> See [l1-tweening-system.md §3](l1-tweening-system.md) for technology-agnostic
> invariants. Go-specific compliance is tabulated in §4.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: tween self-cleans on completion or target destruction | On `Elapsed >= Duration` in `LoopOnce`, the advance system queues `commands.Remove[Tween](entity)` (or `Despawn` if configured). A component remove-hook on the target's despawn drops any attached tween — no orphaned tween survives its target. |
| **INV-2**: supports standard easing functions | `EasingFn func(float64) float64` with a stdlib library: `Linear`, `EaseInQuad`…`EaseOutBounce` (reusing `pkg/math` curve primitives). `Tween.Easing` defaults to `Linear` when nil. |
| **INV-3**: respects the attached time dimension | The advance system selects `dt` from `RealTime.Delta()` or `VirtualTime.Delta()` based on `Tween.TimeDimension`; a paused `VirtualTime` freezes Virtual tweens while Real tweens (e.g. pause-menu fades) keep running. |

## Go Package

```
pkg/tween/
  tween.go     // Tween component, LoopMode, TimeDimension
  easing.go    // EasingFn + standard easing library (reuses pkg/math curves)
  lerp.go      // generic Lerp[T] for float32/Vec2/Vec3/Color
internal/tween/
  system.go    // advance system: delta → progress → easing → lerp → apply
  apply.go     // reflection write-back of CurrentValue to TargetField
  lifecycle.go // completion (Once/Loop/PingPong/Despawn), self-cleanup hooks
```

`pkg/tween` is public, pure-data + pure functions. `internal/tween` is engine-private.

## Type Definitions

```go
type LoopMode uint8 // LoopOnce, Loop, PingPong

type TimeDimension uint8 // Virtual, Real

type Tween struct { // component
    TargetField   string        // reflection path, e.g. "Transform.Translation.X"
    StartValue    any           // float32 | Vec2 | Vec3 | Color
    EndValue      any
    Duration      float64       // seconds
    Elapsed       float64
    Easing        EasingFn      // nil ⇒ Linear
    LoopMode      LoopMode
    TimeDimension TimeDimension
    DespawnOnDone bool          // if true, despawn the entity instead of removing the Tween
}

type EasingFn func(t float64) float64 // domain/range [0,1]

// Standard easing library (selection).
var (
    Linear      EasingFn
    EaseInQuad  EasingFn
    EaseOutQuad EasingFn
    EaseInOutQuad EasingFn
    EaseOutBounce EasingFn
    // … full set in easing.go
)
```

## Key Methods

```go
func NewTweenPlugin() app.Plugin // registers the Tweening schedule + advance system

// Generic interpolation (INV-2) — pure, 0-alloc for value types.
func Lerp[T Lerpable](start, end T, t float32) T // float32 | Vec2 | Vec3 | Color

// Advance (internal): one pass over active tweens.
//   dt := timeForDimension(tw.TimeDimension)
//   tw.Elapsed += dt
//   t := clamp01(tw.Elapsed / tw.Duration)
//   v := Lerp(tw.StartValue, tw.EndValue, float32(tw.Easing(t)))
//   apply(entity, tw.TargetField, v)
//   handleCompletion(tw)  // Once/Loop/PingPong/Despawn (INV-1)
```

## Performance Strategy

- **Cached setter**: `TargetField` is resolved once (on tween insert, via a
  component add-hook) into a typed write accessor; per-frame apply avoids
  reflection lookups (mirrors `l2-animation-system-go` property-path caching).
- **Parallel advance**: independent target entities advance via `task.ForBatched`;
  each tween writes only its own field — `-race` clean, no locks.
- **0-alloc steady state** (C-027): value-typed `Lerp` and reused accessors mean
  the advance loop performs no heap allocation.
- **Deferred completion**: removal/despawn is queued to the command buffer and
  applied at the schedule's flush point — never mutates the world mid-iteration.

## Error Handling

| Condition | Behavior |
| :--- | :--- |
| `TargetField` unresolved (bad path) | Tween removed on first apply; `slog.Debug` once per `(type,path)` — no panic |
| `Start`/`End` value type mismatch | `ErrTweenTypeMismatch` at insert (add-hook validation); tween not activated |
| `Duration <= 0` | Tween applies `EndValue` immediately, then completes (degenerate but safe) |
| Target entity despawned mid-tween | Remove-hook drops the tween; no dangling state (INV-1) |

```go
var ErrTweenTypeMismatch = errors.New("tween: Start/End value types differ or unsupported")
```

## Testing Strategy

- **INV-1 self-cleanup (C29 gate)**: despawn a target mid-tween → assert no tween
  component remains and no goroutine/state leak (`NumGoroutine` baseline).
- **INV-2 easing**: table-driven easing samples vs reference (endpoints exact; monotonic where expected).
- **INV-3 time dimension**: pause `VirtualTime` → Virtual tweens freeze, Real tweens advance.
- **LoopMode**: Once removes; Loop wraps `Elapsed`; PingPong reverses direction at bounds.
- **Race gate (C-005)**: parallel advance over 10k tweens on distinct entities under `-race`.
- **Benchmarks**: `BenchmarkTweenAdvance` / `BenchmarkLerpVec3` — 0 alloc/op.

## 7. Drawbacks & Alternatives

- **Drawback**: `any`-typed Start/End values box value types and defer type
  checking to insert time.
  **Alternative**: a generic `Tween[T]` component per value type.
  **Decision**: a single `Tween` component keeps the API uniform and avoids a
  combinatorial set of registered component types; the add-hook validates types
  once and caches a typed accessor, so the per-frame path stays unboxed.
- **Drawback**: overlaps with `l2-animation-system-go` for simple property fades.
  **Alternative**: fold tweening into animation clips.
  **Decision**: L1 §1 — tweens are imperative/fire-and-forget (UI, camera) and
  must not require authoring a clip asset. Distinct, complementary systems.

## Canonical References

<!-- All paths verified on disk; easing + LoopMode + time-dimension + self-cleanup
     validated by examples/tweening (hash ×20, T-5T03) + the C29 P5 gate (T-5T05).
     BenchmarkLerpVec3 = 0 B/op 0 allocs/op (C-027). -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |
| [TWEEN] | `pkg/tween/tween.go` | `Tween` component, `LoopMode`, `TimeDimension`. |
| [EASING] | `pkg/tween/easing.go` | `EasingFn` + easing library (quad / cubic / sine / bounce / elastic). |
| [LERP] | `pkg/tween/lerp.go` | Generic `Lerp[T]`, `LerpAny`, `ErrTweenTypeMismatch`. |
| [SYSTEM] | `internal/tween/system.go` | `writeAccessor` (reflection path), `AdvanceTween` (Loop / PingPong / LoopOnce). |
| [SYSTEM_TEST] | `internal/tween/system_test.go` | Advance / loop / ping-pong / despawn + reflection write tests. |
| [CONTRACT] | `pkg/tween/tween_test.go` | Easing + generic `Lerp` 0-alloc contract tests. |
| [EXAMPLE] | `examples/tweening/main.go` | EaseOutQuad LoopOnce, Loop wrap, PingPong, despawn — hash stable ×20 (T-5T03). |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-28 | Initial L2 draft — Go translation of l1-tweening-system v0.1.0. `Tween` component, easing library + generic `Lerp`, `Tweening` schedule advance with Real/Virtual delta, self-cleanup on completion/despawn (INV-1), cached reflection write-back. Authored ahead of Phase 5 Track E (`/magic.spec`). Draft — L1 parent Draft + no validating examples/tweening/ yet. |
