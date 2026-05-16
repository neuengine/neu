package changedetect

import "testing"

func TestTickIsNewerThan(t *testing.T) {
	tests := []struct {
		name string
		tick Tick
		last Tick
		want bool
	}{
		{"strictly newer", 5, 4, true},
		{"equal is not newer", 4, 4, false},
		{"older", 3, 4, false},
		{"zero vs zero", 0, 0, false},
		{"any vs zero", 1, 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.tick.IsNewerThan(tc.last); got != tc.want {
				t.Fatalf("Tick(%d).IsNewerThan(%d) = %v, want %v", tc.tick, tc.last, got, tc.want)
			}
		})
	}
}

// TestComponentTicks is the T-2E01 Verify target: an added component's
// Added tick is set on insert; the Changed tick advances only on an explicit
// mutation, never on a read.
func TestComponentTicks(t *testing.T) {
	t.Run("NewComponentTicks sets both fields to the insert tick", func(t *testing.T) {
		ct := NewComponentTicks(7)
		if ct.Added != 7 || ct.Changed != 7 {
			t.Fatalf("NewComponentTicks(7) = %+v, want {7 7}", ct)
		}
	})

	t.Run("read does not advance Changed", func(t *testing.T) {
		ct := NewComponentTicks(3)
		// Pure reads — IsAdded/IsChanged must be side-effect free.
		_ = ct.IsAdded(0)
		_ = ct.IsChanged(0)
		if ct.Changed != 3 || ct.Added != 3 {
			t.Fatalf("reads mutated ticks: %+v, want {3 3}", ct)
		}
	})

	t.Run("SetChanged advances Changed only, preserving Added", func(t *testing.T) {
		ct := NewComponentTicks(3)
		ct.SetChanged(9)
		if ct.Added != 3 {
			t.Fatalf("Added changed to %d, want 3 (insert time must persist)", ct.Added)
		}
		if ct.Changed != 9 {
			t.Fatalf("Changed = %d, want 9", ct.Changed)
		}
	})

	cases := []struct {
		name           string
		ticks          ComponentTicks
		lastSystemTick Tick
		wantAdded      bool
		wantChanged    bool
	}{
		{"fresh insert seen by older system", NewComponentTicks(5), 4, true, true},
		{"insert at same tick as last run", NewComponentTicks(4), 4, false, false},
		{"mutated after add", ComponentTicks{Added: 2, Changed: 8}, 5, false, true},
		{"added long ago, never changed", ComponentTicks{Added: 1, Changed: 1}, 10, false, false},
		{"added included in changed", ComponentTicks{Added: 9, Changed: 9}, 8, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.ticks.IsAdded(tc.lastSystemTick); got != tc.wantAdded {
				t.Errorf("IsAdded(%d) = %v, want %v", tc.lastSystemTick, got, tc.wantAdded)
			}
			if got := tc.ticks.IsChanged(tc.lastSystemTick); got != tc.wantChanged {
				t.Errorf("IsChanged(%d) = %v, want %v", tc.lastSystemTick, got, tc.wantChanged)
			}
		})
	}
}

func TestColumnTicks(t *testing.T) {
	t.Run("Observe raises maxima only", func(t *testing.T) {
		var c ColumnTicks
		c.Observe(ComponentTicks{Added: 3, Changed: 5})
		c.Observe(ComponentTicks{Added: 1, Changed: 2}) // lower — must not lower aggregate
		c.Observe(ComponentTicks{Added: 4, Changed: 4})
		if c.ColumnAddedTick != 4 {
			t.Errorf("ColumnAddedTick = %d, want 4", c.ColumnAddedTick)
		}
		if c.ColumnChangedTick != 5 {
			t.Errorf("ColumnChangedTick = %d, want 5", c.ColumnChangedTick)
		}
	})

	t.Run("MayHave* gate the O(1) column skip", func(t *testing.T) {
		c := ColumnTicks{ColumnChangedTick: 6, ColumnAddedTick: 6}
		if !c.MayHaveChanged(5) {
			t.Error("MayHaveChanged(5) = false, want true (column changed at 6)")
		}
		if c.MayHaveChanged(6) {
			t.Error("MayHaveChanged(6) = true, want false (nothing newer than last run)")
		}
		if !c.MayHaveAdded(5) {
			t.Error("MayHaveAdded(5) = false, want true")
		}
		if c.MayHaveAdded(6) {
			t.Error("MayHaveAdded(6) = true, want false")
		}
	})

	t.Run("Reset zeroes the aggregate", func(t *testing.T) {
		c := ColumnTicks{ColumnChangedTick: 9, ColumnAddedTick: 9}
		c.Reset()
		if c != (ColumnTicks{}) {
			t.Fatalf("Reset left %+v, want zero", c)
		}
	})
}
