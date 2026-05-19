package render

import (
	"sync"

	gpu "github.com/neuengine/neu/pkg/render"
)

// ResourceTracker reference-counts GPU resources and defers their destruction
// (l1-render-core INV-3, §4.6). A resource whose refcount reaches zero is not
// destroyed immediately: it is recorded with the frame in which it was freed
// and destroyed by [ResourceTracker.EndFrame] on a LATER frame. This guarantees
// a resource the current frame's command buffer still references is never
// destroyed in-flight.
type ResourceTracker struct {
	backend gpu.RenderBackend

	mu      sync.Mutex
	refs    map[gpu.RID]int32
	pending map[gpu.RID]uint64 // RID → frame in which refcount hit 0
}

// NewResourceTracker returns a tracker that destroys resources via backend.
func NewResourceTracker(backend gpu.RenderBackend) *ResourceTracker {
	return &ResourceTracker{
		backend: backend,
		refs:    make(map[gpu.RID]int32),
		pending: make(map[gpu.RID]uint64),
	}
}

// Retain increments rid's reference count (and cancels a pending free if the
// resource was revived before its deferred-delete frame). Nil handles are
// ignored. Safe to call from any goroutine.
func (t *ResourceTracker) Retain(rid gpu.RID) {
	if rid.IsNil() {
		return
	}
	t.mu.Lock()
	t.refs[rid]++
	delete(t.pending, rid) // revived before GC — keep it alive
	t.mu.Unlock()
}

// Release decrements rid's reference count. On reaching zero the resource is
// queued for deferred destruction at frame, not destroyed now. Releasing an
// untracked or nil handle is a no-op. Safe to call from any goroutine.
func (t *ResourceTracker) Release(rid gpu.RID, frame uint64) {
	if rid.IsNil() {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	c, ok := t.refs[rid]
	if !ok {
		return
	}
	c--
	if c > 0 {
		t.refs[rid] = c
		return
	}
	delete(t.refs, rid)
	t.pending[rid] = frame // destroyed on a strictly later EndFrame
}

// EndFrame destroys every resource whose deferred-delete frame is strictly
// older than the current frame — i.e. freed in a previous frame, never the
// current in-flight one (INV-3). Call once per frame from the server goroutine
// after Present.
func (t *ResourceTracker) EndFrame(frame uint64) {
	t.mu.Lock()
	var dead []gpu.RID
	for rid, freed := range t.pending {
		if freed < frame {
			dead = append(dead, rid)
		}
	}
	for _, rid := range dead {
		delete(t.pending, rid)
	}
	t.mu.Unlock()

	// Destroy outside the lock — backend.Destroy must not re-enter the tracker.
	for _, rid := range dead {
		t.backend.Destroy(rid)
	}
}

// RefCount reports rid's current strong reference count (0 if untracked or
// pending deferred deletion). Test/diagnostics only.
func (t *ResourceTracker) RefCount(rid gpu.RID) int32 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.refs[rid]
}

// PendingFree reports whether rid is awaiting deferred destruction.
func (t *ResourceTracker) PendingFree(rid gpu.RID) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, ok := t.pending[rid]
	return ok
}
