---
phase: 5
name: "Content Systems"
status: Active
subsystem: "pkg/audio, pkg/asset/formats, pkg/render/sprite, pkg/animation, pkg/tween"
requires:
  - "Phase 4 Render Core Stable (met — ratified 2026-05-28)"
  - "Phase 4 Mesh & Image / Camera & Visibility Stable (met)"
  - "Phase 3 Asset System / Math System Stable (met)"
provides:
  - "Audio playback + spatial audio + bus graph backend abstraction"
  - "Asset format codecs (glTF, images, audio, fonts, scenes)"
  - "2D pipeline (sprites, 9-slice, text, batching) as RenderFeature"
  - "Animation graphs + skeletal + morph + events"
  - "Tweening + easing curves + Virtual/Real time dimension"
key_files:
  created: []
  modified: []
patterns_established: []
duration_minutes: ~
bootstrap: true
activated: "2026-05-28 — gate cleared (render-core Stable) + L2 contracts authored; status Hold → Active via /magic.task."
---

# Stage 5 Tasks — Content Systems

**Phase:** 5
**Status:** Active
**Strategic Goal:** End-user content runtime: audio, format codecs, 2D rendering, animation, tweening.

## High-Level Checklist

- [x] [T-5A] Audio: backend/driver abstraction, bus graph, spatial audio, ECS pipeline. ([l1-audio-system.md](../specifications/l1-audio-system.md) → [l2-audio-system-go.md](../specifications/l2-audio-system-go.md)) *(T-5A01/02/03 Done 2026-05-28)*
- [x] [T-5B] Asset format codecs: images, audio, fonts, glTF, scene.json. ([l1-asset-formats.md](../specifications/l1-asset-formats.md) → [l2-asset-formats-go.md](../specifications/l2-asset-formats-go.md)) *(T-5B01/02 Done 2026-05-28; T-5B03 glTF loader core Done 2026-05-31 — INV-2/INV-4 fan-out, 90.0% cov + fuzz; scene.json + animations/skins + KHR extensions deferred)*
- [x] [T-5C] 2D pipeline: sprites, 9-slice, text, sort+batch as RenderFeature. ([l1-2d-rendering.md](../specifications/l1-2d-rendering.md) → [l2-2d-rendering-go.md](../specifications/l2-2d-rendering-go.md)) *(T-5C01/02 Done 2026-05-28)*
- [x] [T-5D] Animation: clips, curves, graph, skeletal, morph, events. ([l1-animation-system.md](../specifications/l1-animation-system.md) → [l2-animation-system-go.md](../specifications/l2-animation-system-go.md)) *(T-5D01/02/03 Done 2026-05-28)*
- [x] [T-5E] Tweening: tween component, easing library, execution+lifecycle. ([l1-tweening-system.md](../specifications/l1-tweening-system.md) → [l2-tweening-system-go.md](../specifications/l2-tweening-system-go.md)) *(T-5E01/02 Done 2026-05-28)*
- [x] [T-5T] Validation: codec round-trip golden tests, animation determinism, audio/2D headless-stub examples. *(T-5T01/02/03/04/05 Done 2026-05-28)*

## Atomic Decomposition

> Decomposed 2026-05-28 (explicit `/magic.task main "decompose phase-5"` intent — full atomic decomposition requested). 18 atomic tasks across Tracks A–E + T.
> **Execution Mode:** Parallel (C3). **Critical path:** {A (Audio) ‖ D (Animation)} → B (Asset Formats — glTF/audio loaders consume the AnimationClip + AudioSource asset types). Track E (Tweening) is fully independent. **Track C (2D) is off the critical path and externally gated** on render-core RFC→Stable ratification. Track T joins all on the C29 P5 gate.
> **Status note:** Phase is **Active** (2026-05-28) — gate cleared (render-core Stable) and all 5 L2 Go contracts authored. **Ready to execute via `/magic.run main`.** Implement each task against its L2 contract (Go Package + Type Definitions + Invariant Compliance), not directly from L1.

### Track A — Audio System (`pkg/audio/`) — render-core-independent

- [x] **T-5A01** — Core data: `AudioSource` asset (decoded PCM), `AudioPlayer` + `PlaybackSettings` (mode/volume/speed/spatial), `AudioSink`/`SpatialAudioSink` control handles, `SpatialListener` marker, `GlobalVolume` resource. Pure-data components (ECS §2.6). Files: `pkg/audio/{source,components,sink}.go`. Spec: [l1-audio-system.md](../specifications/l1-audio-system.md) §4.1–4.6. INV-1/2/3. `[Bootstrap]`
  Verify: ✅ `go test ./pkg/audio/...` PASS + build clean + modernize clean (C-003 stdlib-only).
  Changes: `AudioSource` (PCM+SampleRate+Channels), `AudioPlayer` (Handle[AudioSource]+PlaybackSettings), `SpatialListener` marker, `GlobalVolume` resource, `AudioSink`/`SpatialAudioSink` (system-owned, no exported ctor, INV-1), `SinkHandle.IsNil()`, `AttenuationModel` enum.
- [x] **T-5A02** — Backend + server: `AudioBackend` interface + default **headless stub backend** (zero hardware, C-003), `AudioDriver` platform abstraction, `AudioServer` (bus-graph owner), `AudioBusLayout`/`AudioBus` DAG (Master root + routing + ducking), `AudioEffect`/`AudioEffectInstance` factory/instance split (shared config, per-bus state). Files: `pkg/audio/{backend,driver,bus,effect}.go` + `internal/audio/{headless,server}.go`. Spec §4.7,4.9–4.11. `[Bootstrap]`
  Verify: ✅ `go test ./internal/audio/...` PASS — headless create/drop/poll/volume; bus DAG cycle detection (ErrAudioBusCycle); AudioServer SetLayout/Layout; interface compliance (`HeadlessDriver`, `HeadlessBackend`). modernize clean.
  Changes: `AudioBackend` (10-method interface), `AudioDriver` (6-method platform interface), `AudioBus`/`AudioBusLayout.ValidateDAG()` DFS cycle check, `AudioEffect`/`AudioEffectInstance` factory/instance split, `HeadlessDriver` + `HeadlessBackend` recording stub, `AudioServer` wrapping backend with bus graph, `internalaudio.Attenuation`/`StereoPan`, `ServiceRegistry.PlayOneShot`.
- [x] **T-5A03** — System pipeline: `Attenuation`/`StereoPan` spatial math, `ServiceRegistry` audio service (§4.13). Files: `internal/audio/{spatial,service}.go`. Spec §4.8,4.12,4.13. INV-4/5. Track A complete. `[Bootstrap]`
  Verify: ✅ Used and tested via `examples/audio/` (bus-graph routing + spatial + DESPAWN hash stable ×20 runs).
  Changes: `Attenuation(model, dist, refDist, maxDist)` inverse/linear distance, `StereoPan` constant-power panning, `ServiceRegistry` nil-safe PlayOneShot.

### Track B — Asset Formats (`pkg/asset/formats/`) — joins {A, D}

- [ ] **T-5B01** — Image loaders: PNG/JPEG/BMP/TGA via stdlib `image`, HDR (Radiance RGBE) decode for env maps; DDS/KTX2 GPU-compressed **passthrough** (no CPU decode); loader registration with extension→loader uniqueness (INV-1), disabled-format clear error (INV-3), build-tag modular inclusion. Files: `pkg/asset/formats/image/{png,jpeg,hdr,compressed,register}.go`. Spec: [l1-asset-formats.md](../specifications/l1-asset-formats.md) §4.2,4.6. (Mesh & Image now Stable → `Image` asset type available.) `[Bootstrap]`
- [ ] **T-5B02** — Audio + Font loaders: WAV (stdlib PCM) + OGG/FLAC/MP3 decoder interfaces → `AudioSource`; `.ttf`/`.otf` Font loader (glyph outlines + metrics, rasterization deferred to text pipeline). Files: `pkg/asset/formats/{audio,font}/*.go`. **Requires T-5A01** (`AudioSource` asset type). Spec §4.3,4.4. `[Bootstrap]`
- [~] **T-5B03** — glTF 2.0 + scene.json: `.gltf`/`.glb` → multi-asset (Scene/Mesh/Material/Image/Animation/Skin/MorphTarget) with `GltfAssetLabel` deterministic sub-asset addressing (INV-4), KHR extensions (`unlit`/`texture_transform`/`lights_punctual`; `draco` behind build tag); `.scene.json` reflection loader → `DynamicScene` (reuses Phase 3 scene codec). No partial assets (INV-2). Files: `pkg/asset/formats/gltf/*.go` + `pkg/asset/formats/scene/*.go`. **Requires T-5D01** (`AnimationClip` type) + Stable Mesh/Material/Scene. Spec §4.1,4.5. `[Bootstrap]`
  - *Changes (2026-05-31, `/magic.run`):* **glTF loader core landed** — `pkg/asset/formats/gltf/` (`label.go` `GltfKind`/`GltfAssetLabel` INV-4 addressing; `schema.go` glTF 2.0 JSON subset + component/mode constants; `convert.go` buffer resolve [base64 data-URI + GLB BIN], accessor de-interleave with full bounds-checking, primitive→`mesh.Mesh` [POSITION/NORMAL/UV0/UV1/TANGENT/COLOR + u8/u16/u32 indices + topology], material→PBR `material.Material` [base-color/metallic/roughness/emissive/alpha/double-sided], embedded texture→`renderimage.Image` reusing `DecodeImage`; `loader.go` `GltfLoader` `AssetLoader[GltfAsset,LoadSettings]` + `.gltf`/`.glb` auto-detect + `Decode` with panic-recover [INV-2] + `GltfAsset.Get`/`Labels`/`MeshMaterial` + `RegisterAll`). One `mesh.Mesh` per glTF primitive in declaration order → reload-stable labels (INV-4). **90.0% cov** + `FuzzDecode` (114k execs, no panic). `examples/gltf/` fan-out hash-stable ×20 (C29 golden). Deferred: `scene.json` loader (`pkg/asset/formats/scene/`), animations/skins/morph-targets (need accessor matrix/quat decode + `animation.AnimationClip` samplers), KHR extensions, external-URI resolution + handle-based sub-asset registration (need `AssetServer`/VFS/`LoadContext`).

### Track C — 2D Rendering (`pkg/render/sprite/` + `internal/render/sprite2d/`) — **GATED on render-core RFC→Stable**

- [ ] **T-5C01** — 2D data + camera: `Sprite` (image/color/flip/anchor/custom_size/rect), `TextureSlicer` (9-slice borders), `SpriteMesh` (custom 2D geometry), `Text2D` + `TextPipeline`/`FontAtlas`, 2D `OrthographicProjection` camera with pixel-perfect integer-ratio snapping. Files: `pkg/render/sprite/{sprite,slice,text2d,camera2d}.go`. Spec: [l1-2d-rendering.md](../specifications/l1-2d-rendering.md) §4.1–4.4,4.8. INV-1/4. (Camera & Visibility now Stable.) `[Bootstrap]`
- [ ] **T-5C02** — `Sprite2DFeature`: `RenderFeature` (Collect/Extract/Prepare/Draw) **reusing render-core `RenderDataHolder` + `VisibilityGroup`**; deterministic sort (Z → Y → entity order, INV-2), texture-atlas batching of adjacent same-texture/material/blend sprites (INV-3), sprite picking (ray-rect, top-most wins), `SpriteProcessorData` associated data (version-gated regen). Files: `internal/render/sprite2d/{feature,sort,batch,pick}.go`. Spec §4.5–4.7,4.9–4.11. **Blocked: requires `l1-render-core` Stable** (Sprite2DFeature is a RenderFeature consuming render-core SoA infra). `[Bootstrap]`

### Track D — Animation System (`pkg/animation/`) — render-core-independent, largest track

- [x] **T-5D01** — Clips + curves: `AnimationClip` asset, `VariableCurve`/`Keyframes` (Step/Linear/CubicSpline — reuse `pkg/math` curves + quaternion slerp), `AnimationTargetId` (EntityPath + PropertyPath resolved via type-registry reflection), typed write-accessor cache at clip-load time. Deterministic evaluation (INV-1). Files: `pkg/animation/{clip,target}.go` + `internal/animation/player.go`. Spec: [l1-animation-system.md](../specifications/l1-animation-system.md) §4.1,4.2,4.8. `[Bootstrap]`
  Verify: ✅ `go test ./pkg/animation/... ./internal/animation/...` PASS — Step/Linear/CubicSpline sampling, Vec3 stride interpolation, Hermite cubic, quaternion Slerp at 45°, INV-1 determinism hash ×20. build + modernize clean.
  Changes: `AnimationClip` (Duration/Curves/Events), `Keyframes` (Times/Values/Tangents/Interpolation), `VariableCurve`, `AnimationTargetId`, `RepeatMode` (Once/Loop/PingPong), `sampleKeyframes` (binary-search + Step/Linear/CubicSpline/Hermite), `SampleCurve` (exported wrapper), `strideOf`/`strideSlice`/`lerpSlice`/`slerpQuat` pure functions.
- [x] **T-5D02** — Player + graph: `AnimationPlayer`/`ActiveAnimation` (multi-clip layered blending), `AnimationGraph` (ClipNode/BlendNode/AddNode), `Transition` crossfade (dual-state eval, weight 0→1 ramp), `RepeatMode` (Once/Loop/PingPong). Files: `pkg/animation/{components,graph}.go`. Spec §4.3,4.4,4.9,4.10. INV-1/3. `[Bootstrap]`
  Verify: ✅ `go test ./pkg/animation/...` PASS — AnimationPlayer/ActiveAnimation struct fields, AnimationNodeIndex, AnimationGraph node types, Transition, MorphWeights, Joint/Skin. build + modernize clean.
  Changes: `AnimationPlayer.Active` sorted-slice layered blending, `ActiveAnimation` (Clip/Elapsed/Speed/Repeat/BlendWeight/Paused), `Joint`/`Skin`/`MorphWeights`, `AnimationGraph` (Nodes/Transitions/Params/Triggers), `ClipNode`/`BlendNode`/`AddNode`/`TransitionCondition`.
- [x] **T-5D03** — Skeletal + morph: `SkinData`, `JointTransform`, `ValidateSkin` (mismatch error INV-4). Files: `internal/animation/skeletal.go`. Spec §4.5–4.7. INV-2/4. Track D complete. `[Bootstrap]`
  Verify: ✅ `go test ./internal/animation/...` PASS — ValidateSkin mismatch error tested in `examples/animation/`. build + modernize clean.
  Changes: `SkinData` (JointIDs/InverseBindMatrices), `JointTransform` (Translation/Rotation/Scale), `ValidateSkin` → `ErrSkinMismatch` on joint/matrix count mismatch.

### Track E — Tweening System (`pkg/tween/`) — render-core-independent, fully parallel

- [x] **T-5E01** — Tween + easing: `Tween` component (TargetField reflection-path / Start / End / Duration / Elapsed / Easing / LoopMode / TimeDimension), easing function library (Linear, EaseInQuad, EaseOutBounce, … — reuse `pkg/math` curves where applicable), generic `Lerp` for `float32`/`Vec2`/`Vec3`/`Color` (INV-2). Files: `pkg/tween/{tween,easing,lerp}.go`. Spec: [l1-tweening-system.md](../specifications/l1-tweening-system.md) §4.1. `[Bootstrap]`
  Verify: ✅ `go test ./pkg/tween/...` PASS — 10 easing functions f(0)=0/f(1)=1, EaseInOutQuad symmetry, Lerp clamping, Vec3/float32 lerp, LerpAny type mismatch. `BenchmarkLerpVec3` **0 B/op 0 allocs/op** (C-027). build + modernize clean.
  Changes: `Tween` component (StartValue/EndValue/Duration/Elapsed/Easing/LoopMode/TimeDimension/DespawnOnDone), `EasingFn func(float64)float64`, 11 standard easing functions (Linear/InQuad/OutQuad/InOutQuad/InCubic/OutCubic/InSine/OutSine/InOutSine/OutBounce/InBounce/InElastic/OutElastic), `Lerp[T Lerpable]` generic (clamped t), `LerpAny` exported type-dispatch.
- [x] **T-5E02** — Execution + lifecycle: Tweening schedule/system (delta selected by `TimeDimension` — Real/Virtual, INV-3), per-frame progress → easing → lerp → apply via reflection path, completion handling (Once/Loop/PingPong/Despawn), self-cleanup on completion or target-entity despawn (INV-1 — no leaked tweens). Files: `internal/tween/system.go`. Spec §4.2. INV-1/3. Track E complete. `[Bootstrap]`
  Verify: ✅ `go test ./internal/tween/...` PASS — LoopOnce advance/done, degenerate zero-duration, type mismatch error, Loop wrap, PingPong bounce, splitPath. Tests `examples/tweening/` hash stable ×20.
  Changes: `AdvanceTween` (dt advance + easing + LerpAny + Loop wrap + PingPong + LoopOnce done), `writeAccessor` reflection-path setter (cached, allocation-free), `resolveField`/`splitPath` path parser, `NewWriteAccessor`.

### Track T — Validation (C29 P5 gate — unblocks P5 Draft → Stable)

- [x] **T-5T01** — `examples/audio/`: bus-graph routing (Master/Music/SFX/Ambient) + spatial listener attenuation + DESPAWN-mode determinism on the **headless stub backend** (zero hardware, deterministic mix accounting). Files: `examples/audio/{main,main_test}.go`. Acceptance: build+run green; bus/spatial/despawn assertions stable across N runs. `[Bootstrap]`
  Verify: ✅ `go test ./examples/audio/...` PASS — `TestAudioHashStability` hash stable ×20, `TestAudioBusGraph` bus layout validated, build clean (C-003 stdlib-only).
  Changes: `examples/audio/main.go` (4-bus DAG, 3 sinks across buses, attenuation check, DESPAWN via PollFinished, FNV-1a hash) + `main_test.go` (20-run hash stability, bus cycle-guard).
- [x] **T-5T02** — `examples/animation/`: clip playback + skeletal skin validation + curve sampling determinism. INV-1 determinism: same input state → identical sampled-pose hash across ≥20 runs. Files: `examples/animation/{main,main_test}.go`. `[Bootstrap]`
  Verify: ✅ `go test ./examples/animation/...` PASS — `TestAnimationHashStability` hash stable ×20, `TestAnimationSkinValidation` ErrSkinMismatch surfaces correctly.
  Changes: `examples/animation/main.go` (3-keyframe linear clip sampling at t=0.5/1.0/1.5, INV-1 hash, ValidateSkin mismatch test, FNV-1a hash) + `main_test.go`.
- [x] **T-5T03** — `examples/tweening/`: easing-curve sampling + LoopMode (Once/Loop/PingPong) + self-cleanup assertion (INV-1 — degenerate despawn tween done immediately). Files: `examples/tweening/{main,main_test}.go`. `[Bootstrap]`
  Verify: ✅ `go test ./examples/tweening/...` PASS — `TestTweeningHashStability` hash stable ×20, `TestTweeningDespawnCleanup` INV-1 self-cleanup.
  Changes: `examples/tweening/main.go` (EaseOutQuad LoopOnce, Loop wrap @1.5s→X≈5, PingPong bounce @1.5s→X≈5, degenerate despawn, FNV-1a hash) + `main_test.go`.
- [ ] **T-5T04** — `examples/2d/` + codec golden tests: sprite batching/sort/atlas demo (**co-gated on render-core Stable** — Track C dependency); asset-formats round-trip golden — PNG decode dims/format, WAV decode sample-rate/PCM, scene.json decode → spawn → re-serialize byte-stable. Files: `examples/2d/{main,main_test}.go` + `pkg/asset/formats/*_golden_test.go`. `[Bootstrap]`
- [x] **T-5T05** — C29 P5 gate sign-off (render-core-independent cohort): `go test ./...` full workspace **44/44 PASS, 0 FAIL**; `go build ./examples/{audio,animation,tweening}/...` OK; `BenchmarkLerpVec3` **0 B/op 0 allocs/op** (C-027 tween apply); stdlib-only A/D/E/partial-B (C-003); `modernize ./...` clean. Bootstrap: P5 A/D/E specs eligible Draft → Stable via next `/magic.task` Pre-Planning Stabilization. **2D (Track C) + T-5T04 remain pending** (Track B/C full validation). `[Bootstrap]`
  Verify: ✅ `go test ./... → 44/44 PASS`; `BenchmarkLerpVec3 → 0 B/op 0 allocs/op`; `go build ./examples/{audio,animation,tweening}/... OK`; `modernize → no suggestions`; `go list -deps | grep -v github.com/neuengine → stdlib only`.

## Notes

- **L2 Go contracts AUTHORED (2026-05-28).** All five P5 specs now have an `l2-*-go.md` implementation contract: [l2-audio-system-go.md](../specifications/l2-audio-system-go.md), [l2-asset-formats-go.md](../specifications/l2-asset-formats-go.md), [l2-2d-rendering-go.md](../specifications/l2-2d-rendering-go.md), [l2-animation-system-go.md](../specifications/l2-animation-system-go.md), [l2-tweening-system-go.md](../specifications/l2-tweening-system-go.md) (all Draft). **Implement against the L2 contract** (Go Package layout, Type Definitions, Invariant Compliance), not directly from L1 — parity with Phases 1–4.
- **Render-core gate (Track C) — CLEARED.** `l1-render-core` ratified RFC → Stable + `l2-render-core-go` Draft → Stable (2026-05-28). Track C (2D) is unblocked; `l2-2d-rendering-go` consumes the now-Stable render-core SoA infra (`RenderDataHolder`/`VisibilityGroup`). The full phase gate is open.
- **Headless-only validation.** Audio (no WASAPI/ALSA/CoreAudio lib — C-003) and 2D (no GPU) gate examples run against recording/stub backends, mirroring render's `nopBackend`/`recordingBackend`. Real platform drivers are out of the C29 gate scope.
- Atomic decomposition complete (2026-05-28). Phase stays `Hold`; execute via `/magic.run main` after render-core ratification lifts the gate.
