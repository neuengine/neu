package audio

import "testing"

func TestPlaybackSettings_Defaults(t *testing.T) {
	s := DefaultPlaybackSettings()
	if s.Volume != 1 {
		t.Errorf("Volume = %v, want 1", s.Volume)
	}
	if s.Speed != 1 {
		t.Errorf("Speed = %v, want 1", s.Speed)
	}
	if s.Mode != PlaybackOnce {
		t.Errorf("Mode = %v, want PlaybackOnce", s.Mode)
	}
	if s.Spatial {
		t.Error("Spatial should default to false")
	}
}

func TestAudioSink_Handle_NoExportedConstructor(t *testing.T) {
	// AudioSink can be constructed by zero-value; the system sets the handle.
	var s AudioSink
	if !s.handle.IsNil() {
		t.Error("zero AudioSink should have nil handle")
	}
}

func TestSinkHandle_IsNil(t *testing.T) {
	var h SinkHandle
	if !h.IsNil() {
		t.Error("zero SinkHandle should be nil")
	}
	h = SinkHandle(1)
	if h.IsNil() {
		t.Error("non-zero SinkHandle should not be nil")
	}
}

func TestGlobalVolume_ZeroValue(t *testing.T) {
	var gv GlobalVolume
	if gv.Value != 0 {
		t.Errorf("zero GlobalVolume.Value = %v, want 0", gv.Value)
	}
}

func TestAudioSource_ZeroValue(t *testing.T) {
	var src AudioSource
	if src.SampleRate != 0 || src.Channels != 0 || len(src.Samples) != 0 {
		t.Error("zero AudioSource should have empty fields")
	}
}

func TestSpatialAudioSink_EmbedsSink(t *testing.T) {
	var s SpatialAudioSink
	s.Volume = 0.5
	if s.Volume != 0.5 {
		t.Error("SpatialAudioSink should embed AudioSink by value")
	}
}
