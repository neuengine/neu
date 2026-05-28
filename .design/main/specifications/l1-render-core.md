# Render Core

**Version:** 0.6.0
**Status:** RFC
**Layer:** concept

## Overview

The render core defines how frames are produced. A dedicated render SubApp owns its own World, receives extracted data each frame, and executes a render graph — a DAG of passes that declare their resource dependencies. A pluggable backend interface allows multiple GPU APIs without changing upper layers.

## Related Specifications

- [App Framework](l1-app-framework.md)
- [Mesh & Image](l1-mesh-and-image.md)
- [Materials & Lighting](l1-materials-and-lighting.md)
- [Platform System](l1-platform-system.md) — RenderBackend selection per platform

## 1. Motivation

Rendering must be decoupled from gameplay so the main World never touches GPU state directly. A graph-based approach lets the engine reorder, merge, or cull passes automatically. A backend interface future-proofs the engine against API churn.

## 2. Constraints & Assumptions

- The render SubApp runs **after** the main app finishes its Update schedule for the current frame.
- Extract functions are the **only** bridge between the main World and the render World — no shared mutable state.
- All GPU resource handles are opaque to code outside the render SubApp.
- The render graph must be rebuildable each frame (dynamic pass insertion is allowed).

## 3. Core Invariants

1. Every render pass must declare its inputs and outputs before execution.
2. The graph must be acyclic; a cycle is a hard error at graph-build time.
3. GPU resources referenced by a pass must be alive for the duration of that pass.
4. Extract runs exactly once per frame, before any render pass executes.
5. Backend implementations must satisfy the full `RenderBackend` interface — partial implementations are not permitted.

## 4. Detailed Design

### 4.1 Render SubApp & Extract Pattern

The render SubApp maintains its own World, schedule, and resource table. Each frame:

```plaintext
Main App Update
      │
      ▼
  Extract Phase ── copies relevant data ──► Render World
      │
      ▼
Render Schedule (Render World)
      │
      ▼
  Present
```

Extract functions are registered per plugin. Each receives a read-only view of the main World and a mutable reference to the render World. Data is copied, not shared.

### 4.2 Render Graph

The graph is a directed acyclic graph of `RenderPass` nodes. Each node declares:

- **Inputs**: texture handles or buffer handles it reads.
- **Outputs**: texture handles or buffer handles it writes.
- **Phase**: which render phase it belongs to (or `None` for utility passes).

The graph compiler topologically sorts passes, inserts barriers, and allocates transient resources.

### 4.3 Render Phases

Phases execute in a fixed order within the main scene pass:

| Phase | Sort Strategy | Notes |
| :--- | :--- | :--- |
| Opaque | Front-to-back by depth | Minimises overdraw |
| AlphaMask | Front-to-back by depth | Discard fragments below threshold |
| Transparent | Back-to-front by depth | Requires correct blend order |
| UI | Submission order | Screen-space overlay |

Each phase owns a list of draw functions. Draw functions are batched: items sharing the same pipeline, bind group, and vertex buffer are merged into a single draw call where possible.

### 4.4 Backend Abstraction

`RenderBackend` is an interface with the following surface (pseudo-code):

```plaintext
RenderBackend
  CreateBuffer(descriptor) -> BufferHandle
  CreateTexture(descriptor) -> TextureHandle
  CreatePipeline(descriptor) -> PipelineHandle
  CreateBindGroup(descriptor) -> BindGroupHandle
  BeginRenderPass(descriptor)
  Draw(pipeline, bind_groups, vertex_buffers, index_buffer, instances)
  EndRenderPass()
  Submit()
  Present()
  Destroy(handle)
```

`Destroy` releases all GPU-side resources associated with the handle. Callers must ensure the handle is no longer referenced by in-flight commands before calling `Destroy`; the backend must not free resources that may still be used by the current frame. Concrete backends (OpenGL, Vulkan, WebGPU, software rasteriser) implement this interface. The active backend is selected at app initialisation and cannot change at runtime.

### 4.5 Server Pattern and Handle-Based API

The render SubApp exposes its functionality through an opaque handle-based API. Callers never hold pointers to internal GPU objects — they hold `RID` (Resource ID) values, 64-bit opaque handles. The server owns all actual objects; callers interact exclusively through handles and a command queue.

**Handle structure**: A resource handle packs three fields into a single opaque value: a **kind tag** (buffer, texture, pipeline, bind group, scenario, render view), a **dense index** (the server's internal slot number), and a **generation counter** (detects stale handles after slot reuse). Callers compare handles by value; they never inspect internal fields.

**Command queue**: All calls from the main app to the render server are serialized through a thread-safe command queue. If the caller is on the server's dedicated goroutine, calls execute directly. Otherwise, the command is pushed onto the queue and (optionally) blocks for a return value.

**Two-phase resource creation**: Resource creation is split into two steps to avoid caller stalls:

```plaintext
1. Allocate() -> RID          // synchronous, returns handle immediately
2. Initialize(rid, data)      // queued to server goroutine, runs asynchronously
```

The caller can immediately use the RID to queue further setup commands (e.g., assign a material to a mesh) without waiting for initialization to complete. This eliminates pipeline bubbles during asset loading.

**Scenario**: A `Scenario` RID represents a self-contained 3D render world — all instances, lights, and environment settings for one logical scene. Multiple viewports can render the same Scenario or different ones independently. This enables:

- Editor viewports showing different angles of the same scene.
- Sub-worlds (portals, picture-in-picture) without duplicating entity data.
- Clean isolation between gameplay rendering and editor preview.

The same handle + command queue pattern applies to other server subsystems (physics, audio) for consistent thread-safe separation of frontend logic from backend computation.

### 4.6 GPU Resource Management

Buffers, textures, pipelines, and bind groups are stored as resources in the render World. A resource tracker performs reference counting; resources with zero references at the end of a frame are queued for deferred deletion (next frame) to avoid destroying in-flight GPU objects.

### 4.7 Pipeline Specialization

A pipeline key is derived from (shader_id, material_properties, vertex_layout, render_phase). The engine maintains a pipeline cache keyed on this tuple. Cache misses trigger asynchronous compilation; a fallback pipeline is used until the specialised variant is ready.

### 4.8 Draw Functions

A draw function is a registered callable per render phase. During the render schedule, the engine collects visible entities for each phase, sorts them, batches by pipeline + bind group, and invokes the corresponding draw function with the batched data.

### 4.9 Multi-Phase Render Pipeline

The render schedule within the render SubApp executes in four distinct phases, each with specific threading and data-access characteristics:

```plaintext
Phase 1 — Collect:
  - Initialize per-frame render state.
  - Enumerate visibility groups and discover active RenderViews.
  - Thread: Main render thread.

Phase 2 — Extract:
  - Fast, parallel copy of relevant entity data into render-specific structures.
  - Uses ThreadLocal scratch buffers to avoid synchronization overhead.
  - IMPORTANT: Does NOT block main simulation. Collects a "snapshot" of the world state.
  - Thread: Parallel workers.

Phase 3 — Prepare:
  - CPU-heavy computation: GPU resource allocation, buffer uploads, shader permutation.
  - Parallelizable per-light, per-view, or per-object.
  - Can overlap with the next frame's main simulation logic.
  - Thread: Parallel workers.

Phase 4 — Draw:
  - Issue actual GPU commands (DrawCalls) per stage.
  - Sequence-dependent within a single command encoder.
  - Thread: Render thread (or parallel encoders if backend supports).
```

**Frame counter tracking**: Each `RenderView` stores `LastFrameCollected` to skip redundant work. The render system increments a global `FrameCounter`; views are only re-processed when their state falls behind.

### 4.10 RenderFeature Extension Points

Render capabilities are added via `RenderFeature` — specialized processors that participate in all pipeline phases.

**Decoupling Principle**: `RenderObject` instances are NOT ECS `Entity` objects. They are light-weight Proxies created by a `RenderFeature` during the Extract phase. This allows the renderer to manage its own spatial hierarchy and resource lifecycle without being tied to the main ECS world's structural changes.

```plaintext
RenderFeature (interface)
  Initialize()
  Collect()                          // discover renderable objects
  Extract()                          // copy data from ECS to internal RenderObjects
  PrepareEffectPermutations(ctx)     // compile/select shader variants
  Prepare(ctx)                       // allocate GPU resources, fill buffers
  Draw(ctx, view, stage)             // issue draw commands
  Flush(ctx)                         // release per-frame temporaries
```

Each feature represents a complete capability (MeshRendering, ShadowMapping, PostProcessing). Features own their data and do not pollute components with GPU-specific state.

Features register with the RenderSystem and receive callbacks at each phase. Multiple features can contribute draw commands to the same RenderStage (e.g., both mesh and particle features contribute to the Opaque stage).

### 4.11 Visibility Culling

A `VisibilityGroup` manages spatial partitioning and frustum culling for render objects:

```plaintext
VisibilityGroup
  render_objects:  []RenderObject
  render_data:     RenderDataHolder     // struct-of-arrays per-object data

  TryCollect(view: RenderView):
    if view.LastFrameCollected >= FrameCounter: return   // already collected
    frustum = BuildFrustum(view.ViewProjection)
    ParallelFor(render_objects, func(obj):
      if !obj.Enabled: skip
      if !obj.CullingMask.Matches(view.CullingMask): skip
      if !frustum.Intersects(obj.BoundingBox): skip
      view.VisibleObjects.Add(obj)
    )
```

**Culling mask**: Each RenderObject has a `RenderGroup` bitmask, and each RenderView has a `CullingMask`. Only objects whose group matches the view's mask are considered. This enables camera-specific visibility (e.g., minimap camera only sees terrain and markers, not UI).

**Parallel culling**: Frustum tests run in parallel over the object array using batched dispatch. Each batch appends to a thread-local collector; results are merged after all batches complete.

### 4.12 Struct-of-Arrays Render Data

Render objects store their per-object data in a struct-of-arrays layout for cache-efficient iteration:

```plaintext
RenderDataHolder
  arrays:      []DataArray              // one contiguous array per data type
  definitions: map[DataKey]arrayIndex   // key → array mapping

  // Static keys (per-object, persistent):
  RenderStageMask: StaticKey[uint32]    // which stages this object participates in
  WorldMatrix:     StaticKey[Mat4]      // cached world transform

  // Dynamic keys (per-frame, regenerated):
  SortKey:         DynamicKey[uint64]   // computed sort value for ordering
```

Each `RenderObject` receives an index into all arrays. Iterating over all world matrices for frustum culling reads a single contiguous `[]Mat4` — no pointer chasing through heterogeneous object graphs. New data types can be registered at runtime without modifying the RenderObject struct.

This pattern also enables direct GPU buffer binding: a contiguous `[]Mat4` array maps directly to an instance buffer without marshalling.

## 5. Open Questions

1. Should transient resource allocation use a pool or a per-frame linear allocator? The reference-counting tracker (§4.6) pins resources for pass duration; a linear allocator could reduce fragmentation for short-lived render targets.
2. How should async pipeline compilation failures be surfaced to the developer? (Fallback pipeline is used in the interim — surfacing channel TBD.)
3. What is the maximum number of render passes before performance degrades on target hardware? Profile-driven; no hard limit defined yet.

> **Resolved — Q4**: Physics and audio servers follow the same handle + command queue pattern as the render server (stated in §4.5). This provides consistent thread-safe separation of frontend logic from backend computation across all server subsystems.
>
> **Resolved — Q5**: Physics callback inversion (pushing a state context into body callbacks during integration) is a physics-specific architecture decision. Moved to `l1-physics-system.md`.

## Canonical References

<!-- Downstream agents: read ALL files below before implementing or extending the render core. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |
| `[BACKEND]` | `pkg/render/backend.go` | `RenderBackend` interface, `RID`, `ResourceKind`, descriptors — public contract |
| `[GRAPH]` | `internal/render/graph.go` | `RenderGraph`, Kahn DAG build, barrier insertion, `ErrRenderGraphCycle` |
| `[SERVER]` | `internal/render/server.go` | `Server`: RID allocator, command queue, two-phase create pattern |
| `[RESOURCES]` | `internal/render/resources.go` | `ResourceTracker`: ref-count lifecycle, deferred deletion |
| `[PHASES]` | `internal/render/phases.go` | Four-phase render schedule (Collect / Extract / Prepare / Draw) |
| `[FEATURE]` | `internal/render/feature.go` | `RenderFeature` interface, `RenderObject` proxy pattern |
| `[VISIBILITY]` | `internal/render/visibility.go` | `VisibilityGroup`, parallel frustum cull, culling mask |
| `[RENDERDATA]` | `internal/render/renderdata.go` | `RenderDataHolder` SoA: `StaticKey`/`DynamicKey` arrays |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-03-25 | Initial draft from architecture analysis |
| 0.3.0 | 2026-03-26 | Added server pattern (RID + command queue), two-phase resource creation, Scenario concept, physics callback open questions |
| 0.4.0 | 2026-03-26 | Added multi-phase pipeline (Collect/Extract/Prepare/Draw), RenderFeature, visibility culling, struct-of-arrays render data |
| 0.5.0 | 2026-03-29 | Synchronized version with INDEX.md and applied registry sanitization (MD032/MD022) |
| 0.6.0 | 2026-05-28 | RFC promotion: added `Destroy(handle)` to backend interface (§4.4); added handle bit-layout note (§4.5); resolved Q4 (server pattern parity) and Q5 (physics callbacks moved to l1-physics-system.md); populated Canonical References with Phase 4 bootstrap files. Planned examples: `examples/3d/`, `examples/shader/`. |
