package render

import (
	"testing"

	"github.com/neuengine/neu/pkg/math"
)

func TestRenderDataSoA(t *testing.T) {
	h := NewRenderDataHolder()
	wm := RegisterStaticKey[math.Mat4](h, "WorldMatrix")
	sk := RegisterDynamicKey[uint32](h, "SortKey")
	h.Reserve(100)

	if got := len(wm.Slice(h)); got != 100 {
		t.Fatalf("WorldMatrix slice len = %d, want 100", got)
	}
	if got := len(sk.Slice(h)); got != 100 {
		t.Fatalf("SortKey slice len = %d, want 100", got)
	}

	// Mutate in place; read back through a fresh Slice call.
	want := math.Mat4{
		{X: 1, Y: 2, Z: 3, W: 4},
		{X: 5, Y: 6, Z: 7, W: 8},
		{X: 9, Y: 10, Z: 11, W: 12},
		{X: 13, Y: 14, Z: 15, W: 16},
	}
	wm.Slice(h)[42] = want
	if got := wm.Slice(h)[42]; got != want {
		t.Fatalf("WorldMatrix[42] = %v, want %v", got, want)
	}

	// Slice must alias the SAME backing array across calls (GPU-bindable,
	// zero-copy) — &slice[0] is stable.
	if &wm.Slice(h)[0] != &wm.Slice(h)[0] {
		t.Fatal("Slice returned a copy; must alias holder storage")
	}

	// Columns are independent.
	sk.Slice(h)[42] = 0xABCD
	if wm.Slice(h)[42] != want {
		t.Fatal("writing SortKey corrupted WorldMatrix column")
	}

	// Idempotent re-registration of same name+type returns a usable key.
	wm2 := RegisterStaticKey[math.Mat4](h, "WorldMatrix")
	if wm2.Slice(h)[42] != want {
		t.Fatal("re-registered key does not alias the existing column")
	}

	// Reserve never shrinks; grows all columns.
	h.Reserve(250)
	if len(wm.Slice(h)) != 250 || len(sk.Slice(h)) != 250 {
		t.Fatalf("Reserve(250) did not grow all columns")
	}
	if wm.Slice(h)[42] != want {
		t.Fatal("Reserve lost existing data on grow")
	}
}

func TestRenderDataSoA_TypeMismatchPanics(t *testing.T) {
	h := NewRenderDataHolder()
	_ = RegisterStaticKey[math.Mat4](h, "K")
	defer func() {
		if recover() == nil {
			t.Fatal("re-registering key K with a different type must panic")
		}
	}()
	_ = RegisterStaticKey[uint32](h, "K") // different T → panic
}
