package definition

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/neuengine/neu/internal/ecs/event"
	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
	"github.com/neuengine/neu/pkg/asset"
	def "github.com/neuengine/neu/pkg/definition"
	"github.com/neuengine/neu/pkg/task"
)

func sceneResolver() TypeResolver {
	return realizeResolver(map[string]reflect.Type{
		"Position": reflect.TypeFor[Position](),
		"Label":    reflect.TypeFor[Label](),
	})
}

func sendDefEvent(w *world.World, kind asset.AssetEventKind, path string) {
	event.Bus[asset.AssetEvent[def.Definition]](w).Send(asset.AssetEvent[def.Definition]{Kind: kind, Path: path})
}

func TestHotReloadSystemReinstantiatesOnEvent(t *testing.T) {
	w := world.NewWorld()
	event.RegisterEvent[asset.AssetEvent[def.Definition]](w)
	resolve := sceneResolver()
	store := NewInstanceStore()

	d := decodeScene(t, []string{"Position", "Label"}, sceneJSON)
	in1, _ := store.Instantiate(w, "/s.json", &d, resolve)

	decode := func(string) (*def.Definition, error) {
		dd := decodeScene(t, []string{"Position", "Label"}, sceneJSON)
		return &dd, nil
	}
	sys := NewHotReloadSystem(store, decode, resolve)

	sendDefEvent(w, asset.AssetModified, "/s.json")
	sys.Run(w)

	for _, e := range in1.Entities {
		if w.Contains(e) {
			t.Errorf("old entity %v not despawned on reload", e)
		}
	}
	cur, ok := store.Get("/s.json")
	if !ok || len(cur.Entities) == 0 {
		t.Fatal("no live instance after reload")
	}
	for _, e := range cur.Entities {
		if !w.Contains(e) {
			t.Errorf("new entity %v not spawned on reload", e)
		}
	}
}

func TestHotReloadSystemNoBusNoOp(t *testing.T) {
	w := world.NewWorld() // no AssetEvent bus registered
	sys := NewHotReloadSystem(NewInstanceStore(),
		func(string) (*def.Definition, error) { return nil, nil }, realizeResolver(nil))
	sys.Run(w) // reader stays nil → must not panic
	sys.Run(w) // second frame still no bus
}

func TestApplyDefinitionReloadsSkipsNonModified(t *testing.T) {
	w := world.NewWorld()
	event.RegisterEvent[asset.AssetEvent[def.Definition]](w)
	reader := event.NewEventReader[asset.AssetEvent[def.Definition]](w)

	called := 0
	decode := func(string) (*def.Definition, error) { called++; return nil, nil }
	sendDefEvent(w, asset.AssetCreated, "/x.json") // not Modified
	applyDefinitionReloads(w, reader, NewInstanceStore(), decode, realizeResolver(nil))

	if called != 0 {
		t.Errorf("decode called %d times for a non-Modified event, want 0", called)
	}
}

func TestApplyDefinitionReloadsDecodeErrorSkips(t *testing.T) {
	w := world.NewWorld()
	event.RegisterEvent[asset.AssetEvent[def.Definition]](w)
	reader := event.NewEventReader[asset.AssetEvent[def.Definition]](w)
	store := NewInstanceStore()

	sendDefEvent(w, asset.AssetModified, "/x.json")
	applyDefinitionReloads(w, reader, store,
		func(string) (*def.Definition, error) { return nil, errors.New("boom") }, realizeResolver(nil))

	if _, ok := store.Get("/x.json"); ok {
		t.Error("a decode error must not create an instance")
	}
}

func TestDefinitionLoaderDecodes(t *testing.T) {
	l := DefinitionLoader{
		Types:   validatorResolver{known: map[string]bool{"Position": true, "Label": true}},
		Actions: def.NewActionRegistry(),
	}
	if exts := l.Extensions(); len(exts) != 1 || exts[0] != ".json" {
		t.Errorf("Extensions() = %v, want [.json]", exts)
	}
	d, err := l.Load(strings.NewReader(sceneJSON), struct{}{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if d.Kind != def.KindScene {
		t.Errorf("Kind = %v, want scene", d.Kind)
	}
}

// ── fake appface.Builder for plugin wiring ───────────────────────────────────

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
func (b *fakeBuilder) SetResource(any) appface.Builder              { b.resources++; return b }
func (b *fakeBuilder) InitResource(any) appface.Builder             { return b }
func (b *fakeBuilder) AddPlugin(appface.Plugin) appface.Builder     { return b }
func (b *fakeBuilder) AddPlugins(appface.PluginGroup) appface.Builder { return b }

var _ appface.Builder = (*fakeBuilder)(nil)

func TestHotReloadPluginBuildNoServer(t *testing.T) {
	b := newFakeBuilder()
	HotReloadPlugin{
		Decode:  func(string) (*def.Definition, error) { return nil, nil },
		Realize: realizeResolver(nil),
	}.Build(b)

	if names := b.systems[appface.PreUpdate]; len(names) != 1 || names[0] != "definition.HotReload" {
		t.Errorf("PreUpdate systems = %v, want [definition.HotReload]", names)
	}
	if b.resources != 1 {
		t.Errorf("SetResource calls = %d, want 1 (the InstanceStore)", b.resources)
	}
}

func TestHotReloadPluginBuildWithServer(t *testing.T) {
	vfs := asset.NewVFS()
	vfs.Mount("/", fstest.MapFS{}, false)
	_, io := task.NewTaskPools(task.TaskPoolConfig{})
	t.Cleanup(func() { io.Shutdown() })
	srv := asset.NewAssetServer(vfs, io)

	b := newFakeBuilder()
	HotReloadPlugin{
		Server:  srv,
		Types:   validatorResolver{known: map[string]bool{}},
		Actions: def.NewActionRegistry(),
		Decode:  func(string) (*def.Definition, error) { return nil, nil },
		Realize: realizeResolver(nil),
	}.Build(b)

	if event.Bus[asset.AssetEvent[def.Definition]](b.w) == nil {
		t.Error("Build with a server must register the AssetEvent[Definition] bus (WatchAssetType)")
	}
}
