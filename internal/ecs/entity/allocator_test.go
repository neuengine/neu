package entity

import (
	"testing"
)

func TestNewEntityAllocator(t *testing.T) {
	t.Parallel()

	a := NewEntityAllocator(16)
	if a.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", a.Len())
	}
	if a.Cap() != 0 {
		t.Fatalf("Cap() = %d, want 0", a.Cap())
	}

	if a := NewEntityAllocator(-5); a == nil {
		t.Fatal("negative capacity must not yield nil")
	}
}

func TestAllocateProducesValidEntities(t *testing.T) {
	t.Parallel()

	a := NewEntityAllocator(0)
	e1 := a.Allocate()
	e2 := a.Allocate()

	if !e1.IsValid() {
		t.Fatal("first allocation must be valid (not the null sentinel)")
	}
	if !e2.IsValid() {
		t.Fatal("second allocation must be valid")
	}
	if e1.Index() == e2.Index() {
		t.Fatal("distinct allocations must have distinct indices")
	}
	if e1.Generation() != 1 || e2.Generation() != 1 {
		t.Fatalf("first-time slots must use generation 1; got %d / %d",
			e1.Generation(), e2.Generation())
	}
	if a.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", a.Len())
	}
}

func TestNullEntityIsNeverProduced(t *testing.T) {
	t.Parallel()

	a := NewEntityAllocator(0)
	for i := range 16 {
		e := a.Allocate()
		if !e.IsValid() {
			t.Fatalf("Allocate() returned the null sentinel on iteration %d", i)
		}
	}
}

func TestFreeAndReuseWithGenerationBump(t *testing.T) {
	t.Parallel()

	a := NewEntityAllocator(4)
	e1 := a.Allocate() // index 0, gen 1
	e2 := a.Allocate() // index 1, gen 1

	a.Free(e1)
	if a.IsAlive(e1) {
		t.Fatal("freed entity must not be alive")
	}
	if !a.IsAlive(e2) {
		t.Fatal("untouched entity must remain alive")
	}
	if a.Len() != 1 {
		t.Fatalf("Len() after Free = %d, want 1", a.Len())
	}

	e3 := a.Allocate()
	if e3.Index() != e1.Index() {
		t.Fatalf("freelist must be LIFO: reused index = %d, want %d",
			e3.Index(), e1.Index())
	}
	if e3.Generation() != e1.Generation()+1 {
		t.Fatalf("generation must increment on reuse: got %d, want %d",
			e3.Generation(), e1.Generation()+1)
	}
	if a.IsAlive(e1) {
		t.Fatal("stale reference (old generation) must not register as alive")
	}
	if !a.IsAlive(e3) {
		t.Fatal("reused entity must be alive")
	}
}

func TestFreelistLIFOOrdering(t *testing.T) {
	t.Parallel()

	a := NewEntityAllocator(4)
	e0 := a.Allocate()
	e1 := a.Allocate()
	e2 := a.Allocate()

	a.Free(e0)
	a.Free(e1)
	a.Free(e2)

	r0 := a.Allocate()
	r1 := a.Allocate()
	r2 := a.Allocate()

	if r0.Index() != e2.Index() || r1.Index() != e1.Index() || r2.Index() != e0.Index() {
		t.Fatalf("freelist must be LIFO: got indices [%d %d %d], want [%d %d %d]",
			r0.Index(), r1.Index(), r2.Index(),
			e2.Index(), e1.Index(), e0.Index())
	}
}

func TestFreeRejectsInvalidEntities(t *testing.T) {
	t.Parallel()

	a := NewEntityAllocator(0)
	a.Free(Entity{}) // null sentinel — no-op
	if a.Len() != 0 {
		t.Fatal("Free(null) must be a no-op")
	}

	a.Free(NewEntity(99, 1)) // out of range — no-op
	if a.Len() != 0 {
		t.Fatal("Free(out-of-range) must be a no-op")
	}

	e := a.Allocate()
	a.Free(e)
	a.Free(e) // double-free with stale generation — no-op
	if a.Len() != 0 {
		t.Fatalf("double Free must not under-flow: Len() = %d", a.Len())
	}
}

func TestIsAliveBoundaryConditions(t *testing.T) {
	t.Parallel()

	a := NewEntityAllocator(0)
	if a.IsAlive(Entity{}) {
		t.Fatal("null entity must never be alive")
	}
	if a.IsAlive(NewEntity(0, 1)) {
		t.Fatal("out-of-range index must not be alive")
	}

	e := a.Allocate()
	if !a.IsAlive(e) {
		t.Fatal("freshly allocated entity must be alive")
	}

	stale := NewEntity(e.Index(), e.Generation()+1)
	if a.IsAlive(stale) {
		t.Fatal("future-generation reference must not be alive")
	}
}

func TestAllocateMany(t *testing.T) {
	t.Parallel()

	a := NewEntityAllocator(0)
	if got := a.AllocateMany(0); got != nil {
		t.Fatalf("AllocateMany(0) = %v, want nil", got)
	}
	if got := a.AllocateMany(-3); got != nil {
		t.Fatalf("AllocateMany(-3) = %v, want nil", got)
	}

	batch := a.AllocateMany(5)
	if len(batch) != 5 {
		t.Fatalf("AllocateMany(5) returned %d entities", len(batch))
	}
	if a.Len() != 5 {
		t.Fatalf("Len() = %d, want 5", a.Len())
	}

	seen := make(map[uint32]bool, 5)
	for _, e := range batch {
		if !a.IsAlive(e) {
			t.Fatalf("batch entity %v must be alive", e)
		}
		if seen[e.Index()] {
			t.Fatalf("AllocateMany returned duplicate index %d", e.Index())
		}
		seen[e.Index()] = true
	}
}

func TestAllocateManyMixesFreelistAndExtension(t *testing.T) {
	t.Parallel()

	a := NewEntityAllocator(0)
	first := a.AllocateMany(3)
	a.Free(first[0])
	a.Free(first[1])

	if a.Len() != 1 {
		t.Fatalf("Len() after partial free = %d, want 1", a.Len())
	}

	batch := a.AllocateMany(4) // 2 reused + 2 fresh
	if len(batch) != 4 {
		t.Fatalf("expected 4 entities, got %d", len(batch))
	}
	if a.Len() != 5 {
		t.Fatalf("Len() = %d, want 5", a.Len())
	}
	for _, e := range batch {
		if !a.IsAlive(e) {
			t.Fatalf("entity %v must be alive after mixed batch alloc", e)
		}
	}
}

func TestReserveDoesNotAllocateEntities(t *testing.T) {
	t.Parallel()

	a := NewEntityAllocator(0)
	a.Reserve(0)
	a.Reserve(-10)
	if a.Len() != 0 || a.Cap() != 0 {
		t.Fatalf("Reserve with non-positive n must be a no-op")
	}

	a.Reserve(64)
	if a.Len() != 0 {
		t.Fatalf("Reserve must not produce live entities; Len() = %d", a.Len())
	}
	if cap(a.generations) < 64 {
		t.Fatalf("Reserve must grow generations capacity; got %d", cap(a.generations))
	}
}

func TestAllocFreeChurnInvariant(t *testing.T) {
	t.Parallel()

	a := NewEntityAllocator(0)
	const n = 1000
	live := make([]Entity, 0, n)

	for range n {
		live = append(live, a.Allocate())
	}
	for i := 0; i < n; i += 2 {
		a.Free(live[i])
	}
	if a.Len() != n/2 {
		t.Fatalf("Len() after half-free = %d, want %d", a.Len(), n/2)
	}
	for i := range n {
		want := i%2 == 1
		if a.IsAlive(live[i]) != want {
			t.Fatalf("entity %d alive=%v, want %v", i, a.IsAlive(live[i]), want)
		}
	}
}

func BenchmarkAllocate(b *testing.B) {
	a := NewEntityAllocator(b.N)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = a.Allocate()
	}
}

func BenchmarkAllocateFree(b *testing.B) {
	a := NewEntityAllocator(64)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		e := a.Allocate()
		a.Free(e)
	}
}

func BenchmarkIsAlive(b *testing.B) {
	a := NewEntityAllocator(1)
	e := a.Allocate()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = a.IsAlive(e)
	}
}

func FuzzAllocateFreeCycle(f *testing.F) {
	f.Add(uint8(10), uint8(170))

	f.Fuzz(func(t *testing.T, allocCount, freeMask uint8) {
		a := NewEntityAllocator(0)
		entities := make([]Entity, 0, allocCount)
		for i := 0; i < int(allocCount); i++ {
			entities = append(entities, a.Allocate())
		}

		freed := 0
		for i, e := range entities {
			if (freeMask>>(uint(i)%8))&1 == 1 {
				if a.IsAlive(e) {
					a.Free(e)
					freed++
				}
			}
		}

		if a.Len() != int(allocCount)-freed {
			t.Fatalf("Len() invariant broken: got %d, want %d",
				a.Len(), int(allocCount)-freed)
		}
		for _, e := range a.AllocateMany(freed) {
			if !a.IsAlive(e) {
				t.Fatal("re-allocated entity must be alive")
			}
		}
	})
}
