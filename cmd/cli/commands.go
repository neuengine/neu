package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/neuengine/neu/pkg/plugin"
)

// engineVersion is the CLI's reported engine version (kept in sync with the
// engine release; a build-time injection replaces this later).
const engineVersion = "0.1.0"

// --- doctor ---

type doctorCmd struct{}

func (doctorCmd) Name() string { return "doctor" }
func (doctorCmd) Help() string { return "report environment, version, and workspace health" }

func (doctorCmd) Run(_ context.Context, _ []string, out *Output) error {
	info := map[string]string{
		"engine_version": engineVersion,
		"go_version":     runtime.Version(),
		"os":             runtime.GOOS,
		"arch":           runtime.GOARCH,
	}
	out.Linef("neuengine doctor")
	out.Linef("  engine:  %s", info["engine_version"])
	out.Linef("  go:      %s", info["go_version"])
	out.Linef("  os/arch: %s/%s", info["os"], info["arch"])
	return out.WriteJSON(info)
}

// --- scaffold ---

type scaffoldCmd struct{}

func (scaffoldCmd) Name() string { return "scaffold" }
func (scaffoldCmd) Help() string {
	return "generate a stub (component|system|plugin) — overwrite-safe"
}

func (scaffoldCmd) Run(_ context.Context, args []string, out *Output) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: scaffold <component|system|plugin> <path>")
	}
	target, path := args[0], args[1]
	content := scaffoldTemplate(target)
	if content == "" {
		return fmt.Errorf("unknown scaffold target %q (want component|system|plugin)", target)
	}
	return writeFileSafe(path, content, out)
}

// scaffoldTemplate returns the stub content for a target, or "" if unknown.
func scaffoldTemplate(target string) string {
	switch target {
	case "component":
		return "package game\n\n// TODO: component fields (pure data).\ntype Component struct{}\n"
	case "system":
		return "package game\n\n// TODO: system logic over a Query.\nfunc System() {}\n"
	case "plugin":
		return "[plugin]\nid = \"com.example.plugin\"\nversion = \"0.1.0\"\nmode = \"in-process\"\n\n[compatibility]\nengine_version = \"^0.1.0\"\n"
	default:
		return ""
	}
}

// writeFileSafe writes content to path, never silently overwriting (INV-1):
// an existing file is skipped unless --force; --dry-run previews only.
func writeFileSafe(path, content string, out *Output) error {
	if _, err := os.Stat(path); err == nil && !out.Force {
		out.Linef("skip: %s already exists (use --force to overwrite)", path)
		return nil
	}
	if out.Dry {
		out.Linef("would write %s (%d bytes)", path, len(content))
		return nil
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	out.Linef("wrote %s", path)
	return out.WriteJSON(map[string]string{"wrote": path})
}

// --- plugin (consumes the pkg/plugin SDK) ---

type pluginCmd struct{}

func (pluginCmd) Name() string { return "plugin" }
func (pluginCmd) Help() string { return "manage plugins: validate, list" }

func (pluginCmd) Run(_ context.Context, args []string, out *Output) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: plugin <validate|list> ...")
	}
	switch args[0] {
	case "validate":
		return pluginValidate(args[1:], out)
	case "list":
		return pluginList(args[1:], out)
	default:
		return fmt.Errorf("unknown plugin subcommand %q (want validate|list)", args[0])
	}
}

// pluginValidate parses + validates a plugin.toml (INV-1 of plugin distribution).
func pluginValidate(args []string, out *Output) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: plugin validate <path-to-plugin.toml>")
	}
	data, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	m, err := plugin.ParseManifest(data)
	if err != nil {
		return err
	}
	if err := m.Validate(); err != nil {
		return err
	}
	out.Linef("ok: %s v%s (%s)", m.ID, m.Version, m.Mode)
	return out.WriteJSON(map[string]string{"id": string(m.ID), "version": m.Version, "mode": m.Mode.String()})
}

// pluginList scans a directory for <sub>/plugin.toml and lists each plugin.
func pluginList(args []string, out *Output) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read dir: %w", err)
	}
	type item struct{ ID, Version, Mode string }
	var found []item
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		manifestPath := filepath.Join(dir, e.Name(), "plugin.toml")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue // dir without a manifest is skipped silently (L1 §4.4)
		}
		m, err := plugin.ParseManifest(data)
		if err != nil {
			continue
		}
		found = append(found, item{string(m.ID), m.Version, m.Mode.String()})
	}
	for _, it := range found {
		out.Linef("%s\tv%s\t%s", it.ID, it.Version, it.Mode)
	}
	if len(found) == 0 {
		out.Linef("no plugins found in %s", dir)
	}
	return out.WriteJSON(map[string]any{"plugins": found})
}
