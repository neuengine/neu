// Package audio provides component-driven sound playback integrated with the
// ECS world. AudioSource assets hold decoded PCM; AudioPlayer components
// request playback; AudioSink handles are inserted by the audio system.
//
// Bootstrap: l2-audio-system-go Draft (C29 P5 gate open).
package audio

// PlaybackMode controls what happens when a clip finishes.
type PlaybackMode uint8

const (
	// PlaybackOnce plays the clip once then removes the AudioSink.
	PlaybackOnce PlaybackMode = iota
	// PlaybackLoop restarts the clip on completion.
	PlaybackLoop
	// PlaybackDespawn plays once then despawns the entity (INV-5).
	PlaybackDespawn
)

// PlaybackSettings are attached to an AudioPlayer to configure how a clip plays.
type PlaybackSettings struct {
	Mode    PlaybackMode
	Volume  float32 // 0..1; default 1.0
	Speed   float32 // playback rate multiplier; default 1.0
	Spatial bool    // true requires a Transform on the entity
	Bus     string  // target bus name; "" routes to "Master"
}

// DefaultPlaybackSettings returns sensible defaults (once, full volume, non-spatial).
func DefaultPlaybackSettings() PlaybackSettings {
	return PlaybackSettings{Mode: PlaybackOnce, Volume: 1, Speed: 1}
}

// AudioSource holds decoded PCM sample data. Instances are loaded asynchronously
// via the asset system; they are immutable once loaded.
type AudioSource struct {
	// Samples holds interleaved, normalised [-1, 1] PCM data.
	Samples    []float32
	SampleRate uint32
	Channels   uint16
}
