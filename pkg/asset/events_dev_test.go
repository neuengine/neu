//go:build dev

package asset

import (
	"io/fs"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/neuengine/neu/pkg/task"
)

// slashSyncFS is a race-safe fs.FS that tolerates a leading "/" in names, so the
// reload watcher and the AssetServer share one path string (the watcher stats
// "/a.txt"; the VFS mount at "/" and the loaded[] key use the same vpath).
type slashSyncFS struct {
	mu sync.RWMutex
	m  fstest.MapFS
}

func (s *slashSyncFS) Open(name string) (fs.File, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.m.Open(strings.TrimPrefix(name, "/"))
}

func (s *slashSyncFS) set(name string, f *fstest.MapFile) {
	s.mu.Lock()
	s.m[strings.TrimPrefix(name, "/")] = f
	s.mu.Unlock()
}

// TestNewReloadWatcherEmitsOnFileChange proves the full dev chain:
// file mtime change → fileWatcher → dispatchReload → Reload[A] → AssetEvent emit.
func TestNewReloadWatcherEmitsOnFileChange(t *testing.T) {
	base := time.Now().Truncate(time.Second)
	sfs := &slashSyncFS{m: fstest.MapFS{"a.txt": {Data: []byte("v1"), ModTime: base}}}
	vfs := NewVFS()
	vfs.Mount("/", sfs, false)
	_, io := task.NewTaskPools(task.TaskPoolConfig{})
	t.Cleanup(func() { io.Shutdown() })
	srv := NewAssetServer(vfs, io)
	RegisterLoader[string, struct{}](srv, &textLoader{})

	var events atomic.Int64
	WatchReloads[string](srv, func(AssetEvent[string]) { events.Add(1) })

	h := Load[string](srv, "/a.txt") // records loaded["/a.txt"]
	drainLoad(srv, h, time.Second)

	w := srv.NewReloadWatcher(sfs, 15*time.Millisecond)
	defer w.Stop()
	w.Watch("/a.txt")

	sfs.set("/a.txt", &fstest.MapFile{Data: []byte("v2"), ModTime: base.Add(time.Second)})

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if events.Load() > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("reload watcher did not emit an AssetEvent within 1s of an mtime change")
}
