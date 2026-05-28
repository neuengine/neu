package mesh

import (
	"hash/fnv"
	"sort"
)

// layoutElement is one vertex attribute descriptor within a VertexLayout.
type layoutElement struct {
	Kind   AttrKind
	Format VertexFormat
	Offset uint32
}

// VertexLayout is the immutable, hashed description of vertex attribute
// arrangement used as a fragment of the pipeline-specialisation key.
// The hash is FNV-1a over the sorted element list — deterministic across runs.
type VertexLayout struct {
	stride   uint32
	elements []layoutElement
	Hash     uint64 // FNV-1a; part of pipeline-spec key (PipelineDesc.LayoutHash)
}

// Stride returns the per-vertex byte stride (interleaved layout).
func (l VertexLayout) Stride() uint32 { return l.stride }

// Elements returns the attribute descriptors in sorted (by AttrKind) order.
func (l VertexLayout) Elements() []layoutElement { return l.elements }

// buildLayout constructs a VertexLayout from an attribute map.
// Attributes are sorted by AttrKind to ensure a stable hash regardless of
// insertion order. This is the only point where map iteration order matters.
func buildLayout(attrs map[AttrKind]VertexAttribute) VertexLayout {
	// Collect and sort attribute entries by kind for determinism.
	type entry struct {
		kind AttrKind
		fmt  VertexFormat
	}
	entries := make([]entry, 0, len(attrs))
	for k, a := range attrs {
		entries = append(entries, entry{k, a.Format})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].kind < entries[j].kind })

	elements := make([]layoutElement, len(entries))
	var offset, stride uint32
	for i, e := range entries {
		sz := e.fmt.Size()
		elements[i] = layoutElement{Kind: e.kind, Format: e.fmt, Offset: offset}
		offset += sz
		stride += sz
	}

	hash := hashLayout(elements)
	return VertexLayout{stride: stride, elements: elements, Hash: hash}
}

// hashLayout computes FNV-1a over the element descriptor list. The encoding
// writes kind (1 byte), format (1 byte), offset (4 bytes LE) per element,
// then stride (4 bytes LE) as a final discriminant.
func hashLayout(elements []layoutElement) uint64 {
	h := fnv.New64a()
	var buf [6]byte
	for _, e := range elements {
		buf[0] = byte(e.Kind)
		buf[1] = byte(e.Format)
		buf[2] = byte(e.Offset)
		buf[3] = byte(e.Offset >> 8)
		buf[4] = byte(e.Offset >> 16)
		buf[5] = byte(e.Offset >> 24)
		_, _ = h.Write(buf[:])
	}
	return h.Sum64()
}
