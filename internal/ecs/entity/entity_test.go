package entity

import (
	"math"
	"testing"
	"unsafe"
)

func TestEntityIDPackUnpack(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		index      uint32
		generation uint32
	}{
		{"zero", 0, 0},
		{"index_only", 42, 0},
		{"generation_only", 0, 7},
		{"both", 1234, 9999},
		{"max_index", math.MaxUint32, 1},
		{"max_generation", 0, math.MaxUint32},
		{"max_both", math.MaxUint32, math.MaxUint32},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			id := NewEntityID(tc.index, tc.generation)
			if got := id.Index(); got != tc.index {
				t.Fatalf("Index() = %d, want %d", got, tc.index)
			}
			if got := id.Generation(); got != tc.generation {
				t.Fatalf("Generation() = %d, want %d", got, tc.generation)
			}
		})
	}
}

func TestEntityIDLayout(t *testing.T) {
	t.Parallel()

	id := NewEntityID(0xDEADBEEF, 0xCAFEBABE)
	want := uint64(0xCAFEBABE)<<32 | uint64(0xDEADBEEF)
	if uint64(id) != want {
		t.Fatalf("packed layout = %#016x, want %#016x", uint64(id), want)
	}
}

func TestEntityIDIsNull(t *testing.T) {
	t.Parallel()

	if !EntityID(0).IsNull() {
		t.Fatal("EntityID(0) must be null")
	}
	if NewEntityID(0, 1).IsNull() {
		t.Fatal("EntityID with generation 1 must not be null")
	}
	if NewEntityID(1, 0).IsNull() {
		t.Fatal("EntityID with index 1 must not be null")
	}
}

func TestEntityZeroValueIsNullSentinel(t *testing.T) {
	t.Parallel()

	var e Entity
	if e.IsValid() {
		t.Fatal("zero-value Entity must be invalid (null sentinel)")
	}
	if e.ID() != 0 {
		t.Fatalf("zero-value Entity.ID() = %d, want 0", e.ID())
	}
	if e.Index() != 0 || e.Generation() != 0 {
		t.Fatalf("zero-value Entity must have index=0 and generation=0")
	}
}

func TestEntityAccessors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		index      uint32
		generation uint32
		valid      bool
	}{
		{"null_sentinel", 0, 0, false},
		{"slot_zero_alive", 0, 1, true},
		{"typical", 100, 5, true},
		{"max_values", math.MaxUint32, math.MaxUint32, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := NewEntity(tc.index, tc.generation)
			if e.Index() != tc.index {
				t.Fatalf("Index() = %d, want %d", e.Index(), tc.index)
			}
			if e.Generation() != tc.generation {
				t.Fatalf("Generation() = %d, want %d", e.Generation(), tc.generation)
			}
			if e.IsValid() != tc.valid {
				t.Fatalf("IsValid() = %v, want %v", e.IsValid(), tc.valid)
			}
			if e.ID() != NewEntityID(tc.index, tc.generation) {
				t.Fatalf("ID() round-trip mismatch")
			}
		})
	}
}

func TestFromIDRoundTrip(t *testing.T) {
	t.Parallel()

	original := NewEntity(7, 3)
	wrapped := FromID(original.ID())
	if wrapped != original {
		t.Fatalf("FromID round-trip mismatch: got %+v, want %+v", wrapped, original)
	}
}

func TestEntitySize(t *testing.T) {
	t.Parallel()

	if got := unsafe.Sizeof(EntityID(0)); got != 8 {
		t.Fatalf("EntityID size = %d, want 8", got)
	}
	if got := unsafe.Sizeof(Entity{}); got != 8 {
		t.Fatalf("Entity size = %d, want 8 (must fit in a register)", got)
	}
}

func TestEntityComparable(t *testing.T) {
	t.Parallel()

	a := NewEntity(10, 2)
	b := NewEntity(10, 2)
	c := NewEntity(10, 3)

	if a != b {
		t.Fatal("entities with identical id must be equal")
	}
	if a == c {
		t.Fatal("entities with different generations must not be equal")
	}

	m := map[Entity]int{a: 1}
	if m[b] != 1 {
		t.Fatal("Entity must be usable as a map key")
	}
}

func BenchmarkNewEntityID(b *testing.B) {
	var sink EntityID
	for i := 0; i < b.N; i++ {
		sink = NewEntityID(uint32(i), 1)
	}
	_ = sink
}

func BenchmarkEntityIDIndex(b *testing.B) {
	id := NewEntityID(123456, 7)
	var sink uint32
	for i := 0; i < b.N; i++ {
		sink = id.Index()
	}
	_ = sink
}

func FuzzEntityIDRoundTrip(f *testing.F) {
	f.Add(uint32(0), uint32(0))
	f.Add(uint32(1), uint32(1))
	f.Add(uint32(math.MaxUint32), uint32(math.MaxUint32))

	f.Fuzz(func(t *testing.T, index, generation uint32) {
		id := NewEntityID(index, generation)
		if id.Index() != index {
			t.Fatalf("index mismatch: got %d, want %d", id.Index(), index)
		}
		if id.Generation() != generation {
			t.Fatalf("generation mismatch: got %d, want %d", id.Generation(), generation)
		}
		e := FromID(id)
		if e.ID() != id {
			t.Fatalf("Entity.ID round-trip mismatch")
		}
	})
}
