package world

import (
	"testing"

	"github.com/neuengine/neu/internal/ecs/component"
)

// Compile-time: *DeferredWorld must satisfy component.HookContext.
var _ component.HookContext = (*DeferredWorld)(nil)

type Gold struct{ Amount int }

func TestNewDeferredWorld(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	dw := NewDeferredWorld(w)
	if dw == nil {
		t.Fatal("NewDeferredWorld must return non-nil")
	}
	if dw.World() != w {
		t.Fatal("DeferredWorld.World() must return the original World")
	}
}

func TestWorldNewDeferred(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	dw := w.NewDeferred()
	if dw == nil {
		t.Fatal("World.NewDeferred() must return non-nil")
	}
	if dw.World() != w {
		t.Fatal("World.NewDeferred().World() must return the same World")
	}
}

func TestDeferredWorldIsAlive(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	dw := w.NewDeferred()
	e := w.SpawnEmpty()

	if !dw.IsAlive(e) {
		t.Fatal("IsAlive must be true for newly spawned entity")
	}
	_ = w.Despawn(e)
	if dw.IsAlive(e) {
		t.Fatal("IsAlive must be false after Despawn")
	}
}

func TestSetAndGetDeferredResource(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	dw := w.NewDeferred()

	SetDeferredResource(dw, Gold{Amount: 500})

	got, ok := DeferredResource[Gold](dw)
	if !ok {
		t.Fatal("DeferredResource must find Gold after SetDeferredResource")
	}
	if got.Amount != 500 {
		t.Fatalf("Gold.Amount = %d, want 500", got.Amount)
	}
}

func TestDeferredResourceNotFound(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	dw := w.NewDeferred()

	ptr, ok := DeferredResource[Gold](dw)
	if ok || ptr != nil {
		t.Fatal("DeferredResource must return nil,false when not set")
	}
}

func TestDeferredResourceSharedWithWorld(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	dw := w.NewDeferred()

	// Set via World, read via DeferredWorld.
	SetResource(w, Gold{Amount: 100})
	got, ok := DeferredResource[Gold](dw)
	if !ok || got.Amount != 100 {
		t.Fatalf("DeferredWorld must see resources set via World; got %+v ok=%v", got, ok)
	}

	// Set via DeferredWorld, read via World.
	SetDeferredResource(dw, Gold{Amount: 200})
	got2, ok := Resource[Gold](w)
	if !ok || got2.Amount != 200 {
		t.Fatalf("World must see resources set via DeferredWorld; got %+v ok=%v", got2, ok)
	}
}

func TestRemoveDeferredResource(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	dw := w.NewDeferred()

	SetDeferredResource(dw, Gold{Amount: 1})
	if !RemoveDeferredResource[Gold](dw) {
		t.Fatal("RemoveDeferredResource must return true when resource existed")
	}
	if RemoveDeferredResource[Gold](dw) {
		t.Fatal("second RemoveDeferredResource must return false")
	}
}

func TestContainsDeferredResource(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	dw := w.NewDeferred()

	if ContainsDeferredResource[Gold](dw) {
		t.Fatal("must be false before insertion")
	}
	SetDeferredResource(dw, Gold{Amount: 1})
	if !ContainsDeferredResource[Gold](dw) {
		t.Fatal("must be true after insertion")
	}
}

func TestApplyDeferredDoesNotPanic(t *testing.T) {
	t.Parallel()

	w := NewWorld()
	dw := w.NewDeferred()

	// Both stub variants must not panic.
	dw.ApplyDeferred()
	w.ApplyDeferred()
}
