package audio

// SinkSettings configures a sink at creation time.
type SinkSettings struct {
	Mode   PlaybackMode
	Volume float32
	Speed  float32
	Bus    string
}

// SinkParams carries runtime mutations applied by audio_control_sync.
type SinkParams struct {
	Volume  float32
	Speed   float32
	Paused  bool
	Stopped bool
}

// AudioBackend is the interface implemented by all audio mixing backends.
// The default implementation is a headless stub (zero hardware, C-003).
// Platform-specific backends (WASAPI, ALSA, CoreAudio, WebAudio) implement
// this interface behind a build-tag-gated driver.
type AudioBackend interface {
	// CreateSink creates a sink for the given source. Returns SinkHandle(0) on failure.
	CreateSink(s SinkSettings, src *AudioSource) SinkHandle
	// UpdateSink applies runtime parameter changes to an active sink.
	UpdateSink(h SinkHandle, p SinkParams)
	// DropSink stops playback and releases all resources for the sink.
	DropSink(h SinkHandle)
	// SetMasterVolume sets the global output gain (applied at the hardware tap).
	SetMasterVolume(v float32)
	// PollFinished returns handles of sinks that completed since the last poll.
	// Called once per frame by audio_cleanup; the backend clears its finished list.
	PollFinished() []SinkHandle
}
