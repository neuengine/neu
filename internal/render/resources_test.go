package render

import (
	"sync"
	"testing"

	gpu "github.com/neuengine/neu/pkg/render"
)

// recBackend records Destroy calls for deferred-deletion assertions.
type recBackend struct {
	mu        sync.Mutex
	destroyed []gpu.RID
}

func (*recBackend) CreateBuffer(gpu.BufferDesc) gpu.RID       { return 0 }
func (*recBackend) CreateTexture(gpu.TextureDesc) gpu.RID     { return 0 }
func (*recBackend) CreatePipeline(gpu.PipelineDesc) gpu.RID   { return 0 }
func (*recBackend) CreateBindGroup(gpu.BindGroupDesc) gpu.RID { return 0 }
func (*recBackend) BeginRenderPass(gpu.RenderPassDesc)        {}
func (*recBackend) Draw(gpu.DrawCmd)                          {}
func (*recBackend) EndRenderPass()                            {}
func (*recBackend) Submit()                                   {}
func (*recBackend) Present()                                  {}
func (b *recBackend) Destroy(r gpu.RID) {
	b.mu.Lock()
	b.destroyed = append(b.destroyed, r)
	b.mu.Unlock()
}
func (b *recBackend) count() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.destroyed)
}

// TestResourceTracker_DeferredDeleteOneFrameAfterRelease pins the core INV-3
// timing: a resource released in frame N survives frame N's EndFrame and is
// destroyed exactly once at frame N+1's EndFrame — never in-flight.
func TestResourceTracker_DeferredDeleteOneFrameAfterRelease(t *testing.T) {
	b := &recBackend{}
	tr := NewResourceTracker(b)
	rid := gpu.MakeRID(gpu.KindTexture, 7, 1)

	tr.Retain(rid)
	tr.Release(rid, 0) // freed in frame 0

	if !tr.PendingFree(rid) {
		t.Fatal("rid not pending after refcount → 0")
	}
	tr.EndFrame(0) // same frame: must NOT destroy (in-flight)
	if b.count() != 0 {
		t.Fatalf("destroyed during freeing frame (in-flight!): %d", b.count())
	}
	tr.EndFrame(1) // next frame: destroy exactly once
	if b.count() != 1 {
		t.Fatalf("destroyed %d times at frame+1, want exactly 1", b.count())
	}
	if b.destroyed[0] != rid {
		t.Fatalf("destroyed %d, want %d", b.destroyed[0], rid)
	}
	tr.EndFrame(2) // idempotent: no double free
	if b.count() != 1 {
		t.Fatalf("double-free: count = %d", b.count())
	}
}

func TestResourceTracker_RetainCancelsPendingFree(t *testing.T) {
	b := &recBackend{}
	tr := NewResourceTracker(b)
	rid := gpu.MakeRID(gpu.KindBuffer, 1, 1)

	tr.Retain(rid)
	tr.Release(rid, 0) // pending
	tr.Retain(rid)     // revived before its deferred-delete frame
	if tr.PendingFree(rid) {
		t.Fatal("Retain did not cancel pending free")
	}
	tr.EndFrame(10)
	if b.count() != 0 {
		t.Fatalf("revived resource destroyed: %d", b.count())
	}
}

func TestResourceTracker_MultiRef(t *testing.T) {
	b := &recBackend{}
	tr := NewResourceTracker(b)
	rid := gpu.MakeRID(gpu.KindPipeline, 3, 1)

	tr.Retain(rid)
	tr.Retain(rid)
	tr.Retain(rid)
	tr.Release(rid, 0)
	tr.Release(rid, 0)
	if tr.RefCount(rid) != 1 {
		t.Fatalf("RefCount = %d, want 1", tr.RefCount(rid))
	}
	if tr.PendingFree(rid) {
		t.Fatal("pending while refs still held")
	}
	tr.Release(rid, 0)
	tr.EndFrame(1)
	if b.count() != 1 {
		t.Fatalf("want 1 destroy after last release, got %d", b.count())
	}
}

func TestResourceTracker_NilAndUntrackedNoop(t *testing.T) {
	b := &recBackend{}
	tr := NewResourceTracker(b)
	tr.Retain(gpu.RID(0)) // nil
	tr.Release(gpu.RID(0), 0)
	tr.Release(gpu.MakeRID(gpu.KindBuffer, 99, 1), 0) // never retained
	tr.EndFrame(5)
	if b.count() != 0 {
		t.Fatalf("nil/untracked caused destroy: %d", b.count())
	}
}

// TestResourceTracker_ConcurrentRetainRelease — balanced Retain/Release across
// goroutines settles to a single deferred destroy. Run under -race (C-005).
func TestResourceTracker_ConcurrentRetainRelease(t *testing.T) {
	b := &recBackend{}
	tr := NewResourceTracker(b)
	rid := gpu.MakeRID(gpu.KindTexture, 42, 1)

	const g = 64
	tr.Retain(rid) // hold one ref so it never hits 0 mid-storm
	var wg sync.WaitGroup
	for range g {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tr.Retain(rid)
			tr.Release(rid, 0)
		}()
	}
	wg.Wait()
	if tr.RefCount(rid) != 1 {
		t.Fatalf("RefCount = %d after balanced storm, want 1", tr.RefCount(rid))
	}
	tr.Release(rid, 0)
	tr.EndFrame(1)
	if b.count() != 1 {
		t.Fatalf("want exactly 1 destroy, got %d", b.count())
	}
}
