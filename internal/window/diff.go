package window

import pkgwindow "github.com/neuengine/neu/pkg/window"

// DiffWindow computes the set of changed fields between a window's previous and
// current state. Only changed fields get a non-nil pointer, so an unchanged
// window yields a diff whose HasChanges() is false and the sync system skips
// the ApplyChanges call entirely (INV-4). Focused is excluded: it is
// event-driven (read-only), not a user mutation the backend applies.
func DiffWindow(prev, cur pkgwindow.Window) pkgwindow.WindowDiff {
	var d pkgwindow.WindowDiff
	if prev.Title != cur.Title {
		v := cur.Title
		d.Title = &v
	}
	if prev.Mode != cur.Mode {
		v := cur.Mode
		d.Mode = &v
	}
	if prev.Resolution != cur.Resolution {
		v := cur.Resolution
		d.Resolution = &v
	}
	if prev.Position != cur.Position {
		v := cur.Position
		d.Position = &v
	}
	if prev.Cursor != cur.Cursor {
		v := cur.Cursor
		d.Cursor = &v
	}
	if prev.PresentMode != cur.PresentMode {
		v := cur.PresentMode
		d.PresentMode = &v
	}
	if prev.Visible != cur.Visible {
		v := cur.Visible
		d.Visible = &v
	}
	if prev.Decorations != cur.Decorations {
		v := cur.Decorations
		d.Decorations = &v
	}
	return d
}
