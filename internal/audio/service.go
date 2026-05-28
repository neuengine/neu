package audio

import (
	"sync"

	pkgaudio "github.com/neuengine/neu/pkg/audio"
	"github.com/neuengine/neu/pkg/asset"
)

// ServiceRegistry provides cross-cutting audio access without direct coupling.
// A physics or UI system can play a sound without importing the audio package
// by going through the service interface (L1 §4.13).
type ServiceRegistry struct {
	mu      sync.RWMutex
	backend pkgaudio.AudioBackend
}

// NewServiceRegistry creates a registry backed by the given AudioBackend.
// Pass nil to create a no-op registry (headless builds).
func NewServiceRegistry(backend pkgaudio.AudioBackend) *ServiceRegistry {
	return &ServiceRegistry{backend: backend}
}

// PlayOneShot creates a one-shot sink for the given source handle.
// If the backend is nil or the source handle is weak, the call is a no-op.
func (r *ServiceRegistry) PlayOneShot(src asset.Handle[pkgaudio.AudioSource], volume float32) {
	r.mu.RLock()
	b := r.backend
	r.mu.RUnlock()
	if b == nil || src.IsWeak() {
		return
	}
	settings := pkgaudio.SinkSettings{
		Mode:   pkgaudio.PlaybackOnce,
		Volume: volume,
		Bus:    "SFX",
	}
	b.CreateSink(settings, nil)
}
