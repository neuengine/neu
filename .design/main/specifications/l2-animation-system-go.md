# Animation System — Go Implementation

**Version:** 0.1.0
**Status:** Draft
**Layer:** go
**Implements:** [l1-animation-system.md](l1-animation-system.md)

## Overview

Go-level design for the animation system: immutable `AnimationClip` assets of
sampled `VariableCurve`s, an `AnimationPlayer` component driving playback, an
`AnimationGraph` asset (state machine of clip/blend/add nodes with cross-fade
transitions), skeletal animation over `Joint`/`Skin` hierarchies, morph-target
blend shapes, timed animation events, and reflection-resolved property paths so
any registered component field is animatable. Evaluation is deterministic and
parallelizable per root entity. Assets + pure-data components are public
(`pkg/animation/`); the evaluator, blend tree, and systems are engine-private
(`internal/animation/`).

## Related Specifications

- [l1-animation-system.md](l1-animation-system.md) — L1 concept specification (parent)
- [l2-math-system-go.md](l2-math-system-go.md) — `Quat.Slerp`, curve interpolation, `Mat4` for skinning
- [l2-hierarchy-system-go.md](l2-hierarchy-system-go.md) — joints are `ChildOf` entities; transform propagation
- [l2-type-registry-go.md](l2-type-registry-go.md) — reflection resolves `PropertyPath` → typed write accessor
- [l2-asset-system-go.md](l2-asset-system-go.md) — clips and graphs are `asset.Handle`s
- [l2-task-system-go.md](l2-task-system-go.md) — per-root parallel evaluation via `ForBatched`
- [l2-event-system-go.md](l2-event-system-go.md) — animation events dispatched through the `EventBus`

## 1. Motivation

Animation is data-driven: clips and graphs are authored content, runtime
variation comes only from player speed/weight/state. A single curve-evaluation
engine that targets *any* reflected property handles skeletal rigs, morph faces,
material fades, and UI motion alike — no per-feature animation code. Determinism
(INV-1) is required so networked/replay scenarios reproduce exactly.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: generics for typed curve sampling; `iter.Seq` over active animations.
- **C-003**: stdlib only; interpolation reuses `pkg/math` (no external math/anim libs).
- **C-005**: per-root evaluation is parallel and MUST be `-race` clean — graphs on
  distinct root hierarchies share no state.
- **C-027**: per-frame sample scratch (pose buffers) is `sync.Pool`-recycled.
- **Determinism (INV-1)**: no `map` iteration order in evaluation output, no wall-clock
  reads, no PRNG — sampling is a pure function of `(clip, time, weights)`.
- **Schedule (INV-3)**: animation systems run in `PostUpdate`, ordered `Before`
  the hierarchy transform-propagation system.
- Property paths are resolved + cached at clip-load time, not per frame.

## 3. Core Invariants

> [!NOTE]
> See [l1-animation-system.md §3](l1-animation-system.md) for technology-agnostic
> invariants. Go-specific compliance is tabulated in §4.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: deterministic output for same input | Curve sampling is a pure function; active animations are evaluated in a stable slice order (sorted by `AnimationNodeIndex`), never `map` order; no PRNG/wall-clock. A golden pose-hash test pins determinism across ≥20 runs. |
| **INV-2**: skeletal respects hierarchy | Skeletal clips write **local** `Transform` on `Joint` entities only; the existing `l2-hierarchy-system-go` propagation computes `GlobalTransform`. The system never writes world-space directly. |
| **INV-3**: runs PostUpdate before transform propagation | `AnimationPlugin` adds `animationEvaluate` to `PostUpdate` with `.Before(hierarchy.PropagateTransforms)`; verified by a schedule-order assertion test. |
| **INV-4**: missing joints/morph targets silently skipped | `resolveTarget` returns `(accessor, ok)`; `!ok` (entity path or property unresolved) skips that curve — no panic, no error. A clip authored for a full skeleton runs on a partial one. |

## Go Package

```
pkg/animation/
  clip.go        // AnimationClip asset, VariableCurve, Keyframes, Interpolation
  graph.go       // AnimationGraph asset, ClipNode/BlendNode/AddNode, Transition
  components.go  // AnimationPlayer, ActiveAnimation, Joint, Skin, MorphWeights
  target.go      // AnimationTargetId (EntityPath + PropertyPath), RepeatMode
  event.go       // AnimationEvent (time + payload)
internal/animation/
  player.go      // active-animation advance, clip sampling, repeat/pingpong
  graph_eval.go  // state machine, blend-tree weights, cross-fade dual-state eval
  skeletal.go    // joint local-transform write, skin/inverse-bind for GPU skinning
  morph.go       // MorphWeights application to mesh morph offsets
  propertypath.go// reflection: PropertyPath → cached typed write accessor
  events.go      // [prev,cur] interval crossing → EventBus dispatch
  systems.go     // animationEvaluate (PostUpdate), parallel per-root partition
```

`pkg/animation` is public, assets + pure-data components. `internal/animation` is engine-private.

## Type Definitions

```go
type Interpolation uint8 // Step, Linear, CubicSpline

type Keyframes struct {
    Times    []float32
    Values   []float32 // flattened; stride = component width (1/3/4/...)
    Tangents []float32 // CubicSpline only: in/out per keyframe
    Interp   Interpolation
}

type VariableCurve struct {
    Target    AnimationTargetId
    Keyframes Keyframes
}

type AnimationClip struct { // asset
    Duration float32
    Curves   []VariableCurve
    Events   []AnimationEvent
}

type AnimationTargetId struct {
    EntityPath string // relative to player entity, e.g. "Armature/Hips/Spine"
    Property   string // reflection path, e.g. "Transform.Translation"
}

type RepeatMode uint8 // Once, Loop, PingPong

type ActiveAnimation struct {
    Clip        asset.Handle[AnimationClip]
    Elapsed     float32
    Speed       float32 // negative ⇒ reverse
    Repeat      RepeatMode
    BlendWeight float32
    Paused      bool
}

type AnimationPlayer struct { // component
    Active map[AnimationNodeIndex]ActiveAnimation // layered playback
}

type AnimationNodeIndex uint32

// Graph nodes (sum type via interface).
type AnimationNode interface{ kind() nodeKind }
type ClipNode  struct{ Clip asset.Handle[AnimationClip] }
type BlendNode struct{ Param string; Children []AnimationNodeIndex; Thresholds []float32 }
type AddNode   struct{ Base, Additive AnimationNodeIndex }

type Transition struct {
    Target        AnimationNodeIndex
    Condition     TransitionCondition // threshold | trigger | clipFinished
    BlendDuration float32
    BlendCurve    math.EasingFn
}

type Joint struct{ Index uint16 } // marks a skeleton joint entity
type Skin  struct {
    Joints              []entity.EntityID
    InverseBindMatrices []math.Mat4
}
type MorphWeights struct{ Weights []float32 } // [0,1] per target

type AnimationEvent struct {
    Time    float32
    Payload any // dispatched via EventBus
}
```

## Key Methods

```go
func NewAnimationPlugin() app.Plugin // registers systems (PostUpdate, Before propagation)

// Player control.
func (p *AnimationPlayer) Play(node AnimationNodeIndex, clip asset.Handle[AnimationClip], repeat RepeatMode)
func (p *AnimationPlayer) Seek(node AnimationNodeIndex, t float32)
func (p *AnimationPlayer) SetSpeed(node AnimationNodeIndex, s float32)

// Sampling (pure — INV-1).
func (c *AnimationClip) Sample(t float32, out *Pose) // writes resolved curve values
func sampleCurve(k Keyframes, t float32) []float32   // Step/Linear/Slerp/CubicSpline

// Reflection (INV-4).
func resolveTarget(reg *typereg.Registry, root entity.EntityID, id AnimationTargetId) (writeAccessor, bool)
```

## Performance Strategy

- **Per-root parallel evaluation** (L1 §4.10): partition active players by root
  entity, `task.ForBatched` over partitions — independent hierarchies, no locks.
- **Cached write accessors**: `PropertyPath` resolved once at clip load into a
  typed setter closure; per-frame eval avoids reflection lookups (L1 §4.8).
- **`sync.Pool` pose buffers** (C-027): the per-entity `Pose` (joint transforms +
  morph weights) is recycled; steady-state eval is 0-alloc.
- **Slerp via `pkg/math`**: quaternion rotations use `Quat.Slerp` (0-alloc).
- **Cross-fade**: during a `Transition`, source + target sub-trees are sampled
  into two pose buffers and lerped by a ramping weight — two samples, one blend.

## Error Handling

| Condition | Behavior |
| :--- | :--- |
| Unresolved entity path / property | Curve skipped (INV-4); `slog.Debug` once per `(clip,target)` |
| Clip handle not yet loaded | Player holds last pose; no advance until `Loaded` |
| Graph transition topology has an illegal cycle (non-looping) | `ErrAnimGraphCycle` at graph load — fail fast |
| Skin joint count ≠ inverse-bind-matrix count | `ErrSkinMismatch` at load; entity renders unskinned |
| Morph weight slice longer than mesh targets | Extra weights ignored; `slog.Debug` |

```go
var (
    ErrAnimGraphCycle = errors.New("animation: illegal cycle in graph transitions")
    ErrSkinMismatch   = errors.New("animation: skin joints / inverse-bind-matrix length mismatch")
)
```

## Testing Strategy

- **INV-1 determinism (C29 gate)**: sample a graph blend over a fixed timeline;
  assert identical pose hash across ≥20 runs.
- **INV-3 schedule order**: assert `animationEvaluate` runs before transform propagation.
- **INV-4 partial animation**: clip targeting absent joints → no panic, present joints animate.
- **Curve correctness**: Step/Linear/CubicSpline + quaternion Slerp vs reference; events fire exactly once on `[prev,cur]` crossing.
- **Race gate (C-005)**: parallel eval over 1k independent skeletons under `-race`.
- **Benchmarks**: `BenchmarkSampleClip` (0 alloc/op steady), `BenchmarkGraphBlend`.

## 7. Drawbacks & Alternatives

- **Drawback**: reflection-resolved property paths add a load-time cost and a
  string-typed target.
  **Alternative**: code-generated typed targets per animatable component.
  **Decision**: reflection (cached at load) keeps clips data-driven and editor-
  authorable; codegen is a future optimization (deferred to `l2-codegen-tools`).
- **Drawback**: per-root partitioning under-utilizes cores when one hierarchy
  dominates (a 200-joint boss).
  **Alternative**: per-curve parallelism.
  **Decision**: per-root is determinism-safe and contention-free; per-curve adds
  merge complexity. Revisit if profiling shows a single-root bottleneck.

## Canonical References

<!-- MANDATORY for Stable status. Stub — populate when implementation lands
     (Phase 5 Track D). Stable promotion blocked until: (1) examples/animation/
     validates clip+graph+skeletal determinism (T-5T02); (2) C29 P5 gate (T-5T05). -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-28 | Initial L2 draft — Go translation of l1-animation-system v0.1.0. Clip/VariableCurve sampling, AnimationGraph (clip/blend/add + cross-fade), skeletal Joint/Skin, MorphWeights, reflection-resolved targets, per-root parallel deterministic eval. Authored ahead of Phase 5 Track D (`/magic.spec`). Draft — L1 parent Draft + no validating examples/animation/ yet. |
