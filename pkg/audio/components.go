package audio

import "github.com/neuengine/neu/pkg/asset"

// AudioPlayer is an ECS component that requests playback of an AudioSource.
// The audio system creates an AudioSink automatically when playback begins.
// Removing this component stops playback and drops the sink (INV-2).
type AudioPlayer struct {
	Source   asset.Handle[AudioSource]
	Settings PlaybackSettings
}

// SpatialListener is a marker component placed on exactly one entity
// (typically the camera or player). The audio system queries its GlobalTransform
// each frame to compute relative positions for all SpatialAudioSink entities.
// Ties (multiple active listeners) are resolved by lowest EntityID.
type SpatialListener struct{}

// GlobalVolume is a world resource holding the master volume applied
// multiplicatively to every sink output (INV-3). Defaults to 1.0.
type GlobalVolume struct{ Value float32 }
