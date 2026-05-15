package component

import (
	"fmt"
	"reflect"
	"unsafe"
)

// DefaultChunkSize is the byte size of a single Table chunk. It matches the
// 16 KB target from the L2 component-system spec — sized to fit comfortably
// in modern L1d caches while amortising allocator overhead.
const DefaultChunkSize = 16 * 1024

// Table is the column-oriented storage backend for an archetype: every row
// has the same set of components, identified by [ID]. Rows are physically
// laid out across one or more 16 KB chunks; within a chunk the columns are
// stored SOA. Columns are sorted internally by alignment-then-size for cache
// efficiency, but the public API addresses them by their original spec
// index.
//
// Removal uses swap-and-pop on the dense row arrays so iteration stays
// contiguous. Tables are NOT safe for concurrent mutation.
type Table struct {
	// Pointer-containing fields — kept together to minimise GC scan range.

	// Public-API ordering: same as the spec slice supplied to NewTable.
	cols             []ColumnSpec
	colByID          map[ID]int
	publicToOriginal []int // identity in the simple case; kept for clarity

	// Internal layout (sorted by alignment desc, then size desc, then ID asc).
	sortedCols     []ColumnSpec
	sortedOffset   []uintptr // byte offset within a chunk where each sorted column starts
	publicToSorted []int     // public index → index in sortedCols

	chunks [][]byte

	// Non-pointer scalar fields — outside GC scan range.
	chunkSize int
	chunkRows int // rows per chunk (>= 1)
	rowStride uintptr
	nRows     int
}

// NewTable creates a Table for the given column spec. The chunkSize argument
// must be ≥ the largest single-row stride; pass 0 to use [DefaultChunkSize].
func NewTable(specs []ColumnSpec, chunkSize int) *Table {
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}

	t := &Table{
		chunkSize: chunkSize,
		cols:      append([]ColumnSpec(nil), specs...),
		colByID:   make(map[ID]int, len(specs)),
	}
	for i, c := range t.cols {
		t.colByID[c.ID] = i
	}

	t.sortedCols, t.publicToSorted = sortAndIndex(specs)
	t.publicToOriginal = make([]int, len(specs))
	for i := range specs {
		t.publicToOriginal[i] = i
	}

	t.computeLayout()
	return t
}

// ChunkSize returns the per-chunk byte budget.
func (t *Table) ChunkSize() int { return t.chunkSize }

// ChunkRows returns the number of rows that fit in one chunk.
func (t *Table) ChunkRows() int { return t.chunkRows }

// RowStride returns the sum of the column sizes (excluding alignment
// padding). Useful for diagnostics and benchmarks.
func (t *Table) RowStride() uintptr { return t.rowStride }

// Len returns the number of rows currently stored.
func (t *Table) Len() int { return t.nRows }

// Capacity returns the row capacity provided by the currently allocated
// chunks (no further growth required up to this value).
func (t *Table) Capacity() int { return len(t.chunks) * t.chunkRows }

// Columns returns a copy of the column specs in their original order.
func (t *Table) Columns() []ColumnSpec {
	out := make([]ColumnSpec, len(t.cols))
	copy(out, t.cols)
	return out
}

// AddRow appends a new row, copying values from the provided map keyed by
// component ID. Components missing from the map are zero-initialised.
// Returns the index of the newly added row.
func (t *Table) AddRow(values map[ID]any) int {
	row := t.nRows
	if row >= t.Capacity() {
		t.allocChunk()
	}
	t.nRows++

	// Zero-fill the new row first to make the contract explicit, then copy
	// any caller-supplied values on top.
	for publicIdx, spec := range t.cols {
		if spec.Size == 0 {
			continue
		}
		dst := t.columnRowBytes(publicIdx, row)
		zero(dst)
	}
	for id, v := range values {
		idx, ok := t.colByID[id]
		if !ok {
			panic(fmt.Sprintf("component.Table.AddRow: unknown component ID %d", id))
		}
		t.setRowValue(idx, row, v)
	}
	return row
}

// SetCell writes a value into (row, public column index). Panics on type
// mismatch (excluding zero-size columns where any value is accepted).
func (t *Table) SetCell(col, row int, value any) {
	t.boundsCheck(col, row)
	t.setRowValue(col, row, value)
}

// CellPtr returns an unsafe.Pointer to the component cell for (row, public
// column index). Returns nil for zero-size columns. The pointer is valid
// until the next structural change (AddRow / RemoveRow).
func (t *Table) CellPtr(col, row int) unsafe.Pointer {
	t.boundsCheck(col, row)
	if t.cols[col].Size == 0 {
		return nil
	}
	bytes := t.columnRowBytes(col, row)
	return unsafe.Pointer(&bytes[0])
}

// RemoveRow removes the row at index `row` using swap-and-pop. Returns the
// index of the row that was moved into `row`'s slot, or -1 if the removed
// row was the last one (no swap occurred).
func (t *Table) RemoveRow(row int) int {
	if row < 0 || row >= t.nRows {
		panic(fmt.Sprintf("component.Table.RemoveRow: row %d out of range [0,%d)", row, t.nRows))
	}
	last := t.nRows - 1
	movedFrom := -1
	if row != last {
		for publicIdx, spec := range t.cols {
			if spec.Size == 0 {
				continue
			}
			copy(t.columnRowBytes(publicIdx, row), t.columnRowBytes(publicIdx, last))
		}
		movedFrom = last
	}
	t.nRows--

	// Drop trailing chunk if it just became empty.
	if t.nRows%t.chunkRows == 0 && len(t.chunks)*t.chunkRows > t.nRows && len(t.chunks) > 0 {
		t.chunks = t.chunks[:len(t.chunks)-1]
	}
	return movedFrom
}

// Reset clears the table without releasing chunk memory.
func (t *Table) Reset() { t.nRows = 0 }

// HasColumn reports whether the table has a column for the given component ID.
func (t *Table) HasColumn(id ID) bool {
	_, ok := t.colByID[id]
	return ok
}

// CellPtrByID returns a pointer to the component cell at (component ID, row).
// Returns (nil, false) if the ID has no column in this table.
// Returns (nil, true) for zero-size (tag) columns — the component is present
// but has no physical data.
func (t *Table) CellPtrByID(id ID, row int) (unsafe.Pointer, bool) {
	col, ok := t.colByID[id]
	if !ok {
		return nil, false
	}
	if row < 0 || row >= t.nRows {
		panic(fmt.Sprintf("component.Table.CellPtrByID: row %d out of range [0,%d)", row, t.nRows))
	}
	if t.cols[col].Size == 0 {
		return nil, true
	}
	b := t.columnRowBytes(col, row)
	return unsafe.Pointer(&b[0]), true
}

// SetCellByID writes a value into the cell at (component ID, row).
// Panics if the ID has no column or the value type does not match.
func (t *Table) SetCellByID(id ID, row int, value any) {
	col, ok := t.colByID[id]
	if !ok {
		panic(fmt.Sprintf("component.Table.SetCellByID: unknown component ID %d", id))
	}
	t.boundsCheck(col, row)
	t.setRowValue(col, row, value)
}

// RowValues extracts all component values in row into a map keyed by component
// ID. Used during archetype migration to carry data between tables.
func (t *Table) RowValues(row int) map[ID]any {
	if row < 0 || row >= t.nRows {
		panic(fmt.Sprintf("component.Table.RowValues: row %d out of range [0,%d)", row, t.nRows))
	}
	out := make(map[ID]any, len(t.cols))
	for i, spec := range t.cols {
		if spec.Size == 0 {
			out[spec.ID] = reflect.New(spec.Type).Elem().Interface()
			continue
		}
		b := t.columnRowBytes(i, row)
		v := reflect.NewAt(spec.Type, unsafe.Pointer(&b[0])).Elem().Interface()
		out[spec.ID] = v
	}
	return out
}

// computeLayout fills sortedOffset, chunkRows, rowStride based on
// t.sortedCols and t.chunkSize.
func (t *Table) computeLayout() {
	var sumSize uintptr
	for _, c := range t.sortedCols {
		sumSize += c.Size
	}
	t.rowStride = sumSize

	if sumSize == 0 {
		// Tag-only table: no physical chunks, but rows still tracked.
		t.chunkRows = 1024
		t.sortedOffset = make([]uintptr, len(t.sortedCols))
		return
	}

	guess := t.chunkSize / int(sumSize)
	if guess < 1 {
		panic(fmt.Sprintf(
			"component.Table: row stride %d exceeds chunk size %d",
			sumSize, t.chunkSize,
		))
	}
	for guess >= 1 {
		offsets, total := layoutOffsets(t.sortedCols, guess)
		if int(total) <= t.chunkSize {
			t.chunkRows = guess
			t.sortedOffset = offsets
			return
		}
		guess--
	}
	panic("component.Table.computeLayout: failed to fit any row in chunk")
}

func layoutOffsets(cols []ColumnSpec, rows int) ([]uintptr, uintptr) {
	offs := make([]uintptr, len(cols))
	var off uintptr
	for i, c := range cols {
		off = alignUp(off, c.Align)
		offs[i] = off
		off += uintptr(rows) * c.Size
	}
	return offs, off
}

// allocChunk grows the chunk pool by one chunk.
func (t *Table) allocChunk() {
	if t.rowStride == 0 {
		return // tag-only table — no physical storage required
	}
	t.chunks = append(t.chunks, make([]byte, t.chunkSize))
}

// columnRowBytes returns the byte slice for (public column index, row).
// Caller must ensure the column has non-zero size.
func (t *Table) columnRowBytes(publicIdx, row int) []byte {
	chunkIdx := row / t.chunkRows
	rowInChunk := row % t.chunkRows
	chunk := t.chunks[chunkIdx]
	sorted := t.publicToSorted[publicIdx]
	off := t.sortedOffset[sorted] + uintptr(rowInChunk)*t.cols[publicIdx].Size
	return chunk[off : off+t.cols[publicIdx].Size]
}

// setRowValue copies a typed value into the cell, panicking on type
// mismatch.
func (t *Table) setRowValue(publicIdx, row int, value any) {
	spec := t.cols[publicIdx]
	if spec.Size == 0 {
		return
	}
	if reflect.TypeOf(value) != spec.Type {
		panic(fmt.Sprintf(
			"component.Table: value type %v does not match column %d type %v",
			reflect.TypeOf(value), publicIdx, spec.Type,
		))
	}
	dst := t.columnRowBytes(publicIdx, row)
	tmp := reflect.New(spec.Type).Elem()
	tmp.Set(reflect.ValueOf(value))
	src := unsafe.Slice((*byte)(unsafe.Pointer(tmp.UnsafeAddr())), spec.Size)
	copy(dst, src)
}

func (t *Table) boundsCheck(col, row int) {
	if col < 0 || col >= len(t.cols) {
		panic(fmt.Sprintf("component.Table: column %d out of range [0,%d)", col, len(t.cols)))
	}
	if row < 0 || row >= t.nRows {
		panic(fmt.Sprintf("component.Table: row %d out of range [0,%d)", row, t.nRows))
	}
}

// sortAndIndex returns sortedCols and a publicToSorted lookup such that
// publicToSorted[publicIdx] = sorted-position of that column.
func sortAndIndex(specs []ColumnSpec) (sortedCols []ColumnSpec, publicToSorted []int) {
	sortedCols, originalIndex := sortColumnsByAlignDesc(specs)
	publicToSorted = make([]int, len(specs))
	for sortedPos, originalPos := range originalIndex {
		publicToSorted[originalPos] = sortedPos
	}
	return sortedCols, publicToSorted
}

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
