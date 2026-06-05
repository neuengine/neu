//go:build editor

// Command hot-reload-daemon is the standalone, editor-less orchestrator host for
// the hot-reload code-restart cycle (l1-hot-reload §4.7). It watches a project,
// rebuilds on change, and drives the snapshot/restore handshake with the running
// engine. This entry point warms the build cache and validates the build
// command; the continuous file-watch + engine-IPC loop is wired by the host that
// embeds the orchestrator (the dev FileWatcher is build-tag-scoped).
//
// Editor-only: the //go:build !editor stub in main_stub.go keeps `go build ./...`
// green in production builds while excluding the daemon entirely (INV-4).
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/neuengine/neu/internal/hotreload"
)

func main() {
	build := flag.String("build", "go build ./...", "build command run on each change")
	snapDir := flag.String("snapshot-dir", ".hot-reload", "directory for state snapshots")
	flag.Parse()

	orch := hotreload.NewReloadOrchestrator(strings.Fields(*build), *snapDir)
	fmt.Fprintf(os.Stderr, "hot-reload-daemon: build=%q snapshot-dir=%q\n", *build, *snapDir)
	fmt.Fprintln(os.Stderr, "warming build cache...")
	if err := orch.Rebuild(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "build OK — ready. (continuous watch loop is wired by the orchestrator host)")
}
