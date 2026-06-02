package plugin

import (
	"testing"

	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
	pkgplugin "github.com/neuengine/neu/pkg/plugin"
)

// --- fakeBuilder (same pattern as diag/builtins_test.go) --------------------

type testBuilder struct {
	w       *world.World
	systems map[string][]string
}

func newTestBuilder() *testBuilder {
	return &testBuilder{w: world.NewWorld(), systems: map[string][]string{}}
}

func (b *testBuilder) World() *world.World { return b.w }
func (b *testBuilder) AddSystem(s string, sys scheduler.System) appface.Builder {
	b.systems[s] = append(b.systems[s], sys.Name())
	return b
}
func (b *testBuilder) AddSystems(s string, sys ...scheduler.System) appface.Builder {
	for _, ss := range sys {
		b.AddSystem(s, ss)
	}
	return b
}
func (b *testBuilder) SetResource(any) appface.Builder                { return b }
func (b *testBuilder) InitResource(any) appface.Builder               { return b }
func (b *testBuilder) AddPlugin(appface.Plugin) appface.Builder       { return b }
func (b *testBuilder) AddPlugins(appface.PluginGroup) appface.Builder { return b }

var _ appface.Builder = (*testBuilder)(nil)

// --- minimal Plugin and FullPlugin fakes -------------------------------------

type buildOnlyPlugin struct{ built bool }

func (p *buildOnlyPlugin) Build(_ appface.Builder) { p.built = true }

type fullPlugin struct {
	built, readied, finished, cleaned bool
}

func (p *fullPlugin) Build(_ appface.Builder)   { p.built = true }
func (p *fullPlugin) Ready(_ appface.Builder)   { p.readied = true }
func (p *fullPlugin) Finish(_ appface.Builder)  { p.finished = true }
func (p *fullPlugin) Cleanup(_ appface.Builder) { p.cleaned = true }

var _ appface.Plugin     = (*buildOnlyPlugin)(nil)
var _ appface.FullPlugin = (*fullPlugin)(nil)

// --- registry tests ----------------------------------------------------------

func TestRegisterAndLookupFactory(t *testing.T) {
	t.Parallel()
	resetFactories()
	defer resetFactories()

	id := pkgplugin.PluginID("com.test.registry")
	if _, ok := LookupFactory(id); ok {
		t.Fatal("factory should not exist before registration")
	}

	p := &buildOnlyPlugin{}
	RegisterFactory(id, func() appface.Plugin { return p })

	f, ok := LookupFactory(id)
	if !ok {
		t.Fatal("factory not found after registration")
	}
	if got := f(); got != p {
		t.Fatal("factory should return the registered plugin instance")
	}
}

func TestRegisterFactory_Overwrite(t *testing.T) {
	t.Parallel()
	resetFactories()
	defer resetFactories()

	id := pkgplugin.PluginID("com.test.overwrite")
	p1 := &buildOnlyPlugin{}
	p2 := &buildOnlyPlugin{}

	RegisterFactory(id, func() appface.Plugin { return p1 })
	RegisterFactory(id, func() appface.Plugin { return p2 }) // overwrite

	f, _ := LookupFactory(id)
	if got := f(); got != p2 {
		t.Fatal("second RegisterFactory should overwrite the first")
	}
}

// --- LoadInProcess tests -----------------------------------------------------

func validInProcManifest() pkgplugin.Manifest {
	return pkgplugin.Manifest{
		ID:            "com.test.inproc",
		Version:       "1.0.0",
		EngineVersion: "^1.0.0",
		Mode:          pkgplugin.ModeInProcess,
		Entry:         pkgplugin.EntrySpec{PackagePath: "com.test", Factory: "New"},
	}
}

func TestLoadInProcess_BuildOnly(t *testing.T) {
	t.Parallel()
	resetFactories()
	defer resetFactories()

	man := validInProcManifest()
	m := NewManager(engineV("1.0.0"))
	if err := m.Register(man, nil); err != nil {
		t.Fatalf("Register: %v", err)
	}

	p := &buildOnlyPlugin{}
	RegisterFactory(man.ID, func() appface.Plugin { return p })

	b := newTestBuilder()
	got, err := LoadInProcess(m, man.ID, b)
	if err != nil {
		t.Fatalf("LoadInProcess: %v", err)
	}
	if got != p {
		t.Fatal("should return the plugin instance")
	}
	if !p.built {
		t.Error("Build should have been called")
	}
	if st, _ := m.State(man.ID); st != pkgplugin.StateActive {
		t.Errorf("state = %v, want Active", st)
	}
}

func TestLoadInProcess_FullPlugin_CallsReady(t *testing.T) {
	t.Parallel()
	resetFactories()
	defer resetFactories()

	man := validInProcManifest()
	m := NewManager(engineV("1.0.0"))
	if err := m.Register(man, nil); err != nil {
		t.Fatalf("Register: %v", err)
	}

	fp := &fullPlugin{}
	RegisterFactory(man.ID, func() appface.Plugin { return fp })

	b := newTestBuilder()
	got, err := LoadInProcess(m, man.ID, b)
	if err != nil {
		t.Fatalf("LoadInProcess: %v", err)
	}
	if got != fp {
		t.Fatal("should return the fullPlugin instance")
	}
	if !fp.built || !fp.readied {
		t.Errorf("Build=%v Ready=%v — both should be true", fp.built, fp.readied)
	}
	if fp.finished || fp.cleaned {
		t.Error("Finish/Cleanup must not be called by LoadInProcess")
	}
}

func TestLoadInProcess_NoFactory(t *testing.T) {
	t.Parallel()
	resetFactories()
	defer resetFactories()

	man := validInProcManifest()
	m := NewManager(engineV("1.0.0"))
	if err := m.Register(man, nil); err != nil {
		t.Fatalf("Register: %v", err)
	}

	if _, err := LoadInProcess(m, man.ID, newTestBuilder()); err == nil {
		t.Fatal("expected error when no factory is registered")
	}
}

func TestLoadInProcess_NotInManager(t *testing.T) {
	t.Parallel()
	resetFactories()
	defer resetFactories()

	id := pkgplugin.PluginID("com.test.inproc")
	RegisterFactory(id, func() appface.Plugin { return &buildOnlyPlugin{} })

	m := NewManager(engineV("1.0.0"))
	// NOT registered in manager
	if _, err := LoadInProcess(m, id, newTestBuilder()); err == nil {
		t.Fatal("expected error when plugin is not in manager")
	}
}

// --- FinishInProcess tests ---------------------------------------------------

func TestFinishInProcess_BuildOnly(t *testing.T) {
	t.Parallel()
	m := NewManager(engineV("1.0.0"))
	man := validInProcManifest()
	if err := m.Register(man, nil); err != nil {
		t.Fatalf("Register: %v", err)
	}
	m.SetState(man.ID, pkgplugin.StateActive)

	p := &buildOnlyPlugin{}
	FinishInProcess(m, man.ID, p, newTestBuilder())

	if st, _ := m.State(man.ID); st != pkgplugin.StateDisabled {
		t.Errorf("state = %v, want Disabled", st)
	}
}

func TestFinishInProcess_FullPlugin(t *testing.T) {
	t.Parallel()
	m := NewManager(engineV("1.0.0"))
	man := validInProcManifest()
	if err := m.Register(man, nil); err != nil {
		t.Fatalf("Register: %v", err)
	}
	m.SetState(man.ID, pkgplugin.StateActive)

	fp := &fullPlugin{built: true, readied: true}
	FinishInProcess(m, man.ID, fp, newTestBuilder())

	if !fp.finished || !fp.cleaned {
		t.Errorf("Finish=%v Cleanup=%v — both should be true", fp.finished, fp.cleaned)
	}
	if st, _ := m.State(man.ID); st != pkgplugin.StateDisabled {
		t.Errorf("state = %v, want Disabled", st)
	}
}
