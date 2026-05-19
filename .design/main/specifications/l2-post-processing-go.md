# Post-Processing — Go Implementation

**Version:** 0.1.0
**Status:** Draft
**Layer:** go
**Implements:** [l1-post-processing.md](l1-post-processing.md)

## Overview

Go-level design for the per-camera post-process pipeline: an ordered chain of
full-screen passes appended after the main render phases. The stack is a
camera-owned component; each enabled effect becomes a render-graph node with
auto-chained input/output textures, operating in linear HDR until the
tonemapping boundary, then LDR. Effect settings are individual pure-data
components (presence = enabled). A canonical fixed ordering makes the pipeline
deterministic regardless of insertion order. Public
(`pkg/render/postprocess`).

## Related Specifications

- [l1-post-processing.md](l1-post-processing.md) — L1 concept specification (parent)
- [l2-render-core-go.md](l2-render-core-go.md) — each effect is a `RenderPass` node; graph chains/omits them
- [l2-camera-and-visibility-go.md](l2-camera-and-visibility-go.md) — camera entities own the stack + per-effect settings
- [l2-materials-and-lighting-go.md](l2-materials-and-lighting-go.md) — HDR lighting output is the post-process input

## 1. Motivation

Image-space techniques (AA, tonemapping, bloom, DoF) operate on the rendered
frame, not meshes. A dedicated, per-camera, reorderable pass chain keeps these
concerns out of the scene graph and makes effects toggleable at zero GPU cost
when disabled.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: `iota` `EffectSlot` enum keyed array; generics not required.
- **C-003**: stdlib only; shaders compiled by the backend behind `render`.
- **C-027**: ping-pong intermediate textures are pooled and reused per camera.
- **HDR until tonemap**: bloom/CG run in linear HDR; tonemapping is the single
  HDR→LDR transition; spatial AA (FXAA/SMAA) runs LDR after it.
- **Order is canonical**, not insertion order — a fixed slot table defines it.
- **Disabled = absent node** (not multiply-by-zero) — true zero GPU work.

## 3. Core Invariants

> [!NOTE]
> See [l1-post-processing.md §3](l1-post-processing.md) for the four
> technology-agnostic invariants. Go-specific compliance is tabulated in §4.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: Tonemapping is last color op before encoding | `EffectSlot` ordering places `SlotTonemapping` after all HDR effects; the graph builder rejects (`ErrPostOrder`) any color-space-altering node scheduled after it — only LDR AA may follow. |
| **INV-2**: Deterministic order regardless of insertion | Effects index a fixed `[slotCount]passBuilder` table by `EffectSlot`; the graph is assembled by iterating slots in enum order, never component insertion order. |
| **INV-3**: Disabled effect is zero-cost | A missing settings component ⇒ no graph node is created for that slot; adjacent nodes' I/O RIDs are reconnected at build — no bound resources, no dispatch. |
| **INV-4**: All passes on the render world | Settings are *read* during render-core Extract into render-world structs; every post pass executes in the render SubApp `DrawStage` against render-world RIDs only. |

## Go Package

```
pkg/render/postprocess/
  stack.go        // EffectSlot, canonical order table, PostProcessStack
  settings.go     // per-effect pure-data components (Bloom, Tonemapping, ...)
  tonemap.go      // Tonemapper enum (Reinhard, ACES, AgX, TonyMcMapface)
  colorgrade.go   // ColorGrading params + 3D-LUT handle
  custom.go       // FullscreenMaterial (user pass), insert_after slot
internal/render/postpass/
  builder.go       // slot→RenderPass construction, auto-chain, omit-disabled
  pingpong.go      // pooled HDR/LDR intermediate target management
```

## Type Definitions

```go
// Canonical fixed order (L1 §4.1) — index IS the order (INV-2).
type EffectSlot uint8
const (
    SlotSSAO EffectSlot = iota
    SlotSSR
    SlotBloom
    SlotDepthOfField
    SlotMotionBlur
    SlotChromaticAberration
    SlotFilmGrain
    SlotColorGrading
    SlotTonemapping            // HDR → LDR boundary (INV-1)
    SlotSpatialAA              // FXAA/SMAA, LDR
    slotCount
)

type PostProcessStack struct{ enabled bitset.Bits } // derived from settings presence

// Pure-data per-effect settings (presence on camera = enabled — INV-3).
type BloomSettings struct {
    Threshold, Intensity, Knee float32
    MaxMipLevel                uint8
}
type Tonemapper uint8 // Reinhard, ReinhardLuminance, ACES, AgX, TonyMcMapface
type TonemappingConfig struct {
    Operator Tonemapper
    Exposure float32
    LUT      asset.Handle[image.Image]   // TonyMcMapface
}
type FxaaSettings struct{ Quality uint8 }
type SmaaSettings struct{ Quality uint8 }
type SsaoSettings struct {
    Radius, Bias, Intensity float32
    SampleCount             uint16
    GroundTruth             bool          // GTAO variant
}
type DofSettings struct {
    FocalDistance, FocalRange, MaxBlur float32
    Bokeh                              BokehShape // Circle | Hexagon
}
type MotionBlurSettings struct{ Intensity float32; MaxSamples uint16 }
type ChromaticAberration struct{ Intensity, RadialPower float32 }
type FilmGrain struct{ Intensity, GrainSize float32; Animated bool }
type ColorGrading struct {
    Exposure, Gamma, Saturation, Contrast float32
    LUT                                   asset.Handle[image.Image] // 3D LUT
}

type FullscreenMaterial struct {                 // custom user pass (§4.5)
    Shader      asset.Handle[material.Shader]
    InputTex    []render.RID
    Params      map[string]ShaderValue
    InsertAfter EffectSlot
}
```

## Key Methods

```go
// Build the post chain for one camera (INV-2,3): iterate slots in enum order,
// emit a RenderPass only for present settings, chain RIDs, splice customs.
func BuildPostChain(cam entity.Entity, w *world.World,
    g *render.RenderGraph, sceneColor render.RID) (render.RID, error)

func (s EffectSlot) IsHDR() bool                  // slot < SlotTonemapping
func validateOrder(nodes []EffectSlot) error      // INV-1: nothing color-altering post-tonemap

// Ping-pong intermediate management (C-027 — pooled per camera).
func acquireTarget(cam entity.Entity, hdr bool) render.RID
func releaseTargets(cam entity.Entity)

// Tonemapping operators (pure, unit-tested vs reference curves).
func (t Tonemapper) Apply(hdr math.Vec3, exposure float32) math.Vec3
```

`BuildPostChain` returns the final LDR RID handed to presentation. AA mutual-
exclusion (only one of FXAA/SMAA; TAA+either; MSAA⊥TAA — L1 §4.2) is enforced
here: conflicting settings ⇒ `ErrAAConflict` with the offending pair.

## Performance Strategy

- **Omit-disabled at graph build** (INV-3): no node, no bind group, no dispatch
  for absent effects — adjacent RIDs reconnect; truly zero-cost.
- **Pooled ping-pong targets** (C-027): two HDR + two LDR intermediates per
  camera, `[:0]`-style reuse across frames — 0 alloc steady-state.
- **Bloom mip chain** capped by `MaxMipLevel`; downsample/upsample reuse the
  same pooled pyramid textures.
- **LOD at distance**: SSAO/GTAO `SampleCount` and bloom mip depth can be scaled
  by a quality resource without rebuilding the graph topology.
- **Single canonical iteration**: chain assembly is O(slotCount) with no
  per-frame sort (order is the enum).

## Error Handling

| Condition | Behavior |
| :--- | :--- |
| Color-altering effect after tonemap | `ErrPostOrder` at graph build; frame falls back to no-post passthrough |
| Conflicting AA settings | `ErrAAConflict`; SMAA preferred, FXAA dropped, `slog.Warn` once |
| HDR effect without camera HDR flag | effect skipped; `slog.Warn` once per camera |
| Custom shader compile failure | that custom node omitted; chain reconnects; `slog.Error` |
| LUT handle unresolved | identity LUT used until asset ready (no stall) |

```go
var (
    ErrPostOrder  = errors.New("postprocess: color-altering effect after tonemapping")
    ErrAAConflict = errors.New("postprocess: conflicting anti-aliasing settings")
)
```

## Testing Strategy

- **Order invariant (INV-1/2)**: shuffled settings insertion ⇒ identical graph
  node order; a synthetic node after tonemap ⇒ `ErrPostOrder`.
- **Zero-cost disable (INV-3)**: disabling an effect removes exactly its node;
  assert graph node count and that I/O RIDs reconnect (golden topology).
- **Tonemapper curves**: `Apply` matches reference ACES/AgX/Reinhard values
  within 1e-4 at sampled luminances (table-driven).
- **AA mutual exclusion**: every disallowed combination returns `ErrAAConflict`.
- **Render-world isolation (INV-4)**: settings mutated mid-frame do not affect
  the in-flight extracted snapshot.
- **Benchmarks**: `BenchmarkBuildPostChain` (0 alloc/op steady; pooled targets).

## 7. Drawbacks & Alternatives

- **Drawback**: fixed canonical order limits exotic effect orderings.
  **Alternative**: user-declared dependency graph per effect.
  **Decision**: L1 INV-2 mandates a fixed order; `FullscreenMaterial.
  InsertAfter` is the sanctioned escape hatch.
- **Drawback**: auto-exposure placement undecided.
  **Decision**: L1 Open Question Q1 — <!-- TBD (L1 §5 Q1): auto-exposure as
  separate slot vs part of tonemapping. Modelled as tonemap sub-step for 0.1.0. -->
- **Drawback**: custom-effect ordering constraints vs built-ins are coarse
  (`InsertAfter` slot only).
  **Decision**: L1 Open Question Q2 — <!-- TBD (L1 §5 Q2): richer custom
  ordering constraint model. -->
- **Drawback**: no low-end perf budget enforcement.
  **Decision**: L1 Open Question Q3 — <!-- TBD (L1 §5 Q3): per-tier
  post-processing budget; LOD hooks present, policy deferred. -->

## Canonical References

<!-- MANDATORY for Stable status. Stub — populate when implementation lands
     (Phase 4). Blocked: (1) L1 parent Draft; (2) C29 no examples/3d/ yet;
     (3) C-002 STOP FACTOR Phase 4. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-18 | Initial L2 draft — Go translation of l1-post-processing v0.1.0. Canonical EffectSlot-ordered chain, omit-disabled graph nodes (zero-cost), pooled ping-pong targets, HDR→LDR tonemap boundary guard, AA mutual-exclusion, FullscreenMaterial custom pass. L1 Q1–Q3 carried as TBD. Draft — L1 parent Draft + C29 + C-002 Phase 4 Hold. |
