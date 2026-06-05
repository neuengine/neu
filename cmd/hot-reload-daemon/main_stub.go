//go:build !editor

package main

import (
	"fmt"
	"os"
)

// main is the production-build stub: hot-reload is a development-only feature
// gated behind the editor build tag (l1-hot-reload INV-4). It keeps
// `go build ./...` green without linking any hot-reload machinery.
func main() {
	fmt.Fprintln(os.Stderr, "hot-reload-daemon requires the 'editor' build tag")
	os.Exit(1)
}
