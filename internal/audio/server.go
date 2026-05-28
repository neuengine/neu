package audio

import (
	"sync"

	pkgaudio "github.com/neuengine/neu/pkg/audio"
)

// AudioServer owns the bus graph and orchestrates mixing.
// It sits between the ECS systems (frontend) and the AudioDriver (hardware).
// All public methods are goroutine-safe.
type AudioServer struct {
	mu      sync.Mutex
	driver  pkgaudio.AudioDriver
	backend pkgaudio.AudioBackend
	layout  pkgaudio.AudioBusLayout
}

// NewAudioServer creates an AudioServer with the given driver and backend.
// The headless driver/backend pair is used by default (C-003, C29 gate).
func NewAudioServer(driver pkgaudio.AudioDriver, backend pkgaudio.AudioBackend) *AudioServer {
	return &AudioServer{driver: driver, backend: backend}
}

// SetLayout replaces the bus layout. Returns ErrAudioBusCycle if the new
// layout contains a cycle.
func (s *AudioServer) SetLayout(layout pkgaudio.AudioBusLayout) error {
	if err := layout.ValidateDAG(); err != nil {
		return err
	}
	s.mu.Lock()
	s.layout = layout
	s.mu.Unlock()
	return nil
}

// Layout returns the current bus layout (copy).
func (s *AudioServer) Layout() pkgaudio.AudioBusLayout {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.layout
}

// Backend returns the underlying AudioBackend.
func (s *AudioServer) Backend() pkgaudio.AudioBackend {
	return s.backend
}

// CreateSink creates a sink for the given source via the backend.
func (s *AudioServer) CreateSink(settings pkgaudio.SinkSettings, src *pkgaudio.AudioSource) pkgaudio.SinkHandle {
	return s.backend.CreateSink(settings, src)
}

// UpdateSink propagates control changes (volume, pause, etc.) to the backend.
func (s *AudioServer) UpdateSink(h pkgaudio.SinkHandle, p pkgaudio.SinkParams) {
	s.backend.UpdateSink(h, p)
}

// DropSink stops a sink and releases backend resources.
func (s *AudioServer) DropSink(h pkgaudio.SinkHandle) {
	s.backend.DropSink(h)
}

// PollFinished returns sinks that completed since the last poll.
func (s *AudioServer) PollFinished() []pkgaudio.SinkHandle {
	return s.backend.PollFinished()
}
