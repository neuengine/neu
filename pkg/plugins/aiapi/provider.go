//go:build editor

package aiapi

import (
	"context"
	"errors"
	"sort"
	"sync"
)

// ErrUnsupported is returned by optional provider methods (e.g. Embeddings) a
// provider does not implement.
var ErrUnsupported = errors.New("aiapi: operation not supported by provider")

// Chunk is one streaming delta delivered to a chat consumer.
type Chunk struct {
	Delta string
	Final bool
}

// Provider translates between the canonical types and one HTTP provider's wire
// format (L1 §4.4). Adding a provider is one file implementing this interface.
type Provider interface {
	Name() string
	Complete(ctx context.Context, r CanonicalRequest) (CanonicalResponse, error)
	Stream(ctx context.Context, r CanonicalRequest, sink func(Chunk) error) error
	Embeddings(ctx context.Context, in []string) ([][]float32, error)
}

// providerRegistry holds compile-time-registered provider factories (the single
// extension point, L1 §4.12).
var (
	registryMu sync.RWMutex
	registry   = map[string]func(ProviderConfig) Provider{}
)

// RegisterProvider adds a provider factory under name. Called from init() of a
// provider file or a third-party extension package.
func RegisterProvider(name string, factory func(ProviderConfig) Provider) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[name] = factory
}

// Select builds the provider named in cfg.ActiveProvider from its config.
func Select(cfg Config) (Provider, error) {
	registryMu.RLock()
	factory, ok := registry[cfg.ActiveProvider]
	registryMu.RUnlock()
	if !ok {
		return nil, ErrUnsupportedProvider{Name: cfg.ActiveProvider}
	}
	pc, ok := cfg.Providers[cfg.ActiveProvider]
	if !ok {
		return nil, ErrUnsupportedProvider{Name: cfg.ActiveProvider}
	}
	return factory(pc), nil
}

// RegisteredProviders returns the sorted list of registered provider names.
func RegisteredProviders() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
