package oop

import (
	"context"
	"log/slog"

	"github.com/neuengine/neu/pkg/app/appface"
	pkgplugin "github.com/neuengine/neu/pkg/plugin"
	"github.com/neuengine/neu/pkg/protocol"
)

// stateSetterFn lets the proxy update the manager's plugin state without
// importing internal/plugin (which would create a cycle). The caller passes
// manager.SetState as a closure.
type stateSetterFn func(id pkgplugin.PluginID, s pkgplugin.State)

// ProxyPlugin is the host-side appface.FullPlugin that drives an OOP subprocess
// through the Build → Ready → Finish → Cleanup lifecycle over pkg/protocol.
//
// If the subprocess crashes or the transport fails during any phase, the plugin
// is marked Failed and the method returns without panicking (INV-8).
type ProxyPlugin struct {
	id       pkgplugin.PluginID
	sup      *Supervisor
	setState stateSetterFn
	ctx      context.Context    // parent context from Spawn call
	cancel   context.CancelFunc // cancels all in-flight DriveLifecycle calls
}

// NewProxyPlugin creates a ProxyPlugin that routes lifecycle calls through sup.
// setState is called with the new PluginState whenever the lifecycle advances
// or fails (supply manager.SetState from the caller).
func NewProxyPlugin(
	id pkgplugin.PluginID,
	sup *Supervisor,
	setState stateSetterFn,
) *ProxyPlugin {
	ctx, cancel := context.WithCancel(context.Background())
	return &ProxyPlugin{
		id:       id,
		sup:      sup,
		setState: setState,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Build drives the Build lifecycle phase over the OOP transport (INV-4). The
// app.Builder parameter is not forwarded to the subprocess — configuration is
// exchanged via the protocol, not via Go reflection. Build advances the plugin
// state to Active on success.
func (p *ProxyPlugin) Build(app appface.Builder) {
	p.setState(p.id, pkgplugin.StateLoading)
	if err := p.sup.DriveLifecycle(p.ctx, protocol.PhaseBuild); err != nil {
		slog.Error("oop plugin Build failed", "id", p.id, "err", err)
		p.setState(p.id, pkgplugin.StateFailed)
		return
	}
	p.setState(p.id, pkgplugin.StateActive)
}

// Ready drives the Ready lifecycle phase (warm-up after all plugins Built).
func (p *ProxyPlugin) Ready(app appface.Builder) {
	if p.sup.IsFailed() {
		return
	}
	if err := p.sup.DriveLifecycle(p.ctx, protocol.PhaseReady); err != nil {
		slog.Error("oop plugin Ready failed", "id", p.id, "err", err)
		p.setState(p.id, pkgplugin.StateFailed)
	}
}

// Finish drives the Finish lifecycle phase (pre-shutdown, stop accepting work).
func (p *ProxyPlugin) Finish(app appface.Builder) {
	if p.sup.IsFailed() {
		return
	}
	if err := p.sup.DriveLifecycle(p.ctx, protocol.PhaseFinish); err != nil {
		slog.Warn("oop plugin Finish failed", "id", p.id, "err", err)
		p.setState(p.id, pkgplugin.StateFailed)
	}
}

// Cleanup drives the Cleanup lifecycle phase then closes the supervisor. After
// Cleanup the subprocess is shut down and the ProxyPlugin is unusable.
func (p *ProxyPlugin) Cleanup(app appface.Builder) {
	if !p.sup.IsFailed() {
		if err := p.sup.DriveLifecycle(p.ctx, protocol.PhaseCleanup); err != nil {
			slog.Warn("oop plugin Cleanup failed", "id", p.id, "err", err)
		}
	}
	p.cancel() // cancel any stray goroutines
	p.sup.Close()
	p.setState(p.id, pkgplugin.StateDisabled)
}

// compile-time assertions
var _ appface.Plugin = (*ProxyPlugin)(nil)
var _ appface.FullPlugin = (*ProxyPlugin)(nil)
