package changedetect

import "testing"

type pos struct{ X int }

func TestRefIsReadOnlyAndReportsMetadata(t *testing.T) {
	t.Parallel()

	ct := ComponentTicks{Added: 5, Changed: 8}
	p := &pos{X: 42}
	r := NewRef(p, ct, 6)

	if r.Value() != p {
		t.Fatal("Ref.Value did not return the wrapped pointer")
	}
	// Reading a Ref must never advance the source ticks.
	_ = r.Value()
	_ = r.IsChanged()
	_ = r.IsAdded()
	if ct.Changed != 8 || ct.Added != 5 {
		t.Fatalf("Ref mutated source ticks: %+v, want {5 8}", ct)
	}

	if !r.IsChanged() { // Changed 8 > lastSystemTick 6
		t.Error("IsChanged = false, want true (8 > 6)")
	}
	if r.IsAdded() { // Added 5 > 6 is false
		t.Error("IsAdded = true, want false (5 ≤ 6)")
	}
	if r.LastChanged() != 8 {
		t.Errorf("LastChanged = %d, want 8", r.LastChanged())
	}
}

func TestMutMarksChangedOnValue(t *testing.T) {
	t.Parallel()

	ticks := &ComponentTicks{Added: 2, Changed: 2}
	p := &pos{X: 1}
	m := NewMut(p, ticks, 3 /*lastSystemTick*/, 9 /*changeTick*/)

	// IsChanged/IsAdded reflect pre-this-system state regardless of when (or
	// whether) Value() is called — prevChanged was captured at construction.
	if m.IsChanged() { // prevChanged 2 > lastSystemTick 3 → false
		t.Error("IsChanged before mutation = true, want false (2 ≤ 3)")
	}

	got := m.Value()
	if got != p {
		t.Fatal("Mut.Value did not return the wrapped pointer")
	}
	if ticks.Changed != 9 {
		t.Fatalf("Value() did not mark changed: Changed = %d, want 9", ticks.Changed)
	}
	if ticks.Added != 2 {
		t.Fatalf("Value() altered Added: %d, want 2 (insert time must persist)", ticks.Added)
	}
	// Order-independence: IsChanged still answers about the pre-mutation state.
	if m.IsChanged() {
		t.Error("IsChanged after own mutation = true, want false (must ignore this system's write)")
	}
}

func TestMutSetChangedAndBypass(t *testing.T) {
	t.Parallel()

	t.Run("SetChanged marks without dereferencing", func(t *testing.T) {
		ticks := &ComponentTicks{Added: 1, Changed: 1}
		m := NewMut(&pos{}, ticks, 0, 7)
		m.SetChanged()
		if ticks.Changed != 7 {
			t.Fatalf("SetChanged: Changed = %d, want 7", ticks.Changed)
		}
	})

	t.Run("BypassChangeDetection does not mark", func(t *testing.T) {
		ticks := &ComponentTicks{Added: 1, Changed: 1}
		p := &pos{X: 5}
		m := NewMut(p, ticks, 0, 7)
		if m.BypassChangeDetection() != p {
			t.Fatal("BypassChangeDetection returned wrong pointer")
		}
		if ticks.Changed != 1 {
			t.Fatalf("BypassChangeDetection marked changed: %d, want 1", ticks.Changed)
		}
	})

	t.Run("IsAdded reads the live Added tick", func(t *testing.T) {
		ticks := &ComponentTicks{Added: 9, Changed: 9}
		m := NewMut(&pos{}, ticks, 4, 10)
		if !m.IsAdded() { // 9 > 4
			t.Error("IsAdded = false, want true (9 > 4)")
		}
	})
}
