package diag

import (
	"testing"

	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/gametime"
	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
	pkgdiag "github.com/neuengine/neu/pkg/diag"
)

type mark struct{ N int }

// storeWithReaders builds a store with the built-ins registered and readers on
// the given paths (so collection is enabled for them, INV-1).
func storeWithReaders(paths ...pkgdiag.DiagnosticPath) *pkgdiag.DiagnosticsStore {
	s := pkgdiag.NewDiagnosticsStore()
	s.Register(pkgdiag.NewDiagnostic(PathFPS, "fps", 0))
	s.Register(pkgdiag.NewDiagnostic(PathFrameTimeMS, "ms", 0))
	s.Register(pkgdiag.NewDiagnostic(PathEntityCount, "count", 0))
	for _, p := range paths {
		s.AddReader(p)
	}
	return s
}

func latest(t *testing.T, s *pkgdiag.DiagnosticsStore, path pkgdiag.DiagnosticPath) float64 {
	t.Helper()
	d, ok := s.Get(path)
	if !ok {
		t.Fatalf("diagnostic %q not registered", path)
	}
	e, _ := d.Latest()
	return e.Value
}

func approx(a, b float64) bool {
	d := a - b
	return d < 0.01 && d > -0.01
}

func TestRecordFrame(t *testing.T) {
	t.Parallel()
	s := storeWithReaders(PathFPS, PathFrameTimeMS, PathEntityCount)
	recordFrame(s, 0.016, 5) // a 16 ms frame, 5 entities

	if ft := latest(t, s, PathFrameTimeMS); !approx(ft, 16) {
		t.Errorf("frame_time_ms = %v, want 16", ft)
	}
	if fps := latest(t, s, PathFPS); !approx(fps, 62.5) {
		t.Errorf("fps = %v, want 62.5", fps)
	}
	if ec := latest(t, s, PathEntityCount); ec != 5 {
		t.Errorf("entity_count = %v, want 5", ec)
	}
}

func TestRecordFrameZeroDeltaSkipsFPS(t *testing.T) {
	t.Parallel()
	s := storeWithReaders(PathFPS, PathFrameTimeMS)
	recordFrame(s, 0, 0)

	if d, _ := s.Get(PathFPS); d.Len() != 0 {
		t.Error("FPS must be skipped on a zero delta (no divide-by-zero)")
	}
	if d, _ := s.Get(PathFrameTimeMS); d.Len() != 1 {
		t.Error("frame_time_ms must still record on a zero delta")
	}
}

func TestRecordFrameZeroCostGate(t *testing.T) {
	t.Parallel()
	// Registered but NO readers ⇒ nothing collects (INV-1).
	s := pkgdiag.NewDiagnosticsStore()
	s.Register(pkgdiag.NewDiagnostic(PathFPS, "fps", 0))
	recordFrame(s, 0.016, 3)
	if d, _ := s.Get(PathFPS); d.Len() != 0 {
		t.Error("no reader ⇒ no collection (INV-1)")
	}
}

func TestCollectBuiltinsEntityCount(t *testing.T) {
	t.Parallel()
	w := world.NewWorld()
	for range 4 {
		w.Spawn(component.Data{Value: mark{N: 1}})
	}
	s := storeWithReaders(PathEntityCount)
	collectBuiltins(w, s) // no RealTime resource ⇒ dt = 0, timing skipped/zero

	if ec := latest(t, s, PathEntityCount); ec != 4 {
		t.Errorf("entity_count = %v, want 4", ec)
	}
}

// fakeBuilder is a minimal appface.Builder recording what a plugin registers,
// so Build can be tested without importing pkg/app (which would cycle).
type fakeBuilder struct {
	w         *world.World
	systems   map[string][]string
	resources int
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
func (b *fakeBuilder) SetResource(any) appface.Builder                { b.resources++; return b }
func (b *fakeBuilder) InitResource(any) appface.Builder               { return b }
func (b *fakeBuilder) AddPlugin(appface.Plugin) appface.Builder       { return b }
func (b *fakeBuilder) AddPlugins(appface.PluginGroup) appface.Builder { return b }

func TestDiagnosticsPluginBuild(t *testing.T) {
	t.Parallel()
	store := pkgdiag.NewDiagnosticsStore()
	b := newFakeBuilder()
	DiagnosticsPlugin{Store: store}.Build(b)

	if store.Len() != 3 {
		t.Errorf("registered %d diagnostics, want 3 (fps/frame_time/entity_count)", store.Len())
	}
	if names := b.systems[appface.Last]; len(names) != 1 || names[0] != "diag.BuiltinMetrics" {
		t.Errorf("Last-schedule systems = %v, want [diag.BuiltinMetrics]", names)
	}
	if b.resources != 1 {
		t.Errorf("SetResource calls = %d, want 1 (the store)", b.resources)
	}

	// The added system, run against a world with readers, collects.
	store.AddReader(PathEntityCount)
	b.w.Spawn(component.Data{Value: mark{N: 1}})
	collectBuiltins(b.w, store)
	if ec := latest(t, store, PathEntityCount); ec != 1 {
		t.Errorf("after Build+run, entity_count = %v, want 1", ec)
	}
}

func TestDiagnosticsPluginNilStore(t *testing.T) {
	t.Parallel()
	b := newFakeBuilder()
	DiagnosticsPlugin{}.Build(b) // nil Store ⇒ the plugin creates its own
	if names := b.systems[appface.Last]; len(names) != 1 || names[0] != "diag.BuiltinMetrics" {
		t.Errorf("Last systems = %v", names)
	}
	if b.resources != 1 {
		t.Errorf("store should be set as a resource, got %d", b.resources)
	}
}

func TestFrameDeltaSecondsRealTimePresent(t *testing.T) {
	t.Parallel()
	w := world.NewWorld()
	if frameDeltaSeconds(w) != 0 {
		t.Error("no RealTime resource ⇒ delta 0")
	}
	world.SetResource(w, gametime.RealTime{}) // present, zero delta
	if frameDeltaSeconds(w) != 0 {
		t.Error("zero-value RealTime ⇒ delta 0 (resource-present branch)")
	}
}

var _ appface.Builder = (*fakeBuilder)(nil)
