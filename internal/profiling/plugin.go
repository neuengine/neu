//go:build profiling

// Package profiling wires the pkg/diag/profiling protocol into the App: it
// installs the runtime config and exporter and registers the per-frame
// MarkFrame system. All of it is gated behind the profiling build tag (mirrors
// how internal/diag wires pkg/diag). The default build links the no-op plugin
// in plugin_off.go, so release binaries carry zero profiling cost (INV-1).
package profiling

import (
	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
	"github.com/neuengine/neu/pkg/diag/profiling"
)

// ProfilingPlugin installs profiling into the App. The Exporter is injected
// (the app owns its lifecycle — e.g. an *os.File-backed ChromeExporter it
// Shuts down at teardown), mirroring how DiagnosticsPlugin takes an injected
// Store. When Exporter is nil it is resolved from Config.Exporter: ExporterPprof
// yields a self-contained aggregator, anything else (Chrome/Multi/None) falls
// back to the NopExporter since those need caller-owned resources.
type ProfilingPlugin struct {
	// Exporter, when non-nil, is the span sink. nil → resolved from Config.
	Exporter profiling.ProfileExporter
	// Config is the runtime configuration (Enabled gate, exporter selection,
	// MinSpanDuration filter).
	Config profiling.ProfilingConfig
}

// Build implements appface.Plugin: installs config + exporter and registers the
// Last-schedule MarkFrame system (self-gating — a no-op while disabled).
func (p ProfilingPlugin) Build(app appface.Builder) {
	profiling.SetConfig(p.Config)

	exp := p.Exporter
	if exp == nil {
		exp = resolveExporter(p.Config.Exporter)
	}
	profiling.SetExporter(exp)

	app.AddSystem(appface.Last, scheduler.NewFuncSystem("profiling.MarkFrame", func(*world.World) {
		profiling.MarkFrame()
	}))
}

// resolveExporter builds a self-contained default exporter for the selected
// type. Chrome and Multi require caller-owned resources (a file, sub-exporters)
// so they fall back to the NopExporter when not injected.
func resolveExporter(t profiling.ExporterType) profiling.ProfileExporter {
	switch t {
	case profiling.ExporterPprof:
		return profiling.NewPprofExporter()
	default:
		return profiling.NopExporter{}
	}
}
