package changedetect

// Ref is a read-only smart wrapper exposing a component pointer together
// with its change-detection metadata. Obtaining or reading a Ref never
// marks the component as changed. It is a stack value (zero heap cost)
// constructed during query iteration.
//
// A Ref must not be retained across frames: lastSystemTick is captured at
// construction and the comparison becomes meaningless once the world tick
// advances. This is a documented usage constraint, not enforced at runtime.
type Ref[T any] struct {
	value          *T
	ticks          ComponentTicks
	lastSystemTick Tick
}

// NewRef builds a read-only wrapper. lastSystemTick is the tick at which
// the observing system last ran (engine-level callers pass the world's
// last-cleared tick).
func NewRef[T any](value *T, ticks ComponentTicks, lastSystemTick Tick) Ref[T] {
	return Ref[T]{value: value, ticks: ticks, lastSystemTick: lastSystemTick}
}

// Value returns the component pointer. Read-only by convention — mutating
// through it bypasses change detection (use [Mut] when mutation is intended).
func (r Ref[T]) Value() *T { return r.value }

// IsChanged reports whether the component was mutated (or added) since the
// observing system last ran.
func (r Ref[T]) IsChanged() bool { return r.ticks.IsChanged(r.lastSystemTick) }

// IsAdded reports whether the component was inserted since the observing
// system last ran.
func (r Ref[T]) IsAdded() bool { return r.ticks.IsAdded(r.lastSystemTick) }

// LastChanged returns the raw Changed tick.
func (r Ref[T]) LastChanged() Tick { return r.ticks.Changed }

// Mut is a writable smart wrapper. Obtaining mutable access via [Mut.Value]
// marks the component as changed at changeTick — the engine deliberately
// treats "took a mutable pointer" as "mutated" to avoid value-diffing on the
// hot path. IsChanged/IsAdded report the state as observed by the querying
// system, i.e. before this system's own mutation (prevChanged is captured at
// construction so the result is independent of Value() call order).
type Mut[T any] struct {
	value          *T
	ticks          *ComponentTicks // points into live storage
	lastSystemTick Tick
	changeTick     Tick
	prevChanged    Tick // Changed value at construction (pre-this-system)
}

// NewMut builds a writable wrapper over the live ticks slot. changeTick is
// the current world change tick; lastSystemTick is the observing system's
// last-run tick.
func NewMut[T any](value *T, ticks *ComponentTicks, lastSystemTick, changeTick Tick) Mut[T] {
	return Mut[T]{
		value:          value,
		ticks:          ticks,
		lastSystemTick: lastSystemTick,
		changeTick:     changeTick,
		prevChanged:    ticks.Changed,
	}
}

// Value returns a mutable pointer and marks the component changed at
// changeTick. Idempotent: re-taking the pointer does not advance the tick
// further within the same system run.
func (m Mut[T]) Value() *T {
	m.ticks.SetChanged(m.changeTick)
	return m.value
}

// IsChanged reports whether the component was mutated since the observing
// system last ran, evaluated against the pre-mutation Changed tick so the
// answer is unaffected by this system's own [Mut.Value] call.
func (m Mut[T]) IsChanged() bool { return m.prevChanged.IsNewerThan(m.lastSystemTick) }

// IsAdded reports whether the component was inserted since the observing
// system last ran.
func (m Mut[T]) IsAdded() bool { return m.ticks.IsAdded(m.lastSystemTick) }

// SetChanged explicitly marks the component changed at the current tick
// without dereferencing the value (useful when mutating via a stored
// pointer obtained elsewhere).
func (m Mut[T]) SetChanged() { m.ticks.SetChanged(m.changeTick) }

// BypassChangeDetection returns the mutable pointer WITHOUT marking the
// component changed. For cache-warming or non-semantic writes only.
func (m Mut[T]) BypassChangeDetection() *T { return m.value }
