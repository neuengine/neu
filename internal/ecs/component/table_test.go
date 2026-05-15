package component

import (
	"reflect"
	"testing"
	"unsafe"
)

// Misaligned3 is intentionally awkward: a uint8 followed by a uint64 forces
// the layout code to insert padding when sorted by alignment.
type Misaligned3 struct {
	A uint8
	_ [7]byte
	B uint64
}

func newSpec[T any](t *testing.T, id ID) ColumnSpec {
	t.Helper()
	rt := reflect.TypeFor[T]()
	return ColumnSpec{
		ID:    id,
		Size:  rt.Size(),
		Align: uintptr(rt.Align()),
		Type:  rt,
	}
}

func TestTableLayoutFitsChunk(t *testing.T) {
	t.Parallel()

	specs := []ColumnSpec{
		newSpec[Position](t, 1),
		newSpec[Velocity](t, 2),
	}
	tbl := NewTable(specs, DefaultChunkSize)

	if tbl.ChunkRows() < 1 {
		t.Fatalf("chunkRows must be ≥ 1, got %d", tbl.ChunkRows())
	}
	if tbl.RowStride() == 0 {
		t.Fatal("rowStride must be > 0 for non-tag columns")
	}
	// Sanity: rowStride*chunkRows ≤ chunkSize.
	if int(tbl.RowStride())*tbl.ChunkRows() > DefaultChunkSize {
		t.Fatalf("layout overflows chunk: stride=%d rows=%d size=%d",
			tbl.RowStride(), tbl.ChunkRows(), DefaultChunkSize)
	}
}

func TestTableAddRowAndCellPtr(t *testing.T) {
	t.Parallel()

	specs := []ColumnSpec{
		newSpec[Position](t, 1),
		newSpec[Velocity](t, 2),
	}
	tbl := NewTable(specs, DefaultChunkSize)

	row := tbl.AddRow(map[ID]any{
		1: Position{X: 10, Y: 20, Z: 30},
		2: Velocity{DX: 1, DY: 2, DZ: 3},
	})
	if row != 0 {
		t.Fatalf("first row index = %d, want 0", row)
	}
	if tbl.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", tbl.Len())
	}

	pPtr := tbl.CellPtr(0, row)
	vPtr := tbl.CellPtr(1, row)
	gotP := *(*Position)(pPtr)
	gotV := *(*Velocity)(vPtr)
	if gotP != (Position{10, 20, 30}) {
		t.Fatalf("Position cell = %+v, want {10,20,30}", gotP)
	}
	if gotV != (Velocity{1, 2, 3}) {
		t.Fatalf("Velocity cell = %+v, want {1,2,3}", gotV)
	}
}

func TestTableAddRowZeroFillsMissing(t *testing.T) {
	t.Parallel()

	specs := []ColumnSpec{
		newSpec[Position](t, 1),
		newSpec[Velocity](t, 2),
	}
	tbl := NewTable(specs, DefaultChunkSize)
	row := tbl.AddRow(map[ID]any{1: Position{X: 7}})

	v := *(*Velocity)(tbl.CellPtr(1, row))
	if v != (Velocity{}) {
		t.Fatalf("missing column must be zero-filled; got %+v", v)
	}
	p := *(*Position)(tbl.CellPtr(0, row))
	if p != (Position{X: 7}) {
		t.Fatalf("supplied column lost data; got %+v", p)
	}
}

func TestTableAddRowUnknownIDPanics(t *testing.T) {
	t.Parallel()

	specs := []ColumnSpec{newSpec[Position](t, 1)}
	tbl := NewTable(specs, DefaultChunkSize)

	defer func() {
		if recover() == nil {
			t.Fatal("AddRow with unknown ID must panic")
		}
	}()
	tbl.AddRow(map[ID]any{99: Position{}})
}

func TestTableSetCellTypeMismatchPanics(t *testing.T) {
	t.Parallel()

	specs := []ColumnSpec{newSpec[Position](t, 1)}
	tbl := NewTable(specs, DefaultChunkSize)
	row := tbl.AddRow(nil)

	defer func() {
		if recover() == nil {
			t.Fatal("SetCell with wrong type must panic")
		}
	}()
	tbl.SetCell(0, row, Velocity{})
}

func TestTableRemoveRowSwapAndPop(t *testing.T) {
	t.Parallel()

	specs := []ColumnSpec{newSpec[Position](t, 1)}
	tbl := NewTable(specs, DefaultChunkSize)

	r0 := tbl.AddRow(map[ID]any{1: Position{X: 1}})
	r1 := tbl.AddRow(map[ID]any{1: Position{X: 2}})
	r2 := tbl.AddRow(map[ID]any{1: Position{X: 3}})
	_ = r1

	movedFrom := tbl.RemoveRow(r0)
	if movedFrom != r2 {
		t.Fatalf("RemoveRow must report swap source = %d, got %d", r2, movedFrom)
	}
	if tbl.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", tbl.Len())
	}
	got := *(*Position)(tbl.CellPtr(0, r0))
	if got.X != 3 {
		t.Fatalf("after swap-and-pop, row 0 must hold former last (X=3); got X=%v", got.X)
	}

	// Removing the last row should not swap.
	tail := tbl.Len() - 1
	if movedFrom := tbl.RemoveRow(tail); movedFrom != -1 {
		t.Fatalf("removing last row must report movedFrom=-1, got %d", movedFrom)
	}
}

func TestTableRemoveRowOutOfRangePanics(t *testing.T) {
	t.Parallel()

	specs := []ColumnSpec{newSpec[Position](t, 1)}
	tbl := NewTable(specs, DefaultChunkSize)
	tbl.AddRow(nil)

	defer func() {
		if recover() == nil {
			t.Fatal("out-of-range RemoveRow must panic")
		}
	}()
	tbl.RemoveRow(99)
}

func TestTableMultiChunkAllocation(t *testing.T) {
	t.Parallel()

	// Tight chunk size to force chunk crossings within a small test.
	specs := []ColumnSpec{newSpec[Position](t, 1)}
	chunkSize := int(reflect.TypeFor[Position]().Size()) * 4 // exactly 4 rows per chunk
	tbl := NewTable(specs, chunkSize)
	if tbl.ChunkRows() != 4 {
		t.Fatalf("chunkRows = %d, want 4", tbl.ChunkRows())
	}

	const N = 10
	for i := range N {
		tbl.AddRow(map[ID]any{1: Position{X: float32(i)}})
	}
	if tbl.Capacity() < N {
		t.Fatalf("Capacity %d < N %d", tbl.Capacity(), N)
	}
	for i := range N {
		got := *(*Position)(tbl.CellPtr(0, i))
		if got.X != float32(i) {
			t.Fatalf("row %d X = %v, want %d", i, got.X, i)
		}
	}
}

func TestTableTagOnlyHasNoChunks(t *testing.T) {
	t.Parallel()

	specs := []ColumnSpec{newSpec[EnemyTag](t, 1)}
	tbl := NewTable(specs, DefaultChunkSize)
	if tbl.RowStride() != 0 {
		t.Fatalf("tag-only stride must be 0; got %d", tbl.RowStride())
	}

	for range 100 {
		tbl.AddRow(nil)
	}
	if tbl.Len() != 100 {
		t.Fatalf("Len() = %d, want 100", tbl.Len())
	}
	if ptr := tbl.CellPtr(0, 0); ptr != nil {
		t.Fatalf("zero-size CellPtr must be nil; got %v", ptr)
	}
}

func TestTableAlignmentSorting(t *testing.T) {
	t.Parallel()

	// Misaligned3 (align 8) interleaved with Position (align 4); the table
	// must reorder internally so all reads still land at the public index.
	specs := []ColumnSpec{
		newSpec[Position](t, 1),    // public index 0, align 4
		newSpec[Misaligned3](t, 2), // public index 1, align 8
	}
	tbl := NewTable(specs, DefaultChunkSize)

	row := tbl.AddRow(map[ID]any{
		1: Position{X: 1, Y: 2, Z: 3},
		2: Misaligned3{A: 7, B: 1234567890123},
	})

	p := *(*Position)(tbl.CellPtr(0, row))
	m := *(*Misaligned3)(tbl.CellPtr(1, row))
	if p != (Position{1, 2, 3}) {
		t.Fatalf("Position roundtrip failed: %+v", p)
	}
	if m.A != 7 || m.B != 1234567890123 {
		t.Fatalf("Misaligned3 roundtrip failed: %+v", m)
	}

	// Misaligned3 cell must be 8-byte-aligned.
	mPtr := tbl.CellPtr(1, row)
	if uintptr(mPtr)%8 != 0 {
		t.Fatalf("Misaligned3 cell ptr %p violates 8-byte alignment", mPtr)
	}
}

func TestTableSetCellOutOfRangePanics(t *testing.T) {
	t.Parallel()

	specs := []ColumnSpec{newSpec[Position](t, 1)}
	tbl := NewTable(specs, DefaultChunkSize)
	tbl.AddRow(nil)

	tests := []struct {
		name string
		col  int
		row  int
	}{
		{"col_negative", -1, 0},
		{"col_overflow", 2, 0},
		{"row_negative", 0, -1},
		{"row_overflow", 0, 5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				if recover() == nil {
					t.Fatalf("SetCell(%d,%d) must panic", tc.col, tc.row)
				}
			}()
			tbl.SetCell(tc.col, tc.row, Position{})
		})
	}
}

func TestTableResetKeepsCapacity(t *testing.T) {
	t.Parallel()

	specs := []ColumnSpec{newSpec[Position](t, 1)}
	tbl := NewTable(specs, DefaultChunkSize)
	for range 10 {
		tbl.AddRow(nil)
	}
	capBefore := tbl.Capacity()
	tbl.Reset()
	if tbl.Len() != 0 {
		t.Fatalf("Len() after Reset = %d, want 0", tbl.Len())
	}
	if tbl.Capacity() != capBefore {
		t.Fatalf("Capacity must not shrink on Reset; before=%d after=%d", capBefore, tbl.Capacity())
	}
}

func TestTableColumnsAccessor(t *testing.T) {
	t.Parallel()

	specs := []ColumnSpec{newSpec[Position](t, 1), newSpec[Velocity](t, 2)}
	tbl := NewTable(specs, DefaultChunkSize)
	got := tbl.Columns()
	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 2 {
		t.Fatalf("Columns() must preserve original order; got %+v", got)
	}
	got[0].ID = 999 // mutating the copy must not affect the table
	if tbl.Columns()[0].ID == 999 {
		t.Fatal("Columns() must return a defensive copy")
	}
}

func TestTableChunkSizeDefaults(t *testing.T) {
	t.Parallel()

	specs := []ColumnSpec{newSpec[Position](t, 1)}
	tbl := NewTable(specs, 0) // 0 → DefaultChunkSize
	if tbl.ChunkSize() != DefaultChunkSize {
		t.Fatalf("ChunkSize() = %d, want %d", tbl.ChunkSize(), DefaultChunkSize)
	}
}

func TestTableSingleRowExceedingChunkPanics(t *testing.T) {
	t.Parallel()

	// A row that is larger than the chunk must be rejected at construction.
	specs := []ColumnSpec{newSpec[Position](t, 1)}
	defer func() {
		if recover() == nil {
			t.Fatal("constructing a Table whose row exceeds chunk must panic")
		}
	}()
	_ = NewTable(specs, 1) // 1 byte chunk, Position is 12 bytes
}

func TestTableRemoveRowReleasesEmptyChunk(t *testing.T) {
	t.Parallel()

	specs := []ColumnSpec{newSpec[Position](t, 1)}
	chunkSize := int(reflect.TypeFor[Position]().Size()) * 2 // 2 rows per chunk
	tbl := NewTable(specs, chunkSize)

	// Fill exactly two chunks.
	for i := range 4 {
		tbl.AddRow(map[ID]any{1: Position{X: float32(i)}})
	}
	capBefore := tbl.Capacity()

	// Remove last two — second chunk should be released.
	tbl.RemoveRow(3)
	tbl.RemoveRow(2)
	if tbl.Capacity() >= capBefore {
		t.Fatalf("Capacity must shrink after emptying trailing chunk; before=%d after=%d",
			capBefore, tbl.Capacity())
	}
}

func BenchmarkTableAddRow(b *testing.B) {
	specs := []ColumnSpec{newSpec[Position](nil, 1)}
	tbl := NewTable(specs, DefaultChunkSize)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tbl.AddRow(map[ID]any{1: Position{X: 1}})
	}
}

func BenchmarkTableCellPtr(b *testing.B) {
	specs := []ColumnSpec{newSpec[Position](nil, 1)}
	tbl := NewTable(specs, DefaultChunkSize)
	for i := range 1000 {
		tbl.AddRow(map[ID]any{1: Position{X: float32(i)}})
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tbl.CellPtr(0, i%1000)
	}
}

var _ = unsafe.Sizeof(Position{}) // keep unsafe imported even if unused above
