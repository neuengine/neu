# Asset Formats

**Version:** 0.3.0
**Status:** Stable
**Layer:** concept

## Overview

This specification enumerates the file formats the engine can load and describes how each format maps to one or more engine asset types. Every format is handled by a dedicated `AssetLoader` registered with the `AssetServer`. Format support is modular — each loader can be included or excluded via build tags, keeping the binary size minimal for projects that do not need every format.

## Related Specifications

- [Asset System](l1-asset-system.md)
- [Mesh & Image](l1-mesh-and-image.md)
- [Audio System](l1-audio-system.md)

## 1. Motivation

A game engine must consume diverse content authored in external tools. Centralizing format knowledge in well-defined loaders keeps the rest of the engine format-agnostic. Modular inclusion means a 2D-only project need not compile glTF parsing, and a headless server need not compile image decoders.

## 2. Constraints & Assumptions

- Loaders are stateless — all context comes from the `AssetServer` and load settings.
- A single file may produce multiple assets (e.g., a glTF file yields scenes, meshes, materials, textures, and animations).
- Loaders must report structured errors; panicking inside a loader is not permitted.
- Format auto-detection is by file extension; an explicit format override can be passed at load time.
- All loaders execute asynchronously on the asset task pool.

## 3. Core Invariants

1. Each file extension maps to exactly one loader; duplicate registrations are a hard error.
2. A loader must return a fully constructed asset or an error — partial assets are never inserted into the asset store.
3. Disabling a format via build tags removes its loader entirely; attempting to load that format yields a clear "unsupported format" error rather than a silent failure.
4. Sub-asset addressing (e.g., a specific mesh inside a glTF) uses a label scheme that is stable across reloads.

## 4. Detailed Design

> **Implementation Status (v0.3.0).** The Stable surface is the stdlib-first core:
> the **glTF 2.0 loader** (scenes, meshes, materials, embedded textures — §4.1),
> **PNG/JPEG** image decoding (§4.2), **PCM WAV** audio (§4.3), and **`.scene.json`
> decode** → portable `SerializedScene` (§4.5; spawn-to-World deferred). The
> architecture below — stateless `AssetLoader`, build-tag modular inclusion, and the
> invariants in §3 — is fixed and Stable. Every other format in the tables (HDR, DDS,
> KTX2, BMP, WebP, TGA, OGG, FLAC, MP3, AAC, fonts) and glTF
> animations/skins/morph-targets/KHR extensions are marked **Planned**: they are
> intended future loaders, not yet implemented.

### 4.1 glTF 2.0 Loader

The glTF loader consumes `.gltf` (JSON + separate binary) and `.glb` (single binary) files. It produces:

```plaintext
GltfAsset
 ├── Scene(index)      → Scene asset
 ├── Mesh(index)       → Mesh asset (with primitives)
 ├── Material(index)   → Material asset
 ├── Texture(index)    → Image asset
 ├── Animation(index)  → AnimationClip asset
 ├── Skin(index)       → Skeletal skin data
 └── MorphTarget(mesh, primitive, target) → MorphWeights data
```

`GltfAssetLabel` is an enum addressing each sub-asset by type and index. Labels are deterministic: `Mesh(0)` always refers to the first mesh regardless of load order.

Supported glTF extensions: `KHR_materials_unlit`, `KHR_texture_transform`, `KHR_draco_mesh_compression` (behind build tag), `KHR_lights_punctual`.

> **Implemented:** Scene/Mesh/Material/Texture fan-out with stable `GltfAssetLabel`
> addressing over `.gltf` (JSON) and `.glb` (binary); one mesh per glTF primitive in
> declaration order. **Planned:** Animation, Skin, MorphTarget sub-assets and all KHR
> extensions.

### 4.2 Image Format Loaders

Each image format has its own loader producing an `Image` asset:

| Format | Extensions | Notes |
| :--- | :--- | :--- |
| PNG | `.png` | RGBA, 8/16-bit |
| JPEG | `.jpg`, `.jpeg` | RGB, lossy |
| HDR | `.hdr` | Radiance RGBE, used for environment maps |
| DDS | `.dds` | GPU-compressed (BC1-BC7), loaded without CPU decode |
| KTX2 | `.ktx2` | Basis Universal or ASTC, GPU-compressed |
| BMP | `.bmp` | Uncompressed RGB/RGBA |
| WebP | `.webp` | Lossy and lossless |
| TGA | `.tga` | Legacy format, uncompressed or RLE |

GPU-compressed formats (DDS, KTX2) are uploaded directly; the loader selects a transcode target matching the current GPU backend capabilities.

> **Implemented:** PNG, JPEG (stdlib `image`). **Planned:** HDR, DDS, KTX2, BMP, WebP, TGA.

### 4.3 Audio Format Loaders

Each audio format produces an `AudioSource` asset:

| Format | Extensions | Notes |
| :--- | :--- | :--- |
| WAV | `.wav` | Uncompressed PCM, lowest latency |
| OGG/Vorbis | `.ogg` | Compressed, good quality-to-size ratio |
| FLAC | `.flac` | Lossless compression |
| MP3 | `.mp3` | Lossy, wide compatibility |
| AAC | `.aac`, `.m4a` | Lossy, behind optional build tag |

> **Implemented:** WAV (stdlib PCM). **Planned:** OGG/Vorbis, FLAC, MP3, AAC.

### 4.4 Font Loader

Loads `.ttf` and `.otf` files into a `Font` asset containing glyph outlines and metrics. Rasterization into glyph atlases is deferred to the text pipeline at render time.

> **Planned:** the font loader is not yet implemented; it is gated on the text-pipeline
> integration (and the `x/image/font` ADR raised by the UI system).

### 4.5 Scene File Format

The engine defines its own JSON-based scene format (`.scene.json`). A scene file is a serialized snapshot of entities and their components, using the reflection system for type resolution. The loader produces a `DynamicScene` asset that can be spawned into any World.

```plaintext
{
  "entities": [
    { "components": [ { "type": "Transform", "value": { ... } }, ... ] },
    ...
  ]
}
```

> **Implemented (decode):** the `.scene.json` loader (`pkg/asset/formats/scene`)
> decodes a scene file into the portable `scene.SerializedScene` (interned form),
> reusing the Phase 3 JSON codec + range-validating the interning indices (INV-2).
> **Deferred:** hydrating the `SerializedScene` into live World entities (`scene`
> spawn, needs a `TypeRegistry`) is the scene system's job, not the loader's.

### 4.6 Loader Registration

At app build time, each format plugin registers its loader:

```
AssetServer.register_loader(PngImageLoader, &["png"])
AssetServer.register_loader(GltfLoader, &["gltf", "glb"])
```

Build tags control which registration calls are compiled. A convenience `DefaultFormatsPlugin` registers all enabled loaders in one step.

> **Implemented:** per-package `RegisterAll(server)` for the glTF, image, and audio
> loaders. **Planned:** the unified `DefaultFormatsPlugin` and cross-package
> duplicate-extension detection that hardens INV-1 into a registration-time hard error
> (the current `AssetServer` registry is last-wins).

## 5. Open Questions

- Should the engine support a custom binary scene format for faster load times in shipping builds?
- How should format-specific load settings (e.g., JPEG quality threshold, glTF coordinate system override) be passed through the asset pipeline?
- Is runtime format detection (magic bytes) worth the complexity, or is extension-based sufficient?
- Which **Planned** formats (§4.2–4.5) graduate first, and which ship only as opt-in build-tag plugins versus engine-core defaults? (Scoped out of the v0.2.0 Stable surface.)

## Canonical References

<!-- Authoritative sources for the v0.2.0 Stable surface (glTF + stdlib image + WAV).
     Planned formats (§4.2–4.5) will add rows as their loaders land. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |
| gltf-loader | pkg/asset/formats/gltf/loader.go | glTF `.gltf`/`.glb` decode, `GltfAsset` fan-out, `GltfAssetLabel` addressing (INV-2, INV-4) |
| gltf-convert | pkg/asset/formats/gltf/convert.go | Accessor de-interleave + mesh/material/texture conversion |
| image-loader | pkg/asset/formats/image/stdlib.go | PNG/JPEG stdlib decode → `Image` (INV-2) |
| audio-loader | pkg/asset/formats/audio/wav.go | PCM WAV decode → `AudioSource` (INV-2) |
| scene-loader | pkg/asset/formats/scene/scene.go | `.scene.json` decode → portable `SerializedScene` + index validation (INV-2; spawn deferred) |
| gltf-example | examples/gltf/main.go | glTF fan-out + label-stability golden (INV-4, hash-stable ×20) |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-03-25 | Initial draft from architecture analysis |
| 0.3.0 | 2026-06-01 | Graduated `.scene.json` from Planned → Implemented (decode) (`/magic.run`): `pkg/asset/formats/scene` decodes a scene file into the portable `scene.SerializedScene` (reusing the Phase 3 JSON codec + range-validating interning indices, INV-2). Reconciled to reality — the loader yields `SerializedScene` (the on-disk form), not `DynamicScene` (in-memory); spawn-to-World (`scene` spawn, needs a TypeRegistry) stays deferred. Canonical References + §4.5 updated. |
| 0.2.0 | 2026-05-31 | Narrowed to match implementation (`/magic.task` Option A) + promoted Draft → Stable. Stable surface = glTF 2.0 loader (scene/mesh/material/texture fan-out + stable `GltfAssetLabel`, INV-2/INV-4) + stdlib PNG/JPEG + PCM WAV. HDR/DDS/KTX2/BMP/WebP/TGA/OGG/FLAC/MP3/AAC/fonts/`.scene.json` + glTF animations/skins/morph/KHR explicitly marked **Planned** (deferred future loaders). Canonical References populated; INV-1 cross-package conflict-detection noted as a planned hardening. Validated by `examples/gltf` (×20). |
