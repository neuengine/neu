package plugin

import (
	"fmt"
	"io/fs"
	"path"

	pkgplugin "github.com/neuengine/neu/pkg/plugin"
)

// DiscoveredPlugin is a plugin a [Source] found: its parsed manifest plus the
// directory it lives in (for resolving the entry binary / checksum on load) and
// the originating source name (for the manager's audit log).
type DiscoveredPlugin struct {
	Manifest pkgplugin.Manifest
	Dir      string
	Source   string
}

// Source enumerates the plugins available from one location. The engine scans
// several (e.g. bundled, user, project, and an env-configured path — the L1
// "4 sources"); each is just a Source, so the count is configuration, not code.
type Source interface {
	// Name identifies the source in audit logs.
	Name() string
	// Discover returns the plugins found and a (possibly empty) list of per-
	// manifest errors. A single malformed manifest must not abort the scan, so
	// errors are collected rather than returned as the sole result.
	Discover() (found []DiscoveredPlugin, errs []error)
}

// DirSource discovers plugins by walking an [fs.FS] for `plugin.toml` manifests
// — one plugin per directory that contains one. Backing the source with fs.FS
// (not a raw path) keeps it stdlib-only and unit-testable with fstest.MapFS.
type DirSource struct {
	SourceName string
	FS         fs.FS
}

// Name implements [Source].
func (d DirSource) Name() string { return d.SourceName }

// Discover implements [Source]: it walks d.FS, parses every `plugin.toml`, and
// collects parse failures without aborting the rest of the scan.
func (d DirSource) Discover() (found []DiscoveredPlugin, errs []error) {
	walkErr := fs.WalkDir(d.FS, ".", func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			errs = append(errs, err)
			return nil // keep scanning siblings
		}
		if entry.IsDir() || entry.Name() != "plugin.toml" {
			return nil
		}
		data, err := fs.ReadFile(d.FS, p)
		if err != nil {
			errs = append(errs, fmt.Errorf("plugin: read %s: %w", p, err))
			return nil
		}
		man, err := pkgplugin.ParseManifest(data)
		if err != nil {
			errs = append(errs, fmt.Errorf("plugin: parse %s: %w", p, err))
			return nil
		}
		found = append(found, DiscoveredPlugin{Manifest: man, Dir: path.Dir(p), Source: d.SourceName})
		return nil
	})
	if walkErr != nil {
		errs = append(errs, walkErr)
	}
	return found, errs
}
