# Camera & Visibility — Go Implementation

**Version:** 0.1.0
**Status:** Stable
**Layer:** go
**Implements:** [l1-camera-and-visibility.md](l1-camera-and-visibility.md)

## Overview

Go-level design for cameras and the three-layer visibility model. `Camera` is a
pure-data component (render target, viewport, clear config, HDR flag, order);
`PerspectiveProjection`/`OrthographicProjection` produce a `math.Mat4`.
Visibility resolves `Visibility` → `InheritedVisibility` (hierarchy walk) →
`ViewVisibility` (frustum cull). `Frustum` is six `math.HalfSpace` planes
extracted from the view-projection matrix; culling is false-negative-free over
`Aabb`. A `CameraUpdateSystems` set runs in `PostUpdate`. Public
(`pkg/render/camera`).

## Related Specifications

- [l1-camera-and-visibility.md](l1-camera-and-visibility.md) — L1 concept specification (parent)
- [l2-render-core-go.md](l2-render-core-go.md) — `RenderTarget`, per-camera render-graph execution, `RenderView`
- [l2-hierarchy-system-go.md](l2-hierarchy-system-go.md) — `ChildOf`/`Children` drive visibility propagation
- [l2-math-system-go.md](l2-math-system-go.md) — `Mat4`, `Frustum`, `Aabb`, `BoundingSphere`

## 1. Motivation

Without visibility management every entity is submitted regardless of being on
screen or hidden. A layered model gives explicit user control plus automatic
frustum-cull performance, and multiple ordered cameras enable split-screen,
render-to-texture, and minimaps.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: `iota` enums for `Visibility`/`ScalingMode`; generics not required.
- **C-003**: stdlib only; math via `pkg/math` (no external linear-algebra dep).
- **C-027**: per-frame visible-entity slices are pooled per camera.
- **≥1 active camera** required for any output (asserted by the render SubApp).
- **AABB culling**: false positives accepted; tighter bounds require a
  user-supplied `Aabb` override component.
- **Projection recompute** only when params or viewport size change (dirty flag).
- **`Option<Viewport>`** → `*Viewport` (nil = full render target).

## 3. Core Invariants

> [!NOTE]
> See [l1-camera-and-visibility.md §3](l1-camera-and-visibility.md) for
> technology-agnostic invariants. Go-specific compliance is tabulated in §4.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **1**: Entity rendered only if `ViewVisibility` true | The render extract skips any render object whose source entity's `ViewVisibility.Visible == false`; this is the only gate into the draw queue. |
| **2**: Child hidden if parent inherited-hidden | `propagateVisibility` walks `ChildOf` depth-first; a child's `InheritedVisibility` is `parent.Inherited && self != Hidden` — parent-false forces child-false regardless of self. |
| **3**: Frustum cull has no false negatives | `Frustum.IntersectsAabb` uses the conservative positive-vertex test; an AABB touching/inside any plane is kept — proven by a property test over random AABB/frustum pairs. |
| **4**: Camera order deterministic, ID tiebreak | Cameras sorted by `(Order, entity.EntityID)`; `sort.Slice` with the ID as a total tiebreaker — no nondeterministic ordering for equal `Order`. |
| **5**: Perspective near > 0 | `PerspectiveProjection.Matrix()` returns `ErrInvalidNearPlane` if `near <= 0`; the camera is skipped (not rendered) rather than producing a degenerate matrix. |

## Go Package

```
pkg/render/camera/
  camera.go       // Camera, RenderTarget, Viewport, ClearColorConfig
  projection.go   // Perspective/Orthographic, Matrix(), ScalingMode
  bundles.go      // Camera2D, Camera3D convenience constructors
  visibility.go   // Visibility, InheritedVisibility, ViewVisibility components
  frustum.go      // Frustum, Aabb, BoundingSphere, CascadesFrusta
internal/render/cameraupd/
  systems.go       // CameraUpdateSystems set (PostUpdate), propagation+cull
```

## Type Definitions

```go
type RenderTarget struct {
    Kind   TargetKind                   // Window | Texture | OffScreen
    Window entity.Entity                 // when Kind==Window
    Image  asset.Handle[image.Image]     // when Kind==Texture
}

type ClearMode uint8 // ClearCustom, ClearNone, ClearInherit
type ClearColorConfig struct {
    Mode  ClearMode
    Color math.LinearRgba
}

type Camera struct {                     // pure-data component
    Target   RenderTarget
    Viewport *Viewport                    // nil = full target
    Clear    ClearColorConfig
    HDR      bool
    Order    int32                         // lower renders first
    IsActive bool
}

type PerspectiveProjection struct {
    FovY   float32                          // radians
    Aspect float32                          // auto from viewport
    Near   float32                          // > 0 (INV-5)
    Far    float32
}
type OrthographicProjection struct {
    Area        math.Rect
    Near, Far   float32
    ScalingMode ScalingMode                 // WindowSize | FixedVertical | FixedHorizontal
}

type Visibility uint8 // Inherited (default), Hidden, Visible
type InheritedVisibility struct{ Visible bool } // computed
type ViewVisibility struct{ Visible bool }       // computed

type Frustum struct{ Planes [6]math.HalfSpace }  // near,far,L,R,T,B
type CascadesFrusta struct{ Frusta []Frustum }
```

## Key Methods

```go
func Camera2D() []any                              // Camera+Ortho+Transform+Visibility+Frustum
func Camera3D() []any                              // Camera+Perspective+Transform+Visibility+Frustum

func (p PerspectiveProjection) Matrix() (math.Mat4, error)  // INV-5
func (p OrthographicProjection) Matrix() math.Mat4

func FrustumFromViewProj(vp math.Mat4) Frustum     // 6-plane extraction
func (f Frustum) IntersectsAabb(b math.Aabb) bool  // conservative (INV-3)

// CameraUpdateSystems (PostUpdate), in order (L1 §4.7):
func updateProjections(w *world.World)             // dirty-flag gated
func computeViewMatrices(w *world.World)            // from GlobalTransform
func extractFrusta(w *world.World)
func propagateVisibility(w *world.World)            // INV-2, hierarchy DFS
func cullFrustum(w *world.World, pool *task.ComputePool) // INV-1,3; ForBatched

func SortedActiveCameras(w *world.World) []entity.Entity   // (Order, EntityID) — INV-4
```

`cullFrustum` dispatches `task.ForBatched` over entities carrying `Aabb` +
`InheritedVisibility==true`; entities without an `Aabb` (lights, audio) are
treated as always visible per L1 §4.5. Results write `ViewVisibility` — the
single gate the render extract reads (INV-1).

## Performance Strategy

- **Dirty-flag projection recompute** (L1 §2): a `projDirty` tag set by param/
  viewport-resize systems; `updateProjections` only rebuilds tagged cameras.
- **Parallel frustum cull**: `task.ForBatched` over the `Aabb` set, thread-local
  visible collectors merged after join — `-race` clean, O(n/workers).
- **Hierarchy propagation reuses Phase 2** `l2-hierarchy-system-go` traversal —
  no second tree structure; visibility piggybacks the existing `Children` walk.
- **Pooled visible slices** (C-027): each camera's `VisibleObjects` slice is
  retained and `[:0]`-reset per frame — 0 alloc steady-state.
- **BoundingSphere pre-test** (L1 §4.6): coarse sphere reject before the
  6-plane AABB test cuts plane math for clearly-off-screen objects.

## Error Handling

| Condition | Behavior |
| :--- | :--- |
| Perspective `near <= 0` | `ErrInvalidNearPlane`; camera skipped this frame, `slog.Warn` |
| No active camera | render SubApp emits one `slog.Warn` per frame; nothing drawn (not a panic) |
| `RenderTarget` window entity despawned | camera deactivated; `slog.Warn` once |
| Orthographic zero area | `ErrDegenerateOrtho`; camera skipped |

```go
var (
    ErrInvalidNearPlane = errors.New("camera: perspective near plane must be > 0")
    ErrDegenerateOrtho  = errors.New("camera: orthographic area is degenerate")
)
```

## Testing Strategy

- **Visibility propagation**: parent `Hidden` ⇒ all descendants
  `InheritedVisibility==false` regardless of own setting (DFS table test).
- **Frustum no-false-negative**: property test — random AABBs known-inside a
  random frustum are always kept (INV-3).
- **Deterministic order**: cameras with equal `Order` always sort by entity ID;
  shuffled input ⇒ identical output (INV-4).
- **Near-plane guard**: `near <= 0` returns the sentinel; camera excluded.
- **Cull race gate**: 10k entities, `-race`; visible set identical to the
  sequential reference.
- **Benchmarks**: `BenchmarkCullFrustumSoA`, `BenchmarkProjMatrix` (0 alloc/op).

## 7. Drawbacks & Alternatives

- **Drawback**: AABB-only culling over-draws thin/diagonal geometry.
  **Alternative**: occlusion culling (software / GPU-driven).
  **Decision**: L1 Open Question Q1 — <!-- TBD (L1 §5 Q1): occlusion culling
  alongside frustum; AABB-only for 0.1.0. -->
- **Drawback**: bitmask visibility layers vs hierarchy interaction undefined.
  **Decision**: L1 Open Question Q2 — <!-- TBD (L1 §5 Q2): layer-bitmask ×
  hierarchy semantics. RenderGroup mask lives in render-core; binding TBD. -->
- **Drawback**: per-camera full graph re-execution scales linearly with camera
  count.
  **Decision**: L1 Open Question Q3 — <!-- TBD (L1 §5 Q3): simultaneous active
  camera ceiling; measure before sharing passes across cameras. -->

## Canonical References

<!-- MANDATORY for Stable status. Stub — populate when implementation lands
     (Phase 4). Blocked: (1) L1 parent Draft; (2) C29 no examples/camera/ yet;
     (3) C-002 STOP FACTOR Phase 4. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-18 | Initial L2 draft — Go translation of l1-camera-and-visibility v0.1.0. Pure-data Camera, projection Matrix() with near>0 guard, three-layer visibility reusing hierarchy traversal, conservative positive-vertex frustum test, (Order,EntityID) deterministic sort, ForBatched parallel cull. L1 Q1–Q3 carried as TBD. Draft — L1 parent Draft + C29 + C-002 Phase 4 Hold. |
