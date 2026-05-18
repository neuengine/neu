//go:build dev

package asset

import (
	"io/fs"
	"sync"
	"time"
)

// fileWatcher implements a poll-based mtime watcher for dev builds.
// It periodically checks recorded files for modification time changes and
// calls a reload hook when a change is detected. No OS inotify binding is
// used — this preserves C-003 (stdlib-only) at the cost of ~poll-interval
// latency (§7 trade-off documented in l2-asset-system-go.md).
type fileWatcher struct {
	mu       sync.Mutex
	watched  map[string]time.Time // path → last-seen mtime
	fsys     fs.FS
	onReload func(path string) // called when mtime changes
	ticker   *time.Ticker
	done     chan struct{}
}

// newFileWatcher returns a watcher that polls at the given interval.
// onReload is called (on the poll goroutine) for each changed path.
func newFileWatcher(fsys fs.FS, interval time.Duration, onReload func(string)) *fileWatcher {
	w := &fileWatcher{
		watched:  make(map[string]time.Time),
		fsys:     fsys,
		onReload: onReload,
		ticker:   time.NewTicker(interval),
		done:     make(chan struct{}),
	}
	go w.loop()
	return w
}

// Watch registers path for polling. Subsequent polls compare its mtime to the
// value recorded here. If path does not exist yet it is silently ignored on
// the next poll until it appears.
func (w *fileWatcher) Watch(path string) {
	w.mu.Lock()
	if _, ok := w.watched[path]; !ok {
		mtime := w.stat(path)
		w.watched[path] = mtime
	}
	w.mu.Unlock()
}

// Unwatch removes path from polling.
func (w *fileWatcher) Unwatch(path string) {
	w.mu.Lock()
	delete(w.watched, path)
	w.mu.Unlock()
}

// Stop shuts down the poll goroutine.
func (w *fileWatcher) Stop() {
	w.ticker.Stop()
	close(w.done)
}

func (w *fileWatcher) loop() {
	for {
		select {
		case <-w.done:
			return
		case <-w.ticker.C:
			w.poll()
		}
	}
}

func (w *fileWatcher) poll() {
	w.mu.Lock()
	var changed []string
	for path, prev := range w.watched {
		cur := w.stat(path)
		if !cur.IsZero() && cur != prev {
			w.watched[path] = cur
			changed = append(changed, path)
		}
	}
	w.mu.Unlock()

	for _, path := range changed {
		w.onReload(path)
	}
}

// stat returns the modification time of path in fsys, or the zero Time on
// any error (file absent, permission denied, etc.).
func (w *fileWatcher) stat(path string) time.Time {
	info, err := fs.Stat(w.fsys, path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
