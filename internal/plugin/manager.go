// Package plugin implements the engine-side plugin loader: the PluginManager
// resource (registry, lifecycle states, capability grants) and the
// capability-enforcing context proxy. The public SDK (interfaces, manifest,
// capability vocabulary, SemVer constraints) lives in pkg/plugin; this package
// holds the runtime logic that drives plugins through their lifecycle.
//
// The full in-process and out-of-process loaders (subprocess spawn + lifecycle
// over pkg/protocol) land with App integration; this file provides the
// registry, compatibility resolution, duplicate-ID guard, and capability proxy
// that the loaders build on and that the contract tests exercise.
//
// Bootstrap: l2-plugin-distribution-go Draft (Phase 6 Track N).
package plugin

import (
	"log/slog"
	"sync"

	pkgplugin "github.com/neuengine/neu/pkg/plugin"
)

// record is one registered plugin's manager state.
type record struct {
	manifest pkgplugin.Manifest
	granted  pkgplugin.CapabilitySet
	state    pkgplugin.State
}

// Manager owns the plugin registry and enforces engine-version compatibility +
// ID uniqueness. It is the engine resource that the loaders and the plugin
// manager UI read.
type Manager struct {
	mu       sync.RWMutex
	registry map[pkgplugin.PluginID]*record
	engine   pkgplugin.Version
}

// NewManager returns a manager that gates plugins against the running engine
// version.
func NewManager(engine pkgplugin.Version) *Manager {
	return &Manager{registry: make(map[pkgplugin.PluginID]*record), engine: engine}
}

// Register validates a manifest, checks engine compatibility (INV-2) and ID
// uniqueness (INV-5), and records the plugin as Approved with its granted
// capability set. It does not yet execute plugin code (that is the loader's job).
func (m *Manager) Register(man pkgplugin.Manifest, granted pkgplugin.CapabilitySet) error {
	if err := man.Validate(); err != nil { // INV-1
		return err
	}
	ok, err := man.CompatibleWith(m.engine) // INV-2
	if err != nil {
		return err
	}
	if !ok {
		return pkgplugin.ErrEngineIncompatible
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, dup := m.registry[man.ID]; dup { // INV-5
		return pkgplugin.ErrDuplicateID
	}
	m.registry[man.ID] = &record{manifest: man, granted: granted, state: pkgplugin.StateApproved}
	return nil
}

// State returns a plugin's lifecycle state.
func (m *Manager) State(id pkgplugin.PluginID) (pkgplugin.State, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.registry[id]
	if !ok {
		return 0, false
	}
	return r.state, true
}

// SetState transitions a plugin's state (used by the loaders).
func (m *Manager) SetState(id pkgplugin.PluginID, s pkgplugin.State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.registry[id]; ok {
		r.state = s
	}
}

// Count returns the number of registered plugins.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.registry)
}

// Context builds a capability-enforcing PluginContext for a registered plugin.
func (m *Manager) Context(id pkgplugin.PluginID, logger *slog.Logger) (pkgplugin.PluginContext, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.registry[id]
	if !ok {
		return nil, false
	}
	return &capabilityProxy{id: id, granted: r.granted, logger: logger}, true
}

// capabilityProxy implements pkgplugin.PluginContext, rejecting + logging any
// engine-API call outside the granted capability set (INV-3).
type capabilityProxy struct {
	id      pkgplugin.PluginID
	granted pkgplugin.CapabilitySet
	logger  *slog.Logger
	cfg     []byte
}

func (p *capabilityProxy) Commands() pkgplugin.CommandIssuer { return taggedCommands{id: p.id} }
func (p *capabilityProxy) Config() []byte                    { return p.cfg }
func (p *capabilityProxy) Capabilities() pkgplugin.CapabilitySet { return p.granted }
func (p *capabilityProxy) Logger() *slog.Logger              { return p.logger }

// Check enforces INV-3: an ungranted capability returns a CapabilityError and
// is logged with plugin ID + capability (the call site is the caller's frame).
func (p *capabilityProxy) Check(c pkgplugin.Capability) error {
	if p.granted.Has(c) {
		return nil
	}
	if p.logger != nil {
		p.logger.Warn("plugin capability denied", "plugin", string(p.id), "capability", string(c))
	}
	return pkgplugin.CapabilityError{Plugin: p.id, Capability: c}
}

// taggedCommands stamps the plugin ID on every command it issues (INV-7).
type taggedCommands struct{ id pkgplugin.PluginID }

func (t taggedCommands) PluginID() pkgplugin.PluginID { return t.id }

var _ pkgplugin.PluginContext = (*capabilityProxy)(nil)
