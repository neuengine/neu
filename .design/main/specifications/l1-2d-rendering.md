# 2D Rendering

**Version:** 0.2.0
**Status:** Stable
**Layer:** concept

## Overview

The 2D rendering pipeline handles sprites, text, and custom 2D meshes. It operates within the render core's extract-prepare-render model, copying visible 2D entities from the main world into the render world each frame. Sprites are sorted by layer and batched aggressively to minimize draw calls. An orthographic camera drives projection, with an optional pixel-perfect mode.

## Related Specifications

- [Render Core](l1-render-core.md)
- [Mesh & Image](l1-mesh-and-image.md)
- [Camera & Visibility](l1-camera-and-visibility.md)

## 1. Motivation

Many games are purely 2D or mix 2D overlays with 3D scenes. A dedicated 2D path avoids forcing sprite-based games through a full 3D mesh pipeline. Batching sprites that share a texture into a single draw call is critical for performance when thousands of sprites are on screen.

## 2. Constraints & Assumptions

- 2D rendering shares the same render graph as 3D; it is a pass within that graph, not a separate pipeline.
- Sprite images are regular `Image` assets loaded through the asset system.
- All coordinates are in world-space; screen-space UI is handled by the UI system, not this pipeline.
- Transparent sprites require correct back-to-front sorting; the engine does not use order-independent transparency for 2D.

## 3. Core Invariants

1. A `Sprite` component without a valid image handle produces no draw call — it is silently skipped.
2. Sorting order is deterministic: primary by Z, secondary by Y (when enabled), tertiary by entity creation order.
3. Sprites sharing the same texture atlas region and material are always batched into one draw call when adjacent in sort order.
4. The 2D camera's orthographic projection maps world units to screen pixels according to its scale; the pixel-perfect option snaps this to integer ratios.

## 4. Detailed Design

### 4.1 Sprite Component

```
Sprite {
    image:       Handle<Image>
    color:       Color          // multiplicative tint, default White
    flip_x:      bool
    flip_y:      bool
    anchor:      Anchor         // Center | TopLeft | Custom(Vec2) ...
    custom_size: Option<Vec2>   // overrides image dimensions
    rect:        Option<Rect>   // sub-region of the image (atlas frame)
}
```

### 4.2 TextureSlice and TextureSlicer

`TextureSlicer` defines 9-slice borders for a sprite, allowing corners to stay fixed while edges and center stretch. Attached alongside a `Sprite`, it causes the extraction system to emit nine quads instead of one.

```
TextureSlicer {
    border:          BorderRect   // top, bottom, left, right in pixels
    center_scale:    ScaleMode    // Stretch | Tile
    max_corner_scale: f32         // cap on corner enlargement
}
```

### 4.3 SpriteMesh

For non-rectangular 2D shapes, `SpriteMesh` references a custom `Mesh` asset with 2D vertex positions and UV coordinates. It passes through the same batching and sorting logic but uses the mesh geometry instead of a generated quad.

### 4.4 Text2D

World-space text is rendered via the `Text2D` component. It delegates to the `TextPipeline` resource, which rasterizes glyphs into a `FontAtlas` texture. The result is a dynamically generated mesh of glyph quads positioned in world coordinates.

```
Text2D {
    text:   String
    font:   Handle<Font>
    size:   f32
    color:  Color
    anchor: Anchor
}
```

### 4.5 Extraction Phase

The `ExtractSprites` system runs during the render world's Extract schedule. It queries all entities with `Sprite` + `Transform` (or `Text2D` + `Transform`), copies the relevant data into the render world as `ExtractedSprite` structs, and discards entities that are not visible to any 2D camera.

### 4.6 Sorting

Extracted sprites are sorted in a dedicated system:

1. **Z-order** — the `Transform` translation Z component.
2. **Y-sort** (optional, enabled per camera) — lower Y values draw first, giving a top-down depth effect.
3. **Entity order** — stable tie-breaker using entity index.

### 4.7 Batching

After sorting, the batcher walks the sorted list and merges consecutive sprites that share the same texture handle, material, and blend mode into a single instanced or vertex-merged draw call. A texture atlas greatly increases batch efficiency.

### 4.8 2D Camera

A 2D camera is an entity with `Camera` + `OrthographicProjection`. The projection defines a scaling mode (fixed width, fixed height, or fit) and a near/far range for Z sorting. Pixel-perfect mode rounds the projection scale to the nearest integer ratio and snaps the camera translation to whole pixels.

### 4.9 Sprite Picking

The 2D picking backend performs ray-rectangle intersection against each sprite's axis-aligned bounding rect in world space. It respects the same sort order so the topmost sprite wins. Custom `SpriteMesh` entities use their mesh AABB instead.

### 4.10 2D RenderFeature

The 2D pipeline registers as a `RenderFeature` (see [render-core.md §4.10](l1-render-core.md)), participating in the same Collect → Extract → Prepare → Draw phases as the 3D pipeline:

```plaintext
Sprite2DFeature implements RenderFeature
  Collect():
    // Enumerate visibility groups for 2D cameras
    // Build per-camera sprite lists

  Extract():
    // Copy Sprite + Transform data into ExtractedSprite structs
    // ThreadLocal scratch buffers for parallel extraction
    // Does NOT block simulation

  Prepare():
    // Sort extracted sprites (Z → Y → entity order)
    // Batch consecutive sprites sharing texture + material + blend
    // Upload batched vertex data to GPU buffers

  Draw(ctx, view, stage):
    // Issue one draw call per batch
    // Bind atlas texture, set blend mode, draw instanced quads
```

**Shared infrastructure**: The 2D feature reuses the same `RenderDataHolder` (struct-of-arrays) and `VisibilityGroup` infrastructure as 3D. A 2D camera creates its own `RenderView` with an orthographic frustum, and culling uses the same batched parallel dispatch. This eliminates duplicate code while allowing 2D-specific optimizations (quad generation, atlas batching).

**Mixed 2D/3D**: Because both pipelines are RenderFeatures contributing to the same render graph, mixing 2D sprites and 3D meshes in a single scene works naturally — they share the render stage and are interleaved by Z-order during sorting.

### 4.11 Sprite Associated Data

The sprite extraction system uses the associated data pattern (see [component-system.md §4.10](l1-component-system.md)) to cache GPU-side resources per sprite entity:

```plaintext
SpriteProcessorData
  vertex_buffer_offset:  uint32       // offset into shared vertex buffer
  atlas_region:          UVRect       // cached texture coordinates
  last_image_version:    uint64       // change tick of source image
  batch_key:             uint64       // precomputed sort/batch key
```

When a sprite's image handle or color changes, `IsDataValid()` detects the version mismatch and regenerates the vertex data and atlas coordinates. Unchanged sprites skip this entirely — only their transform matrix is updated (a single `Mat4` write per frame).

## 5. Open Questions

- Should sprite sheets / texture atlases be a first-class asset type or remain user-managed sub-rects?
- How should animated sprites (flipbook sequences) be expressed — as an animation clip or a dedicated component?
- What is the maximum batch size before diminishing returns from vertex buffer uploads?

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
| 0.1.0 | 2026-03-25 | Initial draft from architecture analysis |
| 0.2.0 | 2026-03-26 | Added 2D RenderFeature integration, sprite associated data caching |
| — | — | Planned examples: `examples/2d/` |
