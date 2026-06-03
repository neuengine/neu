//go:build editor

package aiapi

import (
	"context"
	"errors"
	"sync"
)

// ErrServiceNotReady is returned by the AI service before its provider and
// credentials have been resolved (during the plugin's Ready phase). Consumers
// degrade gracefully rather than fail hard (L1 §4.12 optional-access pattern).
var ErrServiceNotReady = errors.New("aiapi: service not ready")

// AIService is the cross-cutting AI-completion surface other engine systems use
// without importing provider internals — the L1 §4.12 service-registry pattern
// (mirrors internal/audio.ServiceRegistry). A consumer retrieves it as a World
// resource and degrades gracefully when the service is absent or not yet ready.
type AIService interface {
	// Complete performs a non-streaming completion against the active provider.
	Complete(ctx context.Context, r CanonicalRequest) (CanonicalResponse, error)
	// Stream performs a streaming completion, delivering each chunk to sink.
	Stream(ctx context.Context, r CanonicalRequest, sink func(Chunk) error) error
	// Ready reports whether a provider is resolved and the service can serve.
	Ready() bool
}

// ServiceRegistry is the resource the AI API plugin publishes so the assistant
// manager (or any system) can reach the active provider through a stable
// interface. It is the plugin's ServiceRegistry registration target (L1 §4.11
// Finish). The provider + ready flag are RWMutex-guarded so activation (Ready)
// and teardown (Cleanup) are safe against concurrent Complete/Stream calls.
type ServiceRegistry struct {
	provider Provider
	mu       sync.RWMutex
	ready    bool
}

// NewServiceRegistry returns an inactive registry (no provider, not ready).
func NewServiceRegistry() *ServiceRegistry { return &ServiceRegistry{} }

// activate installs the resolved provider and marks the service ready. Called by
// the plugin's Ready phase once provider + credentials resolve.
func (s *ServiceRegistry) activate(p Provider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.provider = p
	s.ready = true
}

// deactivate clears the provider and marks the service not-ready. Called by the
// plugin's Cleanup phase.
func (s *ServiceRegistry) deactivate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.provider = nil
	s.ready = false
}

// Ready reports whether the service has a resolved provider.
func (s *ServiceRegistry) Ready() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ready
}

// Complete forwards to the active provider, or returns ErrServiceNotReady.
func (s *ServiceRegistry) Complete(ctx context.Context, r CanonicalRequest) (CanonicalResponse, error) {
	p, err := s.active()
	if err != nil {
		return CanonicalResponse{}, err
	}
	return p.Complete(ctx, r)
}

// Stream forwards to the active provider, or returns ErrServiceNotReady.
func (s *ServiceRegistry) Stream(ctx context.Context, r CanonicalRequest, sink func(Chunk) error) error {
	p, err := s.active()
	if err != nil {
		return err
	}
	return p.Stream(ctx, r, sink)
}

// active returns the current provider under the read lock, or ErrServiceNotReady.
func (s *ServiceRegistry) active() (Provider, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.ready || s.provider == nil {
		return nil, ErrServiceNotReady
	}
	return s.provider, nil
}

var _ AIService = (*ServiceRegistry)(nil)
