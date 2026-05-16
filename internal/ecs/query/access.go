package query

import (
	"strconv"
	"strings"

	"github.com/neuengine/neu/internal/ecs/component"
)

// Access describes the component access requirements of a query or system.
// It feeds the scheduler's conflict detector: two systems may run in
// parallel only when their Access sets do not [Access.Conflicts].
//
// Three access modes are tracked:
//
//   - Read: shared, immutable access (multiple readers allowed).
//   - Write: exclusive access for mutation (one writer, no concurrent readers).
//   - Exclusive: full reservation of the component on the system's behalf
//     (no other reader or writer may run concurrently); used for structural
//     operations such as resource swaps and command-buffer flushes.
//
// Each mode is stored as a 128-bit [Mask]. The zero value is a valid empty
// access. Read/Write/Exclusive sets may overlap — the resolution rule lives
// in [Access.Conflicts] and [Access.Validate]: declaring the same component
// in both Read and Write is allowed (Write supersedes Read for conflict
// purposes), but declaring it Exclusive together with Read or Write is a
// configuration error.
type Access struct {
	Read      Mask
	Write     Mask
	Exclusive Mask
}

// AddRead marks id as read by the access set.
func (a *Access) AddRead(id component.ID) { a.Read.Set(id) }

// AddWrite marks id as written by the access set.
func (a *Access) AddWrite(id component.ID) { a.Write.Set(id) }

// AddExclusive marks id as exclusively reserved by the access set.
func (a *Access) AddExclusive(id component.ID) { a.Exclusive.Set(id) }

// Touches reports whether id appears in any of the three sets.
func (a Access) Touches(id component.ID) bool {
	return a.Read.Has(id) || a.Write.Has(id) || a.Exclusive.Has(id)
}

// IsEmpty reports whether the access set requests nothing.
func (a Access) IsEmpty() bool {
	return a.Read.IsZero() && a.Write.IsZero() && a.Exclusive.IsZero()
}

// Merge returns the union of a and other across all three sets.
func (a Access) Merge(other Access) Access {
	return Access{
		Read:      a.Read.Or(other.Read),
		Write:     a.Write.Or(other.Write),
		Exclusive: a.Exclusive.Or(other.Exclusive),
	}
}

// Conflicts reports whether a and other cannot run concurrently. The
// conflict rules:
//
//   - Exclusive vs anything (read, write, exclusive) on the same component.
//   - Write vs Write on the same component.
//   - Write vs Read on the same component.
//   - Read vs Read: never a conflict.
//
// Symmetric: a.Conflicts(b) == b.Conflicts(a).
func (a Access) Conflicts(other Access) bool {
	// Exclusive on either side blocks any access on the other side.
	otherTouched := other.Read.Or(other.Write).Or(other.Exclusive)
	if a.Exclusive.Intersects(otherTouched) {
		return true
	}
	aTouched := a.Read.Or(a.Write).Or(a.Exclusive)
	if other.Exclusive.Intersects(aTouched) {
		return true
	}
	// Write conflicts with any read or write on the other side.
	if a.Write.Intersects(other.Write) || a.Write.Intersects(other.Read) {
		return true
	}
	if other.Write.Intersects(a.Read) {
		return true
	}
	return false
}

// IsDisjoint reports whether a and other can run concurrently. Inverse of
// [Access.Conflicts].
func (a Access) IsDisjoint(other Access) bool { return !a.Conflicts(other) }

// Validate reports whether the access set is internally consistent. Returns
// nil for valid sets.
//
// Rules enforced:
//
//   - Exclusive must not overlap Read or Write — a component cannot be both
//     "shared with myself" and "denied to everyone else".
//
// Note: Read and Write may overlap — declaring a component Read and Write
// together is interpreted as "I read and write it", and Conflicts treats
// the Write as authoritative.
func (a Access) Validate() error {
	if a.Exclusive.Intersects(a.Read) || a.Exclusive.Intersects(a.Write) {
		conflicting := a.Exclusive.And(a.Read.Or(a.Write))
		return &accessConfigError{
			reason: "exclusive set overlaps read/write set",
			ids:    conflicting.IDs(),
		}
	}
	return nil
}

// String returns a debug rendering of the access set.
func (a Access) String() string {
	var b strings.Builder
	b.WriteString("Access{Read=")
	b.WriteString(a.Read.String())
	b.WriteString(", Write=")
	b.WriteString(a.Write.String())
	b.WriteString(", Exclusive=")
	b.WriteString(a.Exclusive.String())
	b.WriteByte('}')
	return b.String()
}

type accessConfigError struct {
	reason string
	ids    []component.ID
}

func (e *accessConfigError) Error() string {
	var b strings.Builder
	b.WriteString("query.Access: ")
	b.WriteString(e.reason)
	b.WriteString(" on components ")
	for i, id := range e.ids {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.FormatUint(uint64(id), 10))
	}
	return b.String()
}
