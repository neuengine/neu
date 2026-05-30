package platform

import (
	"github.com/neuengine/neu/pkg/app/appface"
	pkgplatform "github.com/neuengine/neu/pkg/platform"
)

// Plugin inserts the immutable PlatformProfile resource so systems can query
// platform capabilities from the first frame (INV-2). It is the concrete,
// build-tag-selected PlatformPlugin that DefaultPlugins adds automatically
// (INV-3): the default and headless builds link different NewPlatformProfile
// implementations, but the plugin wiring is identical.
type Plugin struct{}

// New returns the platform plugin for the current build.
func New() *Plugin { return &Plugin{} }

// Build inserts the profile as a resource. SetResource at build time makes the
// profile available before the first schedule run; it is never re-inserted, so
// downstream systems read an immutable value (INV-2).
func (Plugin) Build(app appface.Builder) {
	app.SetResource(NewPlatformProfile())
}

var (
	_ appface.Plugin             = (*Plugin)(nil)
	_ pkgplatform.PlatformPlugin = (*Plugin)(nil)
)
