# Materials & Lighting — Go Implementation

**Version:** 0.1.0
**Status:** Draft
**Layer:** go
**Implements:** [l1-materials-and-lighting.md](l1-materials-and-lighting.md)

## Overview

Go-level design for surface materials and scene lighting. `Material` is a
shader handle plus a typed `MaterialParameters` bag (textures/floats/vectors/
colors) and an `AlphaMode` that deterministically maps to a render phase. Light
components (`PointLight`, `SpotLight`, `DirectionalLight`, `AmbientLight`,
`EnvironmentMapLight`, `IrradianceVolume`) are pure data. PBR metallic-roughness
is the default shading model with clamped parameters; shadow maps (cascaded /
cube / single) are allocated before the shadow pass. Material specialization
keys feed the render-core pipeline cache. Public (`pkg/render/material`,
`pkg/render/light`).

## Related Specifications

- [l1-materials-and-lighting.md](l1-materials-and-lighting.md) — L1 concept specification (parent)
- [l2-render-core-go.md](l2-render-core-go.md) — pipeline cache key, render phases, shadow-pass scheduling
- [l2-mesh-and-image-go.md](l2-mesh-and-image-go.md) — `Handle[Image]` material textures
- [l2-camera-and-visibility-go.md](l2-camera-and-visibility-go.md) — light culling per view frustum

## 1. Motivation

A declarative material model lets surfaces be described as data, not code.
Separating light definitions from material parameters means the engine can
evaluate any material×light combination (clustered/culled) without
combinatorial user-side branching.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: generics for the typed parameter accessors; `iota` enums for
  `AlphaMode`/`LightKind`.
- **C-003**: stdlib only — no shader-compiler dependency in core (compilation is
  delegated to the backend behind `render.RenderBackend`).
- **C-027**: per-frame light-cluster and bind-group scratch buffers are pooled.
- **Default shading = PBR metallic-roughness**; custom shaders fully override.
- **Unlimited scene lights**: the engine clusters/culls per tile/frustum — no
  fixed light-count cap in the data model.
- **`Option<Phase>`** → `*render.RenderPhase` (nil = derive from `AlphaMode`).

## 3. Core Invariants

> [!NOTE]
> See [l1-materials-and-lighting.md §3](l1-materials-and-lighting.md) for
> technology-agnostic invariants. Go-specific compliance is tabulated in §4.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **1**: Material references a valid shader | `Material.Shader` is `asset.Handle[Shader]`; `Material.Validate()` returns `ErrMaterialNoShader` if the handle is nil/zero before pipeline build. |
| **2**: PBR values physically clamped | Setters clamp: `metallic`,`roughness`,`occlusion` ∈ [0,1], `base_color`/`emissive` ≥ 0; `MaterialParameters.Sanitize()` is idempotent and called at upload. |
| **3**: Directional cascades cover near→max | `CascadeShadowConfig.splits()` derives split distances spanning `camera.near` to `cfg.MaxDistance`; a config whose last split < `MaxDistance` is rejected at build (`ErrCascadeCoverage`). |
| **4**: Shadow resources allocated before shadow pass | `prepareShadows` (render-core Prepare phase) allocates every shadow-caster's map RID and inserts the shadow pass `Before` the lighting pass in the graph — enforced by graph edge, not convention. |
| **5**: Alpha mode → phase mapping not overridable | `AlphaMode.Phase()` is a pure total function; `Material.render_phase_hint` may only re-order *within* the mapped phase, never cross the opaque/transparent boundary (asserted in `Material.resolvePhase`). |

## Go Package

```
pkg/render/material/
  material.go     // Material, MaterialParameters, AlphaMode, Sanitize
  pbr.go          // StandardPBR builder, default parameter table
  speckey.go      // specialization key = (shader,layout,alpha,phase)
pkg/render/light/
  light.go        // PointLight, SpotLight, DirectionalLight, AmbientLight
  ibl.go          // EnvironmentMapLight, IrradianceVolume (SH probes)
  shadow.go       // CascadeShadowConfig, shadow-map descriptors
internal/render/lighting/
  cluster.go       // tile/frustum light clustering (ForBatched)
  shadowpass.go    // shadow render-graph pass construction
```

## Type Definitions

```go
type AlphaMode uint8 // Opaque, Mask, Blend, Premultiplied, Add
func (a AlphaMode) Phase() render.RenderPhase                  // total, INV-5

type MaterialParameters struct {
    Textures map[string]asset.Handle[image.Image]
    Floats   map[string]float32
    Vectors  map[string]math.Vec4
    Colors   map[string]math.LinearRgba
}

type Material struct {                                          // asset
    Shader      asset.Handle[Shader]
    Alpha       AlphaMode
    DoubleSided bool
    Params      MaterialParameters
    PhaseHint   *render.RenderPhase                              // intra-phase only
}

type LightKind uint8 // Point, Spot, Directional, Ambient

type PointLight struct {                                        // component
    Color     math.LinearRgba
    Intensity float32
    Radius    float32
    Shadow    *CubeShadow
}
type SpotLight struct {
    Color                  math.LinearRgba
    Intensity              float32
    InnerAngle, OuterAngle float32
    Shadow                 *SingleShadow
}
type DirectionalLight struct {
    Color     math.LinearRgba
    Intensity float32
    Cascades  *CascadeShadowConfig
}
type AmbientLight struct{ Color math.LinearRgba }

type EnvironmentMapLight struct {
    Diffuse, Specular asset.Handle[image.Image]                  // pre-filtered cube
}
type IrradianceVolume struct {
    GridSize [3]uint32
    Probes   []math.Vec4                                          // SH coeffs
}

type CascadeShadowConfig struct {
    Count       uint8                                             // 1..4
    SplitMode   SplitMode                                          // Logarithmic | Manual
    Splits      []float32                                          // Manual mode
    Overlap     float32
    MapSize     uint32
    MaxDistance float32
}

type SpecializationKey struct {
    Shader uint64
    Layout uint64                                                  // mesh.VertexLayout.hash
    Alpha  AlphaMode
    Phase  render.RenderPhase
}
```

## Key Methods

```go
func StandardPBR() *Material                                    // default param table (§4.2)
func (m *Material) SetFloat(name string, v float32) *Material    // clamps PBR scalars (INV-2)
func (m *Material) SetColor(name string, c math.LinearRgba) *Material
func (m *Material) Validate() error                              // INV-1
func (p *MaterialParameters) Sanitize()                          // idempotent clamp (INV-2)
func (m *Material) SpecKey(layout mesh.VertexLayout) SpecializationKey // → pipeline cache

func (c *CascadeShadowConfig) Splits(near float32) ([]float32, error) // INV-3
func buildShadowPass(g *render.RenderGraph, casters []ShadowCaster)   // INV-4: edge Before lighting

func ClusterLights(view *render.RenderView, lights []LightRef,
    pool *task.ComputePool) ClusterGrid                          // tile/frustum cull
```

`m.SpecKey` is the bridge to render-core's pipeline cache (L1 §4.8 →
[l2-render-core-go §Performance](l2-render-core-go.md)); a changed key triggers
async pipeline recompile with a fallback pipeline meanwhile.

## Performance Strategy

- **Clustered lighting** (L1 §2): `ClusterLights` bins lights into froxels via
  `task.ForBatched` — unlimited scene lights, bounded per-tile cost.
- **Bind-group invalidation, not rebuild**: a parameter change marks the
  material's bind group dirty; only changed groups are re-created next frame.
- **Specialization key cached**: `SpecKey` is recomputed only when shader,
  layout, alpha, or phase changes — not per draw.
- **Shadow atlas**: all single/spot maps share one atlas texture; cascades use a
  layered array — fewer render-target switches.
- **`sync.Pool`** (C-027): cluster grids and per-frame light SSBO scratch reused.

## Error Handling

| Condition | Behavior |
| :--- | :--- |
| Material with nil shader | `ErrMaterialNoShader` at `Validate` (pre-pipeline) |
| Out-of-range PBR value | clamped silently by `Sanitize`; logged at `slog.Debug` if clamping occurred |
| Cascade config not covering range | `ErrCascadeCoverage` at `Splits` (pre-shadow-pass) |
| Shadow map alloc failure | shadow disabled for that light this frame; `slog.Warn`; lighting still renders |
| Custom shader compile failure | fallback to error material (magenta); surfaced via diagnostics |

```go
var (
    ErrMaterialNoShader = errors.New("material: nil shader handle")
    ErrCascadeCoverage  = errors.New("shadow: cascades do not cover near→max range")
)
```

## Testing Strategy

- **Clamp tests**: `Sanitize` is idempotent; metallic/roughness/occlusion
  clamped to [0,1]; negative emissive → 0 (table-driven).
- **Alpha→phase mapping**: every `AlphaMode` maps to the L1-specified phase;
  `PhaseHint` cannot cross opaque/transparent (INV-5 negative test).
- **Cascade coverage**: configs that under-cover `MaxDistance` return
  `ErrCascadeCoverage`; valid logarithmic split sums verified.
- **Shadow ordering**: graph asserts a shadow-pass→lighting-pass edge exists for
  every shadow-casting light (INV-4).
- **Cluster correctness**: a light fully outside a tile contributes to zero
  froxels; race-clean under `-race`.
- **Benchmarks**: `BenchmarkClusterLights`, `BenchmarkSpecKey` (0 alloc/op).

## 7. Drawbacks & Alternatives

- **Drawback**: only metallic-roughness PBR built in.
  **Alternative**: also ship specular-glossiness.
  **Decision**: L1 Open Question Q1 — <!-- TBD (L1 §5 Q1): secondary
  specular-glossiness workflow; metallic-roughness only for 0.1.0. -->
- **Drawback**: default cascade count is a guess.
  **Decision**: L1 Open Question Q2 — <!-- TBD (L1 §5 Q2): default
  CascadeShadowConfig.Count; placeholder 4, revisit with profiling. -->
- **Drawback**: clustered shading always on adds per-frame binning cost even for
  few lights.
  **Alternative**: forward path under a light-count threshold.
  **Decision**: L1 Open Question Q3 — <!-- TBD (L1 §5 Q3): light-count
  threshold for mandatory clustered shading. -->

## Canonical References

<!-- MANDATORY for Stable status. Stub — populate when implementation lands
     (Phase 4). Blocked: (1) L1 parent Draft; (2) C29 no examples/3d/ yet;
     (3) C-002 STOP FACTOR Phase 4. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-18 | Initial L2 draft — Go translation of l1-materials-and-lighting v0.1.0. Typed MaterialParameters, idempotent PBR clamp, total AlphaMode→phase map, CascadeShadowConfig coverage check, graph-edge-enforced shadow-before-lighting, ForBatched light clustering. L1 Q1–Q3 carried as TBD. Draft — L1 parent Draft + C29 + C-002 Phase 4 Hold. |
