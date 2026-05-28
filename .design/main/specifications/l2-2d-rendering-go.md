# 2D Rendering — Go Implementation

**Version:** 0.1.0
**Status:** Stable
**Layer:** go
**Implements:** [l1-2d-rendering.md](l1-2d-rendering.md)

## Overview

Go-level design for the 2D pipeline: sprites, 9-slice, custom 2D meshes, and
world-space text rendered as a `RenderFeature` inside the existing render core.
`Sprite2DFeature` reuses the render core's `RenderDataHolder` (struct-of-arrays)
and `VisibilityGroup` (parallel cull), extracting visible 2D entities into the
render world, sorting them deterministically, and batching adjacent same-texture
sprites into single draw calls. An orthographic 2D camera drives projection with
an optional pixel-perfect mode. Pure-data components are public
(`pkg/render/sprite/`); the feature, sorter, and batcher are engine-private
(`internal/render/sprite2d/`).

## Related Specifications

- [l1-2d-rendering.md](l1-2d-rendering.md) — L1 concept specification (parent)
- [l2-render-core-go.md](l2-render-core-go.md) — `RenderFeature`, `RenderDataHolder`, `VisibilityGroup`, render graph
- [l2-mesh-and-image-go.md](l2-mesh-and-image-go.md) — sprite images are `image.Image`; `SpriteMesh` references `mesh.Mesh`; atlas reuse
- [l2-camera-and-visibility-go.md](l2-camera-and-visibility-go.md) — 2D camera reuses `OrthographicProjection` + visibility layers
- [l2-hierarchy-system-go.md](l2-hierarchy-system-go.md) — sprite `GlobalTransform` drives quad placement
- [l2-asset-system-go.md](l2-asset-system-go.md) — `Font` + `Image` handles

## 1. Motivation

A dedicated 2D path avoids forcing sprite games through the full 3D mesh
pipeline, and aggressive texture-atlas batching is essential when thousands of
sprites are on screen. Because the 2D feature is a `RenderFeature` contributing
to the same render graph as 3D, mixed 2D/3D scenes interleave naturally by
Z-order — no duplicate pipeline, no separate render pass tree.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: generics for the SoA keys; `iter.Seq` over extracted sprites.
- **C-003**: stdlib only; glyph rasterization uses a vetted `golang.org/x/image`
  font rasterizer (remote stdlib priority) — recorded in an ADR if added.
- **C-005**: extract + sort run in parallel and MUST be `-race` clean; the main
  world is read-only during extract (render-core INV-4 isolation).
- **C-027**: extracted-sprite structs and the batch vertex scratch are `sync.Pool`-recycled.
- Transparent sprites use back-to-front sorting; **no** order-independent transparency for 2D.
- All sprite coordinates are world-space; screen-space UI is the UI system's domain, not this one.

## 3. Core Invariants

> [!NOTE]
> See [l1-2d-rendering.md §3](l1-2d-rendering.md) for technology-agnostic
> invariants. Go-specific compliance is tabulated in §4.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **1**: `Sprite` without a valid image ⇒ no draw call | `extractSprites` skips entities where `Sprite.Image.IsNil()` or the handle's `LoadState != Loaded`; no `ExtractedSprite` is produced — silently absent from the batch list. |
| **2**: deterministic sort (Z → Y → entity order) | The sort key packs `(zBits, yBits, entityIndex)` into a `uint64`; `slices.SortStableFunc` on the key. Entity index is the stable final tie-breaker, so ties never reorder frame-to-frame. |
| **3**: adjacent same-texture/material/blend ⇒ one draw call | After sort, `batchSprites` walks the list; consecutive sprites sharing `(atlasTexture, material, blendMode)` merge into one instanced draw. A `batchKey uint64` is precomputed so the walk is a single comparison per step. |
| **4**: ortho maps world→pixels per scale; pixel-perfect snaps integer ratios | The 2D `OrthographicProjection` derives its matrix from the scale mode (FixedWidth/FixedHeight/Fit); pixel-perfect mode rounds the scale to the nearest integer ratio and snaps the camera translation to whole pixels before building the view matrix. |

## Go Package

```
pkg/render/sprite/
  sprite.go      // Sprite, Anchor, Rect, flip flags — pure data
  slice.go       // TextureSlicer (9-slice), BorderRect, ScaleMode
  spritemesh.go  // SpriteMesh (references mesh.Mesh for non-rect shapes)
  text2d.go      // Text2D component; TextPipeline + FontAtlas resources
  camera2d.go    // 2D OrthographicProjection helpers, pixel-perfect mode
internal/render/sprite2d/
  feature.go     // Sprite2DFeature implements render.RenderFeature
  extract.go     // ECS → ExtractedSprite (parallel, ThreadLocal scratch)
  sort.go        // batchKey + Z→Y→entity sort
  batch.go       // merge adjacent sprites → instanced draw calls
  slicer.go      // 9-slice quad generation (1 sprite → 9 quads)
  text.go        // glyph rasterization → FontAtlas, glyph-quad mesh
  pick.go        // 2D picking: ray-rect against sprite AABB, topmost wins
```

`pkg/render/sprite` is public, pure-data. `internal/render/sprite2d` is engine-private.

## Type Definitions

```go
type Anchor uint8 // Center, TopLeft, TopRight, BottomLeft, BottomRight, Custom

type Sprite struct { // component — pure data
    Image      asset.Handle[image.Image]
    Color      math.LinearRgba // multiplicative tint, default white
    FlipX      bool
    FlipY      bool
    Anchor     Anchor
    AnchorVec  math.Vec2       // used when Anchor == Custom
    CustomSize *math.Vec2      // overrides image dims (nil ⇒ native)
    Rect       *math.Rect      // sub-region (atlas frame); nil ⇒ whole image
}

type ScaleMode uint8 // Stretch, Tile

type TextureSlicer struct { // alongside Sprite ⇒ 9 quads
    Border         math.Vec4 // top, bottom, left, right (pixels)
    CenterScale    ScaleMode
    MaxCornerScale float32
}

type SpriteMesh struct{ Mesh asset.Handle[mesh.Mesh] } // custom 2D geometry

type Text2D struct {
    Text   string
    Font   asset.Handle[Font]
    Size   float32
    Color  math.LinearRgba
    Anchor Anchor
}

// Render-world structs (internal).
type ExtractedSprite struct {
    Transform math.Mat4
    AtlasUV   math.Rect
    Color     math.LinearRgba
    batchKey  uint64 // (atlasTexture, material, blend)
    sortKey   uint64 // (zBits, yBits, entityIndex)
}

type Sprite2DFeature struct{ /* owns RenderObjects, SoA holder, batch buffers */ }
// implements render.RenderFeature: Collect/Extract/Prepare/Draw/Flush
```

## Key Methods

```go
func NewSprite2DFeature() *Sprite2DFeature        // AddFeature on the RenderSubApp
func NewSprite2DPlugin() app.Plugin               // registers the feature + extract fn

// RenderFeature hooks (render-core contract).
func (f *Sprite2DFeature) Collect(ctx *render.CollectContext)   // enumerate 2D cameras / views
func (f *Sprite2DFeature) Extract(ctx *render.ExtractContext)   // Sprite+Transform → ExtractedSprite
func (f *Sprite2DFeature) Prepare(ctx *render.PrepareContext)   // sort → batch → upload vertex buffers
func (f *Sprite2DFeature) Draw(ctx *render.DrawContext, view *render.RenderView, stage render.RenderPhase)

// Picking (L1 §4.9).
func PickSprite(view *render.RenderView, worldRay math.Ray2D) (entity.EntityID, bool)
```

## Performance Strategy

- **Shared render-core infra**: reuses `RenderDataHolder` (SoA) and
  `VisibilityGroup` parallel cull — a 2D camera builds an orthographic frustum and
  culls through the same batched dispatch; no duplicate culling code (L1 §4.10).
- **Atlas batching**: precomputed `batchKey` makes batch merging a single linear
  pass after the sort; a texture atlas collapses thousands of sprites into a few draws.
- **Sprite associated data** (L1 §4.11): `SpriteProcessorData` caches the vertex
  offset + atlas UV + image change tick; an unchanged sprite only rewrites its
  `Mat4` (one write/frame), regenerating vertices only on image/color version mismatch.
- **`sync.Pool` extract + vertex scratch** (C-027): recycled across frames; steady-state 0-alloc.

## Error Handling

| Condition | Behavior |
| :--- | :--- |
| `Sprite.Image` nil or unloaded | Entity skipped during extract (INV-1); no draw call, no error |
| `Text2D` font unloaded | Text skipped until `Loaded`; glyph atlas built lazily on first ready frame |
| `SpriteMesh` references a non-2D mesh | `slog.Debug`; falls back to mesh AABB for picking, renders raw geometry |
| Atlas region out of image bounds | Clamped to image bounds; `slog.Debug` once per image |
| Pixel-perfect with non-integer viewport | Snaps to nearest integer ratio; sub-pixel residue logged at Debug |

## Testing Strategy

- **INV-2 deterministic sort (C29 gate)**: a scene of overlapping sprites yields a
  byte-stable batch/order hash across runs; Y-sort toggle changes order predictably.
- **INV-1 skip**: sprite with nil image ⇒ zero draw calls.
- **INV-3 batching**: N adjacent same-atlas sprites ⇒ exactly 1 draw call; a material
  break splits into 2.
- **Golden topology**: the render graph node count for a fixed 2D scene is stable (reuses render-core graph test).
- **Race gate (C-005)**: parallel extract over 10k sprites under `-race`.
- **Benchmarks**: `BenchmarkBatchSprites` (0 alloc/op steady), `BenchmarkSpriteSort`.

## 7. Drawbacks & Alternatives

- **Drawback**: back-to-front transparency sort can thrash batches when many
  differently-textured transparent sprites interleave.
  **Alternative**: order-independent transparency (OIT).
  **Decision**: L1 §2 forbids OIT for 2D (determinism + simplicity); atlas usage
  keeps batches coherent in practice. Kept.
- **Drawback**: world-space text regenerates a glyph mesh on string change.
  **Alternative**: GPU-side glyph instancing.
  **Decision**: mesh regen is gated by a change tick (only on text edit); GPU
  glyph instancing is a future optimization.

## Canonical References

<!-- All paths verified on disk; sort/batch/pick algorithms validated by examples/2d
     (sort-key hash ×20, T-5T04) + the C29 P5 gate (T-5T05). Scope note: the
     Sprite2DFeature is headless — Extract/Collect/Draw are no-ops pending ECS-query +
     GPU-backend wiring (mirrors render-core's nopBackend); world-space text rendering
     and pixel-perfect camera are deferred follow-ups. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |
| [SPRITE] | `pkg/render/sprite/sprite.go` | `Sprite`, `Anchor`, `Rect2D`, `TextureSlicer`, `SpriteMesh` components. |
| [FEATURE] | `internal/render/sprite2d/feature.go` | `Sprite2DFeature` (RenderFeature), `ExtractedSprite`, `SortSprites`, `BuildSortKey`, `BatchKey`, `PickSprite`. |
| [EXAMPLE] | `examples/2d/main.go` | 6 sprites → 3 batches, Z→Y→entity sort, pick hit — sort-key hash stable ×20 (T-5T04). |
| [CONFORMANCE] | `examples/2d/main_test.go` | INV-2 sort determinism + INV-3 batch count ×20. |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-28 | Initial L2 draft — Go translation of l1-2d-rendering v0.2.0. Sprite/TextureSlicer/SpriteMesh/Text2D components, `Sprite2DFeature` reusing render-core `RenderDataHolder`+`VisibilityGroup`, deterministic Z→Y→entity sort, atlas batching, sprite associated data, 2D ortho + pixel-perfect camera. Authored ahead of Phase 5 Track C (`/magic.spec`); unblocked by render-core Stable. Draft — L1 parent Draft + no validating examples/2d/ yet. |
