// Package main demonstrates the Neu asset system:
//   - Typed Assets[T] store: Add, Get, Drop (generational invalidation)
//   - Async loading via IOPool: background decode, handle lifecycle
//   - Hot-reload roundtrip: drop stale handle, reload into new handle
//
// Run:  go run ./examples/asset
// Test: go test -race ./examples/asset
package main

import (
	"fmt"
	"sync"

	"github.com/neuengine/neu/pkg/asset"
	"github.com/neuengine/neu/pkg/task"
)

func main() {
	// ── Phase 1: typed store lifecycle ────────────────────────────────────────
	store := asset.NewAssets[string]()

	h1 := store.Add("hello, engine")
	val, ok := store.Get(h1.ID())
	if !ok || *val != "hello, engine" {
		fmt.Printf("FAIL store add/get: ok=%v val=%v\n", ok, val)
		return
	}
	fmt.Printf("PASS add/get:  %q\n", *val)

	// Drop the handle — generational invalidation should evict the slot.
	id1 := h1.ID()
	h1.Drop()
	if _, ok := store.Get(id1); ok {
		fmt.Println("FAIL: stale ID still resolves after Drop")
		return
	}
	fmt.Println("PASS drop:     slot evicted after last Drop")

	// ── Phase 2: async decode via IOPool ──────────────────────────────────────
	// Simulate an async loader that writes into the store from an IOPool goroutine.
	_, ioPool := task.NewTaskPools(task.TaskPoolConfig{})
	defer ioPool.Shutdown()

	asyncStore := asset.NewAssets[string]()
	var wg sync.WaitGroup
	wg.Add(1)
	var asyncHandle asset.Handle[string]
	_ = ioPool.Go(func() {
		defer wg.Done()
		// Simulate decoding work.
		decoded := "async-loaded-value"
		asyncHandle = asyncStore.Add(decoded)
	})
	wg.Wait()

	av, ok := asyncStore.Get(asyncHandle.ID())
	if !ok || *av != "async-loaded-value" {
		fmt.Printf("FAIL async load: ok=%v\n", ok)
		return
	}
	fmt.Printf("PASS async:    %q\n", *av)

	// ── Phase 3: hot-reload roundtrip ─────────────────────────────────────────
	// Simulate hot-reload: drop the stale handle and load a new value.
	oldID := asyncHandle.ID()
	asyncHandle.Drop()

	// Verify stale handle is invalid.
	if _, ok := asyncStore.Get(oldID); ok {
		fmt.Println("FAIL: stale async ID still resolves after Drop")
		return
	}

	// Load the "reloaded" version.
	h2 := asyncStore.Add("reloaded-value")
	rv, ok := asyncStore.Get(h2.ID())
	if !ok || *rv != "reloaded-value" {
		fmt.Printf("FAIL hot-reload: ok=%v\n", ok)
		return
	}
	fmt.Printf("PASS reload:   %q\n", *rv)
	h2.Drop()

	fmt.Println("PASS: asset hot-reload roundtrip complete")
}
