# Asset Formats — Go Implementation

**Version:** 0.2.0
**Status:** Stable
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
| **1**: each extension maps to exactly one loader; duplicate = hard error | Each format package exposes `RegisterAll(srv)` registering its loader for its unique extensions via `asset.RegisterLoader`. *(Planned hardening:* the shared `AssetServer` registry is currently last-wins; the `ErrLoaderConflict` type exists, and a registration-time cross-package duplicate-extension guard is a planned addition to fully enforce the hard-error contract.) |
| **2**: full asset or error; never partial | Each `Load` builds the asset in a local value and returns `(asset, nil)` only on full success; any decode error returns `(zero, err)` and the `AssetServer` marks the slot `Failed` — nothing partial enters the store. |
| **3**: disabled format ⇒ clear unsupported error | The mechanism is structural and Stable: the dep-free core compiles only stdlib loaders, and `ErrUnsupportedFormat{Ext}` is the typed result for an unregistered extension. *(The build-tag-gated optional formats that would exercise this path — OGG/MP3/FLAC/DDS/KTX2 — are Planned, so the invariant is proven by construction, not yet by a shipped opt-in format.)* |
| **4**: sub-asset labels stable across reloads | `GltfAssetLabel` encodes `(kind, index)` deterministically; one `mesh.Mesh` is emitted per glTF primitive in declaration order, so `Mesh(0)` always resolves to the first primitive. The `label → index` map is rebuilt identically on reload — verified by `examples/gltf` (fan-out hash stable ×20) + `FuzzDecode`. |

## Go Package

Each subpackage owns its `LoadSettings` and error types ([implemented] vs [planned]
marks the v0.2.0 Stable surface):

```
pkg/asset/formats/
  gltf/
    label.go         // GltfKind + GltfAssetLabel deterministic addressing   [implemented]
    schema.go        // glTF 2.0 JSON subset + component/mode constants       [implemented]
    convert.go       // buffer resolve + accessor decode → mesh/material/texture [implemented]
    loader.go        // .gltf/.glb parse → GltfAsset, Decode, Get, RegisterAll [implemented]
  image/
    stdlib.go        // PNG/JPEG via stdlib image (always compiled)          [implemented]
    hdr.go           // Radiance RGBE decode (stdlib-only)                    [planned]
    compressed.go    // DDS/KTX2 blob+metadata passthrough (//go:build gpu_compressed) [planned]
  audio/
    wav.go           // PCM WAV (stdlib-only, always compiled)               [implemented]
    vorbis.go mp3.go flac.go  // OGG/MP3/FLAC (//go:build opt-in deps)        [planned]
  font/
    truetype.go      // .ttf/.otf glyph outlines + metrics (x/image/font — ADR) [planned]
  scene/
    json.go          // .scene.json → scene.DynamicScene (reflection codec)  [planned]
  // A unified register.go/settings.go + DefaultFormatsPlugin + cross-package
  // ErrLoaderConflict detection are [planned] (the AssetServer registry is last-wins today).
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

> **Implementation note (v0.2.0).** The shipped `GltfAsset` holds the decoded
> sub-assets **by value** (`[]*mesh.Mesh`, `[]*material.Material`,
> `[]renderimage.Image`, `[]GltfScene`) addressed by a `map[GltfAssetLabel]int`,
> rather than `asset.Handle[T]`. The handle form requires registering each sub-asset
> into the `AssetServer` through a load context, which the current
> `asset.AssetLoader[A,S]` signature (`Load(r io.Reader, settings S)`) does not
> expose — handle-based sub-asset registration is deferred to App integration. The
> value form fully satisfies INV-2 (full-or-error) and INV-4 (stable labels)
> standalone. `RegisterAll` reuses `asset.RegisterLoader[A,S](srv, loader)`
> (extensions come from `Extensions()`); there is no `LoadContext` parameter.

## Key Methods

```go
// glTF (INV-2, INV-4) — implemented.
func (GltfLoader) Load(r io.Reader, s LoadSettings) (GltfAsset, error)
func (GltfLoader) Extensions() []string                   // ".gltf", ".glb"
func Decode(raw []byte) (GltfAsset, error)                // JSON/GLB auto-detect, panic-safe (INV-2)
func (a *GltfAsset) Get(label GltfAssetLabel) (any, bool) // INV-4 stable lookup
func (a *GltfAsset) Labels() []GltfAssetLabel             // deterministic (kind, index) order
func RegisterAll(srv *asset.AssetServer) error            // registers GltfLoader for .gltf/.glb

// Images (INV-2) — implemented; stdlib path is 0 external deps.
func (StdlibImageLoader) Load(r io.Reader, _ LoadSettings) (renderimage.Image, error)

// scene.json → DynamicScene (reuses Phase 3 reflection codec) — planned.
// func (SceneJSONLoader) Load(r io.Reader, _ LoadSettings) (scene.DynamicScene, error)
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

<!-- v0.2.0 Stable surface: glTF loader + stdlib image + WAV. Planned formats
     (HDR/DDS/KTX2/OGG/MP3/FLAC/fonts/scene.json) add rows as their loaders land. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |
| gltf-label | pkg/asset/formats/gltf/label.go | `GltfKind` + `GltfAssetLabel` deterministic `(kind, index)` addressing (INV-4) |
| gltf-schema | pkg/asset/formats/gltf/schema.go | glTF 2.0 JSON subset + OpenGL component/mode constants |
| gltf-convert | pkg/asset/formats/gltf/convert.go | Buffer resolve (base64/GLB-BIN) + accessor de-interleave + mesh/material/texture conversion |
| gltf-loader | pkg/asset/formats/gltf/loader.go | `GltfLoader`, `.gltf`/`.glb` split, `Decode` (panic-safe INV-2), `GltfAsset.Get`/`Labels`, `RegisterAll` |
| gltf-test | pkg/asset/formats/gltf/gltf_test.go | Fan-out + label-stability + error-path tests + `FuzzDecode` (90.0% cov) |
| image-loader | pkg/asset/formats/image/stdlib.go | PNG/JPEG stdlib decode → `renderimage.Image` (INV-2) |
| audio-loader | pkg/asset/formats/audio/wav.go | PCM WAV decode → `audio.AudioSource` (INV-2) |
| gltf-example | examples/gltf/main.go | glTF fan-out + label-stability golden (INV-4, hash-stable ×20) |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-28 | Initial L2 draft — Go translation of l1-asset-formats v0.1.0. Stdlib-first image/audio loaders, build-tag-gated optional formats, glTF multi-asset fan-out with stable `GltfAssetLabel`, `.scene.json` via Phase 3 codec, `AssetLoader` registration with extension-uniqueness. Authored ahead of Phase 5 Track B (`/magic.spec`). Draft — L1 parent Draft + consumes Track A/D asset types + no validating golden tests yet. |
| 0.2.0 | 2026-05-31 | Narrowed to the implemented surface + promoted Draft → Stable (`/magic.task` Option A). Stable: `pkg/asset/formats/gltf` (`.gltf`/`.glb` decode → value-based `GltfAsset` fan-out of mesh/material/texture/scene, deterministic `GltfAssetLabel`, panic-safe `Decode`, 90.0% cov + `FuzzDecode`), `image` (PNG/JPEG), `audio` (WAV). Corrected to reality: value-fan-out instead of `asset.Handle[T]` (no `LoadContext` in `AssetLoader.Load`); INV-1 cross-package conflict-detection + INV-3 build-tag optional formats marked planned; `register.go`/`settings.go`/`DefaultFormatsPlugin`/`scene.json`/font/HDR/DDS/KTX2/OGG/MP3/FLAC marked `[planned]`. Canonical References populated; `Implements` parent now Stable (layer-clean). |
