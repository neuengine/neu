package audio

import (
	"testing"

	pkgaudio "github.com/neuengine/neu/pkg/audio"
)

func TestHeadlessBackend_CreateAndDrop(t *testing.T) {
	b := NewHeadlessBackend()
	src := &pkgaudio.AudioSource{SampleRate: 44100, Channels: 2}
	settings := pkgaudio.SinkSettings{Volume: 1, Mode: pkgaudio.PlaybackOnce}

	h := b.CreateSink(settings, src)
	if h.IsNil() {
		t.Fatal("CreateSink returned nil handle")
	}
	if b.ActiveCount() != 1 {
		t.Errorf("ActiveCount = %d, want 1", b.ActiveCount())
	}

	b.DropSink(h)
	if b.ActiveCount() != 0 {
		t.Errorf("ActiveCount after drop = %d, want 0", b.ActiveCount())
	}
}

func TestHeadlessBackend_MasterVolume(t *testing.T) {
	b := NewHeadlessBackend()
	if b.MasterVolume() != 1 {
		t.Errorf("default master volume = %v, want 1", b.MasterVolume())
	}
	b.SetMasterVolume(0.5)
	if b.MasterVolume() != 0.5 {
		t.Errorf("master volume = %v, want 0.5", b.MasterVolume())
	}
}

func TestHeadlessBackend_PollFinished(t *testing.T) {
	b := NewHeadlessBackend()
	src := &pkgaudio.AudioSource{}
	h := b.CreateSink(pkgaudio.SinkSettings{Mode: pkgaudio.PlaybackOnce}, src)

	// No finished sinks yet.
	if finished := b.PollFinished(); len(finished) != 0 {
		t.Errorf("PollFinished = %v, want empty", finished)
	}

	b.FinishSink(h)
	finished := b.PollFinished()
	if len(finished) != 1 || finished[0] != h {
		t.Errorf("PollFinished = %v, want [%v]", finished, h)
	}
	// Second poll should be empty (cleared).
	if got := b.PollFinished(); len(got) != 0 {
		t.Errorf("second PollFinished = %v, want empty", got)
	}
}

func TestHeadlessBackend_MultipleHandlesDistinct(t *testing.T) {
	b := NewHeadlessBackend()
	src := &pkgaudio.AudioSource{}
	h1 := b.CreateSink(pkgaudio.SinkSettings{}, src)
	h2 := b.CreateSink(pkgaudio.SinkSettings{}, src)
	if h1 == h2 {
		t.Errorf("two sinks have the same handle %v", h1)
	}
}

func TestAudioBusLayout_ValidateDAG_NoCycle(t *testing.T) {
	layout := pkgaudio.AudioBusLayout{Buses: []pkgaudio.AudioBus{
		{Name: "Master", Output: ""},
		{Name: "Music", Output: "Master"},
		{Name: "SFX", Output: "Master"},
	}}
	if err := layout.ValidateDAG(); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestAudioBusLayout_ValidateDAG_Cycle(t *testing.T) {
	layout := pkgaudio.AudioBusLayout{Buses: []pkgaudio.AudioBus{
		{Name: "A", Output: "B"},
		{Name: "B", Output: "A"},
	}}
	if err := layout.ValidateDAG(); err == nil {
		t.Error("expected ErrAudioBusCycle, got nil")
	}
}

func TestAudioServer_SetLayout(t *testing.T) {
	driver := &HeadlessDriver{}
	_ = driver.Init(44100, 2)
	backend := NewHeadlessBackend()
	srv := NewAudioServer(driver, backend)

	layout := pkgaudio.AudioBusLayout{Buses: []pkgaudio.AudioBus{
		{Name: "Master", Output: ""},
		{Name: "Music", Output: "Master"},
	}}
	if err := srv.SetLayout(layout); err != nil {
		t.Errorf("SetLayout: %v", err)
	}
	if got := srv.Layout(); len(got.Buses) != 2 {
		t.Errorf("Layout buses = %d, want 2", len(got.Buses))
	}
}

func TestHeadlessDriver_Interface(t *testing.T) {
	var _ pkgaudio.AudioDriver = &HeadlessDriver{}
}

func TestHeadlessBackend_Interface(t *testing.T) {
	var _ pkgaudio.AudioBackend = &HeadlessBackend{}
}
