# Audio System
**Version:** 0.3.0
**Status:** Stable
**Layer:** concept

## Overview
The audio system provides component-driven sound playback integrated with the ECS world. Each audio source is an asset; playback is controlled through components attached to entities. Spatial audio positions sound relative to a listener entity in 3D space. A backend abstraction allows the actual mixing and output to be handled by pluggable audio engines.

## Related Specifications
- [Asset System](l1-asset-system.md)
- [App Framework](l1-app-framework.md)
- [Platform System](l1-platform-system.md) — AudioDriver selection per platform

## 1. Motivation
Games require both global (music, UI sounds) and positional (footsteps, explosions) audio. Representing playback as ECS components means audio lifetime is tied to entity lifetime, enabling patterns like "despawn the bullet entity and its sound stops automatically." A backend interface avoids hard-coupling to any single audio library.

## 2. Constraints & Assumptions
- Audio mixing runs on a dedicated thread managed by the backend; the ECS side only issues commands.
- Asset loading is asynchronous — an `AudioPlayer` referencing an unloaded asset produces silence until the asset is ready.
- Spatial audio requires a `Transform` component on both the source entity and the listener entity.
- At most one `SpatialListener` entity may be active at a time.
- The system supports common sample rates (22 050, 44 100, 48 000 Hz) but does not resample at runtime — the backend is responsible for sample-rate conversion.

## 3. Core Invariants
1. An `AudioSink` is created automatically when an `AudioPlayer` begins playback; user code never constructs sinks directly.
2. Removing the `AudioPlayer` component from an entity stops and drops its associated sink.
3. `GlobalVolume` is applied multiplicatively to every sink's individual volume.
4. Spatial attenuation is computed every frame from the current `Transform` of source and listener.
5. A sink in the `DESPAWN` playback mode must trigger entity despawn exactly once upon completion.

## 4. Detailed Design

### 4.1 AudioSource Asset
An `AudioSource` holds decoded PCM sample data loaded from disk. Supported container formats are WAV, OGG/Vorbis, FLAC, and MP3. The asset system resolves format via file extension and delegates to the matching `AssetLoader`.

### 4.2 AudioPlayer Component
Attached to an entity to request playback of an `AudioSource` asset handle.

Fields (pseudo-code):
```
AudioPlayer {
    source:   Handle<AudioSource>
    settings: PlaybackSettings
}
```

### 4.3 PlaybackSettings
```
PlaybackSettings {
    mode:    PlaybackMode   // LOOP | ONCE | DESPAWN
    volume:  f32            // 0.0 .. 1.0, default 1.0
    speed:   f32            // playback rate multiplier, default 1.0
    spatial: bool           // if true, requires Transform
}
```

### 4.4 AudioSink / SpatialAudioSink
`AudioSink` is a control handle inserted by the audio system after playback begins. It exposes pause, resume, stop, and runtime volume/speed adjustment. When `spatial` is true, a `SpatialAudioSink` is inserted instead, adding distance-based attenuation and stereo panning derived from the relative position to the `SpatialListener`.

### 4.5 SpatialListener
A marker component placed on exactly one entity (typically the camera or player). The audio system queries its `Transform` each frame to compute relative positions for all spatial sources.

### 4.6 GlobalVolume Resource
A world resource holding a single `f32` master volume applied to all sinks before output. Defaults to 1.0.

### 4.7 AudioBackend Interface
```
AudioBackend {
    fn create_sink(settings, source_data) -> SinkHandle
    fn update_sink(handle, params)
    fn drop_sink(handle)
    fn set_master_volume(volume)
}
```
A default backend is provided. Alternative backends (e.g., web audio, console-specific) implement the same interface and are registered at app build time.

### 4.8 System Pipeline
1. **audio_added** — detects new `AudioPlayer` components, creates sinks via the backend.
2. **audio_control_sync** — propagates `AudioSink` mutations (pause/resume/volume) to the backend.
3. **spatial_audio_update** — recalculates attenuation and panning for every `SpatialAudioSink`.
4. **audio_cleanup** — detects removed `AudioPlayer` components or finished playback, drops sinks, and despawns entities in `DESPAWN` mode.

### 4.9 Audio Bus Graph

Instead of per-sound volume and effect settings, the audio system uses a named bus graph for scalable mixing:

```plaintext
AudioBus
  name:     StringName        // e.g., "Master", "Music", "SFX", "Dialogue", "Ambient"
  volume:   f32               // bus volume (0.0..1.0)
  mute:     bool
  solo:     bool
  effects:  []AudioEffectSlot // ordered effect chain
  output:   StringName        // target bus name ("Master" by default, "" for hardware out)

AudioBusLayout (Resource)
  buses: []AudioBus           // the full bus graph
```

Each `AudioPlayer` routes to a bus by name. The bus graph is a DAG rooted at the "Master" bus. Audio flows through effect chains on each bus before being routed to the parent bus. This enables:

- Adjusting all SFX volume with one knob without touching individual sources.
- Applying reverb to all ambient sounds via an effect on the "Ambient" bus.
- Audio ducking: lower the "Music" bus volume when the "Dialogue" bus has active sources.

### 4.10 Audio Effect Factory/Instance Split

Audio effects follow a factory pattern separating configuration from state:

```plaintext
AudioEffect (Resource — stateless configuration)
  // e.g., ReverbEffect { room_size: 0.8, damping: 0.5 }
  fn CreateInstance() -> AudioEffectInstance

AudioEffectInstance (stateful, per-bus)
  // holds internal buffers, delay lines, filter state
  fn Process(buffer: []f32, sample_rate: uint32)
```

The same `AudioEffect` resource (e.g., a reverb preset) can be used on multiple buses simultaneously, each with independent processing state. This avoids duplicating configuration while keeping per-bus state isolated. Effects are processed on the audio thread; the ECS side only manages effect assignment.

### 4.11 AudioDriver Abstraction

The audio system separates the logical audio server from the hardware interface:

```plaintext
AudioDriver (interface)
  Init(mix_rate: uint32, channels: uint32) -> error
  Start()
  GetMixRate() -> uint32
  Lock()          // acquire audio thread mutex
  Unlock()
  Close()

AudioServer (internal)
  driver:    AudioDriver
  bus_graph: AudioBusLayout
  // Manages the bus graph, spatial audio, and mixing
  // Calls driver.Lock()/Unlock() around buffer fills
```

`AudioDriver` is the platform-specific layer (ALSA, CoreAudio, WASAPI, WebAudio). `AudioServer` is the platform-independent layer that manages the bus graph, spatial computations, and effect processing. This separation allows porting to new platforms by implementing only the driver interface.

### 4.12 Associated Emitter Data

The audio system uses the associated data pattern (see [component-system.md §4.10](l1-component-system.md)) to cache runtime audio state per entity without polluting the `AudioPlayer` component:

```plaintext
AudioProcessorData
  emitter:            AudioEmitter         // platform audio object
  audio_component:    *AudioPlayer         // back-reference for validation
  transform:          *TransformComponent  // cached transform reference
  is_playing:         bool                 // runtime playback state
  pending_instances:  []SoundInstance      // queued but not yet playing
```

The audio system generates this data when an entity gains an `AudioPlayer` component and validates it each frame via `IsDataValid()` — if the entity's Transform changed (e.g., reparented), the emitter position is updated. On removal, all associated `SoundInstance` objects are stopped and released.

This keeps `AudioPlayer` a pure data component (source handle + settings) while the audio system owns all runtime state (emitters, instances, playback tracking). Multiple audio systems (e.g., spatial + UI) can each maintain independent associated data for the same component.

### 4.13 Audio Service Pattern

The audio engine registers as a service accessible to other subsystems without direct coupling:

```plaintext
ServiceRegistry
  Register[T](service: T)
  Get[T]() -> T

// During initialization:
services.Register[AudioSystem](audioSystem)

// From any processor that needs audio:
audio := services.Get[AudioSystem]()
audio.PlayOneShot(soundHandle, position)
```

This enables cross-cutting audio access — a physics collision system can trigger impact sounds, a UI system can play click feedback, and an animation system can sync sound cues to keyframes — all without importing the audio package directly. The service registry resolves at runtime, allowing optional audio (headless builds simply don't register the service).

## 5. Open Questions

- Should streaming playback (decode-on-the-fly) be a separate asset type or a flag on `AudioSource`?
- What distance attenuation model should be default — inverse distance, linear, or configurable per-source?
- How should audio ducking be automated — explicit ducking rules on buses, or a sidechain-compressor effect?

## Canonical References

<!-- MANDATORY for Stable status. List authoritative source files that downstream agents
     MUST read before implementing this spec. Use relative paths from project root.
     Stub state — fill with concrete files when implementation begins (Phase 1+). -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

<!-- Empty table = no canonical sources yet. Populate one row per authoritative file
     when implementation lands (Phase 1+). Stable promotion requires ≥1 row. -->

## Document History
| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-03-25 | Initial draft from architecture analysis |
| 0.2.0 | 2026-03-26 | Added audio bus graph, effect factory/instance split, AudioDriver abstraction |
| 0.3.0 | 2026-03-26 | Added associated emitter data pattern, audio service registry |
| — | — | Planned examples: `examples/audio/` |
