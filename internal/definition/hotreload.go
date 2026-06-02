package definition

import (
	"io"
	"log/slog"

	iasset "github.com/neuengine/neu/internal/asset"
	"github.com/neuengine/neu/internal/ecs/event"
	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
	"github.com/neuengine/neu/pkg/asset"
	def "github.com/neuengine/neu/pkg/definition"
)

// DefinitionLoader is an asset.AssetLoader that decodes (and validates) a
// definition. Registering it with the AssetServer lets definitions participate
// in hot-reload: a file change drives asset.Reload, which fires
// asset.AssetEvent[def.Definition] on the bus.
//
// It handles ".json" (the AssetServer keys on path.Ext); compound-extension
// routing — .scene.json vs .ui.json — is a deferred AssetServer enhancement, so
// an app currently registers one ".json" loader.
type DefinitionLoader struct {
	Types   def.TypeResolver
	Actions *def.ActionRegistry
}

// Extensions implements asset.AssetLoader.
func (DefinitionLoader) Extensions() []string { return []string{".json"} }

// Load implements asset.AssetLoader.
func (l DefinitionLoader) Load(r io.Reader, _ struct{}) (def.Definition, error) {
	return def.Load(r, l.Types, l.Actions)
}

// DecodeFunc re-decodes the definition at path to its current value. The app
// wires it to read its VFS (def.Load); tests inject a stub. It is the *value*
// source for hot-reload — the AssetEvent only signals *that* a path changed, so
// sourcing the value here sidesteps the asset store's async-decode timing.
type DecodeFunc func(path string) (*def.Definition, error)

// applyDefinitionReloads drains the definition AssetEvents the reader has seen
// and, for each Modified, re-decodes the path and re-instantiates it through the
// store (despawning the prior entities). Decode / realization errors are logged,
// never fatal — a bad reload must not halt the frame. The IsEmpty gate keeps the
// quiet-frame path allocation-free (C-004).
func applyDefinitionReloads(
	w *world.World,
	reader *event.EventReader[asset.AssetEvent[def.Definition]],
	store *InstanceStore,
	decode DecodeFunc,
	realize TypeResolver,
) {
	if reader.IsEmpty() {
		return
	}
	for ev := range reader.All() {
		if ev.Kind != asset.AssetModified {
			continue
		}
		d, err := decode(ev.Path)
		if err != nil {
			slog.Error("definition: hot-reload decode failed", "path", ev.Path, "error", err)
			continue
		}
		if _, errs := store.Reload(w, ev.Path, d, realize); len(errs) > 0 {
			slog.Error("definition: hot-reload re-instantiation had errors", "path", ev.Path, "errors", errs)
		}
	}
}

// NewHotReloadSystem returns the PreUpdate system that turns a definition
// AssetEvent[Modified] into an InstanceStore.Reload. The reader is created lazily
// on the first frame the bus exists, so the system is a no-op until something
// registers asset.AssetEvent[def.Definition] (via WatchAssetType).
func NewHotReloadSystem(store *InstanceStore, decode DecodeFunc, realize TypeResolver) scheduler.System {
	var reader *event.EventReader[asset.AssetEvent[def.Definition]]
	return scheduler.NewFuncSystem("definition.HotReload", func(w *world.World) {
		if reader == nil {
			if event.Bus[asset.AssetEvent[def.Definition]](w) == nil {
				return
			}
			reader = event.NewEventReader[asset.AssetEvent[def.Definition]](w)
		}
		applyDefinitionReloads(w, reader, store, decode, realize)
	})
}

// HotReloadPlugin wires definition hot-reload into the App: it registers the
// DefinitionLoader with the asset server, opens the asset.AssetEvent[Definition]
// bus (WatchAssetType), publishes the InstanceStore as a resource, and adds the
// PreUpdate hot-reload system. The app supplies Decode (the value source, wired
// to its VFS) and the realization resolver. Auto file-watching is started
// separately on the server (dev builds: AssetServer.NewReloadWatcher).
type HotReloadPlugin struct {
	Server  *asset.AssetServer
	Types   def.TypeResolver    // validation-time (DefinitionLoader)
	Actions *def.ActionRegistry // validation-time (DefinitionLoader)
	Decode  DecodeFunc          // realization value source
	Realize TypeResolver        // realization-time name→Go type
	Store   *InstanceStore      // nil ⇒ a fresh store is created
}

// Build implements appface.Plugin.
func (p HotReloadPlugin) Build(b appface.Builder) {
	w := b.World()
	if p.Server != nil {
		asset.RegisterLoader[def.Definition, struct{}](p.Server, DefinitionLoader{Types: p.Types, Actions: p.Actions})
		iasset.WatchAssetType[def.Definition](p.Server, w)
	}
	store := p.Store
	if store == nil {
		store = NewInstanceStore()
	}
	b.SetResource(store)
	b.AddSystem(appface.PreUpdate, NewHotReloadSystem(store, p.Decode, p.Realize))
}
