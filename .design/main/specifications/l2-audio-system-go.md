# Audio System — Go Implementation

**Version:** 0.1.0
**Status:** Draft
**Layer:** go
**Implements:** [l1-audio-system.md](l1-audio-system.md)

## Overview

Go-level design for the audio system: component-driven playback wired into the
ECS world, a named **bus graph** (DAG rooted at `Master`) for scalable mixing, a
factory/instance effect split, spatial attenuation from `Transform`, and a
two-layer backend split — a platform-independent `AudioServer` over a
platform-specific `AudioDriver`. The frontend (ECS systems) only issues
commands; all mixing runs on a backend-owned goroutine. Pure-data components
and the backend/driver interfaces are public (`pkg/audio/`); the server, bus
mixer, and systems are engine-private (`internal/audio/`).

## Related Specifications

- [l1-audio-system.md](l1-audio-system.md) — L1 concept specification (parent)
- [l2-asset-system-go.md](l2-asset-system-go.md) — `AudioSource` is an `asset.Handle[AudioSource]`; async load on the IOPool
- [l2-app-framework-go.md](l2-app-framework-go.md) — `AudioPlugin` registers systems + resources, owns backend lifecycle
- [l2-hierarchy-system-go.md](l2-hierarchy-system-go.md) — spatial sources/listener read `GlobalTransform`
- [l2-task-system-go.md](l2-task-system-go.md) — per-source spatial recompute via `ForBatched`
- [l1-platform-system.md](l1-platform-system.md) — concrete `AudioDriver` selected per platform

## 1. Motivation

Audio lifetime must follow entity lifetime (despawn a bullet, its sound stops).
Representing playback as ECS components gives that for free, while a backend
interface keeps the engine independent of any mixing library. The
`AudioServer`/`AudioDriver` split lets the mixing/bus logic be tested headlessly
(no hardware), satisfying CI and the C29 P5 validation gate with a stub driver.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: generics for the typed service registry; `slog` for diagnostics.
- **C-003**: the core (`pkg/audio`, `internal/audio`) is stdlib-only. The default
  headless driver is pure Go. A real OS driver (WASAPI/ALSA/CoreAudio/WebAudio)
  uses a single cgo/syscall binding behind `AudioDriver`, justified per backend in an ADR.
- **C-027**: per-frame command structs (sink create/update) and the mix scratch
  buffers are `sync.Pool`-recycled — the steady-state frame loop is allocation-free.
- **C-005**: `spatial_audio_update` runs in parallel over sources and MUST be
  `-race` clean; the main world is read-only during the audio systems' read phase.
- The mix thread is owned by the driver; the ECS side never blocks on it —
  control changes are queued and applied under `driver.Lock()`.
- At most one active `SpatialListener` entity; ties resolved by lowest `EntityID` (deterministic).
- No runtime resampling — the driver performs sample-rate conversion.

## 3. Core Invariants

> [!NOTE]
> See [l1-audio-system.md §3](l1-audio-system.md) for technology-agnostic
> invariants. Go-specific compliance is tabulated in §4.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **1**: `AudioSink` auto-created on playback; never user-constructed | `AudioSink` has no exported constructor; the `audioAdded` system detects new `AudioPlayer` components (via `Added[AudioPlayer]` change filter) and inserts the sink as a component. A user `world.Insert`-ing a raw `AudioSink` is a no-op — the system owns the `SinkHandle`. |
| **2**: Removing `AudioPlayer` stops + drops its sink | `audioCleanup` observes `RemovedComponents[AudioPlayer]`; for each removed entity it calls `backend.DropSink(handle)` and removes the `AudioSink` component in the same command buffer flush. |
| **3**: `GlobalVolume` applied multiplicatively to every sink | `GlobalVolume` is a world resource (`float32`); `audioControlSync` passes `bus.Volume * sink.Volume * globalVolume` to `backend.UpdateSink`. The driver applies the master gain once at the `Master` bus tap. |
| **4**: Spatial attenuation recomputed every frame | `spatialAudioUpdate` runs each frame in `PostUpdate` (after transform propagation) via `task.ForBatched` over all `SpatialAudioSink` entities, reading the listener's `GlobalTransform` once into a frame-local snapshot. |
| **5**: DESPAWN-mode sink despawns entity exactly once | `AudioProcessorData.despawnFired bool` guards a single `commands.Despawn(entity)` when the backend reports `SinkFinished` and `Mode == PlaybackDespawn`; idempotent on repeated finished reports. |

## Go Package

```
pkg/audio/
  source.go      // AudioSource asset (decoded PCM), PlaybackMode/PlaybackSettings
  components.go  // AudioPlayer, SpatialListener (marker), GlobalVolume resource — pure data
  sink.go        // AudioSink, SpatialAudioSink — control handles (no exported ctor)
  backend.go     // AudioBackend, SinkHandle, SinkSettings/SinkParams interfaces
  driver.go      // AudioDriver interface (platform layer)
  bus.go         // AudioBus, AudioBusLayout resource (DAG config)
  effect.go      // AudioEffect (factory) / AudioEffectInstance (stateful)
internal/audio/
  server.go      // AudioServer: bus graph, mix orchestration, driver Lock/Unlock
  mixer.go       // bus-graph traversal, effect-chain processing, ducking
  systems.go     // audioAdded / audioControlSync / spatialAudioUpdate / audioCleanup
  spatial.go     // distance attenuation + stereo panning math
  service.go     // ServiceRegistry[T], PlayOneShot cross-cutting entry
  headless.go    // default 0-hardware driver + recording backend (C29 gate)
```

`pkg/audio` is public, data + interfaces only. `internal/audio` is engine-private.

## Type Definitions

```go
// AudioSource — decoded PCM asset (loaded by l2-asset-formats-go loaders).
type AudioSource struct {
    Samples    []float32 // interleaved, normalized [-1,1]
    SampleRate uint32
    Channels   uint16
}

type PlaybackMode uint8 // PlaybackOnce, PlaybackLoop, PlaybackDespawn

type PlaybackSettings struct {
    Mode    PlaybackMode
    Volume  float32 // 0..1, default 1
    Speed   float32 // rate multiplier, default 1
    Spatial bool
    Bus     string  // target bus name; "" ⇒ "Master"
}

type AudioPlayer struct { // component — pure data
    Source   asset.Handle[AudioSource]
    Settings PlaybackSettings
}

type SpatialListener struct{}        // marker component
type GlobalVolume struct{ Value float32 } // resource, default 1

type SinkHandle uint64 // opaque backend handle (0 = nil)

type AudioBackend interface {
    CreateSink(s SinkSettings, src *AudioSource) SinkHandle
    UpdateSink(h SinkHandle, p SinkParams)
    DropSink(h SinkHandle)
    SetMasterVolume(v float32)
    PollFinished() []SinkHandle // sinks that completed since last poll (Once/Despawn)
}

type AudioDriver interface {
    Init(mixRate, channels uint32) error
    Start()
    MixRate() uint32
    Lock()   // acquire mix-thread mutex around buffer fills
    Unlock()
    Close() error
}

type AudioBus struct {
    Name    string
    Volume  float32
    Mute    bool
    Solo    bool
    Effects []AudioEffectInstance // ordered chain
    Output  string                // parent bus; "" ⇒ hardware out
}

type AudioBusLayout struct{ Buses []AudioBus } // resource — DAG rooted at "Master"

type AudioEffect interface{ CreateInstance() AudioEffectInstance } // stateless config
type AudioEffectInstance interface{ Process(buf []float32, sampleRate uint32) } // per-bus state
```

## Key Methods

```go
// Plugin wiring.
func NewAudioPlugin(backend AudioBackend) app.Plugin // registers systems + resources

// Server (internal — driven by systems).
func (s *AudioServer) RouteToBus(h SinkHandle, bus string)
func (s *AudioServer) MixFrame(out []float32) // bus-graph traversal under driver.Lock

// Cross-cutting service (L1 §4.13) — optional, nil in headless builds.
func (r *ServiceRegistry) PlayOneShot(src asset.Handle[AudioSource], pos math.Vec3)

// Spatial (L1 §4.4) — pure functions, table-tested.
func Attenuation(model AttenuationModel, dist, refDist, maxDist float32) float32
func StereoPan(listener, source math.Mat4) (left, right float32)
```

## Performance Strategy

- **Headless stub driver** (default): fills a discard buffer with deterministic
  mix accounting — zero hardware, zero external deps, used by the C29 P5 gate.
- **Parallel spatial recompute**: `task.ForBatched` over `SpatialAudioSink`s;
  each batch reads the shared listener snapshot (read-only) — `-race` clean.
- **`sync.Pool` mix scratch + command blocks** (C-027): recycled across frames.
- **Associated emitter data** (L1 §4.12): `AudioProcessorData` cached per entity,
  revalidated by `IsDataValid()` against the `GlobalTransform` change tick — an
  unchanged source only updates its gain, not its full emitter state.
- **Ducking**: sidechain read of the `Dialogue` bus RMS lowers the `Music` bus
  gain in `MixFrame` — one comparison per mixed frame, no per-sample branch.

## Error Handling

| Condition | Behavior |
| :--- | :--- |
| `AudioPlayer` references an unloaded asset | Silence until `LoadState == Loaded`; no error, sink created lazily on first ready frame |
| Bus name not found in `AudioBusLayout` | Routes to `Master`; warns once via `slog` (deduped by bus name) |
| Bus graph contains a cycle (Output chain) | `ErrAudioBusCycle` at `AudioServer` init — fail fast, before any mix |
| `SpatialAudioSink` without `Transform` on source or listener | Source treated as non-spatial (center pan, no attenuation); `slog.Debug` |
| Driver `Init` fails (no device) | Fall back to headless driver; `slog.Warn`; engine continues (headless builds expected) |

```go
var (
    ErrAudioBusCycle  = errors.New("audio: bus graph contains a cycle")
    ErrNoActiveDriver = errors.New("audio: no driver registered")
)
```

## Testing Strategy

- **Headless determinism (C29 gate)**: stub driver mix accounting is byte-stable
  across runs for a fixed scene (bus routing + spatial + despawn).
- **Bus graph**: cycle fixture ⇒ `ErrAudioBusCycle`; routing/ducking assertions.
- **INV-5 despawn-once**: feed repeated `SinkFinished` for a Despawn sink; assert
  exactly one `Despawn` command emitted.
- **Spatial math**: table-driven `Attenuation`/`StereoPan` vs reference values.
- **Race gate (C-005)**: `spatialAudioUpdate` over 10k sources under `-race`.
- **Benchmarks**: `BenchmarkMixFrame` (0 alloc/op steady), `BenchmarkSpatialUpdate`.

## 7. Drawbacks & Alternatives

- **Drawback**: the bus graph adds indirection vs per-sink volume.
  **Alternative**: flat per-sink mixing.
  **Decision**: L1 §4.9 mandates the bus graph for scalable mixing/ducking;
  flat mixing does not scale to grouped effects. Kept.
- **Drawback**: a headless default driver produces no sound out of the box.
  **Alternative**: ship a real driver by default.
  **Decision**: C-003 forbids a mandatory external audio dep in core; real
  drivers are opt-in plugins. Headless keeps CI green and core dep-free.

## Canonical References

<!-- MANDATORY for Stable status. Stub — populate when implementation lands
     (Phase 5 Track A). Stable promotion blocked until: (1) examples/audio/ validates
     the headless backend (T-5T01); (2) the C29 P5 gate (T-5T05) is green. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-28 | Initial L2 draft — Go translation of l1-audio-system v0.3.0. AudioBackend/AudioDriver split, bus-graph DAG, effect factory/instance, spatial systems, headless stub driver for the C29 gate. Authored ahead of Phase 5 Track A (`/magic.spec`). Draft — L1 parent Draft + no validating examples/audio/ yet. |
