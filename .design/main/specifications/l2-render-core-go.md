# Render Core — Go Implementation

**Version:** 0.1.0
**Status:** Draft
**Layer:** go
**Implements:** [l1-render-core.md](l1-render-core.md)

## Overview

Go-level design for the render core: a dedicated render **SubApp** owning its
own `*world.World`, a per-frame **extract** bridge from the main world, a
rebuildable **render graph** (DAG of passes), an opaque **RID** handle + thread-
safe **command queue** server, a pluggable `RenderBackend` interface, and a
four-phase render schedule (Collect → Extract → Prepare → Draw). Render objects
are renderer-owned proxies (not ECS entities) stored in a struct-of-arrays
`RenderDataHolder` for cache-efficient culling and direct GPU buffer binding.
Engine-private (`internal/render/`); only the backend interface, `RID`, and
pure-data components are public (`pkg/render/`).

## Related Specifications

- [l1-render-core.md](l1-render-core.md) — L1 concept specification (parent)
- [l2-app-framework-go.md](l2-app-framework-go.md) — render runs as a `SubApp` after the main `Update` schedule
- [l2-task-system-go.md](l2-task-system-go.md) — Collect/Extract/Prepare run on the `ComputePool` via `ForBatched`
- [l2-math-system-go.md](l2-math-system-go.md) — `Mat4`, `Frustum`, `Aabb` for view-projection and culling
- [l2-asset-system-go.md](l2-asset-system-go.md) — mesh/image/shader handles resolved during Prepare
- [l1-platform-system.md](l1-platform-system.md) — concrete `RenderBackend` selected per platform

## 1. Motivation

Gameplay code must never touch GPU state. A SubApp with its own World plus a
copy-only extract step gives hard isolation; a graph compiler lets the engine
reorder/merge/cull passes; an `RID` + command-queue server makes the frontend
thread-safe against a backend goroutine; a single `RenderBackend` interface
absorbs GPU-API churn (OpenGL/Vulkan/WebGPU/software) without leaking upward.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: generics for typed resource tables and `iter.Seq` over render objects.
- **C-003**: stdlib only for the core; concrete backends may use a single cgo/syscall binding behind the interface, justified per backend in an ADR.
- **C-027**: per-frame command/draw-item structs are `sync.Pool`-recycled — steady-state frame loop is allocation-free.
- **C-005**: Collect/Extract/Prepare are parallel and MUST be `-race` clean; the main world is read-only during Extract.
- **No shared mutable state**: extract copies bytes; render world holds no pointers into the main world.
- **Backend immutability**: the active `RenderBackend` is chosen at app init and never swapped at runtime.
- **`Option<T>` mapping**: L1 `Option<Viewport>` → `*Viewport` (nil = full target); `Option<IndexBuffer>` → `*BufferRID`.

## 3. Core Invariants

> [!NOTE]
> See [l1-render-core.md §3](l1-render-core.md) for technology-agnostic
> invariants. Go-specific compliance is tabulated in §4.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **1**: Pass declares inputs/outputs before execution | `RenderPass` interface requires `Inputs() []RID` and `Outputs() []RID`; the graph compiler calls both during `Graph.Build()` before any `Execute` — a pass that returns empty I/O for a non-utility phase fails build. |
| **2**: Graph acyclic, cycle = hard build error | `Graph.Build()` runs Kahn topological sort (reused pattern from `internal/ecs/scheduler` DAG); a remaining-edge set ⇒ `ErrRenderGraphCycle` with the offending node IDs. |
| **3**: Referenced GPU resources alive for pass duration | `resourceTracker` ref-counts every `RID`; the compiler pins inputs/outputs of a scheduled pass (`pin++`) and unpins after `EndRenderPass`; pinned RIDs are excluded from the deferred-delete queue. |
| **4**: Extract runs exactly once/frame before any pass | The render `SubApp` schedule has a fixed `ExtractStage` ordered `Before` `PrepareStage`; `frameOnce sync.Once`-style guard (reset per frame) asserts a single extract invocation; passes run only in `DrawStage`. |
| **5**: Backend fully implements `RenderBackend` | `RenderBackend` is a single Go interface; partial impls fail to compile. A `backendConformance` test exercises every method against the software rasteriser reference. |

## Go Package

```
pkg/render/
  backend.go     // RenderBackend interface, descriptors, RID, RenderPhase
  components.go   // pure-data ECS components: RenderTarget, extract markers
internal/render/
  subapp.go       // RenderSubApp: own World + schedule, plug into App
  extract.go      // ExtractFn registry, main→render copy, ThreadLocal scratch
  graph.go        // RenderGraph, RenderPass, Kahn build, barrier insertion
  server.go       // RID allocator, command queue, two-phase create
  resources.go    // resourceTracker (refcount + deferred delete), pipeline cache
  phases.go       // Collect/Extract/Prepare/Draw stages, RenderPhase sort
  feature.go      // RenderFeature interface, RenderObject proxy
  visibility.go   // VisibilityGroup, parallel frustum cull (ForBatched)
  renderdata.go   // RenderDataHolder SoA: StaticKey/DynamicKey arrays
```

`pkg/render` is public, ECS-component-only (pure data, no methods).
`internal/render` is engine-private — callers never import it.

## Type Definitions

```go
// RID — opaque 64-bit resource handle. Kind in high bits, index+gen in low.
type RID uint64

func (r RID) Kind() ResourceKind
func (r RID) IsNil() bool

type ResourceKind uint8 // Buffer, Texture, Pipeline, BindGroup, Scenario, View

type RenderBackend interface {
    CreateBuffer(BufferDesc) RID
    CreateTexture(TextureDesc) RID
    CreatePipeline(PipelineDesc) RID
    CreateBindGroup(BindGroupDesc) RID
    BeginRenderPass(RenderPassDesc)
    Draw(DrawCmd)
    EndRenderPass()
    Submit()
    Present()
    Destroy(RID)
}

type RenderPass interface {
    Name() string
    Phase() RenderPhase           // None for utility passes
    Inputs() []RID
    Outputs() []RID
    Execute(ctx *PassContext)
}

type RenderPhase uint8 // PhaseNone, Opaque, AlphaMask, Transparent, UI

type RenderGraph struct {
    nodes []RenderPass
    built bool
    order []int                   // topo order after Build
}

// RenderObject — renderer-owned proxy, NOT an ECS entity (L1 §4.10).
type RenderObject struct {
    dataIndex int                 // index into every RenderDataHolder array
    enabled   bool
    group     RenderGroup         // culling bitmask
    bounds    math.Aabb
}

type RenderGroup uint32

type RenderView struct {
    ViewProjection     math.Mat4
    CullingMask        RenderGroup
    LastFrameCollected uint64
    VisibleObjects     []int       // dataIndex list, per-frame
}

type RenderFeature interface {
    Initialize(*RenderSubApp)
    Collect(*CollectContext)
    Extract(*ExtractContext)
    PrepareEffectPermutations(*PrepareContext)
    Prepare(*PrepareContext)
    Draw(ctx *DrawContext, view *RenderView, stage RenderPhase)
    Flush(*FlushContext)
}

type RenderDataHolder struct {
    arrays      []dataArray            // one contiguous slice per registered key
    definitions map[dataKey]int
}

type server struct {
    mu      sync.Mutex
    queue   []command            // drained on the render goroutine
    nextRID atomic.Uint64
    onGoro  uint64               // render goroutine id (direct-call fast path)
}
```

## Key Methods

```go
// SubApp wiring (INV-4).
func NewRenderSubApp(backend RenderBackend) *RenderSubApp
func (s *RenderSubApp) RegisterExtract(fn ExtractFn)          // per plugin
func (s *RenderSubApp) AddFeature(f RenderFeature)
func (s *RenderSubApp) RunFrame(main *world.World)            // Collect→Extract→Prepare→Draw

// Graph (INV-1, INV-2, INV-3).
func (g *RenderGraph) AddPass(p RenderPass)
func (g *RenderGraph) Build() error                           // topo sort + barriers
func (g *RenderGraph) Execute(ctx *FrameContext)

// Server / RID (L1 §4.5 — two-phase create, never stalls caller).
func (s *server) Allocate(kind ResourceKind) RID              // sync, immediate
func (s *server) Initialize(rid RID, data any)                // queued, async
func (s *server) Submit(cmd command)                          // direct if on render goro
func (s *server) Drain()                                      // render goro: run queue

// Resource lifetime (INV-3).
func (t *resourceTracker) Retain(RID)
func (t *resourceTracker) Release(RID)                         // 0 refs ⇒ deferred-delete next frame

// Visibility (L1 §4.11 — parallel, false-negative-free / INV per camera spec).
func (vg *VisibilityGroup) TryCollect(v *RenderView, pool *task.ComputePool)

// SoA render data (L1 §4.12).
func RegisterStaticKey[T any](h *RenderDataHolder, name string) StaticKey[T]
func (k StaticKey[T]) Slice(h *RenderDataHolder) []T          // contiguous; GPU-bindable
```

`TryCollect` skips when `v.LastFrameCollected >= frameCounter`; otherwise it
builds the frustum from `v.ViewProjection` and runs `task.ForBatched` over the
object array, each batch appending to a thread-local collector merged after
join — one atomic claim per batch, `-race` clean.

## Performance Strategy

- **SoA render data** (L1 §4.12): per-object `[]Mat4` world matrices are one
  contiguous slice → frustum culling has no pointer chasing and the same slice
  binds directly as a GPU instance buffer (zero marshalling).
- **Parallel Collect/Extract/Prepare**: `task.ForBatched` over render objects;
  `ThreadLocal` scratch buffers avoid lock contention (L1 §4.9 Phase 2/3).
- **`sync.Pool` command & draw-item blocks** (C-027): recycled across frames;
  steady-state frame loop is 0-alloc.
- **Pipeline cache** (L1 §4.7): key = `(shaderID, materialKey, vertexLayout,
  phase)`; miss → async compile on `ComputePool`, fallback pipeline until ready.
- **Deferred deletion**: zero-ref RIDs are destroyed one frame later, never
  in-flight — avoids GPU stalls/validation errors.

## Error Handling

| Condition | Behavior |
| :--- | :--- |
| Graph cycle at build | `ErrRenderGraphCycle` (offending node IDs); frame aborts before any GPU work |
| Pass reads a released RID | `ErrResourceReleased`; graph build fails (pin check) — never a use-after-free |
| `Submit` after SubApp shutdown | `ErrRenderClosed`; command dropped, logged via `log/slog` |
| Backend method on partial impl | Compile error (single interface) — unrepresentable at runtime |
| Async pipeline compile failure | Fallback pipeline retained; error surfaced through diagnostics <!-- TBD (L1 §5 Q2): developer-facing surfacing channel for async compile failures --> |

```go
var (
    ErrRenderGraphCycle = errors.New("render: graph contains a cycle")
    ErrResourceReleased = errors.New("render: pass references a released RID")
    ErrRenderClosed     = errors.New("render: subapp is shut down")
)
```

## Testing Strategy

- **Race gate (C-005)**: parallel Collect/Extract/Prepare over 10k render
  objects under `-race`; assert no data race and `NumGoroutine` returns to
  baseline after `Shutdown`.
- **Graph topology**: table-driven cycle/acyclic fixtures; barrier insertion
  asserted against expected resource-transition list.
- **Extract isolation**: mutate the main world during Extract in a probe test;
  assert the render world snapshot is unaffected (INV — copy not share).
- **RID lifetime**: release a resource still pinned by a scheduled pass; assert
  it survives the frame and is destroyed exactly one frame after last release.
- **Backend conformance**: software rasteriser passes the full `RenderBackend`
  contract suite (golden-image diff for a known scene).
- **Benchmarks**: `BenchmarkFrustumCullSoA` (0 alloc/op steady), `BenchmarkGraphBuild`.

## 7. Drawbacks & Alternatives

- **Drawback**: `RenderBackend` has ~10 methods — violates the "1–3 method
  consumer interface" guideline.
  **Alternative**: split into `BufferBackend`/`PipelineBackend`/… facets.
  **Decision**: a GPU device is one cohesive resource; faceting fragments the
  command stream and complicates submission ordering. Documented exception.
- **Drawback**: copy-only extract duplicates transform/visibility data each
  frame.
  **Alternative**: shared read-locked view of the main world.
  **Decision**: copy is required by L1 INV (no shared mutable state) and keeps
  the render goroutine independent of main-world structural changes.
- **Drawback**: per-frame graph rebuild has compile cost.
  **Alternative**: cache the built graph; rebuild only on topology change.
  **Decision**: L1 Open Question Q1/Q3 — <!-- TBD (L1 §5 Q1,Q3): transient
  allocator strategy + max pass count before degradation; measure before
  caching the built graph. Not in 0.1.0 scope. -->
- **Server pattern parity** (L1 §5 Q4/Q5): whether physics/audio reuse the same
  RID+queue pattern is out of scope here — <!-- TBD (L1 §5 Q4,Q5) -->.

## Canonical References

<!-- MANDATORY for Stable status. Stub — populate when implementation lands
     (Phase 4 Render Pipeline). Stable promotion blocked: (1) L1 parent Draft;
     (2) C29 — no validating examples/3d/ yet; (3) C-002 STOP FACTOR Phase 4. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-18 | Initial L2 draft — Go translation of l1-render-core v0.5.0. RID+command-queue server, RenderBackend interface, Kahn-DAG graph, SubApp extract isolation, four-phase schedule on ComputePool, SoA RenderDataHolder, RenderFeature proxies. L1 Q1–Q5 carried as TBD. Draft — L1 parent Draft + C29 + C-002 Phase 4 Hold. |
