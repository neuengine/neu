package asset

import (
	"errors"
	"io/fs"
	"slices"
	"strings"
)

var (
	ErrReadOnlyMount = errors.New("asset: mount is read-only")
	ErrNoMount       = errors.New("asset: no mount covers path")
)

// mount associates a path prefix with a backing fs.FS provider.
type mount struct {
	prefix string
	fsys   fs.FS
	rw     bool
}

// VFS is a virtual file system built from an ordered list of mounts. Later-
// registered mounts with overlapping prefixes shadow earlier ones (priority
// layering). All paths use forward slashes; the "vfs:///" scheme is stripped
// before resolution.
type VFS struct {
	mounts []mount
}

// NewVFS returns an empty VFS. Use Mount to add providers.
func NewVFS() *VFS { return &VFS{} }

// Mount registers fsys under prefix. Later mounts shadow earlier ones for
// the same prefix. Prefix must end with "/" (e.g. "/engine/", "/app/").
func (v *VFS) Mount(prefix string, fsys fs.FS, rw bool) {
	v.mounts = append(v.mounts, mount{prefix: prefix, fsys: fsys, rw: rw})
}

// Open resolves vpath to the first matching mount (highest priority = latest
// registered) and opens the file. vpath may start with "vfs:///" which is
// stripped before matching.
func (v *VFS) Open(vpath string) (fs.File, error) {
	vpath = stripScheme(vpath)
	// Scan in reverse — latest mount wins.
	for _, m := range slices.Backward(v.mounts) {

		rel, ok := cutPrefix(vpath, m.prefix)
		if !ok {
			continue
		}
		if rel == "" {
			rel = "."
		}
		f, err := m.fsys.Open(rel)
		if errors.Is(err, fs.ErrNotExist) {
			continue // try next lower-priority mount
		}
		return f, err
	}
	return nil, &fs.PathError{Op: "open", Path: vpath, Err: fs.ErrNotExist}
}

// stripScheme removes the "vfs:///" prefix if present.
func stripScheme(p string) string {
	if s, ok := strings.CutPrefix(p, "vfs:///"); ok {
		return "/" + s
	}
	return p
}

// cutPrefix checks if p begins with prefix and returns the remainder.
func cutPrefix(p, prefix string) (string, bool) {
	if !strings.HasPrefix(p, prefix) {
		return "", false
	}
	return p[len(prefix):], true
}
