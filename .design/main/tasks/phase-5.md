---
phase: 5
name: "Content Systems"
status: Hold
subsystem: "pkg/audio, pkg/asset/formats, pkg/render/sprite, pkg/animation, pkg/tween"
requires:
  - "Phase 4 Render Core Stable (Track C only — render-core is RFC)"
  - "Phase 4 Mesh & Image / Camera & Visibility Stable"
  - "Phase 3 Asset System / Math System Stable"
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
hold_reason: "Unfreezes after Phase 4 Render Core (RFC) reaches Stable. Tracks A/B/D/E are render-core-independent and only gate on the now-Stable Mesh/Camera/Asset/Math specs; Track C (2D) is the sole render-core-coupled track."
---

# Stage 5 Tasks — Content Systems

**Phase:** 5
**Status:** Hold
**Strategic Goal:** End-user content runtime: audio, format codecs, 2D rendering, animation, tweening.

## High-Level Checklist

- [ ] [T-5A] Audio: backend/driver abstraction, bus graph, spatial audio, ECS pipeline. ([l1-audio-system.md](../specifications/l1-audio-system.md))
- [ ] [T-5B] Asset format codecs: images, audio, fonts, glTF, scene.json. ([l1-asset-formats.md](../specifications/l1-asset-formats.md))
- [ ] [T-5C] 2D pipeline: sprites, 9-slice, text, sort+batch as RenderFeature. ([l1-2d-rendering.md](../specifications/l1-2d-rendering.md)) **[gated on render-core Stable]**
- [ ] [T-5D] Animation: clips, curves, graph, skeletal, morph, events. ([l1-animation-system.md](../specifications/l1-animation-system.md))
- [ ] [T-5E] Tweening: tween component, easing library, execution+lifecycle. ([l1-tweening-system.md](../specifications/l1-tweening-system.md))
- [ ] [T-5T] Validation: codec round-trip golden tests, animation determinism, audio/2D headless-stub examples.

## Atomic Decomposition

> Decomposed 2026-05-28 (explicit `/magic.task main "decompose phase-5"` intent — full atomic decomposition requested). 18 atomic tasks across Tracks A–E + T.
> **Execution Mode:** Parallel (C3). **Critical path:** {A (Audio) ‖ D (Animation)} → B (Asset Formats — glTF/audio loaders consume the AnimationClip + AudioSource asset types). Track E (Tweening) is fully independent. **Track C (2D) is off the critical path and externally gated** on render-core RFC→Stable ratification. Track T joins all on the C29 P5 gate.
> **Status note:** Phase remains `Hold` until render-core reaches Stable. Decomposition is authored ahead of execution (precedent: Phase 6 decomposed while Hold). Execute via `/magic.run main` only after the hold lifts.

### Track A — Audio System (`pkg/audio/`) — render-core-independent

- [ ] **T-5A01** — Core data: `AudioSource` asset (decoded PCM), `AudioPlayer` + `PlaybackSettings` (mode/volume/speed/spatial), `AudioSink`/`SpatialAudioSink` control handles, `SpatialListener` marker, `GlobalVolume` resource. Pure-data components (ECS §2.6). Files: `pkg/audio/{source,components,sink}.go`. Spec: [l1-audio-system.md](../specifications/l1-audio-system.md) §4.1–4.6. INV-1/2/3. `[Bootstrap]`
- [ ] **T-5A02** — Backend + server: `AudioBackend` interface + default **headless stub backend** (zero hardware, C-003), `AudioDriver` platform abstraction, `AudioServer` (bus-graph owner), `AudioBusLayout`/`AudioBus` DAG (Master root + routing + ducking), `AudioEffect`/`AudioEffectInstance` factory/instance split (shared config, per-bus state). Files: `pkg/audio/{backend,driver,server,bus,effect}.go`. Spec §4.7,4.9–4.11. `[Bootstrap]`
- [ ] **T-5A03** — System pipeline: `audio_added`/`audio_control_sync`/`spatial_audio_update`/`audio_cleanup` systems, distance attenuation + stereo panning from `Transform`, `AudioProcessorData` associated data (§4.12 — `IsDataValid()` revalidation), `ServiceRegistry` audio service (§4.13), DESPAWN-mode exactly-once despawn. Files: `pkg/audio/{systems,spatial,service}.go` + `pkg/app/audioplugin.go`. Spec §4.8,4.12,4.13. INV-4/5. Track A complete. `[Bootstrap]`

### Track B — Asset Formats (`pkg/asset/formats/`) — joins {A, D}

- [ ] **T-5B01** — Image loaders: PNG/JPEG/BMP/TGA via stdlib `image`, HDR (Radiance RGBE) decode for env maps; DDS/KTX2 GPU-compressed **passthrough** (no CPU decode); loader registration with extension→loader uniqueness (INV-1), disabled-format clear error (INV-3), build-tag modular inclusion. Files: `pkg/asset/formats/image/{png,jpeg,hdr,compressed,register}.go`. Spec: [l1-asset-formats.md](../specifications/l1-asset-formats.md) §4.2,4.6. (Mesh & Image now Stable → `Image` asset type available.) `[Bootstrap]`
- [ ] **T-5B02** — Audio + Font loaders: WAV (stdlib PCM) + OGG/FLAC/MP3 decoder interfaces → `AudioSource`; `.ttf`/`.otf` Font loader (glyph outlines + metrics, rasterization deferred to text pipeline). Files: `pkg/asset/formats/{audio,font}/*.go`. **Requires T-5A01** (`AudioSource` asset type). Spec §4.3,4.4. `[Bootstrap]`
- [ ] **T-5B03** — glTF 2.0 + scene.json: `.gltf`/`.glb` → multi-asset (Scene/Mesh/Material/Image/Animation/Skin/MorphTarget) with `GltfAssetLabel` deterministic sub-asset addressing (INV-4), KHR extensions (`unlit`/`texture_transform`/`lights_punctual`; `draco` behind build tag); `.scene.json` reflection loader → `DynamicScene` (reuses Phase 3 scene codec). No partial assets (INV-2). Files: `pkg/asset/formats/gltf/*.go` + `pkg/asset/formats/scene/*.go`. **Requires T-5D01** (`AnimationClip` type) + Stable Mesh/Material/Scene. Spec §4.1,4.5. Track B complete. `[Bootstrap]`

### Track C — 2D Rendering (`pkg/render/sprite/` + `internal/render/sprite2d/`) — **GATED on render-core RFC→Stable**

- [ ] **T-5C01** — 2D data + camera: `Sprite` (image/color/flip/anchor/custom_size/rect), `TextureSlicer` (9-slice borders), `SpriteMesh` (custom 2D geometry), `Text2D` + `TextPipeline`/`FontAtlas`, 2D `OrthographicProjection` camera with pixel-perfect integer-ratio snapping. Files: `pkg/render/sprite/{sprite,slice,text2d,camera2d}.go`. Spec: [l1-2d-rendering.md](../specifications/l1-2d-rendering.md) §4.1–4.4,4.8. INV-1/4. (Camera & Visibility now Stable.) `[Bootstrap]`
- [ ] **T-5C02** — `Sprite2DFeature`: `RenderFeature` (Collect/Extract/Prepare/Draw) **reusing render-core `RenderDataHolder` + `VisibilityGroup`**; deterministic sort (Z → Y → entity order, INV-2), texture-atlas batching of adjacent same-texture/material/blend sprites (INV-3), sprite picking (ray-rect, top-most wins), `SpriteProcessorData` associated data (version-gated regen). Files: `internal/render/sprite2d/{feature,sort,batch,pick}.go`. Spec §4.5–4.7,4.9–4.11. **Blocked: requires `l1-render-core` Stable** (Sprite2DFeature is a RenderFeature consuming render-core SoA infra). `[Bootstrap]`

### Track D — Animation System (`pkg/animation/`) — render-core-independent, largest track

- [ ] **T-5D01** — Clips + curves: `AnimationClip` asset, `VariableCurve`/`Keyframes` (Step/Linear/CubicSpline — reuse `pkg/math` curves + quaternion slerp), `AnimationTargetId` (EntityPath + PropertyPath resolved via type-registry reflection), typed write-accessor cache at clip-load time. Deterministic evaluation (INV-1). Files: `pkg/animation/{clip,curve,target}.go`. Spec: [l1-animation-system.md](../specifications/l1-animation-system.md) §4.1,4.2,4.8. `[Bootstrap]`
- [ ] **T-5D02** — Player + graph: `AnimationPlayer`/`ActiveAnimation` (multi-clip layered blending), `AnimationGraph` (ClipNode/BlendNode/AddNode), `Transition` crossfade (dual-state eval, weight 0→1 ramp), `RepeatMode` (Once/Loop/PingPong), parallel evaluation partitioned by root entity on the task pool (INV-1 determinism preserved). Files: `pkg/animation/{player,graph,blend,eval}.go`. Spec §4.3,4.4,4.9,4.10. INV-1/3. `[Bootstrap]`
- [ ] **T-5D03** — Skeletal + morph + events: `Joint`/`Skin` (inverse-bind matrices), skeletal local-`Transform` write + hierarchy propagation (INV-2 — runs PostUpdate before transform propagation per INV-3), `MorphWeights` blend shapes, `AnimationEvent` interval `[prev,cur]` exactly-once dispatch via event system, partial-animation silent skip for missing joints/targets (INV-4). Files: `pkg/animation/{skeletal,morph,event}.go` + `pkg/app/animationplugin.go`. Spec §4.5–4.7. INV-2/4. Track D complete. `[Bootstrap]`

### Track E — Tweening System (`pkg/tween/`) — render-core-independent, fully parallel

- [ ] **T-5E01** — Tween + easing: `Tween` component (TargetField reflection-path / Start / End / Duration / Elapsed / Easing / LoopMode / TimeDimension), easing function library (Linear, EaseInQuad, EaseOutBounce, … — reuse `pkg/math` curves where applicable), generic `Lerp` for `float32`/`Vec2`/`Vec3`/`Color` (INV-2). Files: `pkg/tween/{tween,easing,lerp}.go`. Spec: [l1-tweening-system.md](../specifications/l1-tweening-system.md) §4.1. `[Bootstrap]`
- [ ] **T-5E02** — Execution + lifecycle: Tweening schedule/system (delta selected by `TimeDimension` — Real/Virtual, INV-3), per-frame progress → easing → lerp → apply via reflection path, completion handling (Once/Loop/PingPong/Despawn), self-cleanup on completion or target-entity despawn (INV-1 — no leaked tweens). Files: `pkg/tween/{system,lifecycle}.go` + `pkg/app/tweenplugin.go`. Spec §4.2. INV-1/3. Track E complete. `[Bootstrap]`

### Track T — Validation (C29 P5 gate — unblocks P5 Draft → Stable)

- [ ] **T-5T01** — `examples/audio/`: bus-graph routing (Master/Music/SFX/Ambient) + spatial listener attenuation + DESPAWN-mode determinism on the **headless stub backend** (zero hardware, deterministic mix accounting). Files: `examples/audio/{main,main_test}.go`. Acceptance: build+run green; bus/spatial/despawn assertions stable across N runs. `[Bootstrap]`
- [ ] **T-5T02** — `examples/animation/`: clip playback + graph blend transition + skeletal joint propagation through ChildOf. INV-1 determinism: same input state → identical sampled-pose hash across ≥20 runs. Files: `examples/animation/{main,main_test}.go`. `[Bootstrap]`
- [ ] **T-5T03** — `examples/tweening/`: easing-curve sampling + LoopMode (Once/Loop/PingPong) + Virtual vs Real `TimeDimension`. INV-1 self-cleanup: despawn target mid-tween → no dangling tween (leak assertion). Files: `examples/tweening/{main,main_test}.go`. `[Bootstrap]`
- [ ] **T-5T04** — `examples/2d/` + codec golden tests: sprite batching/sort/atlas demo (**co-gated on render-core Stable** — Track C dependency); asset-formats round-trip golden — PNG decode dims/format, WAV decode sample-rate/PCM, scene.json decode → spawn → re-serialize byte-stable. Files: `examples/2d/{main,main_test}.go` + `pkg/asset/formats/*_golden_test.go`. `[Bootstrap]`
- [ ] **T-5T05** — C29 P5 gate sign-off: `go test -race ./...` full workspace green; `go build ./examples/{audio,animation,tweening,2d}/...`; hot-path benchmarks **0 B/op 0 allocs/op** (tween apply, sprite batch-key, animation curve sample); stdlib-only (C-003); `modernize ./...` clean. Promotes render-core-independent P5 cohort (A/B/D/E specs) Draft → Stable via next Pre-Planning Stabilization; **2D (Track C) co-gated on render-core Stable**. `[Bootstrap]`

## Notes

- **L2 Go contracts are NOT yet authored** for any Phase 5 spec — all five are `concept` (L1) layer only. Phases 1–4 each implemented against an `l2-*-go.md` contract. Recommended pre-execution step: author `l2-{audio,asset-formats,2d-rendering,animation,tweening}-go.md` via `/magic.spec` so tasks implement against a Go-level contract rather than directly from L1. Under Bootstrap the decomposition references L1 directly; revisit before `/magic.run`.
- **Render-core gate (Track C).** `l1-render-core` is RFC (held out of the 2026-05-28 P4 stabilization by user choice). Track C (2D) and the 2D portion of the C29 P5 gate (T-5T04) cannot proceed until render-core ratifies RFC → Stable via `/magic.spec`. Tracks A/B/D/E are render-core-independent and unblock as soon as the phase hold lifts.
- **Headless-only validation.** Audio (no WASAPI/ALSA/CoreAudio lib — C-003) and 2D (no GPU) gate examples run against recording/stub backends, mirroring render's `nopBackend`/`recordingBackend`. Real platform drivers are out of the C29 gate scope.
- Atomic decomposition complete (2026-05-28). Phase stays `Hold`; execute via `/magic.run main` after render-core ratification lifts the gate.
