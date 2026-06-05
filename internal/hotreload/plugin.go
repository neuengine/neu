//go:build editor

package hotreload

import (
	"strings"

	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
)

// hotReloadFlagPrefix is the launch flag the orchestrator passes to a restored
// process: ./{binary} --hot-reload={snapshot}.
const hotReloadFlagPrefix = "--hot-reload="

// HotReloadPlugin installs in-process shader hot-swap into the App. The code
// hot-restart cycle itself is driven out-of-process by the editor/daemon
// orchestrator (which snapshots, rebuilds, and relaunches); this plugin wires
// the shader reloader's per-frame deferred-release point so a swapped-out
// program is freed after the frame completes, not mid-frame (INV-3).
//
// The whole package is //go:build editor, so none of this exists in a
// production build (INV-4 is structural, enforced by TestNoHotReloadInRelease).
type HotReloadPlugin struct {
	// Shader, when non-nil, drives in-process shader hot-swap; its pending
	// releases are drained once per frame in the Last schedule.
	Shader *ShaderReloader
}

// Build implements appface.Plugin.
func (p HotReloadPlugin) Build(app appface.Builder) {
	if p.Shader != nil {
		app.AddSystem(appface.Last, scheduler.NewFuncSystem("hotreload.ReleaseShaders", func(*world.World) {
			p.Shader.ReleasePending()
		}))
	}
}

// SnapshotPathFromArgs scans args for the `--hot-reload=<path>` launch flag and
// returns the snapshot path and whether it was present. A restored process uses
// this before entering the main loop to detect that it should restore state
// rather than start fresh (l1-hot-reload §4.2 Phase 4).
func SnapshotPathFromArgs(args []string) (string, bool) {
	for _, a := range args {
		if strings.HasPrefix(a, hotReloadFlagPrefix) {
			return strings.TrimPrefix(a, hotReloadFlagPrefix), true
		}
	}
	return "", false
}
