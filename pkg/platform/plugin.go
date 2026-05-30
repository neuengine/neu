package platform

import "github.com/neuengine/neu/pkg/app/appface"

// PlatformPlugin wires all platform-specific backends and inserts the
// PlatformProfile resource. It is an appface.Plugin; DefaultPlugins selects the
// correct concrete implementation per build tags (INV-3). The concrete plugin
// lives in internal/platform.
type PlatformPlugin interface {
	appface.Plugin
}
