package component

import (
	"testing"

	"github.com/neuengine/neu/internal/ecs/changedetect"
	"github.com/neuengine/neu/internal/ecs/entity"
)

func TestTableTickSlotsStampAndRead(t *testing.T) {
	t.Parallel()

	specs := []ColumnSpec{newSpec[Position](t, 1), newSpec[Velocity](t, 2)}
	tbl := NewTable(specs, DefaultChunkSize)
	r0 := tbl.AddRow(map[ID]any{1: Position{}, 2: Velocity{}})

	if ct := tbl.Ticks(0, r0); ct != (changedetect.ComponentTicks{}) {
		t.Fatalf("fresh row ticks = %+v, want zero (unstamped)", ct)
	}

	if !tbl.StampAddedByID(1, r0, 10) {
		t.Fatal("StampAddedByID returned false for present column")
	}
	if ct, _ := tbl.TicksByID(1, r0); ct.Added != 10 || ct.Changed != 10 {
		t.Fatalf("after StampAdded(10): %+v, want {10 10}", ct)
	}

	tbl.StampChangedByID(1, r0, 14)
	if ct, _ := tbl.TicksByID(1, r0); ct.Added != 10 || ct.Changed != 14 {
		t.Fatalf("after StampChanged(14): %+v, want {10 14}", ct)
	}

	// Column aggregate must reflect the max of stamped ticks.
	agg, ok := tbl.ColumnTicksByID(1)
	if !ok || agg.ColumnAddedTick != 10 || agg.ColumnChangedTick != 14 {
		t.Fatalf("ColumnTicksByID(1) = %+v ok=%v, want {14 10}", agg, ok)
	}

	if _, ok := tbl.TicksByID(99, r0); ok {
		t.Fatal("TicksByID for absent column returned ok=true")
	}
	if tbl.StampAddedByID(99, r0, 1) {
		t.Fatal("StampAddedByID for absent column returned true")
	}
}

func TestTableTickSwapAndPopMirrorsBytes(t *testing.T) {
	t.Parallel()

	specs := []ColumnSpec{newSpec[Position](t, 1)}
	tbl := NewTable(specs, DefaultChunkSize)
	for i := range 3 {
		r := tbl.AddRow(map[ID]any{1: Position{X: float32(i)}})
		tbl.StampAddedByID(1, r, changedetect.Tick(10+i)) // row r → tick 10+r
	}

	// Remove row 0; swap-and-pop must move the LAST row's ticks into slot 0,
	// staying aligned with the byte swap-and-pop the Table already performs.
	moved := tbl.RemoveRow(0)
	if moved != 2 {
		t.Fatalf("RemoveRow(0) movedFrom = %d, want 2", moved)
	}
	if got, _ := tbl.TicksByID(1, 0); got.Added != 12 {
		t.Fatalf("row 0 ticks after swap = %+v, want Added 12 (old last row)", got)
	}
	if tbl.Len() != 2 {
		t.Fatalf("Len = %d, want 2", tbl.Len())
	}

	// Removing the final row must not swap (movedFrom == -1) and must shrink.
	if moved := tbl.RemoveRow(tbl.Len() - 1); moved != -1 {
		t.Fatalf("RemoveRow(last) movedFrom = %d, want -1", moved)
	}
}

func TestTableTickTagColumnAndReset(t *testing.T) {
	t.Parallel()

	// A zero-size tag has no payload bytes but must still carry ticks.
	specs := []ColumnSpec{newSpec[EnemyTag](t, 1)}
	tbl := NewTable(specs, DefaultChunkSize)
	r := tbl.AddRow(map[ID]any{1: EnemyTag{}})
	tbl.StampChangedByID(1, r, 5)
	if ct, _ := tbl.TicksByID(1, r); ct.Changed != 5 {
		t.Fatalf("tag column ticks = %+v, want Changed 5", ct)
	}

	tbl.Reset()
	if tbl.Len() != 0 {
		t.Fatalf("Len after Reset = %d, want 0", tbl.Len())
	}
	if agg := tbl.ColumnTicks(0); agg != (changedetect.ColumnTicks{}) {
		t.Fatalf("aggregate after Reset = %+v, want zero", agg)
	}
}

func TestSparseSetTickSlots(t *testing.T) {
	t.Parallel()

	s := NewSparseSet(newSpec[Position](t, 1))
	e1 := entity.NewEntity(1, 1)
	e2 := entity.NewEntity(2, 1)
	e3 := entity.NewEntity(3, 1)
	s.Add(e1, Position{X: 1})
	s.Add(e2, Position{X: 2})
	s.Add(e3, Position{X: 3})

	if ct, ok := s.Ticks(e1); !ok || ct != (changedetect.ComponentTicks{}) {
		t.Fatalf("fresh slot e1 = %+v ok=%v, want zero/true", ct, ok)
	}
	s.StampAdded(e2, 7)
	s.StampChanged(e2, 9)
	if ct, _ := s.Ticks(e2); ct.Added != 7 || ct.Changed != 9 {
		t.Fatalf("e2 ticks = %+v, want {7 9}", ct)
	}
	if agg := s.ColumnTicks(); agg.ColumnAddedTick != 7 || agg.ColumnChangedTick != 9 {
		t.Fatalf("aggregate = %+v, want {9 7}", agg)
	}

	// Removing e1 swap-and-pops e3 into its slot — e3's ticks must follow.
	s.StampAdded(e3, 4)
	if !s.Remove(e1) {
		t.Fatal("Remove(e1) = false")
	}
	if ct, ok := s.Ticks(e3); !ok || ct.Added != 4 {
		t.Fatalf("e3 ticks after swap = %+v ok=%v, want Added 4", ct, ok)
	}
	if _, ok := s.Ticks(e1); ok {
		t.Fatal("Ticks(e1) ok=true after removal")
	}
}

func TestTickAPIEdgeCases(t *testing.T) {
	t.Parallel()

	specs := []ColumnSpec{newSpec[Position](t, 1)}
	tbl := NewTable(specs, DefaultChunkSize)
	r := tbl.AddRow(map[ID]any{1: Position{}})

	// Absent-column negative paths.
	if tbl.StampChangedByID(99, r, 1) {
		t.Error("StampChangedByID(absent) = true, want false")
	}
	if tbl.SetTicksByID(99, r, changedetect.ComponentTicks{Added: 1}) {
		t.Error("SetTicksByID(absent) = true, want false")
	}
	if _, ok := tbl.ColumnTicksByID(99); ok {
		t.Error("ColumnTicksByID(absent) ok = true, want false")
	}
	// SetTicksByID carries an existing component's history wholesale.
	if !tbl.SetTicksByID(1, r, changedetect.ComponentTicks{Added: 2, Changed: 8}) {
		t.Fatal("SetTicksByID(present) = false")
	}
	if ct, _ := tbl.TicksByID(1, r); ct.Added != 2 || ct.Changed != 8 {
		t.Fatalf("after SetTicksByID: %+v, want {2 8}", ct)
	}
	if agg := tbl.ColumnTicks(0); agg.ColumnChangedTick != 8 {
		t.Fatalf("aggregate after SetTicksByID = %+v, want ColumnChangedTick 8", agg)
	}
	assertPanics(t, "ColumnTicks out-of-range", func() { tbl.ColumnTicks(5) })

	// SparseSet absent-entity negative paths + SetTicks.
	s := NewSparseSet(newSpec[Position](t, 1))
	ghost := entity.NewEntity(404, 1)
	if s.StampAdded(ghost, 1) {
		t.Error("StampAdded(absent) = true, want false")
	}
	if s.StampChanged(ghost, 1) {
		t.Error("StampChanged(absent) = true, want false")
	}
	if s.SetTicks(ghost, changedetect.ComponentTicks{}) {
		t.Error("SetTicks(absent) = true, want false")
	}
	e := entity.NewEntity(1, 1)
	s.Add(e, Position{})
	if !s.SetTicks(e, changedetect.ComponentTicks{Added: 4, Changed: 6}) {
		t.Fatal("SetTicks(present) = false")
	}
	if ct, _ := s.Ticks(e); ct.Added != 4 || ct.Changed != 6 {
		t.Fatalf("after SetTicks: %+v, want {4 6}", ct)
	}
	if _, ok := s.Ticks(ghost); ok {
		t.Error("Ticks(absent) ok = true, want false")
	}
}

func assertPanics(t *testing.T, what string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("%s: expected panic, got none", what)
		}
	}()
	fn()
}

func TestSparseSetReAddPreservesAddedSlot(t *testing.T) {
	t.Parallel()

	s := NewSparseSet(newSpec[Position](t, 1))
	e := entity.NewEntity(5, 1)
	s.Add(e, Position{X: 1})
	s.StampAdded(e, 3)

	// Overwriting an existing entity is a mutation, not a re-insertion: the
	// storage layer must leave the tick slot intact so Added time persists.
	s.Add(e, Position{X: 2})
	if ct, _ := s.Ticks(e); ct.Added != 3 {
		t.Fatalf("Added after overwrite = %d, want 3 (slot must persist)", ct.Added)
	}

	s.Clear()
	if s.Len() != 0 {
		t.Fatalf("Len after Clear = %d, want 0", s.Len())
	}
	if agg := s.ColumnTicks(); agg != (changedetect.ColumnTicks{}) {
		t.Fatalf("aggregate after Clear = %+v, want zero", agg)
	}
}
