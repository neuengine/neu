package asset

import (
	"sync"
	"testing"
)

func TestContentManagerStoreLoad(t *testing.T) {
	cm := NewContentManager()
	cm.Store("/tex/a.png", "texture-a")

	v, ok := cm.Load("/tex/a.png")
	if !ok || v != "texture-a" {
		t.Fatalf("Load: got (%v, %v), want (texture-a, true)", v, ok)
	}
}

func TestContentManagerRelease(t *testing.T) {
	cm := NewContentManager()
	cm.Store("/tex/b.png", "texture-b")
	// Load increments refcount (now 2)
	cm.Load("/tex/b.png")
	// First release → refcount 1, still present
	evicted := cm.Release("/tex/b.png")
	if evicted {
		t.Fatal("Release should not evict while refcount > 0")
	}
	if cm.Len() != 1 {
		t.Fatalf("Len = %d, want 1", cm.Len())
	}
	// Second release → refcount 0, evicted
	evicted = cm.Release("/tex/b.png")
	if !evicted {
		t.Fatal("second Release must evict when refcount reaches 0")
	}
	if cm.Len() != 0 {
		t.Fatalf("Len after eviction = %d, want 0", cm.Len())
	}
}

func TestContentManagerReleaseMissing(t *testing.T) {
	cm := NewContentManager()
	if cm.Release("/nonexistent") {
		t.Fatal("Release on missing key must return false")
	}
}

func TestContentManagerLoadMissing(t *testing.T) {
	cm := NewContentManager()
	_, ok := cm.Load("/missing")
	if ok {
		t.Fatal("Load on missing key must return false")
	}
}

func TestContentManagerStoreUpdate(t *testing.T) {
	cm := NewContentManager()
	cm.Store("/x", "v1")
	cm.Store("/x", "v2") // update: refcount stays at 1, val changes
	v, ok := cm.Load("/x")
	if !ok || v != "v2" {
		t.Fatalf("Load after Store update: got (%v, %v), want (v2, true)", v, ok)
	}
	if cm.Len() != 1 {
		t.Fatalf("Len after update Store = %d, want 1", cm.Len())
	}
}

func TestContentManagerRace(t *testing.T) {
	cm := NewContentManager()
	cm.Store("/shared", 0)
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			cm.Load("/shared")
			cm.Release("/shared")
		})
	}
	wg.Wait()
}
