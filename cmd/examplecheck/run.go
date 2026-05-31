package main

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// runExample executes `go run ./<root>/<name>` and returns its combined
// stdout+stderr plus the exec error. Combined output is intentional: a build
// failure surfaces on stderr while the PASS/hash line is on stdout.
func runExample(ctx context.Context, root, name string) (string, error) {
	pkg := "./" + filepath.ToSlash(filepath.Join(root, name))
	out, err := exec.CommandContext(ctx, "go", "run", pkg).CombinedOutput()
	return string(out), err
}

// gitChangedPaths returns the files changed since baseref (`git diff --name-only`).
func gitChangedPaths(ctx context.Context, baseref string) ([]string, error) {
	out, err := exec.CommandContext(ctx, "git", "diff", "--name-only", baseref).Output()
	if err != nil {
		return nil, fmt.Errorf("examplecheck: git diff: %w", err)
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil, nil
	}
	return strings.Split(trimmed, "\n"), nil
}
