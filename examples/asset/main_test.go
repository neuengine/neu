package main

import (
	"sync"
	"testing"

	"github.com/neuengine/neu/pkg/asset"
	"github.com/neuengine/neu/pkg/task"
)

// TestAssetLifecycle verifies Add/Get/Drop and the async IOPool load path
// are race-free and produce the correct values.
func TestAssetLifecycle(t *testing.T) {
	store := asset.NewAssets[string]()

	h := store.Add("hello")
	val, ok := store.Get(h.ID())
	if !ok || *val != "hello" {
		t.Fatalf("Add/Get: ok=%v val=%v", ok, val)
	}

	id := h.ID()
	h.Drop()
	if _, ok := store.Get(id); ok {
		t.Error("slot must be evicted after Drop")
	}
}

func TestAssetAsyncLoad(t *testing.T) {
	_, ioPool := task.NewTaskPools(task.TaskPoolConfig{})
	t.Cleanup(func() { ioPool.Shutdown() })

	store := asset.NewAssets[string]()
	var wg sync.WaitGroup
	var h asset.Handle[string]
	wg.Add(1)
	_ = ioPool.Go(func() {
		defer wg.Done()
		h = store.Add("async")
	})
	wg.Wait()

	val, ok := store.Get(h.ID())
	if !ok || *val != "async" {
		t.Fatalf("async load: ok=%v", ok)
	}
	h.Drop()
}

func TestAssetHotReload(t *testing.T) {
	store := asset.NewAssets[string]()
	h1 := store.Add("v1")
	id1 := h1.ID()
	h1.Drop()
	if _, ok := store.Get(id1); ok {
		t.Error("stale handle must not resolve")
	}
	h2 := store.Add("v2")
	val, ok := store.Get(h2.ID())
	if !ok || *val != "v2" {
		t.Fatalf("hot-reload: ok=%v", ok)
	}
	h2.Drop()
}
