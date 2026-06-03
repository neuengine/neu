//go:build editor

package aiapi

import (
	"context"
	"errors"
	"testing"

	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
	pkgplugin "github.com/neuengine/neu/pkg/plugin"
)

// fakeBuilder is a minimal appface.Builder recording resources a plugin sets.
type fakeBuilder struct {
	w         *world.World
	resources []any
}

func newFakeBuilder() *fakeBuilder { return &fakeBuilder{w: world.NewWorld()} }

func (b *fakeBuilder) World() *world.World                                        { return b.w }
func (b *fakeBuilder) AddSystem(_ string, _ scheduler.System) appface.Builder     { return b }
func (b *fakeBuilder) AddSystems(_ string, _ ...scheduler.System) appface.Builder { return b }
func (b *fakeBuilder) SetResource(v any) appface.Builder {
	b.resources = append(b.resources, v)
	return b
}
func (b *fakeBuilder) InitResource(any) appface.Builder               { return b }
func (b *fakeBuilder) AddPlugin(appface.Plugin) appface.Builder       { return b }
func (b *fakeBuilder) AddPlugins(appface.PluginGroup) appface.Builder { return b }

var _ appface.Builder = (*fakeBuilder)(nil)

// --- Manifest tests (dogfood the SDK parser/validator) -----------------------

func TestEmbeddedManifestValidates(t *testing.T) {
	t.Parallel()
	man, err := pkgplugin.ParseManifest(Manifest())
	if err != nil {
		t.Fatalf("ParseManifest(embedded): %v", err)
	}
	if err := man.Validate(); err != nil {
		t.Fatalf("embedded manifest fails SDK Validate: %v", err)
	}
	if string(man.ID) != PluginID {
		t.Errorf("manifest ID = %q, want %q", man.ID, PluginID)
	}
	if man.Mode != pkgplugin.ModeInProcess {
		t.Errorf("manifest mode = %v, want in-process", man.Mode)
	}
	if man.Entry.Factory != "New" {
		t.Errorf("manifest factory = %q, want New", man.Entry.Factory)
	}
	// network.outbound is required (INV-4 — the plugin is useless without it).
	found := false
	for _, c := range man.RequiredCaps {
		if c == pkgplugin.CapNetworkOutbound {
			found = true
		}
	}
	if !found {
		t.Error("manifest must require network.outbound (INV-4)")
	}
}

// --- Lifecycle tests ---------------------------------------------------------

func TestNewImplementsFullPlugin(t *testing.T) {
	t.Parallel()
	p := New()
	if _, ok := p.(appface.FullPlugin); !ok {
		t.Fatal("New() must return an appface.FullPlugin")
	}
}

func TestBuildPublishesService(t *testing.T) {
	t.Parallel()
	p := New().(*AIAPIPlugin)
	b := newFakeBuilder()
	p.Build(b)

	if len(b.resources) != 1 {
		t.Fatalf("Build should publish exactly one resource, got %d", len(b.resources))
	}
	if _, ok := b.resources[0].(*ServiceRegistry); !ok {
		t.Errorf("published resource = %T, want *ServiceRegistry", b.resources[0])
	}
	if p.Service().Ready() {
		t.Error("service must not be ready before Ready() runs")
	}
}

func TestReadyResolvesProviderAndCredentials(t *testing.T) {
	// Not parallel: t.Setenv.
	t.Setenv("AIAPI_LIFECYCLE_KEY", "sk-lifecycle-123")

	p := New().(*AIAPIPlugin)
	cfg := DefaultConfig()
	cfg.ActiveProvider = "openai" // registered via provider_openai.go init
	cfg.Providers["openai"] = ProviderConfig{
		Endpoint:     "https://api.openai.com/v1",
		Model:        "gpt-4o-mini",
		APIKeySource: "env:AIAPI_LIFECYCLE_KEY",
	}
	p.SetConfig(cfg)

	b := newFakeBuilder()
	p.Build(b)
	p.Ready(b)

	if !p.Service().Ready() {
		t.Fatal("service should be ready after provider + credentials resolve")
	}
	if p.Redactor() == nil {
		t.Error("redactor should be seeded after credential resolution (INV-1)")
	}

	// Cleanup deactivates.
	p.Cleanup(b)
	if p.Service().Ready() {
		t.Error("service should be not-ready after Cleanup")
	}
}

func TestBuildResetsNilConfig(t *testing.T) {
	t.Parallel()
	// A plugin with a zero-value config (nil Providers) gets DefaultConfig at Build.
	p := &AIAPIPlugin{registry: NewServiceRegistry()}
	b := newFakeBuilder()
	p.Build(b)
	if p.cfg.Providers == nil {
		t.Error("Build should reset a nil-Providers config to DefaultConfig")
	}
	if len(b.resources) != 1 {
		t.Errorf("Build should still publish the service resource, got %d", len(b.resources))
	}
}

func TestReadyGracefulWhenProviderUnconfigured(t *testing.T) {
	t.Parallel()
	p := New().(*AIAPIPlugin) // DefaultConfig: no active provider
	b := newFakeBuilder()
	p.Build(b)
	p.Ready(b) // Select fails → logs + returns; no panic

	if p.Service().Ready() {
		t.Error("service must stay not-ready when no provider is configured (graceful, INV-4)")
	}
}

func TestReadyGracefulWhenCredentialsMissing(t *testing.T) {
	t.Parallel()
	p := New().(*AIAPIPlugin)
	cfg := DefaultConfig()
	cfg.ActiveProvider = "openai"
	cfg.Providers["openai"] = ProviderConfig{
		Endpoint:     "https://api.openai.com/v1",
		Model:        "gpt-4o-mini",
		APIKeySource: "env:AIAPI_DEFINITELY_MISSING_KEY",
	}
	p.SetConfig(cfg)

	b := newFakeBuilder()
	p.Build(b)
	p.Ready(b)

	if p.Service().Ready() {
		t.Error("service must stay not-ready when the key source is empty (graceful)")
	}
}

func TestFinishIsNoOp(t *testing.T) {
	t.Parallel()
	p := New().(*AIAPIPlugin)
	b := newFakeBuilder()
	// Finish before/after Ready must not panic or change readiness.
	p.Finish(b)
	if p.Service().Ready() {
		t.Error("Finish alone should not make the service ready")
	}
}

// --- ServiceRegistry tests ---------------------------------------------------

func TestServiceRegistryNotReady(t *testing.T) {
	t.Parallel()
	s := NewServiceRegistry()
	if s.Ready() {
		t.Error("new registry should not be ready")
	}
	if _, err := s.Complete(context.Background(), CanonicalRequest{}); !errors.Is(err, ErrServiceNotReady) {
		t.Errorf("Complete before activate = %v, want ErrServiceNotReady", err)
	}
	err := s.Stream(context.Background(), CanonicalRequest{}, func(Chunk) error { return nil })
	if !errors.Is(err, ErrServiceNotReady) {
		t.Errorf("Stream before activate = %v, want ErrServiceNotReady", err)
	}
}

func TestServiceRegistryForwardsToProvider(t *testing.T) {
	t.Parallel()
	s := NewServiceRegistry()
	s.activate(NewFakeProvider("hello"))

	if !s.Ready() {
		t.Fatal("registry should be ready after activate")
	}
	resp, err := s.Complete(context.Background(), CanonicalRequest{})
	if err != nil || resp.Text() != "hello" {
		t.Errorf("Complete = %+v, %v, want text=hello", resp, err)
	}

	var delta string
	if err := s.Stream(context.Background(), CanonicalRequest{}, func(c Chunk) error {
		delta = c.Delta
		return nil
	}); err != nil || delta != "hello" {
		t.Errorf("Stream delta = %q, %v, want hello", delta, err)
	}

	// Deactivate → forwards stop.
	s.deactivate()
	if s.Ready() {
		t.Error("registry should not be ready after deactivate")
	}
	if _, err := s.Complete(context.Background(), CanonicalRequest{}); !errors.Is(err, ErrServiceNotReady) {
		t.Errorf("Complete after deactivate = %v, want ErrServiceNotReady", err)
	}
}
