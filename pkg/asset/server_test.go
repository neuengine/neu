package asset

import (
	"io"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/neuengine/neu/pkg/task"
)

// textLoader decodes a file as a string. Used throughout asset server tests.
type textLoader struct {
	calls atomic.Int64 // counts actual loader invocations
}

func (l *textLoader) Extensions() []string { return []string{".txt"} }
func (l *textLoader) Load(r io.Reader, _ struct{}) (string, error) {
	l.calls.Add(1)
	b, err := io.ReadAll(r)
	return string(b), err
}

func newTestServer(t *testing.T, files map[string]string) (*AssetServer, *task.IOPool, *textLoader) {
	t.Helper()
	mapFS := fstest.MapFS{}
	for name, content := range files {
		mapFS[strings.TrimPrefix(name, "/")] = &fstest.MapFile{Data: []byte(content)}
	}
	vfs := NewVFS()
	vfs.Mount("/", mapFS, false)

	_, io := task.NewTaskPools(task.TaskPoolConfig{})
	t.Cleanup(func() { io.Shutdown() })

	srv := NewAssetServer(vfs, io)
	loader := &textLoader{}
	RegisterLoader[string, struct{}](srv, loader)
	return srv, io, loader
}

// drainLoad waits until the handle's slot transitions out of Loading.
func drainLoad[A any](srv *AssetServer, h Handle[A], timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if GetLoadState(srv, h) != Loading {
			return true
		}
		runtime.Gosched()
	}
	return false
}

// TestServerLoadOff verifies the loader runs on a different goroutine (IOPool).
func TestServerLoadOffCallerGoroutine(t *testing.T) {
	loaderGID := make(chan int64, 1)

	srv, io, _ := newTestServer(t, map[string]string{"/hello.txt": "world"})
	// Swap loader to capture its goroutine ID.
	srv.mu.Lock()
	srv.loaders = newLoaderRegistry()
	srv.mu.Unlock()
	capLoader := &goroutineCapturingLoader{out: loaderGID}
	RegisterLoader[string, struct{}](srv, capLoader)
	_ = io

	h := Load[string](srv, "/hello.txt")
	defer h.Drop()

	callerGID := currentGID()

	select {
	case loaderGID := <-loaderGID:
		if loaderGID == callerGID {
			t.Errorf("loader ran on caller goroutine %d (must run on IOPool)", callerGID)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("loader did not run within timeout")
	}
}

type goroutineCapturingLoader struct {
	out chan<- int64
}

func (l *goroutineCapturingLoader) Extensions() []string { return []string{".txt"} }
func (l *goroutineCapturingLoader) Load(r io.Reader, _ struct{}) (string, error) {
	l.out <- currentGID()
	b, err := io.ReadAll(r)
	return string(b), err
}

// currentGID returns the calling goroutine's ID from the runtime stack trace.
func currentGID() int64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	s := strings.TrimPrefix(string(buf[:n]), "goroutine ")
	if i := strings.IndexByte(s, ' '); i > 0 {
		id, _ := strconv.ParseInt(s[:i], 10, 64)
		return id
	}
	return -1
}

// TestServerConcurrentLoadDedup verifies concurrent Load of same path fires
// the loader exactly once.
func TestServerConcurrentLoadDedup(t *testing.T) {
	unblock := make(chan struct{})
	var loaderCalls atomic.Int64

	vfs := NewVFS()
	vfs.Mount("/", fstest.MapFS{
		"data.txt": &fstest.MapFile{Data: []byte("x")},
	}, false)

	_, io := task.NewTaskPools(task.TaskPoolConfig{})
	t.Cleanup(func() { io.Shutdown() })

	srv := NewAssetServer(vfs, io)
	blocking := &blockingLoader{unblock: unblock, calls: &loaderCalls}
	RegisterLoader[string, struct{}](srv, blocking)

	const n = 8
	handles := make([]Handle[string], n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			handles[i] = Load[string](srv, "/data.txt")
		})
	}
	wg.Wait()

	// Unblock the loader; all handles should resolve.
	close(unblock)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if GetLoadState(srv, handles[0]) == Loaded {
			break
		}
		runtime.Gosched()
	}

	if c := loaderCalls.Load(); c != 1 {
		t.Errorf("loader invoked %d times for %d concurrent Load calls; want 1 (dedup)", c, n)
	}

	for i := range handles {
		handles[i].Drop()
	}
}

// blockingLoader blocks until unblock is closed, then decodes.
type blockingLoader struct {
	unblock <-chan struct{}
	calls   *atomic.Int64
}

func (l *blockingLoader) Extensions() []string { return []string{".txt"} }
func (l *blockingLoader) Load(r io.Reader, _ struct{}) (string, error) {
	l.calls.Add(1)
	<-l.unblock
	b, err := io.ReadAll(r)
	return string(b), err
}

// TestServerLoadNoLoader verifies Load of an unregistered extension → Failed.
func TestServerLoadNoLoader(t *testing.T) {
	vfs := NewVFS()
	vfs.Mount("/", fstest.MapFS{"img.png": &fstest.MapFile{Data: []byte{0}}}, false)
	_, io := task.NewTaskPools(task.TaskPoolConfig{})
	t.Cleanup(func() { io.Shutdown() })

	srv := NewAssetServer(vfs, io)
	// No loader registered for .png
	h := Load[string](srv, "/img.png")
	defer h.Drop()

	if !drainLoad(srv, h, 5*time.Second) {
		t.Fatal("slot did not leave Loading within timeout")
	}
	if s := GetLoadState(srv, h); s != Failed {
		t.Errorf("state = %d, want Failed", s)
	}
}

// TestServerLoadMissingFile verifies Load of a non-existent path → Failed.
func TestServerLoadMissingFile(t *testing.T) {
	vfs := NewVFS()
	vfs.Mount("/", fstest.MapFS{}, false)
	_, io := task.NewTaskPools(task.TaskPoolConfig{})
	t.Cleanup(func() { io.Shutdown() })

	srv := NewAssetServer(vfs, io)
	RegisterLoader[string, struct{}](srv, &textLoader{})
	h := Load[string](srv, "/missing.txt")
	defer h.Drop()

	if !drainLoad(srv, h, 5*time.Second) {
		t.Fatal("slot did not leave Loading within timeout")
	}
	if s := GetLoadState(srv, h); s != Failed {
		t.Errorf("state = %d, want Failed", s)
	}
}

// TestServerLoadAndGet verifies a successful load sets Loaded state and value.
func TestServerLoadAndGet(t *testing.T) {
	srv, _, _ := newTestServer(t, map[string]string{"/hello.txt": "world"})
	h := Load[string](srv, "/hello.txt")
	defer h.Drop()

	if !drainLoad(srv, h, 5*time.Second) {
		t.Fatal("load did not complete within timeout")
	}

	srv.mu.Lock()
	store := storeFor[string](srv)
	srv.mu.Unlock()

	v, ok := store.Get(h.id)
	if !ok || *v != "world" {
		t.Fatalf("Get: got (%v, %v), want (world, true)", v, ok)
	}
}

func TestVFSPriorityShadow(t *testing.T) {
	base := fstest.MapFS{"app/data.txt": &fstest.MapFile{Data: []byte("base")}}
	patch := fstest.MapFS{"data.txt": &fstest.MapFile{Data: []byte("patched")}}

	v := NewVFS()
	v.Mount("/app/", base, false)
	v.Mount("/app/", patch, false) // higher priority

	f, err := v.Open("/app/data.txt")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()
	b, _ := io.ReadAll(f)
	if string(b) != "patched" {
		t.Errorf("got %q, want %q (priority shadowing broken)", b, "patched")
	}
}

func TestVFSMissingMount(t *testing.T) {
	v := NewVFS()
	_, err := v.Open("/nowhere/file.txt")
	if err == nil {
		t.Fatal("expected error for unresolvable path")
	}
}
