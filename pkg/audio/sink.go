package audio

// SinkHandle is an opaque backend handle returned by AudioBackend.CreateSink.
// Zero value (0) represents nil / no sink.
type SinkHandle uint64

// IsNil reports whether the handle is zero.
func (h SinkHandle) IsNil() bool { return h == 0 }

// AudioSink is inserted by the audio system when an AudioPlayer begins playback.
// User code must not construct this directly (INV-1 — the system owns sinks).
// Use it to inspect playback state or send runtime control (pause/resume/volume)
// by mutating its exported fields; the audio_control_sync system propagates them.
type AudioSink struct {
	handle  SinkHandle // unexported: system-owned
	Paused  bool
	Volume  float32 // 0..1 per-sink gain (multiplied by bus + GlobalVolume, INV-3)
	Speed   float32 // playback rate
	Stopped bool    // set by the system when the backend reports SinkFinished
}

// Handle returns the backend handle. Used internally by the audio system.
func (s *AudioSink) Handle() SinkHandle { return s.handle }

// SpatialAudioSink extends AudioSink with distance-based attenuation and
// stereo panning derived from the SpatialListener's GlobalTransform.
// Inserted instead of AudioSink when PlaybackSettings.Spatial is true.
type SpatialAudioSink struct {
	AudioSink
	// AttenuationModel selects how volume falls off with distance.
	AttenuationModel AttenuationModel
}

// AttenuationModel selects the distance attenuation formula.
type AttenuationModel uint8

const (
	AttenuationInverseDistance AttenuationModel = iota
	AttenuationLinear
	AttenuationNone
)
