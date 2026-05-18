//go:build dev

package asset

import (
	"io/fs"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"
)

// syncMapFS wraps fstest.MapFS with a mutex so concurrent watcher reads and
// test writes don't race. The race detector flags concurrent map ops on
// fstest.MapFS because it is a plain map — this wrapper serialises access.
type syncMapFS struct {
	mu sync.RWMutex
	m  fstest.MapFS
}

func (s *syncMapFS) Open(name string) (fs.File, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.m.Open(name)
}

func (s *syncMapFS) set(name string, f *fstest.MapFile) {
	s.mu.Lock()
	s.m[name] = f
	s.mu.Unlock()
}

func TestFileWatcherDetectsChange(t *testing.T) {
	baseTime := time.Now().Truncate(time.Second)
	sfs := &syncMapFS{m: fstest.MapFS{
		"asset.txt": {Data: []byte("v1"), ModTime: baseTime},
	}}

	var reloads atomic.Int64
	w := newFileWatcher(sfs, 20*time.Millisecond, func(string) { reloads.Add(1) })
	defer w.Stop()

	w.Watch("asset.txt")

	sfs.set("asset.txt", &fstest.MapFile{
		Data:    []byte("v2"),
		ModTime: baseTime.Add(time.Second),
	})

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if reloads.Load() > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("watcher did not detect mtime change within 500 ms")
}

func TestFileWatcherNoSpuriousReload(t *testing.T) {
	sfs := &syncMapFS{m: fstest.MapFS{
		"stable.txt": {Data: []byte("unchanged"), ModTime: time.Now()},
	}}

	var reloads atomic.Int64
	w := newFileWatcher(sfs, 20*time.Millisecond, func(string) { reloads.Add(1) })
	defer w.Stop()

	w.Watch("stable.txt")
	time.Sleep(100 * time.Millisecond)

	if n := reloads.Load(); n != 0 {
		t.Errorf("spurious reloads: got %d, want 0", n)
	}
}

func TestFileWatcherUnwatch(t *testing.T) {
	baseTime := time.Now()
	sfs := &syncMapFS{m: fstest.MapFS{
		"tmp.txt": {Data: []byte("a"), ModTime: baseTime},
	}}

	var reloads atomic.Int64
	w := newFileWatcher(sfs, 20*time.Millisecond, func(string) { reloads.Add(1) })
	defer w.Stop()

	w.Watch("tmp.txt")
	w.Unwatch("tmp.txt")

	sfs.set("tmp.txt", &fstest.MapFile{
		Data:    []byte("b"),
		ModTime: baseTime.Add(time.Second),
	})
	time.Sleep(100 * time.Millisecond)

	if n := reloads.Load(); n != 0 {
		t.Errorf("expected 0 reloads after Unwatch; got %d", n)
	}
}

func TestFileWatcherMissingFile(t *testing.T) {
	sfs := &syncMapFS{m: fstest.MapFS{}}
	w := newFileWatcher(sfs, 20*time.Millisecond, func(string) {})
	defer w.Stop()
	w.Watch("ghost.txt")
	time.Sleep(60 * time.Millisecond)
}
