# Workspace Specifications Registry

**Version:** 2.33.0
**Status:** Active

## Overview

Local registry of specifications for this workspace. Organized by priority batch (P1–P8).

## P1 — ECS Core

| File | Description | Status | Layer | Version |
| :--- | :--- | :--- | :--- | :--- |
| [l1-world-system.md](specifications/l1-world-system.md) | Central data store: entities, components, resources, change tracking | Stable | concept | 0.2.1 |
| [l2-world-system-go.md](specifications/l2-world-system-go.md) | World Go implementation: World struct, DeferredWorld, ResourceMap, archetypes, tables | Stable | go | 0.1.0 |
| [l1-entity-system.md](specifications/l1-entity-system.md) | Entity lifecycle, generational IDs, allocation, disabling, abstract concept entities | Stable | concept | 0.3.0 |
| [l2-entity-system-go.md](specifications/l2-entity-system-go.md) | Entity Go implementation: EntityID, Entity, EntityAllocator, EntitySet, EntityMap | Stable | go | 0.1.0 |
| [l1-component-system.md](specifications/l1-component-system.md) | Component registration, storage strategies, hooks, required components | Stable | concept | 0.4.0 |
| [l2-component-system-go.md](specifications/l2-component-system-go.md) | Component Go implementation: ComponentID, ComponentRegistry, hooks, bundles, storage types | Stable | go | 0.1.0 |
| [l1-query-system.md](specifications/l1-query-system.md) | Data access: queries, filters, iteration, access tracking | Stable | concept | 0.2.1 |
| [l2-query-system-go.md](specifications/l2-query-system-go.md) | Query Go implementation: QueryState, filters, Access, ParIter, multi-arity generics | Stable | go | 0.1.0 |
| [l1-ecs-lifecycle-patterns.md](specifications/l1-ecs-lifecycle-patterns.md) | ECS Optimization: bitmask tagging, destructors, cached views, frame delay mitigation, object pooling | Stable | concept | 0.3.1 |
| [l2-pool-go.md](specifications/l2-pool-go.md) | Pool Go impl: `Pool[T]`, `SlicePool[T]` sync.Pool wrappers, C27 hot-path compliance (Implements: l1-ecs-lifecycle-patterns) | Stable | go | 0.1.0 |
| [l2-view-go.md](specifications/l2-view-go.md) | Entity view cache Go impl: reactive archetype subscription, range-over-func iteration, tagger helpers (Implements: l1-ecs-lifecycle-patterns) | Stable | go | 0.1.0 |
| [l1-system-scheduling.md](specifications/l1-system-scheduling.md) | System execution, DAG scheduling, parallel executor, system sets | Stable | concept | 0.4.0 |
| [l2-system-scheduling-go.md](specifications/l2-system-scheduling-go.md) | Go impl: System interface, DAG scheduler, executors, run conditions | Stable | go | 0.1.0 |
| [l1-command-system.md](specifications/l1-command-system.md) | Deferred mutations, command buffers, apply points | Stable | concept | 0.1.0 |
| [l2-command-system-go.md](specifications/l2-command-system-go.md) | Go impl: Command interface, CommandBuffer, entity reservation, flush | Stable | go | 0.1.0 |
| [l1-event-system.md](specifications/l1-event-system.md) | Events, messages, observers, reactive triggers | Stable | concept | 0.3.0 |
| [l2-event-system-go.md](specifications/l2-event-system-go.md) | Go impl: EventBus, MessageChannel, Observers, entity event bubbling | Stable | go | 0.1.0 |
| [l1-type-registry.md](specifications/l1-type-registry.md) | Runtime introspection, field metadata, dynamic type mapping | Stable | concept | 0.2.0 |
| [l2-type-registry-go.md](specifications/l2-type-registry-go.md) | Go impl: TypeRegistry, FieldInfo, DynamicObject, serialization hooks | Stable | go | 0.1.0 |

## P2 — Framework

| File | Description | Status | Layer | Version |
| :--- | :--- | :--- | :--- | :--- |
| [l1-hierarchy-system.md](specifications/l1-hierarchy-system.md) | Parent-child relationships, transform propagation, traversal | Stable | concept | 0.2.0 |
| [l2-hierarchy-system-go.md](specifications/l2-hierarchy-system-go.md) | Go impl: ChildOf, Children, Transform, GlobalTransform, propagation | Stable | go | 0.1.0 |
| [l1-time-system.md](specifications/l1-time-system.md) | Real/virtual/fixed time, timers, fixed timestep loop | Stable | concept | 0.2.0 |
| [l2-time-system-go.md](specifications/l2-time-system-go.md) | Go impl: gametime package, Time/RealTime/VirtualTime/FixedTime, Timer, Stopwatch | Stable | go | 0.1.0 |
| [l1-input-system.md](specifications/l1-input-system.md) | Keyboard, mouse, gamepad, touch; polling + events; picking | Stable | concept | 0.3.0 |
| [l2-input-system-go.md](specifications/l2-input-system-go.md) | Go impl: ButtonInput[T], AxisInput[T], input state systems, plugin | Stable | go | 0.2.0 |
| [l2-input-system-go-codes.md](specifications/l2-input-system-go-codes.md) | Go impl: KeyCode/GamepadButton/GamepadAxis/Touch reference type tables (Implements: l1-input-system) | Stable | go | 0.1.0 |
| [l1-state-system.md](specifications/l1-state-system.md) | Hierarchical state machines, transitions, computed states | Stable | concept | 0.1.0 |
| [l2-state-system-go.md](specifications/l2-state-system-go.md) | Go impl: State[S], NextState[S], SubState, ComputedState, DespawnOnExit | Stable | go | 0.1.0 |
| [l1-change-detection.md](specifications/l1-change-detection.md) | Tick-based change tracking, Added/Changed filters, Ref/Mut wrappers | Stable | concept | 0.1.0 |
| [l2-change-detection-go.md](specifications/l2-change-detection-go.md) | Go impl: Tick, ComponentTicks, Ref[T], Mut[T], RemovedComponents[T] | Stable | go | 0.1.0 |
| [l1-app-framework.md](specifications/l1-app-framework.md) | App builder, plugins, plugin groups, sub-apps, game loop | Stable | concept | 0.4.0 |
| [l2-app-framework-go.md](specifications/l2-app-framework-go.md) | Go impl: App, Plugin, PluginGroup, SubApp, RunMode, DefaultPlugins | Stable | go | 0.1.0 |
| [l1-multi-repo-architecture.md](specifications/l1-multi-repo-architecture.md) | Repository split architecture: pkg-based boundary between engine and editor | Draft | concept | 1.4.0 |
| [l2-multi-repo-architecture-go.md](specifications/l2-multi-repo-architecture-go.md) | Go impl: pkg/editor interfaces + pkg/protocol JSON wire messages, //go:build editor scoping, AST/go-list architecture-guard tests (Implements: l1-multi-repo-architecture) | Draft | go | 0.1.0 |

## P3 — Assets & Math

| File | Description | Status | Layer | Version |
| :--- | :--- | :--- | :--- | :--- |
| [l1-task-system.md](specifications/l1-task-system.md) | Parallelism: worker pools, scoped tasks, parallel iteration | Stable | concept | 0.2.0 |
| [l2-task-system-go.md](specifications/l2-task-system-go.md) | Go impl: ComputePool/IOPool, Chase-Lev work-stealing, RunScope, ForBatched, TaskHandle[T], MainThreadExecutor (Implements: l1-task-system) | Stable | go | 0.1.0 |
| [l1-asset-system.md](specifications/l1-asset-system.md) | Asset server, loaders, handles, hot-reload, IO abstraction | Stable | concept | 0.3.0 |
| [l2-asset-system-go.md](specifications/l2-asset-system-go.md) | Go impl: Handle[A]/Assets[A], AssetLoader registry, fs.FS VFS, stdlib dev watcher, ContentManager refcount (Implements: l1-asset-system) | Stable | go | 0.1.0 |
| [l1-scene-system.md](specifications/l1-scene-system.md) | Scene serialization, dynamic scenes, spawning, entity remapping | Stable | concept | 0.4.0 |
| [l2-scene-system-go.md](specifications/l2-scene-system-go.md) | Go impl: StaticScene gob + DynamicScene reflection, two-pass remap, interned binary codec, SceneSpawner (Implements: l1-scene-system) | Stable | go | 0.1.0 |
| [l1-math-system.md](specifications/l1-math-system.md) | Vectors, matrices, quaternions, colors, geometric primitives | Stable | concept | 0.3.0 |
| [l2-math-system-go.md](specifications/l2-math-system-go.md) | Go impl: Vec/Mat/Quat/Affine3, Dir/Isometry, primitives, Color, Curves, TransformInterpolator (Implements: l1-math-system) | Stable | go | 0.1.0 |

## P4 — Render Pipeline

| File | Description | Status | Layer | Version |
| :--- | :--- | :--- | :--- | :--- |
| [l1-render-core.md](specifications/l1-render-core.md) | Render graph, extract pattern, render world, backend abstraction | Stable | concept | 0.6.0 |
| [l2-render-core-go.md](specifications/l2-render-core-go.md) | Go impl: RID+command-queue server, RenderBackend interface, Kahn-DAG graph, SubApp extract isolation, 4-phase schedule, SoA RenderDataHolder (Implements: l1-render-core) | Stable | go | 0.1.0 |
| [l1-mesh-and-image.md](specifications/l1-mesh-and-image.md) | Mesh assets, vertex layout, image/texture, texture atlases | Stable | concept | 0.1.0 |
| [l2-mesh-and-image-go.md](specifications/l2-mesh-and-image-go.md) | Go impl: immutable attribute-map Mesh, validated index/skin invariants, FNV layout hash, stdlib decode on IOPool, shelf-pack DynamicAtlas (Implements: l1-mesh-and-image) | Stable | go | 0.1.0 |
| [l1-materials-and-lighting.md](specifications/l1-materials-and-lighting.md) | Material system, PBR, light types, shadows, environment maps | Stable | concept | 0.1.0 |
| [l2-materials-and-lighting-go.md](specifications/l2-materials-and-lighting-go.md) | Go impl: typed MaterialParameters, idempotent PBR clamp, total AlphaMode→phase, CascadeShadowConfig, ForBatched light clustering (Implements: l1-materials-and-lighting) | Stable | go | 0.1.0 |
| [l1-camera-and-visibility.md](specifications/l1-camera-and-visibility.md) | Camera, projections, visibility hierarchy, frustum culling | Stable | concept | 0.1.0 |
| [l2-camera-and-visibility-go.md](specifications/l2-camera-and-visibility-go.md) | Go impl: pure-data Camera, projection guards, 3-layer visibility reusing hierarchy walk, conservative frustum test, ForBatched cull (Implements: l1-camera-and-visibility) | Stable | go | 0.1.0 |
| [l1-post-processing.md](specifications/l1-post-processing.md) | Post-process effects, anti-aliasing, tonemapping, bloom | Stable | concept | 0.1.0 |
| [l2-post-processing-go.md](specifications/l2-post-processing-go.md) | Go impl: canonical EffectSlot chain, omit-disabled graph nodes, pooled ping-pong, HDR→LDR tonemap guard, AA mutual-exclusion (Implements: l1-post-processing) | Stable | go | 0.1.0 |

## P5 — Content Systems

| File | Description | Status | Layer | Version |
| :--- | :--- | :--- | :--- | :--- |
| [l1-audio-system.md](specifications/l1-audio-system.md) | Audio playback, spatial audio, backend abstraction | Stable | concept | 0.3.0 |
| [l2-audio-system-go.md](specifications/l2-audio-system-go.md) | Go impl: AudioBackend/AudioDriver split, bus-graph DAG, effect factory/instance, spatial systems, headless stub driver (Implements: l1-audio-system) | Stable | go | 0.1.0 |
| [l1-asset-formats.md](specifications/l1-asset-formats.md) | Asset loaders: glTF, images, audio codecs, scene files | Draft | concept | 0.1.0 |
| [l2-asset-formats-go.md](specifications/l2-asset-formats-go.md) | Go impl: stdlib-first image/audio loaders, build-tag-gated optional formats, glTF multi-asset fan-out + stable GltfAssetLabel, scene.json codec (Implements: l1-asset-formats) | Draft | go | 0.1.0 |
| [l1-2d-rendering.md](specifications/l1-2d-rendering.md) | Sprites, texture slicing, text rendering, 2D pipeline | Stable | concept | 0.2.0 |
| [l2-2d-rendering-go.md](specifications/l2-2d-rendering-go.md) | Go impl: Sprite2DFeature reusing render-core SoA+VisibilityGroup, deterministic Z→Y→entity sort, atlas batching, 2D ortho+pixel-perfect camera (Implements: l1-2d-rendering) | Stable | go | 0.1.0 |
| [l1-animation-system.md](specifications/l1-animation-system.md) | Animation graphs, clips, curves, skeletal animation, morph targets | Stable | concept | 0.1.0 |
| [l2-animation-system-go.md](specifications/l2-animation-system-go.md) | Go impl: clip/curve sampling, AnimationGraph (clip/blend/add + cross-fade), skeletal Joint/Skin, morph, reflection targets, per-root parallel deterministic eval (Implements: l1-animation-system) | Stable | go | 0.1.0 |
| [l1-tweening-system.md](specifications/l1-tweening-system.md) | Interpolation, easing curves, asynchronous animations | Stable | concept | 0.1.0 |
| [l2-tweening-system-go.md](specifications/l2-tweening-system-go.md) | Go impl: Tween component, easing library + generic Lerp, Tweening-schedule advance (Real/Virtual delta), self-cleanup on completion/despawn (Implements: l1-tweening-system) | Stable | go | 0.1.0 |

## P6 — UI & Tools

| File | Description | Status | Layer | Version |
| :--- | :--- | :--- | :--- | :--- |
| [l1-definition-system.md](specifications/l1-definition-system.md) | JSON declarative layer: UI, scenes, flows, templates — data-driven bridge | Draft | concept | 0.5.0 |
| [l1-definition-integration.md](specifications/l1-definition-integration.md) | Definition editor integration, network boundary & serialization contract (extracted from l1-definition-system §4.9–§4.13) | Draft | concept | 0.1.0 |
| [l1-window-system.md](specifications/l1-window-system.md) | Window management, multi-window, platform abstraction | Draft | concept | 0.1.0 |
| [l1-diagnostic-system.md](specifications/l1-diagnostic-system.md) | Diagnostics, profiling, gizmos, error codes, debug overlay | Draft | concept | 0.1.0 |
| [l1-ui-system.md](specifications/l1-ui-system.md) | Layout engine, interaction, text, widgets, styling | Draft | concept | 0.2.0 |
| [l2-benchmark-spec.md](specifications/l2-benchmark-spec.md) | Standardized performance tests and comparisons | Draft | test | 0.2.0 |
| [l1-build-tooling.md](specifications/l1-build-tooling.md) | CI pipeline, golden file testing, benchmarks, migration/release doc formats | Draft | concept | 0.4.0 |
| [l2-codegen-tools.md](specifications/l2-codegen-tools.md) | Automatic boilerplate generation and type-safe query wrappers (Implements: l1-build-tooling) | Draft | tool | 0.2.0 |
| [l1-code-documentation.md](specifications/l1-code-documentation.md) | AI-readable symbol metadata (AI-Meta) + workflow-artifact hygiene for code/docs | Draft | concept | 0.1.0 |
| [l2-code-documentation-go.md](specifications/l2-code-documentation-go.md) | Go impl: `AI-Meta:` godoc grammar, stability vocab, scope tiers, ci detection | Draft | go | 0.1.0 |
| [l1-cli-tooling.md](specifications/l1-cli-tooling.md) | Internal command-line interface for scaffolding, managing assets, and executing engine routines | Draft | concept | 0.2.0 |
| [l1-platform-system.md](specifications/l1-platform-system.md) | Cross-platform abstraction: tiers, capabilities, build tags, backends | Draft | concept | 0.1.0 |
| [l1-ai-assistant-system.md](specifications/l1-ai-assistant-system.md) | AI assistant plugin architecture for editor: agents, capabilities, protocol | Draft | concept | 0.2.0 |
| [l1-plugin-distribution.md](specifications/l1-plugin-distribution.md) | Third-party plugin distribution: manifest, in/out-of-process modes, capabilities, public SDK | Draft | concept | 0.1.0 |
| [l1-ai-api-plugin.md](specifications/l1-ai-api-plugin.md) | First-party AI API plugin (`pkg/plugins/aiapi/`): OpenAI/Anthropic/Gemini/local providers via HTTP | Draft | concept | 0.1.0 |
| [l1-examples-framework.md](specifications/l1-examples-framework.md) | Examples directory structure, conventions, and lifecycle | Draft | concept | 0.4.0 |
| [l1-compatibility-policy.md](specifications/l1-compatibility-policy.md) | Policy on engine versioning and Go toolchain compatibility matrix | Draft | concept | 0.3.0 |
| [l1-error-core.md](specifications/l1-error-core.md) | Structured error taxonomy: E-series codes, localization, severity | Draft | concept | 0.2.0 |
| [l1-visual-graph-system.md](specifications/l1-visual-graph-system.md) | Blueprint-style visual graph programming: node model, execution engine, editor "door" interfaces | Draft | concept | 0.2.0 |
| [l2-visual-graph-editor-bridge.md](specifications/l2-visual-graph-editor-bridge.md) | Go impl: `pkg/editor/graph.go` interfaces (GraphEditorPlugin/NodeRegistryQuery/GraphDebugger) + `pkg/protocol/graph.go` IPC (Implements: l1-visual-graph-system) | Draft | go | 0.1.0 |

## P7 — Advanced Core

| File | Description | Status | Layer | Version |
| :--- | :--- | :--- | :--- | :--- |
| [l1-profiling-protocol.md](specifications/l1-profiling-protocol.md) | Tracy integration, custom spans, pprof mapping, export formats | Draft | concept | 0.1.0 |
| [l1-networking-system.md](specifications/l1-networking-system.md) | Multiplayer boundaries: snapshot/rollback primitives, fixed-step sync | Draft | concept | 0.2.0 |
| [l1-transport.md](specifications/l1-transport.md) | UDP transport: channels, reliability, connection lifecycle, MTU discovery | Draft | concept | 0.1.0 |
| [l1-replication.md](specifications/l1-replication.md) | State replication: markers, entity mapping, visibility, delta compression, priority | Draft | concept | 0.1.0 |
| [l1-snapshot-interpolation.md](specifications/l1-snapshot-interpolation.md) | Sync model: server snapshots, client interpolation buffer, adaptive delay | Draft | concept | 0.1.0 |
| [l1-client-prediction.md](specifications/l1-client-prediction.md) | Sync model: local input prediction, server reconciliation, rollback smoothing | Draft | concept | 0.1.0 |
| [l1-lockstep.md](specifications/l1-lockstep.md) | Sync model: deterministic lockstep, input delay, speculative execution, desync detect | Draft | concept | 0.1.0 |
| [l1-rpc.md](specifications/l1-rpc.md) | Typed network RPC: send/receive, event integration, rate limiting | Draft | concept | 0.1.0 |
| [l1-network-diagnostics.md](specifications/l1-network-diagnostics.md) | Network metrics, alerts, debug overlay, profiling spans, desync reports | Draft | concept | 0.1.0 |
| [l1-hot-reload.md](specifications/l1-hot-reload.md) | Go code hot-restart with state snapshot, shader hot-swap, reload orchestrator | Draft | concept | 0.1.0 |

## P8 — Extended Systems

| File | Description | Status | Layer | Version |
| :--- | :--- | :--- | :--- | :--- |
| [l1-physics-system.md](specifications/l1-physics-system.md) | Physics server, deterministic solver, SubApp integration, interpolation | Draft | concept | 0.1.0 |
| [l1-rigid-body.md](specifications/l1-rigid-body.md) | RigidBody component: mass, damping, axis locks, body types, sleep | Draft | concept | 0.1.0 |
| [l1-collider.md](specifications/l1-collider.md) | Collision shapes: primitives, compound shapes, mesh/convex, filters | Draft | concept | 0.1.0 |
| [l1-physics-query.md](specifications/l1-physics-query.md) | Ray/Shape/Point/Overlap queries, batching, filters, predicates | Draft | concept | 0.1.0 |
| [l1-joints.md](specifications/l1-joints.md) | Joint constraints: Hinge, Piston, Ball, Distance, Fixed, Motorized | Draft | concept | 0.1.0 |
| [l1-collision-events.md](specifications/l1-collision-events.md) | Contact/Trigger events, manifolds, filtering patterns, deferred despawn | Draft | concept | 0.1.0 |
| [l1-physics-materials.md](specifications/l1-physics-materials.md) | Friction/Restitution assets, combine rules, surface tags, hot-reload | Draft | concept | 0.1.0 |
| [l1-character-controller.md](specifications/l1-character-controller.md) | Kinematic capsule movement, iterative sweep, step-up, slope snapping | Draft | concept | 0.1.0 |
| [l1-scripting-system.md](specifications/l1-scripting-system.md) | Scripting bridge (deferred): Lua/Tengo candidates, ECS API reference design | Draft | concept | 0.2.0 |

## Meta Information

- **Maintainer**: Core Team
- **Last Updated**: 2026-05-28
- **Total Specifications**: 101 (65 L1 concept + 34 L2 Go + 1 test + 1 tool) | Stable: 58 | RFC: 0 | Draft: 43
- **Engine Version:** 2.1.28
- **Last Authoring:** 2026-05-28 — **5 new L2 Go contracts authored for Phase 5** (`/magic.spec`): `l2-{audio-system,asset-formats,2d-rendering,animation-system,tweening-system}-go` — each `Implements:` its Draft L1 parent, status `Draft` (L1 parent Draft + no implementation → Canonical References stub → Stable correctly blocked). Achieves L1+L2 parity with Phases 1–4 ahead of Phase 5 execution. 5 new hard L2→L1 edges (29 total), 1:1, acyclic. `l2-2d-rendering-go` references the now-Stable render-core SoA infra.
- **Last Stabilization:** 2026-05-28 — **Phase 5 Content Systems: 8 specs promoted Draft → Stable** (`/magic.task` Pre-Planning Stabilization): 4 L1 + 4 L2 for audio, animation, tweening, 2d-rendering. C29 P5 gate closed by T-5T05 (46/46 pkgs PASS; `examples/{audio,animation,tweening,2d}` hash-stable ×20). The 4 promoted L2 specs' **Canonical References populated** with on-disk-verified source + test + example files (closing the P4-style empty-refs advisory debt for P5). **Held Draft: `l1-asset-formats` + `l2-asset-formats-go`** — the spec's headline glTF multi-asset fan-out (INV-4), `.scene.json` codec, and font loaders are unimplemented (only stdlib image + WAV loaders landed); no end-to-end example in the C29 gate. Not a headless-stub gap (cf. render-core nopBackend) but an absent subsystem → promotion would overclaim. Revisit when the glTF loader lands. **Stable 50 → 58, Draft 51 → 43.** Prior: **Render Core ratified: `l1-render-core` RFC → Stable + `l2-render-core-go` Draft → Stable** (`/magic.spec`). Evidence: Phase 4 complete (19/19) + C29 P4 gate closed by T-4T05; L1 Canonical References already filled, Q4/Q5 resolved, Q1–Q3 annotated non-blocking; L2 Canonical References populated with 11 verified source files + 2 conformance/isolation tests. Bootstrap fully deactivated for **all 10 P4 render specs** — Phase 4 = 10/10 Stable. Phase 5 gate ("Render Core Stable") now cleared. Prior: **P4 Render Pipeline: 8 specs promoted Draft → Stable** (4 L1 + 4 L2: mesh-and-image, materials-and-lighting, camera-and-visibility, post-processing) via Pre-Planning Stabilization — C29 P4 gate satisfied by T-4T05 (`examples/{3d,camera,shader}/` validated, 36/36 pkgs PASS, 0-alloc hot paths). Prior: P1 ECS (2 new L2: `l2-pool-go`, `l2-view-go`) promoted directly to Stable (L1 parent Stable + MVC + C9 Trust Mode). `l1-render-core` promoted Draft → RFC (v0.6.0: +`Destroy`, +handle layout, +Canonical References, Q4/Q5 resolved). Prior: 2026-05-18 — P3 Assets/Math/Concurrency (4 L1 + 4 L2: task, asset, scene, math) promoted Draft → Stable via Pre-Planning Stabilization (C29 P3 gate satisfied by T-3T05 — `examples/{async,asset,scene,math}/` green). Bootstrap deactivated for P3. Prior: 2026-05-17 — P2 Framework (6 L1 + 7 L2) promoted Draft → Stable (gate: `examples/ecs/framework/`; multi-repo l1/l2 stay Draft — RFC-gated, Exit Criterion via /magic.spec)
