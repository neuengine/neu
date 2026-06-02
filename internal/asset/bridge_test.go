package asset

import (
	"io"
	"testing"
	"testing/fstest"

	"github.com/neuengine/neu/internal/ecs/event"
	"github.com/neuengine/neu/internal/ecs/world"
	pkgasset "github.com/neuengine/neu/pkg/asset"
	"github.com/neuengine/neu/pkg/task"
)

// txtLoader is a minimal string loader for the bridge tests.
type txtLoader struct{}

func (txtLoader) Extensions() []string { return []string{".txt"} }
func (txtLoader) Load(r io.Reader, _ struct{}) (string, error) {
	b, err := io.ReadAll(r)
	return string(b), err
}

func newServer(t *testing.T) *pkgasset.AssetServer {
	t.Helper()
	vfs := pkgasset.NewVFS()
	vfs.Mount("/", fstest.MapFS{"a.txt": &fstest.MapFile{Data: []byte("v1")}}, false)
	_, io := task.NewTaskPools(task.TaskPoolConfig{})
	t.Cleanup(func() { io.Shutdown() })
	s := pkgasset.NewAssetServer(vfs, io)
	pkgasset.RegisterLoader[string, struct{}](s, txtLoader{})
	return s
}

func TestWatchAssetTypePublishesReloadOnBus(t *testing.T) {
	s := newServer(t)
	w := world.NewWorld()

	WatchAssetType[string](s, w)

	bus := event.Bus[pkgasset.AssetEvent[string]](w)
	if bus == nil {
		t.Fatal("WatchAssetType must register the AssetEvent bus")
	}

	pkgasset.Load[string](s, "/a.txt")     // initial load — no event
	if bus.Len() != 0 {
		t.Fatalf("initial Load emitted %d events, want 0", bus.Len())
	}

	pkgasset.Reload[string](s, "/a.txt")   // reload — one Modified event
	if bus.Len() != 1 {
		t.Fatalf("Reload emitted %d events on the bus, want 1", bus.Len())
	}

	// Confirm the payload survives the bridge.
	r := event.NewEventReader[pkgasset.AssetEvent[string]](w)
	var seen int
	for ev := range r.All() {
		seen++
		if ev.Kind != pkgasset.AssetModified || ev.Path != "/a.txt" {
			t.Errorf("bridged event = %+v, want {Modified, /a.txt}", ev)
		}
	}
	if seen != 1 {
		t.Errorf("read %d events, want 1", seen)
	}
}
