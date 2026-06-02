package plugin

import (
	"errors"

	pkgplugin "github.com/neuengine/neu/pkg/plugin"
)

// ErrCapabilitiesDenied marks a plugin skipped because a required capability was
// not granted (denied at the prompt or never persisted).
var ErrCapabilitiesDenied = errors.New("plugin: required capabilities not granted")

// DiscoveryReport summarizes one [DiscoverAndRegister] pass for the plugin-
// manager UI / audit log.
type DiscoveryReport struct {
	Registered []pkgplugin.PluginID
	Skipped    map[pkgplugin.PluginID]error
	Errors     []error // source/parse failures with no manifest to attribute
}

// DiscoverAndRegister scans every source, resolves each discovered plugin's
// required-capability grants (persisting TierPrompt approvals), and registers
// the approved ones with the manager — which enforces engine compatibility
// (INV-2) and ID uniqueness (INV-5), moving them to StateApproved. Executing
// plugin code (the in-process factory / OOP subprocess) is a later step; this is
// the discovery + approval half. It never fails fatally: per-plugin problems are
// recorded in the report and the scan continues.
func DiscoverAndRegister(m *Manager, sources []Source, store GrantStore, prompt PromptFunc) DiscoveryReport {
	rep := DiscoveryReport{Skipped: make(map[pkgplugin.PluginID]error)}
	for _, src := range sources {
		found, errs := src.Discover()
		rep.Errors = append(rep.Errors, errs...)
		for _, dp := range found {
			granted, ok := ResolveGrants(dp.Manifest, store, prompt)
			if !ok {
				rep.Skipped[dp.Manifest.ID] = ErrCapabilitiesDenied
				continue
			}
			if err := m.Register(dp.Manifest, granted); err != nil {
				rep.Skipped[dp.Manifest.ID] = err
				continue
			}
			rep.Registered = append(rep.Registered, dp.Manifest.ID)
		}
	}
	return rep
}
