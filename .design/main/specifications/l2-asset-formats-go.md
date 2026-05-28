# Asset Formats — Go Implementation

**Version:** 0.1.0
**Status:** Draft
**Layer:** go
**Implements:** [l1-asset-formats.md](l1-asset-formats.md)

## Overview

Go-level design for the engine's content loaders: each file format is a
stateless `asset.AssetLoader` registered with the `AssetServer` by extension.
Loaders are grouped into build-tag-gated subpackages so a project compiles only
the formats it needs. A single glTF file fans out into many sub-assets
addressed by a stable label scheme. All loaders run asynchronously on the IO
pool and return a fully constructed asset or a structured error — never a
partial asset.

## Related Specifications

- [l1-asset-formats.md](l1-asset-formats.md) — L1 concept specification (parent)
- [l2-asset-system-go.md](l2-asset-system-go.md) — `AssetLoader[A,S]` registry, `fs.FS` VFS, async IOPool load
- [l2-mesh-and-image-go.md](l2-mesh-and-image-go.md) — image loaders produce `image.Image`; glTF produces `mesh.Mesh`
- [l2-materials-and-lighting-go.md](l2-materials-and-lighting-go.md) — glTF produces `material.Material`
- [l2-scene-system-go.md](l2-scene-system-go.md) — `.scene.json` → `scene.DynamicScene`; glTF scenes reuse the codec
- [l2-animation-system-go.md](l2-animation-system-go.md) — glTF animations → `animation.AnimationClip` (+ Skin/MorphTarget)
- [l2-audio-system-go.md](l2-audio-system-go.md) — audio loaders → `audio.AudioSource`

## 1. Motivation

The engine must consume content authored in external tools while keeping the
core format-agnostic. Centralizing format knowledge in dedicated loaders means
a 2D-only or headless project never compiles glTF or image decoders it does not
use. This L2 binds the L1 format catalog to concrete Go decoders (stdlib-first)
and the existing `AssetServer` registration surface.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: generics for `AssetLoader[A,S]`; `image`, `image/png`, `image/jpeg`, `encoding/binary` from stdlib.
- **C-003**: stdlib-first. PNG/JPEG/GIF decode via stdlib `image`. Formats with no
  stdlib decoder (OGG/FLAC/MP3/WebP/Draco) are defined behind build tags and ship
  as opt-in plugins; each external decoder dep requires an ADR. The core compiles dep-free.
- Loaders are **stateless** (L1 §2): all context comes from `AssetServer` + `LoadSettings`.
- Loaders **never panic** (L1 §2): malformed input ⇒ a wrapped error.
- Extension auto-detection; an explicit `Format` override may be passed in `LoadSettings`.
- GPU-compressed formats (DDS/KTX2) are loaded as opaque byte blobs + metadata — no CPU decode.

## 3. Core Invariants

> [!NOTE]
> See [l1-asset-formats.md §3](l1-asset-formats.md) for technology-agnostic
> invariants. Go-specific compliance is tabulated in §4.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **1**: each extension maps to exactly one loader; duplicate = hard error | `RegisterLoader(ext, loader)` checks the server's `map[string]typedLoader`; a second registration for the same extension returns `ErrLoaderConflict` (fail at app build, not silently last-wins). |
| **2**: full asset or error; never partial | Each `Load` builds the asset in a local value and returns `(asset, nil)` only on full success; any decode error returns `(zero, err)` and the `AssetServer` marks the slot `Failed` — nothing partial enters the store. |
| **3**: disabled format ⇒ clear unsupported error | Build-tag-gated subpackages register via `init()`. With the tag off, the extension is unregistered; `AssetServer.Load` of that extension returns `ErrUnsupportedFormat{Ext}` — a clear message, not a silent miss. |
| **4**: sub-asset labels stable across reloads | `GltfAssetLabel` encodes `(kind, index)` deterministically (`Mesh(0)` is always the first mesh by glTF declaration order); the label → sub-asset map is rebuilt identically on reload. |

## Go Package

```
pkg/asset/formats/
  register.go        // DefaultFormatsPlugin: registers all enabled loaders
  settings.go        // LoadSettings (Format override, glTF axis, JPEG quality)
  errors.go          // ErrLoaderConflict, ErrUnsupportedFormat, ErrSubAssetMissing
  image/
    stdlib.go        // PNG/JPEG/GIF/BMP via stdlib image (always compiled)
    hdr.go           // Radiance RGBE decode (stdlib-only)
    compressed.go    // DDS/KTX2 blob+metadata passthrough (//go:build gpu_compressed)
  audio/
    wav.go           // PCM WAV (stdlib-only, always compiled)
    vorbis.go        // OGG/Vorbis (//go:build audio_vorbis — opt-in dep)
    mp3.go flac.go   // (//go:build audio_mp3 / audio_flac)
  font/
    truetype.go      // .ttf/.otf glyph outlines + metrics (stdlib x/image/font candidate — ADR)
  gltf/
    loader.go        // .gltf/.glb parse → GltfAsset
    label.go         // GltfAssetLabel deterministic sub-asset addressing
    convert.go       // glTF primitives → mesh/material/image/animation/skin
  scene/
    json.go          // .scene.json → scene.DynamicScene (reflection codec)
```

## Type Definitions

```go
type LoadSettings struct {
    Format   string // explicit override; "" ⇒ extension auto-detect
    GltfAxis AxisConvention
    // format-specific knobs resolved per loader
}

// GltfAsset — the multi-asset fan-out (L1 §4.1).
type GltfAsset struct {
    Scenes     []asset.Handle[scene.DynamicScene]
    Meshes     []asset.Handle[mesh.Mesh]
    Materials  []asset.Handle[material.Material]
    Textures   []asset.Handle[image.Image]
    Animations []asset.Handle[animation.AnimationClip]
    Skins      []SkinData
    labels     map[GltfAssetLabel]int // stable addressing (INV-4)
}

type GltfAssetLabel struct {
    Kind  GltfKind // Scene, Mesh, Material, Texture, Animation, Skin, MorphTarget
    Index uint32
}

func (l GltfAssetLabel) String() string // e.g. "Mesh(0)" — deterministic, reload-stable

// Loader registration bridge (reuses l2-asset-system-go AssetLoader[A,S]).
func RegisterLoader[A any, S any](srv *asset.AssetServer, l asset.AssetLoader[A, S], exts ...string) error
```

## Key Methods

```go
func NewDefaultFormatsPlugin() app.Plugin // registers all build-tag-enabled loaders

// glTF (INV-2, INV-4).
func (g *GltfLoader) Load(ctx asset.LoadContext, r io.Reader, s LoadSettings) (GltfAsset, error)
func (a *GltfAsset) Get(label GltfAssetLabel) (asset.UntypedHandle, bool) // INV-4 stable lookup

// Images (INV-2) — stdlib path is 0 external deps.
func (l *PngLoader) Load(ctx asset.LoadContext, r io.Reader, _ LoadSettings) (image.Image, error)

// scene.json → DynamicScene (reuses Phase 3 reflection codec).
func (l *SceneJSONLoader) Load(ctx asset.LoadContext, r io.Reader, _ LoadSettings) (scene.DynamicScene, error)
```

## Performance Strategy

- **Async on IOPool**: all `Load` calls run on `task.IOPool` (L1 §2) via the
  existing `AssetServer` dedup path — concurrent loads of the same path share one decode.
- **Streaming-friendly readers**: loaders take `io.Reader`; large `.glb` blobs
  are read in framed chunks, not slurped where the format allows.
- **GPU-compressed passthrough**: DDS/KTX2 return the raw blob + a small header;
  no CPU transcode — the render upload path selects the GPU transcode target.
- **Stable-label map built once** per glTF load; reload rebuilds the identical map.

## Error Handling

| Condition | Behavior |
| :--- | :--- |
| Duplicate extension registration | `ErrLoaderConflict{Ext, Existing}` at registration — fail fast (INV-1) |
| Load of a build-tag-disabled format | `ErrUnsupportedFormat{Ext}` — clear message (INV-3) |
| Malformed file bytes | wrapped decode error (`fmt.Errorf("gltf: %w", err)`); slot ⇒ `Failed`, no partial (INV-2) |
| `GltfAssetLabel` not present | `ErrSubAssetMissing{Label}` from `Get` |
| Loader panic (defensive) | recovered → converted to error (L1 §2: panics not permitted) |

```go
type ErrUnsupportedFormat struct{ Ext string }
type ErrLoaderConflict   struct{ Ext, Existing string }
type ErrSubAssetMissing  struct{ Label GltfAssetLabel }
```

## Testing Strategy

- **Round-trip golden (C29 gate)**: PNG decode dims/format; WAV decode sample-rate/PCM;
  `.scene.json` decode → spawn → re-serialize byte-stable (reuses Phase 3 codec test pattern).
- **INV-1 conflict**: registering two loaders for `"png"` ⇒ `ErrLoaderConflict`.
- **INV-3 build tags**: with `audio_vorbis` off, loading `.ogg` ⇒ `ErrUnsupportedFormat`.
- **INV-4 label stability**: load a multi-mesh `.glb` twice; assert `Mesh(0)` resolves to the same primitive.
- **No-panic fuzz**: `go test -fuzz` truncated/garbage inputs per loader → error, never panic.

## 7. Drawbacks & Alternatives

- **Drawback**: build-tag gating multiplies the build matrix.
  **Alternative**: always compile all loaders.
  **Decision**: L1 §1 + C-003 require modular inclusion to keep binaries lean and
  the core dep-free; tags are the idiomatic Go mechanism. Kept.
- **Drawback**: extension-based detection misclassifies mis-named files.
  **Alternative**: magic-byte sniffing.
  **Decision**: L1 §5 Q3 open; extension + explicit `LoadSettings.Format` override
  covers the common case. Magic-byte detection deferred (non-blocking).

## Canonical References

<!-- MANDATORY for Stable status. Stub — populate when implementation lands
     (Phase 5 Track B). Stable promotion blocked until: (1) codec round-trip golden
     tests pass (T-5T04); (2) C29 P5 gate (T-5T05). Track B joins Tracks A + D
     (AudioSource + AnimationClip asset types). -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-28 | Initial L2 draft — Go translation of l1-asset-formats v0.1.0. Stdlib-first image/audio loaders, build-tag-gated optional formats, glTF multi-asset fan-out with stable `GltfAssetLabel`, `.scene.json` via Phase 3 codec, `AssetLoader` registration with extension-uniqueness. Authored ahead of Phase 5 Track B (`/magic.spec`). Draft — L1 parent Draft + consumes Track A/D asset types + no validating golden tests yet. |
