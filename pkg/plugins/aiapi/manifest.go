//go:build editor

package aiapi

import _ "embed"

// manifestTOML is the plugin's manifest, embedded at build time so the binary is
// self-describing (L1 §4.2). The host loader parses it with pkg/plugin.ParseManifest.
//
//go:embed plugin.toml
var manifestTOML []byte

// Manifest returns the embedded plugin.toml bytes. The host plugin loader feeds
// these to pkg/plugin.ParseManifest + Validate before instantiating the plugin.
func Manifest() []byte { return manifestTOML }
