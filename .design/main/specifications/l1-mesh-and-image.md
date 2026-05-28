# Mesh & Image

**Version:** 0.1.0
**Status:** Stable
**Layer:** concept

## Overview

Meshes and images are the two foundational GPU-side assets. A mesh describes geometry through vertex attributes and index data. An image describes texture data with format and mip information. Both are loaded through the asset system, transferred to the render World, and referenced by components via asset handles.

## Related Specifications

- [Asset System](l1-asset-system.md)
- [Render Core](l1-render-core.md)
- [Animation System](l1-animation-system.md)

## 1. Motivation

Geometry and texture data follow different lifecycles than gameplay state. Treating them as assets with explicit GPU upload steps keeps memory ownership clear and enables streaming, caching, and hot-reload without touching the main World's component storage.

## 2. Constraints & Assumptions

- Mesh and image assets are immutable once uploaded to the GPU; modifications produce new asset versions.
- Vertex layouts are configurable but must be declared before pipeline compilation.
- The engine does not mandate a single vertex format — custom attributes are first-class.
- Image decoding happens on a background thread; a placeholder texture is used until ready.

## 3. Core Invariants

1. Every mesh must have at least a position attribute.
2. Index buffers, if present, must not reference out-of-bounds vertices.
3. Skinned meshes must have matching joint weight and joint index attribute lengths.
4. Image assets must declare a valid GPU-compatible format at creation time.
5. Texture atlas layouts must not contain overlapping regions.

## 4. Detailed Design

### 4.1 Mesh Asset Structure

```plaintext
Mesh
  ├── primitive_topology: TriangleList | TriangleStrip | LineList | ...
  ├── attributes[]
  │     ├── Position   (vec3)
  │     ├── Normal     (vec3)
  │     ├── UV0        (vec2)
  │     ├── UV1        (vec2)  [optional]
  │     ├── Tangent    (vec4)  [optional]
  │     ├── Color      (vec4)  [optional]
  │     └── Custom(id) (vecN)  [optional]
  ├── indices: Option<IndexBuffer>  (u16 or u32)
  └── sub_meshes[]   (offset + count for multi-material)
```

### 4.2 Vertex Layout

A `VertexLayout` describes the set of attributes, their formats, and byte offsets. Layouts are interleaved by default but may be split into separate buffers for specific backends. The layout is hashed and used as part of the pipeline specialization key.

### 4.3 Mesh Components

- `Mesh3D(Handle<Mesh>)` — attaches a 3D mesh asset to an entity.
- `Mesh2D(Handle<Mesh>)` — attaches a 2D mesh asset (typically a quad or custom polygon).

Both are marker components that the render extract phase reads to queue draw items.

### 4.4 Built-in Mesh Primitives

The engine provides generator functions that return `Mesh` assets:

| Primitive | Key Parameters |
| :--- | :--- |
| Cube | size |
| Sphere | radius, sectors, stacks |
| Plane | size, subdivisions |
| Cylinder | radius, height, segments |
| Capsule | radius, height, segments, rings |
| Torus | major_radius, minor_radius, segments, sides |

Generators produce position, normal, UV, and tangent attributes. Index buffers use `u32`.

### 4.5 Skinning

Skinned meshes carry two additional vertex attributes:

- `JointWeights` (vec4) — blend weights for up to 4 joints per vertex.
- `JointIndices` (uvec4) — indices into the skeleton's joint list.

A `SkinnedMesh` component references the joint entity list. The skinning compute pass reads joint transforms and writes the final vertex positions into a staging buffer each frame.

### 4.6 Morph Targets

Morph targets store per-vertex deltas for position, normal, and tangent. A `MorphWeights` component holds a list of blend weights (one per target). The morph compute pass blends the deltas and writes the result before the skinning pass (if both are present).

### 4.7 Image Asset Structure

```plaintext
Image
  ├── format: RGBA8, RGBA16Float, BC7, ASTC4x4, ...
  ├── width, height
  ├── mip_levels: u32
  ├── data: bytes
  └── sampler_descriptor: FilterMode, AddressMode, ...
```

Images are decoded on a background thread and uploaded to the GPU via a staging buffer. Mip maps are generated at load time unless the source format already contains them (e.g., DDS, KTX2).

### 4.8 Texture Atlas

A `TextureAtlasLayout` asset defines named rectangular regions within a single image:

```plaintext
TextureAtlasLayout
  └── regions[]
        ├── name: string
        ├── min: (u32, u32)
        └── max: (u32, u32)
```

The `TextureAtlas` component pairs a `Handle<Image>` with a `Handle<TextureAtlasLayout>` and a current region index. Sprite rendering reads the atlas to compute UV coordinates.

### 4.9 Dynamic Atlas Builder

For runtime-generated content (text glyphs, procedural decals), a `DynamicAtlas` allocates rectangles in a growable GPU texture using a shelf-packing algorithm. When the atlas is full, it doubles in size and re-packs.

### 4.10 Supported Image Formats

Loaders are registered through the asset system. Built-in loaders cover: PNG, JPEG, HDR, BMP, TGA, DDS, KTX2. Additional formats can be added via plugins.

## 5. Open Questions

1. Should meshes support more than 4 joint influences per vertex for high-fidelity characters?
2. What is the maximum texture atlas size before automatic splitting is needed?
3. How should GPU memory pressure trigger mesh/image eviction?

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
| — | — | Planned examples: `examples/3d/` |
