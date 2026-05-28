# Post-Processing

**Version:** 0.1.0
**Status:** Stable
**Layer:** concept

## Overview

Post-processing applies a chain of full-screen passes after the main scene rendering completes. Effects are organized into an ordered pipeline configurable per camera. Each effect reads the previous stage's output texture and writes to the next, culminating in final framebuffer presentation. The pipeline operates in HDR space until tonemapping converts to LDR for display output.

## Related Specifications

- [render-core.md](l1-render-core.md) — Render graph integration and pass scheduling
- [camera-and-visibility.md](l1-camera-and-visibility.md) — Camera entities own post-process settings
- [materials-and-lighting.md](l1-materials-and-lighting.md) — Lighting output feeds the post-process input

## 1. Motivation

Many visual techniques — anti-aliasing, tonemapping, bloom, depth of field — operate on the fully rendered image rather than individual meshes. A dedicated post-processing pipeline keeps these concerns separate from geometry rendering, enables per-camera customization, and allows effects to be toggled or reordered without modifying the scene graph.

## 2. Constraints & Assumptions

- Post-processing runs after all render phases (Opaque, AlphaMask, Transparent, UI) have completed.
- Effects that require auxiliary buffers (depth, velocity, normals) assume those buffers were written during the main passes.
- HDR effects (bloom, tonemapping) require the camera's HDR flag to be enabled.
- The post-process stack is evaluated in order; effects may not reference outputs of later effects.

## 3. Core Invariants

- **INV-1**: Tonemapping is always the last color operation before output encoding. No color-space-altering effect may follow it.
- **INV-2**: Post-process effects run in deterministic order regardless of component insertion order. The pipeline defines a fixed canonical ordering.
- **INV-3**: Disabling an effect is zero-cost — no GPU work is dispatched, not just a multiply-by-zero passthrough.
- **INV-4**: All post-process passes operate on the render world, never the main world.

## 4. Detailed Design

### 4.1 Post-Process Pipeline

The pipeline is a chain of full-screen passes applied after main rendering:

```
Camera
  └── PostProcessStack
        ├── [0] SSAO / GTAO
        ├── [1] SSR
        ├── [2] Bloom
        ├── [3] DepthOfField
        ├── [4] MotionBlur
        ├── [5] ChromaticAberration
        ├── [6] FilmGrain
        ├── [7] ColorGrading
        ├── [8] Tonemapping        ← HDR → LDR boundary
        └── [9] FXAA / SMAA        ← operates in LDR
```

The stack is a component on the camera entity. Each entry is an effect descriptor. The render graph inserts a fullscreen pass per enabled effect, chaining input/output textures automatically. Disabled effects are omitted from the graph — adjacent nodes reconnect seamlessly.

### 4.2 Built-in Effects

**Tonemapping** — Reinhard, ReinhardLuminance, ACES, AgX, TonyMcMapface (LUT-based).

**Bloom** — Bright region light bleed. Threshold extraction → downsample chain (successive half-res blurs) → upsample chain (additive blend) → composite. Parameters: `threshold`, `intensity`, `knee`, `max_mip_level`.

**Anti-aliasing:**

| Method | Type | Notes |
| :--- | :--- | :--- |
| MSAA | Hardware | Configured on the render target, not a post-process pass |
| FXAA | Post-process | Fast, slight blurring, operates on LDR |
| TAA | Post-process | Temporal accumulation, needs velocity buffer and jittered projection |
| SMAA | Post-process | Edge-detection + blending, higher quality than FXAA |

Only one spatial AA method (FXAA, SMAA) should be active at a time. TAA may combine with either. MSAA is mutually exclusive with TAA.

**Ambient Occlusion:**

- SSAO — Screen-space ambient occlusion. Samples depth buffer in a hemisphere per pixel. Parameters: `radius`, `bias`, `intensity`, `sample_count`.
- GTAO — Ground-truth ambient occlusion. Higher quality variant using horizon-based integration. Same parameter set with improved accuracy.

**Depth of Field** — Lens focus simulation via gather-based blur. Parameters: `focal_distance`, `focal_range`, `max_blur`, `bokeh_shape` (Circle, Hexagon).

**Motion Blur** — Per-pixel blur along the velocity vector from the velocity buffer. Parameters: `intensity`, `max_samples`.

**Chromatic Aberration** — Radial RGB channel offset simulating lens fringing. Parameters: `intensity`, `radial_power`.

**Film Grain** — Procedural noise overlay simulating analog film texture. Parameters: `intensity`, `grain_size`, `animated` (per-frame variation).

### 4.3 HDR Pipeline

The rendering pipeline operates in linear HDR space throughout the geometry and lighting passes:

```
Linear HDR Rendering → Post-Process Effects (HDR) → Tonemapping (HDR→LDR) → Output Encoding
```

HDR display support: when the output surface supports HDR (e.g., HDR10, scRGB), tonemapping maps to the extended range instead of [0,1]. The output transfer function is selected based on the display's reported capabilities.

### 4.4 Color Grading

Color grading adjusts the final image appearance. Applied in HDR space before tonemapping:

```
ColorGrading
  exposure:    f32    // EV adjustment (-5..+5)
  gamma:       f32    // mid-tone adjustment
  saturation:  f32    // color intensity multiplier
  contrast:    f32    // tonal range expansion/compression
  lut:         Handle<Image>  // optional 3D LUT for artist-authored color transforms
```

LUT-based correction applies a 3D lookup table (typically 32x32x32 or 64x64x64) for precise color mapping. LUTs are authored in external tools and imported as image assets.

### 4.5 Custom Post-Process Passes

Users define custom full-screen shader passes inserted at configurable points in the pipeline:

```
FullscreenMaterial
  ├── shader:          Handle<Shader>
  ├── input_textures:  [Handle<Texture>]
  ├── parameters:      map<string, ShaderValue>
  └── insert_after:    EffectSlot          // where in the pipeline to insert
```

A `FullscreenMaterial` renders a full-screen triangle. It receives the previous stage's color texture and any auxiliary buffers as bind group inputs. The `insert_after` field determines ordering relative to built-in effects.

### 4.6 Camera-Specific Settings

Each camera can have its own post-process configuration via components:

```
Camera entity:
  ├── BloomSettings       { threshold, intensity, ... }
  ├── TonemappingConfig   { operator, exposure, ... }
  ├── FxaaSettings        { enabled, quality }
  ├── SsaoSettings        { radius, bias, intensity, ... }
  ├── DofSettings         { focal_distance, focal_range, ... }
  ├── ColorGrading        { exposure, gamma, saturation, lut }
  └── ...
```

Effects can be toggled per-camera by adding or removing the corresponding settings component. Missing component means the effect is disabled for that camera.

### 4.7 Performance

- Effects can be toggled per-camera — disabled effects incur zero GPU cost.
- LOD for expensive effects: SSAO/GTAO can reduce sample count at distance; bloom can limit mip chain depth.
- The render graph skips entire pass nodes for disabled effects, avoiding any resource binding or draw call overhead.

### 4.8 Integration with Render Graph

Each enabled post-process effect becomes a render pass node in the render graph. The graph compiler chains them:

```mermaid
graph LR
    Scene["Scene Color (HDR)"] --> SSAO --> Bloom --> DOF --> CG["Color Grading"] --> TM["Tonemapping (HDR→LDR)"] --> FXAA --> Out["Framebuffer"]
```

Disabled effects are simply omitted; the graph reconnects adjacent nodes automatically.

## 5. Open Questions

- Should auto-exposure be a separate effect or part of the tonemapping pass?
- How should custom post-process effects declare their ordering constraints relative to built-in effects?
- What is the performance budget for post-processing on low-end hardware targets?

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
| — | — | Planned examples: `examples/3d/` |
