package asset

import (
	"reflect"
	"testing"
	"time"
)

// captureEmit returns an emit callback plus a pointer to the events it records.
func captureEmit() (func(AssetEvent[string]), *[]AssetEvent[string]) {
	var got []AssetEvent[string]
	return func(ev AssetEvent[string]) { got = append(got, ev) }, &got
}

func TestLoadRecordsPathRef(t *testing.T) {
	srv, _, _ := newTestServer(t, map[string]string{"/a.txt": "one"})
	h := Load[string](srv, "/a.txt")
	if !drainLoad(srv, h, time.Second) {
		t.Fatal("load did not complete")
	}
	srv.mu.Lock()
	ref, ok := srv.loaded["/a.txt"]
	srv.mu.Unlock()
	if !ok {
		t.Fatal("Load did not record a path→ref entry")
	}
	if ref.typ != reflect.TypeFor[string]() {
		t.Errorf("recorded type = %v, want string", ref.typ)
	}
	if !ref.id.IsValid() {
		t.Error("recorded id is invalid")
	}
}

func TestReloadEmitsModified(t *testing.T) {
	srv, _, _ := newTestServer(t, map[string]string{"/a.txt": "one"})
	h := Load[string](srv, "/a.txt")
	drainLoad(srv, h, time.Second)

	emit, got := captureEmit()
	WatchReloads[string](srv, emit)

	rh := Reload[string](srv, "/a.txt")
	drainLoad(srv, rh, time.Second)

	if len(*got) != 1 {
		t.Fatalf("emitted %d events, want 1", len(*got))
	}
	ev := (*got)[0]
	if ev.Kind != AssetModified {
		t.Errorf("Kind = %d, want AssetModified", ev.Kind)
	}
	if ev.Path != "/a.txt" {
		t.Errorf("Path = %q, want /a.txt", ev.Path)
	}
	if !ev.ID.IsValid() {
		t.Error("event ID is invalid")
	}
}

func TestReloadWithoutWatchIsNoOp(t *testing.T) {
	srv, _, _ := newTestServer(t, map[string]string{"/a.txt": "one"})
	// No WatchReloads → Reload must still work and emit nothing (no panic).
	h := Reload[string](srv, "/a.txt")
	if !drainLoad(srv, h, time.Second) {
		t.Fatal("reload without a watcher did not complete")
	}
}

func TestDispatchReloadResolvesTypeAndEmits(t *testing.T) {
	srv, _, _ := newTestServer(t, map[string]string{"/a.txt": "one"})
	h := Load[string](srv, "/a.txt")
	drainLoad(srv, h, time.Second)

	emit, got := captureEmit()
	WatchReloads[string](srv, emit)

	// Simulate the dev watcher firing for a known path (runtime-typed trigger).
	srv.dispatchReload("/a.txt")

	if len(*got) != 1 {
		t.Fatalf("dispatchReload emitted %d events, want 1 (path→type dispatch failed)", len(*got))
	}
	if (*got)[0].Path != "/a.txt" {
		t.Errorf("Path = %q, want /a.txt", (*got)[0].Path)
	}
}

func TestDispatchReloadUnknownPathNoOp(t *testing.T) {
	srv, _, _ := newTestServer(t, map[string]string{"/a.txt": "one"})
	emit, got := captureEmit()
	WatchReloads[string](srv, emit)

	srv.dispatchReload("/never-loaded.txt") // unknown path → no dispatch
	if len(*got) != 0 {
		t.Errorf("unknown path emitted %d events, want 0", len(*got))
	}
}

func TestDispatchReloadKnownPathNoWatcherNoOp(t *testing.T) {
	srv, _, _ := newTestServer(t, map[string]string{"/a.txt": "one"})
	h := Load[string](srv, "/a.txt") // records loaded[path] but no WatchReloads
	drainLoad(srv, h, time.Second)
	srv.dispatchReload("/a.txt") // known path, no dispatcher registered → no-op, no panic
}
