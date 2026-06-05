//go:build editor

package hotreload

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"time"
)

// ReloadMode is the action a changed file triggers (l1-hot-reload §4.6).
type ReloadMode uint8

const (
	// ModeNone: the file is not watched.
	ModeNone ReloadMode = iota
	// ModeCodeRestart: Go source changed → process-restart with snapshot.
	ModeCodeRestart
	// ModeShaderSwap: shader source changed → in-process hot-swap.
	ModeShaderSwap
	// ModeDataReload: data asset changed → Asset System hot-reload.
	ModeDataReload
)

// String returns the mode name.
func (m ReloadMode) String() string {
	switch m {
	case ModeCodeRestart:
		return "CodeRestart"
	case ModeShaderSwap:
		return "ShaderSwap"
	case ModeDataReload:
		return "DataReload"
	default:
		return "None"
	}
}

// RouteFile maps a changed file path to its reload mode by extension. Unknown
// extensions return ModeNone.
func RouteFile(path string) ReloadMode {
	switch filepath.Ext(path) {
	case ".go":
		return ModeCodeRestart
	case ".glsl", ".vert", ".frag", ".comp", ".wgsl", ".spv":
		return ModeShaderSwap
	case ".json", ".png", ".jpg", ".ogg", ".wav", ".gltf", ".glb":
		return ModeDataReload
	default:
		return ModeNone
	}
}

// BuildFunc runs a rebuild and returns an error (with build output) on failure.
// The default uses `go build` via os/exec; tests inject a fake.
type BuildFunc func() error

// ReloadOrchestrator coordinates the code-restart cycle: it routes file changes,
// debounces rapid saves, and runs the rebuild. The FileWatcher loop and the
// engine IPC live in the editor/daemon host that drives this coordinator; the
// build step is an injected seam so it is testable without spawning a compiler.
type ReloadOrchestrator struct {
	build       BuildFunc
	SnapshotDir string
	Debounce    time.Duration
}

// NewReloadOrchestrator returns an orchestrator that rebuilds via `go build`
// with the given command words (e.g. []string{"go","build","-trimpath","-o",bin,target}).
// snapshotDir is where the engine writes the state snapshot.
func NewReloadOrchestrator(buildCommand []string, snapshotDir string) *ReloadOrchestrator {
	return &ReloadOrchestrator{
		build:       goBuild(buildCommand),
		SnapshotDir: snapshotDir,
		Debounce:    200 * time.Millisecond,
	}
}

// WithBuildFunc overrides the build step (for tests or a custom toolchain).
func (o *ReloadOrchestrator) WithBuildFunc(fn BuildFunc) *ReloadOrchestrator {
	if fn != nil {
		o.build = fn
	}
	return o
}

// Route classifies a changed file.
func (o *ReloadOrchestrator) Route(path string) ReloadMode { return RouteFile(path) }

// Rebuild runs the configured build step. A non-nil error means the new binary
// was not produced — the orchestrator host shows the error and offers a fallback
// (l1-hot-reload §4.9); it does NOT proceed to launch.
func (o *ReloadOrchestrator) Rebuild() error {
	if o.build == nil {
		return nil
	}
	return o.build()
}

// goBuild returns a BuildFunc that runs the given command, capturing combined
// output so a compile error surfaces with its messages.
func goBuild(command []string) BuildFunc {
	if len(command) == 0 {
		return func() error { return nil }
	}
	return func() error {
		cmd := exec.Command(command[0], command[1:]...) //nolint:gosec // dev-only, editor-tagged
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("hotreload: build failed: %w\n%s", err, out)
		}
		return nil
	}
}
