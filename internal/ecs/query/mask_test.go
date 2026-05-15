package query

import (
	"testing"

	"github.com/teratron/boltengine/internal/ecs/component"
)

func TestMaskZeroValue(t *testing.T) {
	t.Parallel()

	var m Mask
	if !m.IsZero() {
		t.Fatal("zero-value Mask must be empty")
	}
	if m.Count() != 0 {
		t.Fatalf("zero Mask Count = %d, want 0", m.Count())
	}
	if m.Has(0) || m.Has(MaskBits-1) {
		t.Fatal("zero Mask must report Has=false for any bit")
	}
}

func TestMaskSetHasClear(t *testing.T) {
	t.Parallel()

	cases := []component.ID{0, 1, 31, 63, 64, 65, 100, MaskBits - 1}
	for _, id := range cases {
		var m Mask
		m.Set(id)
		if !m.Has(id) {
			t.Fatalf("Set(%d).Has(%d) = false; want true", id, id)
		}
		if m.Count() != 1 {
			t.Fatalf("Count after Set(%d) = %d; want 1", id, m.Count())
		}
		m.Clear(id)
		if m.Has(id) {
			t.Fatalf("Clear(%d).Has(%d) = true; want false", id, id)
		}
		if !m.IsZero() {
			t.Fatalf("Clear(%d) must restore IsZero=true", id)
		}
	}
}

func TestMaskHasOutOfRange(t *testing.T) {
	t.Parallel()

	var m Mask
	if m.Has(component.ID(MaskBits)) {
		t.Fatal("Has on out-of-range ID must return false")
	}
	if m.Has(component.ID(MaskBits + 100)) {
		t.Fatal("Has on far out-of-range ID must return false")
	}
}

func TestMaskSetOutOfRangePanics(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Fatal("Set with id ≥ MaskBits must panic")
		}
	}()
	var m Mask
	m.Set(component.ID(MaskBits))
}

func TestMaskClearOutOfRangePanics(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Fatal("Clear with id ≥ MaskBits must panic")
		}
	}()
	var m Mask
	m.Clear(component.ID(MaskBits))
}

func TestNewMaskAndMaskFromIDs(t *testing.T) {
	t.Parallel()

	m1 := NewMask(1, 5, 70, 127)
	m2 := MaskFromIDs([]component.ID{1, 5, 70, 127})
	if !m1.Equal(m2) {
		t.Fatalf("NewMask vs MaskFromIDs differ: %s vs %s", m1, m2)
	}
	if m1.Count() != 4 {
		t.Fatalf("Count = %d, want 4", m1.Count())
	}
	for _, id := range []component.ID{1, 5, 70, 127} {
		if !m1.Has(id) {
			t.Fatalf("expected Has(%d)=true", id)
		}
	}
}

func TestMaskEqualAndContains(t *testing.T) {
	t.Parallel()

	a := NewMask(1, 2, 3)
	b := NewMask(1, 2, 3)
	c := NewMask(1, 2)
	d := NewMask(1, 2, 4)

	if !a.Equal(b) {
		t.Fatal("identical masks must be Equal")
	}
	if a.Equal(c) {
		t.Fatal("differing masks must not be Equal")
	}
	if !a.Contains(c) {
		t.Fatal("a={1,2,3} must Contain c={1,2}")
	}
	if c.Contains(a) {
		t.Fatal("c={1,2} must NOT Contain a={1,2,3}")
	}
	if !a.Contains(a) {
		t.Fatal("Contains must be reflexive")
	}
	if a.Contains(d) {
		t.Fatal("a={1,2,3} must NOT Contain d={1,2,4}")
	}

	var empty Mask
	if !a.Contains(empty) {
		t.Fatal("every mask must contain the empty mask")
	}
}

func TestMaskDisjointAndIntersects(t *testing.T) {
	t.Parallel()

	a := NewMask(1, 2, 70)
	b := NewMask(3, 4, 80)
	c := NewMask(2, 80)

	if !a.IsDisjoint(b) {
		t.Fatal("a and b share no bits — must be disjoint")
	}
	if a.Intersects(b) {
		t.Fatal("Intersects must be the inverse of IsDisjoint")
	}
	if a.IsDisjoint(c) {
		t.Fatal("a and c share bit 2 — must NOT be disjoint")
	}
	if !a.Intersects(c) {
		t.Fatal("a and c share bit 2 — must Intersect")
	}
}

func TestMaskBitwiseOps(t *testing.T) {
	t.Parallel()

	a := NewMask(1, 2, 70)
	b := NewMask(2, 3, 80)

	or := a.Or(b)
	if !or.Equal(NewMask(1, 2, 3, 70, 80)) {
		t.Fatalf("Or = %s, want {1,2,3,70,80}", or)
	}
	and := a.And(b)
	if !and.Equal(NewMask(2)) {
		t.Fatalf("And = %s, want {2}", and)
	}
	andNot := a.AndNot(b)
	if !andNot.Equal(NewMask(1, 70)) {
		t.Fatalf("AndNot = %s, want {1,70}", andNot)
	}
}

func TestMaskForEachAscending(t *testing.T) {
	t.Parallel()

	m := NewMask(127, 0, 64, 65, 1, 63)

	var got []component.ID
	m.ForEach(func(id component.ID) bool {
		got = append(got, id)
		return true
	})

	want := []component.ID{0, 1, 63, 64, 65, 127}
	if len(got) != len(want) {
		t.Fatalf("ForEach yielded %d ids, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ForEach[%d] = %d, want %d (full=%v)", i, got[i], want[i], got)
		}
	}
}

func TestMaskForEachEarlyStop(t *testing.T) {
	t.Parallel()

	m := NewMask(1, 2, 3, 70, 80)
	var seen []component.ID
	m.ForEach(func(id component.ID) bool {
		seen = append(seen, id)
		return id != 2 // stop after seeing 2
	})
	if len(seen) != 2 || seen[0] != 1 || seen[1] != 2 {
		t.Fatalf("early-stop sequence = %v, want [1 2]", seen)
	}
}

func TestMaskForEachEarlyStopHighWord(t *testing.T) {
	t.Parallel()

	m := NewMask(1, 65, 70, 80)
	var seen []component.ID
	m.ForEach(func(id component.ID) bool {
		seen = append(seen, id)
		return id != 70 // stop on a hi-word id
	})
	if len(seen) != 3 || seen[2] != 70 {
		t.Fatalf("hi-word early-stop sequence = %v, want [1 65 70]", seen)
	}
}

func TestMaskIDs(t *testing.T) {
	t.Parallel()

	m := NewMask(5, 1, 70, 0)
	got := m.IDs()
	want := []component.ID{0, 1, 5, 70}
	if len(got) != len(want) {
		t.Fatalf("IDs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("IDs[%d] = %d, want %d (full=%v)", i, got[i], want[i], got)
		}
	}

	var empty Mask
	if ids := empty.IDs(); len(ids) != 0 {
		t.Fatalf("empty mask IDs() = %v, want []", ids)
	}
}

func TestMaskString(t *testing.T) {
	t.Parallel()

	var empty Mask
	if got := empty.String(); got != "Mask{}" {
		t.Fatalf("empty String = %q, want %q", got, "Mask{}")
	}
	m := NewMask(2, 1, 64)
	if got := m.String(); got != "Mask{1, 2, 64}" {
		t.Fatalf("String = %q, want %q", got, "Mask{1, 2, 64}")
	}
}

func TestMaskCountAcrossWords(t *testing.T) {
	t.Parallel()

	m := NewMask(0, 1, 63, 64, 65, 127)
	if m.Count() != 6 {
		t.Fatalf("Count across both words = %d, want 6", m.Count())
	}
}
