//go:build editor

// Package aiapi is the first-party AI API plugin: a unified client over
// OpenAI-compatible providers exposing the standard assistant methods. This file
// holds the plugin lifecycle (New → Build → Ready → Finish → Cleanup, L1 §4.11)
// and the ServiceRegistry it publishes so other systems reach the active
// provider through a stable interface.
//
// Bootstrap: l2-ai-api-plugin-go Draft (Phase 6 Track O).
package aiapi

import (
	"log/slog"

	"github.com/neuengine/neu/pkg/app/appface"
	"github.com/neuengine/neu/pkg/errs"
)

// PluginID is the manifest ID (L1 §4.2), exported so the host app can register
// the compile-time factory: internal/plugin.RegisterFactory(aiapi.PluginID, aiapi.New).
const PluginID = "io.teratron.neuengine.aiapi"

// AIAPIPlugin is the first-party AI API plugin. It implements appface.FullPlugin
// so the engine drives its full lifecycle; Ready resolves the provider and
// credentials, activating the published ServiceRegistry.
type AIAPIPlugin struct {
	registry *ServiceRegistry
	redactor *errs.Redactor // seeded with the resolved key for INV-1 log redaction
	cfg      Config
}

// New is the manifest factory (L1 §4.2 `[entry.in_process] factory = "New"`). It
// returns a plugin with default config; the loader may override config via
// SetConfig before Build.
func New() appface.Plugin {
	return &AIAPIPlugin{cfg: DefaultConfig(), registry: NewServiceRegistry()}
}

// SetConfig overrides the plugin's configuration before Build. The loader calls
// it with the user config decoded from PluginContext.Config() (L1 §4.11 Build:
// "load config"). Calling it after Build has no effect on an already-resolved
// provider until the next Ready.
func (p *AIAPIPlugin) SetConfig(cfg Config) { p.cfg = cfg }

// Service returns the plugin's AI service registry — the resource consumers use
// (also exposed for direct wiring and tests).
func (p *AIAPIPlugin) Service() *ServiceRegistry { return p.registry }

// Redactor returns the credential redactor seeded during Ready (nil until the
// active provider's key source resolves). The method-dispatch layer routes its
// log/error output through it so a key never leaks (INV-1).
func (p *AIAPIPlugin) Redactor() *errs.Redactor { return p.redactor }

// Build publishes the ServiceRegistry as a World resource so other systems (the
// assistant manager, editor panels) can discover the AI service. Provider
// resolution is deferred to Ready so a slow credential source never blocks the
// build phase (L1 §4.11).
func (p *AIAPIPlugin) Build(app appface.Builder) {
	if p.cfg.Providers == nil {
		p.cfg = DefaultConfig()
	}
	app.SetResource(p.registry)
}

// Ready resolves the active provider and its credentials, then activates the
// service. It is non-blocking and graceful: if the provider is unconfigured or
// the credential source is unavailable, it logs and leaves the service
// not-ready (other plugins proceed; the user can fix config and reload) — the
// L1 §4.11 "Ready returns false until credentials resolve" semantics, expressed
// through the service's ready flag since appface.FullPlugin.Ready has no return.
func (p *AIAPIPlugin) Ready(app appface.Builder) {
	provider, err := Select(p.cfg)
	if err != nil {
		slog.Warn("aiapi: provider unresolved; AI service stays unavailable", "err", err)
		return
	}

	// Resolve + validate credentials, seeding the redactor (INV-1). The active
	// provider's key source must resolve before the service goes live.
	if pc, ok := p.cfg.Providers[p.cfg.ActiveProvider]; ok && pc.APIKeySource != "" {
		secret, err := resolveSecret(pc.APIKeySource)
		if err != nil {
			slog.Warn("aiapi: credentials unresolved; AI service stays unavailable", "err", err)
			return
		}
		p.redactor = newRedactor(secret)
		zero(secret) // defence-in-depth: wipe the buffer once the redactor has copied it
	}

	p.registry.activate(provider)
	slog.Info("aiapi: service ready", "provider", provider.Name())
}

// Finish is the post-Ready hook where method registration with the assistant
// manager and any background workers are wired (L1 §4.11). The rate limiter is
// pull-based (no goroutine), and method dispatch lands in a later subtask
// (T-6O03·rem), so this only records the phase for now.
func (p *AIAPIPlugin) Finish(app appface.Builder) {
	slog.Debug("aiapi: finish (method dispatch wiring deferred to T-6O03)", "ready", p.registry.Ready())
}

// Cleanup deactivates the service so in-flight consumers see ErrServiceNotReady
// rather than a torn-down provider (L1 §4.11: cancel in-flight, release).
func (p *AIAPIPlugin) Cleanup(app appface.Builder) {
	p.registry.deactivate()
	slog.Info("aiapi: service stopped")
}

var (
	_ appface.Plugin     = (*AIAPIPlugin)(nil)
	_ appface.FullPlugin = (*AIAPIPlugin)(nil)
)
