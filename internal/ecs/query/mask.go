// Package query implements archetype-matching primitives for the ECS runtime:
// component-set bitmasks, read/write/exclusive access metadata, and the
// [QueryState] that combines them. Higher-level multi-arity query types
// (Query1, Query2, …) and concrete filters (With, Without, Changed, Added)
// build on these primitives.
package query

import (
	"math/bits"
	"strconv"
	"strings"

	"github.com/teratron/boltengine/internal/ecs/component"
)

// MaskBits is the maximum number of distinct components a [Mask] can address.
// Phase 1 fixes this at 128 to keep masks register-sized; future phases will
// upgrade to a dynamic-width representation when component counts exceed it.
const MaskBits = 128

// Mask is a 128-bit set of [component.ID]s. The zero value is the empty set
// and is safe to use directly. Mask is a value type — copies are independent.
//
// Layout: bits 0–63 live in lo, bits 64–127 in hi. A component with ID `n`
// is encoded at bit position `n`. IDs ≥ MaskBits cause every mutator
// ([Mask.Set], [Mask.Clear]) to panic — overflow is a programmer error, not
// a runtime condition.
type Mask struct {
	lo, hi uint64
}

// NewMask returns a [Mask] populated from the given component IDs. A nil or
// empty slice returns the zero mask. Panics if any id is out of range.
func NewMask(ids ...component.ID) Mask {
	var m Mask
	for _, id := range ids {
		m.Set(id)
	}
	return m
}

// Set marks id as present in the mask. Panics if id ≥ [MaskBits].
func (m *Mask) Set(id component.ID) {
	pos := uint(id)
	if pos >= MaskBits {
		panic("query.Mask.Set: component ID out of range (max " + strconv.Itoa(MaskBits-1) + ")")
	}
	if pos < 64 {
		m.lo |= 1 << pos
	} else {
		m.hi |= 1 << (pos - 64)
	}
}

// Clear removes id from the mask. Panics if id ≥ [MaskBits].
func (m *Mask) Clear(id component.ID) {
	pos := uint(id)
	if pos >= MaskBits {
		panic("query.Mask.Clear: component ID out of range (max " + strconv.Itoa(MaskBits-1) + ")")
	}
	if pos < 64 {
		m.lo &^= 1 << pos
	} else {
		m.hi &^= 1 << (pos - 64)
	}
}

// Has reports whether id is present in the mask. IDs ≥ [MaskBits] always
// report false (out-of-range queries are non-fatal — only mutation is).
func (m Mask) Has(id component.ID) bool {
	pos := uint(id)
	if pos >= MaskBits {
		return false
	}
	if pos < 64 {
		return m.lo&(1<<pos) != 0
	}
	return m.hi&(1<<(pos-64)) != 0
}

// IsZero reports whether the mask contains no bits.
func (m Mask) IsZero() bool { return m.lo == 0 && m.hi == 0 }

// Equal reports whether m and other carry exactly the same bit set.
func (m Mask) Equal(other Mask) bool { return m.lo == other.lo && m.hi == other.hi }

// Contains reports whether other is a subset of m (every bit set in other is
// also set in m). The empty mask is contained in every mask.
func (m Mask) Contains(other Mask) bool {
	return m.lo&other.lo == other.lo && m.hi&other.hi == other.hi
}

// IsDisjoint reports whether m and other share no bits.
func (m Mask) IsDisjoint(other Mask) bool {
	return m.lo&other.lo == 0 && m.hi&other.hi == 0
}

// Intersects reports whether m and other share at least one bit. Inverse of
// [Mask.IsDisjoint].
func (m Mask) Intersects(other Mask) bool { return !m.IsDisjoint(other) }

// Or returns the bitwise union of m and other.
func (m Mask) Or(other Mask) Mask {
	return Mask{lo: m.lo | other.lo, hi: m.hi | other.hi}
}

// And returns the bitwise intersection of m and other.
func (m Mask) And(other Mask) Mask {
	return Mask{lo: m.lo & other.lo, hi: m.hi & other.hi}
}

// AndNot returns m with every bit also set in other cleared.
func (m Mask) AndNot(other Mask) Mask {
	return Mask{lo: m.lo &^ other.lo, hi: m.hi &^ other.hi}
}

// Count returns the number of components present in the mask.
func (m Mask) Count() int { return bits.OnesCount64(m.lo) + bits.OnesCount64(m.hi) }

// ForEach calls fn for each component ID present in the mask in ascending
// order. Iteration stops early if fn returns false.
func (m Mask) ForEach(fn func(component.ID) bool) {
	lo := m.lo
	for lo != 0 {
		pos := bits.TrailingZeros64(lo)
		if !fn(component.ID(pos)) {
			return
		}
		lo &^= 1 << uint(pos)
	}
	hi := m.hi
	for hi != 0 {
		pos := bits.TrailingZeros64(hi)
		if !fn(component.ID(pos + 64)) {
			return
		}
		hi &^= 1 << uint(pos)
	}
}

// IDs returns the component IDs encoded in the mask, in ascending order.
// Allocates a slice sized to [Mask.Count]; callers performing tight-loop
// matching should prefer [Mask.ForEach].
func (m Mask) IDs() []component.ID {
	out := make([]component.ID, 0, m.Count())
	m.ForEach(func(id component.ID) bool {
		out = append(out, id)
		return true
	})
	return out
}

// String returns a human-readable rendering: "Mask{}" for the empty mask,
// otherwise "Mask{id, id, …}" with IDs in ascending order. Intended for
// debugging and test failure messages.
func (m Mask) String() string {
	if m.IsZero() {
		return "Mask{}"
	}
	var b strings.Builder
	b.WriteString("Mask{")
	first := true
	m.ForEach(func(id component.ID) bool {
		if !first {
			b.WriteString(", ")
		}
		first = false
		b.WriteString(strconv.FormatUint(uint64(id), 10))
		return true
	})
	b.WriteByte('}')
	return b.String()
}

// MaskFromIDs is a convenience builder for callers that already hold a slice
// of component IDs (e.g. an archetype's componentIDs). Equivalent to
// [NewMask] but avoids the variadic copy.
func MaskFromIDs(ids []component.ID) Mask {
	var m Mask
	for _, id := range ids {
		m.Set(id)
	}
	return m
}
