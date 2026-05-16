package component

import (
	"cmp"
	"reflect"
	"slices"
)

// ColumnSpec describes a single column inside a [Table] or [SparseSet]. It is
// the minimum set of metadata required for raw-memory storage: the component
// ID for routing, and the size/alignment for layout math.
type ColumnSpec struct {
	Type  reflect.Type
	Align uintptr
	Size  uintptr
	ID    ID
}

// IsZeroSized reports whether the column carries no payload (a tag).
func (c ColumnSpec) IsZeroSized() bool { return c.Size == 0 }

// ColumnSpecFromInfo derives a ColumnSpec from a registered [Info].
func ColumnSpecFromInfo(info *Info) ColumnSpec {
	return ColumnSpec{
		ID:    info.ID,
		Size:  info.Size,
		Align: info.Alignment,
		Type:  info.Type,
	}
}

// alignUp rounds offset up to the nearest multiple of align. align must be a
// power of two; align == 0 is treated as no alignment requirement.
func alignUp(offset, align uintptr) uintptr {
	if align <= 1 {
		return offset
	}
	return (offset + align - 1) &^ (align - 1)
}

// sortColumnsByAlignDesc returns a permutation of cols (and their original
// indices) sorted by Align desc, then Size desc. Tie-break by ID for full
// determinism — required for archetype hashing.
func sortColumnsByAlignDesc(cols []ColumnSpec) (sorted []ColumnSpec, originalIndex []int) {
	type indexed struct {
		col ColumnSpec
		idx int
	}
	tmp := make([]indexed, len(cols))
	for i, c := range cols {
		tmp[i] = indexed{c, i}
	}

	slices.SortStableFunc(tmp, func(a, b indexed) int {
		if a.col.Align != b.col.Align {
			return cmp.Compare(b.col.Align, a.col.Align) // desc
		}
		if a.col.Size != b.col.Size {
			return cmp.Compare(b.col.Size, a.col.Size) // desc
		}
		return cmp.Compare(a.col.ID, b.col.ID) // asc tie-break
	})

	sorted = make([]ColumnSpec, len(cols))
	originalIndex = make([]int, len(cols))
	for i, t := range tmp {
		sorted[i] = t.col
		originalIndex[i] = t.idx
	}
	return sorted, originalIndex
}
