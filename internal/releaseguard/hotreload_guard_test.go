// Package releaseguard holds architecture guards that must run in the DEFAULT
// (production) build — i.e. without the editor build tag. They cannot live in
// the editor-tagged packages they police, since those compile to nothing here.
package releaseguard

import (
	"os/exec"
	"strings"
	"testing"
)

// TestNoHotReloadInRelease enforces l1-hot-reload INV-4 structurally: the
// internal/hotreload package must contribute ZERO Go files to a production
// (non-editor) build. `go list` without the editor tag must therefore report
// either no buildable files or a build-constraints exclusion — never a populated
// GoFiles set. This catches a future file in that package that forgets its
// //go:build editor constraint.
func TestNoHotReloadInRelease(t *testing.T) {
	t.Parallel()
	out, err := exec.Command("go", "list", "-f", "{{len .GoFiles}}",
		"github.com/neuengine/neu/internal/hotreload").CombinedOutput()
	got := strings.TrimSpace(string(out))
	if err != nil {
		// The expected, healthy state: all files are editor-gated, so a default
		// build has nothing to compile.
		if strings.Contains(got, "build constraints exclude all Go files") {
			return
		}
		t.Fatalf("go list failed unexpectedly: %v\n%s", err, got)
	}
	if got != "0" {
		t.Fatalf("internal/hotreload exposes %s Go files in a non-editor build; "+
			"every file must carry //go:build editor (INV-4)", got)
	}
}

// TestHotReloadDaemonStubInRelease confirms the daemon command still builds in a
// production build (via its //go:build !editor stub) so `go build ./...` stays
// green, while its editor entry point is excluded.
func TestHotReloadDaemonStubInRelease(t *testing.T) {
	t.Parallel()
	out, err := exec.Command("go", "list", "-f", "{{.Name}}",
		"github.com/neuengine/neu/cmd/hot-reload-daemon").CombinedOutput()
	got := strings.TrimSpace(string(out))
	if err != nil {
		t.Fatalf("daemon must remain buildable in production via its stub: %v\n%s", err, got)
	}
	if got != "main" {
		t.Fatalf("daemon package name = %q, want main", got)
	}
}
