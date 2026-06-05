//go:build editor

package hotreload

import (
	"errors"
	"testing"

	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
)

// ─── shader hot-swap (INV-3) ─────────────────────────────────────────────────

type fakeCompiler struct {
	failOn   string
	next     ShaderHandle
	rebinds  [][2]ShaderHandle
	released []ShaderHandle
}

func (f *fakeCompiler) CompileShader(_ string, src []byte) (ShaderHandle, error) {
	if string(src) == f.failOn {
		return 0, errors.New("compile error")
	}
	f.next++
	return f.next, nil
}
func (f *fakeCompiler) RebindMaterials(o, n ShaderHandle) {
	f.rebinds = append(f.rebinds, [2]ShaderHandle{o, n})
}
func (f *fakeCompiler) ReleaseShader(h ShaderHandle) { f.released = append(f.released, h) }

type captureSink struct {
	errs     []string
	reloaded []string
}

func (c *captureSink) ShaderError(p, _ string) { c.errs = append(c.errs, p) }
func (c *captureSink) ShaderReloaded(p string) { c.reloaded = append(c.reloaded, p) }

func TestShaderSwapDoubleBuffered(t *testing.T) {
	t.Parallel()
	fc := &fakeCompiler{}
	sink := &captureSink{}
	r := NewShaderReloader(fc, sink)

	// First compile: handle 1 bound, no rebind/pending (nothing to replace).
	r.Swap("a.frag", []byte("v1"))
	if h, ok := r.Active("a.frag"); !ok || h != 1 {
		t.Fatalf("after first swap active = %d,%v want 1,true", h, ok)
	}
	if len(fc.rebinds) != 0 || len(r.pending) != 0 {
		t.Errorf("first swap should not rebind/queue: rebinds=%v pending=%v", fc.rebinds, r.pending)
	}

	// Second compile: handle 2 bound, materials rebound 1→2, old handle queued.
	r.Swap("a.frag", []byte("v2"))
	if h, _ := r.Active("a.frag"); h != 2 {
		t.Errorf("after second swap active = %d, want 2", h)
	}
	if len(fc.rebinds) != 1 || fc.rebinds[0] != [2]ShaderHandle{1, 2} {
		t.Errorf("rebinds = %v, want [[1 2]]", fc.rebinds)
	}
	// Double-buffered: old handle not released until the frame completes.
	if len(fc.released) != 0 {
		t.Errorf("old handle released mid-frame: %v", fc.released)
	}
	r.ReleasePending()
	if len(fc.released) != 1 || fc.released[0] != 1 {
		t.Errorf("released = %v, want [1] after frame", fc.released)
	}
	if len(sink.reloaded) != 2 {
		t.Errorf("reloaded notifications = %v, want 2", sink.reloaded)
	}
}

func TestShaderCompileErrorKeepsOld(t *testing.T) {
	t.Parallel()
	fc := &fakeCompiler{failOn: "bad"}
	sink := &captureSink{}
	r := NewShaderReloader(fc, sink)
	r.Swap("a.frag", []byte("good")) // handle 1
	r.Swap("a.frag", []byte("bad"))  // compile fails → keep handle 1 (INV-3)

	if h, _ := r.Active("a.frag"); h != 1 {
		t.Errorf("after failed compile active = %d, want 1 (old kept)", h)
	}
	if len(sink.errs) != 1 || sink.errs[0] != "a.frag" {
		t.Errorf("ShaderError not emitted: %v", sink.errs)
	}
	if len(r.pending) != 0 {
		t.Errorf("failed compile must not queue a release: %v", r.pending)
	}
}

// ─── change-scope classifier ─────────────────────────────────────────────────

func TestClassifyChange(t *testing.T) {
	t.Parallel()
	base := `package p
type Health struct { HP int }
func Update() { x := 1; _ = x }`

	bodyOnly := `package p
type Health struct { HP int }
func Update() { y := 2; _ = y }` // only the func body changed

	fieldChanged := `package p
type Health struct { HP int; Armor int }
func Update() { x := 1; _ = x }` // struct field added

	typeAdded := `package p
type Health struct { HP int }
type Mana struct { MP int }
func Update() { x := 1; _ = x }`

	if got := ClassifyChange([]byte(base), []byte(bodyOnly)); got != ScopeSystemOnly {
		t.Errorf("func-body change = %v, want SystemOnly", got)
	}
	if got := ClassifyChange([]byte(base), []byte(fieldChanged)); got != ScopeComponentType {
		t.Errorf("struct-field change = %v, want ComponentType", got)
	}
	if got := ClassifyChange([]byte(base), []byte(typeAdded)); got != ScopeComponentType {
		t.Errorf("type-added change = %v, want ComponentType", got)
	}
	// Parse failure falls back to SystemOnly (safest common case).
	if got := ClassifyChange([]byte(base), []byte("not valid go!!!")); got != ScopeSystemOnly {
		t.Errorf("parse failure = %v, want SystemOnly fallback", got)
	}
	if ScopeComponentType.String() != "ComponentType" || ScopeSystemOnly.String() != "SystemOnly" {
		t.Error("ChangeScope.String wrong")
	}
}

// ─── orchestrator routing + build seam ───────────────────────────────────────

func TestRouteFile(t *testing.T) {
	t.Parallel()
	cases := map[string]ReloadMode{
		"sys.go":     ModeCodeRestart,
		"pbr.frag":   ModeShaderSwap,
		"main.wgsl":  ModeShaderSwap,
		"scene.json": ModeDataReload,
		"hero.png":   ModeDataReload,
		"notes.txt":  ModeNone,
		"Makefile":   ModeNone,
	}
	for path, want := range cases {
		if got := RouteFile(path); got != want {
			t.Errorf("RouteFile(%q) = %v, want %v", path, got, want)
		}
	}
	if ModeCodeRestart.String() != "CodeRestart" || ModeNone.String() != "None" {
		t.Error("ReloadMode.String wrong")
	}
}

func TestOrchestratorBuildSeam(t *testing.T) {
	t.Parallel()
	o := NewReloadOrchestrator([]string{"go", "build", "./..."}, ".hot-reload")
	if o.Route("x.go") != ModeCodeRestart {
		t.Error("Route delegation broken")
	}
	// Inject a fake build to avoid spawning a compiler.
	called := false
	o.WithBuildFunc(func() error { called = true; return nil })
	if err := o.Rebuild(); err != nil || !called {
		t.Errorf("Rebuild via injected seam: err=%v called=%v", err, called)
	}
	o.WithBuildFunc(func() error { return errors.New("compile failed") })
	if err := o.Rebuild(); err == nil {
		t.Error("Rebuild should surface a build error")
	}
}

// ─── plugin + launch flag ────────────────────────────────────────────────────

type recBuilder struct {
	w    *world.World
	last []string
}

func (b *recBuilder) World() *world.World { return b.w }
func (b *recBuilder) AddSystem(s string, sys scheduler.System) appface.Builder {
	if s == appface.Last {
		b.last = append(b.last, sys.Name())
	}
	return b
}
func (b *recBuilder) AddSystems(s string, ss ...scheduler.System) appface.Builder {
	for _, sy := range ss {
		b.AddSystem(s, sy)
	}
	return b
}
func (b *recBuilder) SetResource(any) appface.Builder                { return b }
func (b *recBuilder) InitResource(any) appface.Builder               { return b }
func (b *recBuilder) AddPlugin(appface.Plugin) appface.Builder       { return b }
func (b *recBuilder) AddPlugins(appface.PluginGroup) appface.Builder { return b }

func TestHotReloadPluginRegistersShaderRelease(t *testing.T) {
	t.Parallel()
	b := &recBuilder{w: world.NewWorld()}
	HotReloadPlugin{Shader: NewShaderReloader(&fakeCompiler{}, nil)}.Build(b)
	if len(b.last) != 1 || b.last[0] != "hotreload.ReleaseShaders" {
		t.Errorf("Last systems = %v, want [hotreload.ReleaseShaders]", b.last)
	}
	// nil Shader → no system registered.
	b2 := &recBuilder{w: world.NewWorld()}
	HotReloadPlugin{}.Build(b2)
	if len(b2.last) != 0 {
		t.Errorf("nil Shader should register nothing, got %v", b2.last)
	}
}

func TestSnapshotPathFromArgs(t *testing.T) {
	t.Parallel()
	if p, ok := SnapshotPathFromArgs([]string{"--x", "--hot-reload=/tmp/s.bin", "--y"}); !ok || p != "/tmp/s.bin" {
		t.Errorf("got %q,%v want /tmp/s.bin,true", p, ok)
	}
	if _, ok := SnapshotPathFromArgs([]string{"--other"}); ok {
		t.Error("absent flag should report false")
	}
}
