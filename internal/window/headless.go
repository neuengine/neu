// Package window provides the headless WindowBackend and the pure sync helpers
// (window diffing, primary-window tracking) used by the window systems. The
// public Window component, backend interface, and event types live in
// pkg/window; this package holds the CI-friendly backend and main-thread logic.
//
// Bootstrap: l2-window-system-go Draft (Phase 6 Track B).
package window

import (
	"fmt"
	"sync"

	"github.com/neuengine/neu/pkg/ecs"
	pkgwindow "github.com/neuengine/neu/pkg/window"
)

// HeadlessWindowBackend is a WindowBackend that creates no OS windows. It
// records a call log and replays a scripted event queue, giving deterministic
// behavior for CI and tests (mirrors the audio HeadlessBackend pattern).
type HeadlessWindowBackend struct {
	handles map[ecs.Entity]pkgwindow.RawWindowHandle
	calls   []string
	queue   [][]pkgwindow.PlatformEvent
	seq     uint64
	mu      sync.Mutex
}

// NewHeadlessWindowBackend returns an empty headless backend.
func NewHeadlessWindowBackend() *HeadlessWindowBackend {
	return &HeadlessWindowBackend{handles: make(map[ecs.Entity]pkgwindow.RawWindowHandle)}
}

// CreateWindow assigns a monotonic handle and records the call.
func (b *HeadlessWindowBackend) CreateWindow(e ecs.Entity, _ pkgwindow.WindowDescriptor) (pkgwindow.RawWindowHandle, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.seq++
	h := pkgwindow.NewRawWindowHandle(b.seq)
	b.handles[e] = h
	b.calls = append(b.calls, fmt.Sprintf("Create:%d", e.ID()))
	return h, nil
}

// DestroyWindow drops the handle and records the call.
func (b *HeadlessWindowBackend) DestroyWindow(e ecs.Entity) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.handles, e)
	b.calls = append(b.calls, fmt.Sprintf("Destroy:%d", e.ID()))
	return nil
}

// ApplyChanges records the diff application. A diff with no changes is never
// passed here by the sync system, but is tolerated as a no-op if it is.
func (b *HeadlessWindowBackend) ApplyChanges(e ecs.Entity, diff pkgwindow.WindowDiff) error {
	if !diff.HasChanges() {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls = append(b.calls, fmt.Sprintf("Apply:%d", e.ID()))
	return nil
}

// PollEvents returns the next scripted frame of events (empty when exhausted).
func (b *HeadlessWindowBackend) PollEvents() []pkgwindow.PlatformEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.queue) == 0 {
		return nil
	}
	frame := b.queue[0]
	b.queue = b.queue[1:]
	return frame
}

// ScriptEvents appends a frame of events for a later PollEvents call (test helper).
func (b *HeadlessWindowBackend) ScriptEvents(frame ...pkgwindow.PlatformEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.queue = append(b.queue, frame)
}

// Calls returns the recorded call log (test helper).
func (b *HeadlessWindowBackend) Calls() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.calls...)
}

// ActiveCount returns the number of live window handles (test helper).
func (b *HeadlessWindowBackend) ActiveCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.handles)
}

var _ pkgwindow.WindowBackend = (*HeadlessWindowBackend)(nil)
