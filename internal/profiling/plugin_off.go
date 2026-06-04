//go:build !profiling

package profiling

import (
	"github.com/neuengine/neu/pkg/app/appface"
	"github.com/neuengine/neu/pkg/diag/profiling"
)

// ProfilingPlugin is a no-op without the profiling build tag. Its fields mirror
// the real plugin (the field types are untagged in pkg/diag/profiling) so app
// wiring code compiles identically in both builds, but Build registers nothing
// — release binaries carry zero profiling cost (INV-1).
type ProfilingPlugin struct {
	Exporter profiling.ProfileExporter
	Config   profiling.ProfilingConfig
}

// Build is a no-op without the profiling build tag.
func (ProfilingPlugin) Build(appface.Builder) {}
