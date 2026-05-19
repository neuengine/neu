# Mesh & Image — Go Implementation

**Version:** 0.1.0
**Status:** Draft
**Layer:** go
**Implements:** [l1-mesh-and-image.md](l1-mesh-and-image.md)

## Overview

Go-level design for the two foundational GPU assets. `Mesh` is an immutable
attribute set (`VertexAttribute` map + optional `IndexBuffer` + sub-meshes);
`Image` is decoded pixel data with a GPU-compatible format and sampler
descriptor. Both are loaded via the asset system (`Handle[Mesh]` /
`Handle[Image]`), decoded on the `IOPool`, and uploaded to the render server via
a staging buffer. Primitive generators, skinning/morph attribute layouts,
`VertexLayout` hashing for pipeline keys, and static/dynamic texture atlases are
included. Public (`pkg/render/mesh`, `pkg/render/image`) — pure-data assets and
components.

## Related Specifications

- [l1-mesh-and-image.md](l1-mesh-and-image.md) — L1 concept specification (parent)
- [l2-asset-system-go.md](l2-asset-system-go.md) — `Handle[A]`, async load on `IOPool`, hot-reload
- [l2-render-core-go.md](l2-render-core-go.md) — GPU upload via the render server (RID); `VertexLayout` feeds the pipeline cache
- [l2-math-system-go.md](l2-math-system-go.md) — `Vec2/3/4` attribute element types

## 1. Motivation

Geometry and texture data have GPU lifecycles distinct from gameplay state.
Modelling them as immutable, handle-referenced assets with an explicit upload
step keeps memory ownership unambiguous and enables streaming, caching, and
hot-reload without touching component storage.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: generics for typed `AttributeData[T]`; `iter.Seq` over vertices.
- **C-003**: stdlib decoders for PNG/JPEG (`image/png`, `image/jpeg`); HDR/DDS/
  KTX2/BC/ASTC via registered loader plugins (asset system), not core deps.
- **C-027**: staging buffers are `sync.Pool`-recycled across uploads.
- **Immutability**: an uploaded `Mesh`/`Image` is never mutated; an edit creates
  a new asset version (new `AssetID` generation) — consumers re-resolve handles.
- **Background decode**: `Image` bytes decode on the `IOPool`; a 1×1 placeholder
  `RID` is bound until the real texture is ready (no caller stall).
- **`Option<IndexBuffer>`** → `*IndexBuffer` (nil = non-indexed draw).

## 3. Core Invariants

> [!NOTE]
> See [l1-mesh-and-image.md §3](l1-mesh-and-image.md) for technology-agnostic
> invariants. Go-specific compliance is tabulated in §4.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **1**: Every mesh has a position attribute | `NewMesh` returns `ErrMeshNoPosition` if `AttrPosition` is absent; `Mesh.Validate()` re-checks before GPU upload. |
| **2**: Index buffer in-bounds | `Mesh.Validate()` scans the index slice for `max(idx) < vertexCount`; violation ⇒ `ErrIndexOutOfRange` (validated once at upload, not per-draw). |
| **3**: Skinned joint weights/indices length match | `Validate()` asserts `len(JointWeights) == len(JointIndices) == vertexCount`; mismatch ⇒ `ErrSkinAttributeMismatch`. |
| **4**: Image declares valid GPU format | `ImageFormat` is a closed `iota` enum; `NewImage` rejects `FormatInvalid`; format→backend-format mapping is a total function (no default fallthrough). |
| **5**: Atlas regions non-overlapping | `TextureAtlasLayout.Add` runs an interval check against existing regions; overlap ⇒ `ErrAtlasOverlap` (build-time, not render-time). |

## Go Package

```
pkg/render/mesh/
  mesh.go        // Mesh, VertexAttribute, AttrKind, IndexBuffer, SubMesh
  layout.go       // VertexLayout, hash (pipeline-spec key fragment)
  primitives.go   // Cube/Sphere/Plane/Cylinder/Capsule/Torus generators
  skin.go         // SkinnedMesh, MorphWeights component data
pkg/render/image/
  image.go        // Image, ImageFormat, SamplerDesc, mip metadata
  atlas.go        // TextureAtlasLayout, TextureAtlas, DynamicAtlas (shelf-pack)
  loaders.go      // PNG/JPEG stdlib loaders; loader-registration shims
internal/render/upload/
  staging.go       // staging-buffer pool, async GPU upload via render server
```

## Type Definitions

```go
type AttrKind uint8 // Position, Normal, UV0, UV1, Tangent, Color, JointW, JointI, Custom

type VertexAttribute struct {
    Kind   AttrKind
    Format VertexFormat            // Float32x3, Float32x2, Uint16x4, ...
    Data   []byte                   // tightly packed; element count = Mesh.vertexCount
}

type IndexBuffer struct {
    Wide bool                       // false = u16, true = u32
    Data []byte
}

type SubMesh struct{ Offset, Count uint32 } // for multi-material draws

type Mesh struct {
    Topology    PrimitiveTopology    // TriangleList | TriangleStrip | LineList ...
    attrs       map[AttrKind]VertexAttribute
    indices     *IndexBuffer
    subMeshes   []SubMesh
    vertexCount int
    layout      VertexLayout         // derived; hashed for pipeline key
}

type VertexLayout struct {
    stride   uint32
    elements []layoutElement          // (kind, format, offset)
    hash     uint64                    // FNV-1a, part of pipeline-spec key
}

type ImageFormat uint8 // RGBA8, RGBA16F, RG11B10F, BC7, ASTC4x4, Depth32F, ...

type Image struct {
    Format    ImageFormat
    Width     uint32
    Height    uint32
    MipLevels uint32
    Data      []byte
    Sampler   SamplerDesc
}

type TextureAtlasLayout struct{ regions []AtlasRegion }      // name + min/max
type TextureAtlas struct {                                    // ECS component
    Image  asset.Handle[Image]
    Layout asset.Handle[TextureAtlasLayout]
    Index  int
}
type DynamicAtlas struct {                                    // shelf-packer
    rid    render.RID
    shelves []shelf
}

// Pure-data marker components (read by render extract).
type Mesh3D struct{ Handle asset.Handle[Mesh] }
type Mesh2D struct{ Handle asset.Handle[Mesh] }
type SkinnedMesh struct{ Mesh asset.Handle[Mesh]; Joints []entity.Entity }
type MorphWeights struct{ Weights []float32 }
```

## Key Methods

```go
func NewMesh(topology PrimitiveTopology) *Mesh
func (m *Mesh) SetAttribute(a VertexAttribute) *Mesh           // chainable
func (m *Mesh) SetIndices(ib IndexBuffer) *Mesh
func (m *Mesh) Validate() error                                 // INV-1,2,3
func (m *Mesh) Layout() VertexLayout                            // memoised

// Primitive generators (position+normal+UV+tangent, u32 indices).
func Cube(size float32) *Mesh
func Sphere(radius float32, sectors, stacks int) *Mesh
func Plane(size float32, subdivisions int) *Mesh
// ... Cylinder, Capsule, Torus

func NewImage(f ImageFormat, w, h uint32, data []byte) (*Image, error) // INV-4
func DecodeImage(r io.Reader, hint string) (*Image, error)      // IOPool

func (l *TextureAtlasLayout) Add(name string, min, max [2]uint32) error // INV-5
func (d *DynamicAtlas) Alloc(w, h uint32) (AtlasRegion, bool)   // shelf-pack; grows ×2 on full
```

`DecodeImage` runs on the asset system's `IOPool`; the asset server binds a
shared placeholder texture RID until the upload completes, then atomically
swaps the handle's resolved value.

## Performance Strategy

- **Interleaved layout by default** (L1 §4.2): one vertex buffer, one bind —
  fewer GPU buffer slots; split-buffer only on backend request.
- **`VertexLayout.hash`** (FNV-1a) is computed once and cached — it is a stable
  fragment of the pipeline-specialization key (no per-draw hashing).
- **Staging-buffer pool** (C-027): uploads reuse pooled staging buffers; mip
  generation runs on the `ComputePool` for non-precompressed formats.
- **Atlas packing**: shelf algorithm is O(regions); a full `DynamicAtlas`
  doubles and re-packs once, amortised.

## Error Handling

| Condition | Behavior |
| :--- | :--- |
| Mesh missing position | `ErrMeshNoPosition` at `NewMesh`/`Validate` |
| Index out of range | `ErrIndexOutOfRange` at `Validate` (pre-upload) |
| Skin attr length mismatch | `ErrSkinAttributeMismatch` at `Validate` |
| Unknown image format | `ErrImageFormatInvalid` at `NewImage` |
| Atlas region overlap | `ErrAtlasOverlap` at `Layout.Add` |
| Decode failure | wrapped `fmt.Errorf("image decode %s: %w", hint, err)`; placeholder retained |

```go
var (
    ErrMeshNoPosition       = errors.New("mesh: missing position attribute")
    ErrIndexOutOfRange      = errors.New("mesh: index references out-of-range vertex")
    ErrSkinAttributeMismatch = errors.New("mesh: joint weight/index length mismatch")
    ErrImageFormatInvalid   = errors.New("image: invalid GPU format")
    ErrAtlasOverlap         = errors.New("atlas: overlapping region")
)
```

## Testing Strategy

- **Validation table tests**: missing position, out-of-range index, skin length
  mismatch, bad format, overlapping atlas region — each asserts the sentinel.
- **Primitive golden tests**: each generator's vertex/index counts and AABB
  match a golden fixture; normals unit-length within 1e-6.
- **Decode + placeholder**: assert the placeholder RID is bound synchronously
  and swapped to the real texture after `IOPool` completion (race-clean).
- **Layout hash stability**: same attribute set ⇒ identical hash across runs
  (determinism — feeds pipeline cache).
- **Benchmarks**: `BenchmarkSphereGen`, `BenchmarkAtlasAlloc` (0 alloc/op steady).

## 7. Drawbacks & Alternatives

- **Drawback**: 4-joint skinning cap.
  **Alternative**: variable influence count.
  **Decision**: L1 Open Question Q1 — <!-- TBD (L1 §5 Q1): >4 joint influences
  for high-fidelity characters; fixed at 4 for 0.1.0. -->
- **Drawback**: doubling `DynamicAtlas` can spike GPU memory.
  **Alternative**: multi-page atlas array.
  **Decision**: L1 Open Question Q2 — <!-- TBD (L1 §5 Q2): max atlas size
  before automatic split. -->
- **Drawback**: no GPU-memory-pressure eviction.
  **Decision**: L1 Open Question Q3 — <!-- TBD (L1 §5 Q3): eviction policy
  deferred to streaming work (post Phase 4). -->

## Canonical References

<!-- MANDATORY for Stable status. Stub — populate when implementation lands
     (Phase 4). Blocked: (1) L1 parent Draft; (2) C29 no examples/3d/ yet;
     (3) C-002 STOP FACTOR Phase 4. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-18 | Initial L2 draft — Go translation of l1-mesh-and-image v0.1.0. Immutable attribute-map Mesh, validated index/skin invariants, FNV layout hashing, stdlib PNG/JPEG decode on IOPool with placeholder swap, shelf-pack DynamicAtlas. L1 Q1–Q3 carried as TBD. Draft — L1 parent Draft + C29 + C-002 Phase 4 Hold. |
