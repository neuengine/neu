package render

// Struct-of-arrays render-object data (l1-render-core §4.12). Each registered
// key owns ONE contiguous typed slice indexed by RenderObject.dataIndex.
// Iterating one attribute (e.g. all world matrices for frustum culling) reads
// a single cache-coherent slice with no pointer chasing, and that same slice
// binds directly as a GPU instance buffer (zero marshalling). New keys can be
// registered at runtime without touching the RenderObject struct.

// dataColumn is the type-erased handle the holder stores; concrete columns are
// *typedColumn[T]. Only grow is polymorphic — typed access goes through the
// generic StaticKey/DynamicKey helpers (no per-element boxing).
type dataColumn interface {
	grow(n int)
	length() int
}

type typedColumn[T any] struct{ data []T }

func (c *typedColumn[T]) grow(n int) {
	if n <= len(c.data) {
		return
	}
	if cap(c.data) >= n {
		c.data = c.data[:n]
		return
	}
	nc := make([]T, n)
	copy(nc, c.data)
	c.data = nc
}
func (c *typedColumn[T]) length() int { return len(c.data) }

// RenderDataHolder holds one [dataColumn] per registered key plus a name→column
// index map. Not safe for concurrent key registration; per-index reads/writes
// across distinct dataIndex values are disjoint and lock-free (the cull relies
// on this).
type RenderDataHolder struct {
	arrays      []dataColumn
	definitions map[string]int
	count       int // logical object count (every column is grown to this)
}

// NewRenderDataHolder returns an empty holder.
func NewRenderDataHolder() *RenderDataHolder {
	return &RenderDataHolder{definitions: make(map[string]int)}
}

// Len reports the logical object count (every column has this length).
func (h *RenderDataHolder) Len() int { return h.count }

// Reserve grows every registered column to n entries (one per RenderObject
// dataIndex). Idempotent; never shrinks.
func (h *RenderDataHolder) Reserve(n int) {
	if n < h.count {
		return
	}
	h.count = n
	for _, c := range h.arrays {
		c.grow(n)
	}
}

// StaticKey identifies a per-object, persistent attribute column of type T
// (e.g. WorldMatrix). Zero value is invalid; obtain via [RegisterStaticKey].
type StaticKey[T any] struct{ col int }

// DynamicKey identifies a per-frame, regenerated attribute column of type T
// (e.g. SortKey). Structurally identical to StaticKey — the distinction is
// lifecycle/intent (l1-render-core §4.12), enforced by convention.
type DynamicKey[T any] struct{ col int }

// RegisterStaticKey registers (or returns the existing) persistent column
// named name with element type T. Re-registering a name with a different T
// panics (programmer error — keys are fixed at setup).
func RegisterStaticKey[T any](h *RenderDataHolder, name string) StaticKey[T] {
	return StaticKey[T]{col: registerColumn[T](h, name)}
}

// RegisterDynamicKey registers (or returns) a per-frame column named name.
func RegisterDynamicKey[T any](h *RenderDataHolder, name string) DynamicKey[T] {
	return DynamicKey[T]{col: registerColumn[T](h, name)}
}

func registerColumn[T any](h *RenderDataHolder, name string) int {
	if idx, ok := h.definitions[name]; ok {
		if _, ok := h.arrays[idx].(*typedColumn[T]); !ok {
			panic("render: key " + name + " re-registered with a different type")
		}
		return idx
	}
	col := &typedColumn[T]{}
	col.grow(h.count) // match existing object count
	idx := len(h.arrays)
	h.arrays = append(h.arrays, col)
	h.definitions[name] = idx
	return idx
}

// Slice returns the contiguous backing slice for a static key. The slice
// aliases the holder's storage — mutate in place, bind directly to a GPU
// buffer, do NOT append (use [RenderDataHolder.Reserve] to grow).
func (k StaticKey[T]) Slice(h *RenderDataHolder) []T {
	return h.arrays[k.col].(*typedColumn[T]).data
}

// Slice returns the contiguous backing slice for a dynamic key (see
// [StaticKey.Slice] aliasing semantics).
func (k DynamicKey[T]) Slice(h *RenderDataHolder) []T {
	return h.arrays[k.col].(*typedColumn[T]).data
}
