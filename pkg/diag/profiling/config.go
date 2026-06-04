package profiling

import "time"

// ExporterType selects the active span sink. It is resolved once when the
// ProfilingPlugin builds; adding a new exporter does not change the
// instrumentation API (INV-5).
type ExporterType uint8

const (
	// ExporterNone discards spans (the default — profiling dormant).
	ExporterNone ExporterType = iota
	// ExporterPprof maps spans to runtime/pprof labels (pure Go, no CGo).
	ExporterPprof
	// ExporterChrome writes Trace Event Format JSON for chrome://tracing / Perfetto.
	ExporterChrome
	// ExporterMulti fans spans out to several exporters at once.
	ExporterMulti
)

// String returns the canonical exporter name.
func (e ExporterType) String() string {
	switch e {
	case ExporterPprof:
		return "pprof"
	case ExporterChrome:
		return "chrome"
	case ExporterMulti:
		return "multi"
	case ExporterNone:
		return "none"
	default:
		return "none"
	}
}

// ProfilingConfig is the ECS resource controlling the profiling layer. The
// `profiling` build tag is the outer gate (no tag → no machinery); Enabled is
// the runtime switch within a profiling build, so a profiling binary can ship
// with profiling dormant until toggled.
type ProfilingConfig struct {
	// ChromeOutputPath is the file the Chrome exporter writes to.
	ChromeOutputPath string
	// CustomCategories names user-defined span categories.
	CustomCategories []string
	// MinSpanDuration skips spans shorter than this (noise filter); 0 disables.
	MinSpanDuration time.Duration
	// Exporter selects the active span sink.
	Exporter ExporterType
	// Enabled is the runtime master switch (also requires the build tag).
	Enabled bool
	// AutoInstrument wraps systems/passes automatically.
	AutoInstrument bool
	// MemoryTracking records allocation deltas per span (uses STW ReadMemStats;
	// heavyweight — intended for targeted sessions, off by default).
	MemoryTracking bool
	// GPUTiming collects GPU timestamp queries (deferred; backend-specific).
	GPUTiming bool
}

// DefaultProfilingConfig returns a dormant, Chrome-targeted configuration.
func DefaultProfilingConfig() ProfilingConfig {
	return ProfilingConfig{
		Enabled:          false,
		Exporter:         ExporterChrome,
		AutoInstrument:   true,
		MemoryTracking:   false,
		GPUTiming:        false,
		ChromeOutputPath: "profile.trace.json",
		MinSpanDuration:  0,
	}
}
