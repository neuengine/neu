//go:build profiling

package profiling

import (
	"testing"

	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
	"github.com/neuengine/neu/pkg/diag/profiling"
)

// fakeBuilder records what the plugin registers, avoiding a pkg/app import cycle.
type fakeBuilder struct {
	w       *world.World
	systems map[string][]string
}

func newFakeBuilder() *fakeBuilder {
	return &fakeBuilder{w: world.NewWorld(), systems: map[string][]string{}}
}

func (b *fakeBuilder) World() *world.World { return b.w }
func (b *fakeBuilder) AddSystem(schedule string, s scheduler.System) appface.Builder {
	b.systems[schedule] = append(b.systems[schedule], s.Name())
	return b
}
func (b *fakeBuilder) AddSystems(schedule string, ss ...scheduler.System) appface.Builder {
	for _, s := range ss {
		b.AddSystem(schedule, s)
	}
	return b
}
func (b *fakeBuilder) SetResource(any) appface.Builder                { return b }
func (b *fakeBuilder) InitResource(any) appface.Builder               { return b }
func (b *fakeBuilder) AddPlugin(appface.Plugin) appface.Builder       { return b }
func (b *fakeBuilder) AddPlugins(appface.PluginGroup) appface.Builder { return b }

func TestProfilingPluginRegistersMarkFrame(t *testing.T) {
	cfg := profiling.DefaultProfilingConfig()
	cfg.Enabled = true
	exp := profiling.NewPprofExporter()
	b := newFakeBuilder()

	ProfilingPlugin{Exporter: exp, Config: cfg}.Build(b)

	last := b.systems[appface.Last]
	if len(last) != 1 || last[0] != "profiling.MarkFrame" {
		t.Fatalf("Last schedule systems = %v, want [profiling.MarkFrame]", last)
	}
	// The injected exporter is now active: a span emitted through the live API
	// reaches it.
	_, span := profiling.BeginSpan(t.Context(), "sys.update", profiling.CategorySystem)
	span.End()
	if _, ok := exp.Stat("sys.update"); !ok {
		t.Error("injected exporter did not receive the emitted span")
	}
}

func TestProfilingPluginResolvesDefaultExporter(t *testing.T) {
	cfg := profiling.DefaultProfilingConfig()
	cfg.Enabled = true
	cfg.Exporter = profiling.ExporterPprof
	b := newFakeBuilder()

	// No injected exporter → resolved from Config.Exporter (pprof aggregator).
	ProfilingPlugin{Config: cfg}.Build(b)

	_, span := profiling.BeginSpan(t.Context(), "resolved", profiling.CategorySystem)
	span.End()
	// MarkFrame system still registered.
	if got := b.systems[appface.Last]; len(got) != 1 {
		t.Errorf("Last systems = %v, want one MarkFrame system", got)
	}
}
