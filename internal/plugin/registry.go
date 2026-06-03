package plugin

import (
	"sync"

	"github.com/neuengine/neu/pkg/app/appface"
	pkgplugin "github.com/neuengine/neu/pkg/plugin"
)

// globalRegistry holds the compile-time in-process factory registrations.
// Go cannot look up a function by name at runtime (no .so, per L1 §2), so the
// host application must call RegisterFactory for each in-process plugin it
// links (typically from an init() or an explicit bootstrap function). T-6N03b.
var (
	globalRegistryMu sync.RWMutex
	globalFactories  = map[pkgplugin.PluginID]func() appface.Plugin{}
)

// RegisterFactory records factory as the constructor for the in-process plugin
// with the given id. Safe for concurrent use; later registrations overwrite
// earlier ones (host app controls link order).
func RegisterFactory(id pkgplugin.PluginID, factory func() appface.Plugin) {
	globalRegistryMu.Lock()
	globalFactories[id] = factory
	globalRegistryMu.Unlock()
}

// LookupFactory returns the registered constructor for id (ok=false if absent).
func LookupFactory(id pkgplugin.PluginID) (func() appface.Plugin, bool) {
	globalRegistryMu.RLock()
	f, ok := globalFactories[id]
	globalRegistryMu.RUnlock()
	return f, ok
}

// resetFactories clears the global registry. Tests only.
func resetFactories() {
	globalRegistryMu.Lock()
	globalFactories = map[pkgplugin.PluginID]func() appface.Plugin{}
	globalRegistryMu.Unlock()
}
