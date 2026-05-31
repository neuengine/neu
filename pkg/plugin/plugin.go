// Package plugin is the public, stable SDK third-party plugins compile against.
// It exposes the engine's Plugin lifecycle, the manifest format, the capability
// model, and a capability-checked context — and nothing from internal/. The Go
// `plugin` stdlib package (.so loading) is deliberately not used; in-process
// plugins are linked as Go modules, out-of-process plugins are subprocesses.
//
// Bootstrap: l2-plugin-distribution-go Draft (Phase 6 Track N, C29 gate open).
package plugin

import "github.com/neuengine/neu/pkg/app/appface"

// Plugin is the engine's plugin lifecycle interface, re-exported from the app
// public surface so third parties import only pkg/plugin.
type Plugin = appface.Plugin

// PluginGroup is an ordered collection of plugins, re-exported from the app
// public surface.
type PluginGroup = appface.PluginGroup

// PluginID is a reverse-DNS-style unique identifier, e.g. "com.example.aiapi".
type PluginID string

// Mode is the plugin delivery mode.
type Mode uint8

const (
	// ModeInProcess is a compile-time-linked Go module.
	ModeInProcess Mode = iota
	// ModeOutOfProcess is a subprocess communicating over a transport.
	ModeOutOfProcess
)

// String renders the mode as it appears in the manifest.
func (m Mode) String() string {
	switch m {
	case ModeInProcess:
		return "in-process"
	case ModeOutOfProcess:
		return "out-of-process"
	default:
		return "unknown"
	}
}

// ParseMode maps a manifest mode string to a Mode.
func ParseMode(s string) (Mode, bool) {
	switch s {
	case "in-process":
		return ModeInProcess, true
	case "out-of-process":
		return ModeOutOfProcess, true
	default:
		return 0, false
	}
}

// State is a plugin's lifecycle state in the manager (L1 §4.8).
type State uint8

const (
	StateDiscovered State = iota
	StateApproved
	StateLoading
	StateActive
	StateFailed
	StateDisabled
)

// String renders the state.
func (s State) String() string {
	switch s {
	case StateDiscovered:
		return "Discovered"
	case StateApproved:
		return "Approved"
	case StateLoading:
		return "Loading"
	case StateActive:
		return "Active"
	case StateFailed:
		return "Failed"
	case StateDisabled:
		return "Disabled"
	default:
		return "Unknown"
	}
}
