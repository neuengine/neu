// Package changedetect provides tick-based change-detection primitives:
// per-component Added/Changed ticks, per-column aggregates for O(1) skip,
// and (in later tasks) the Ref/Mut smart wrappers.
//
// It is a leaf package: it must not import any other ecs core package
// (entity, component, world, query). component embeds these types in its
// storage backends, and world stamps them at the boundary by converting
// world.Tick → changedetect.Tick. Keeping this package dependency-free is
// what prevents an import cycle (world → component → changedetect → world).
package changedetect

// Tick is a monotonically increasing logical time counter. It mirrors
// world.Tick (both are uint32) and is bridged by explicit conversion at the
// world boundary so this package stays dependency-free. A uint32 at 60 Hz
// wraps after ~828 days, which exceeds any single game session.
type Tick uint32

// IsNewerThan reports whether t was set strictly after last. The strict
// comparison is the whole of change detection: a component is "changed
// since" a system iff its tick exceeds that system's last-run tick.
func (t Tick) IsNewerThan(last Tick) bool { return t > last }

// ComponentTicks records the insertion and last-mutation ticks for one
// component instance on one entity. Storage backends keep these in a dense
// array parallel to the component data so iteration stays cache-friendly
// and change checks are branch-free uint32 comparisons.
type ComponentTicks struct {
	Added   Tick // tick when this component was first inserted on the entity
	Changed Tick // tick of the most recent mutation (≥ Added by construction)
}

// NewComponentTicks returns ticks with both fields set to changeTick — the
// canonical state immediately after insertion, where an added component is
// also considered changed.
func NewComponentTicks(changeTick Tick) ComponentTicks {
	return ComponentTicks{Added: changeTick, Changed: changeTick}
}

// IsAdded reports whether the component was inserted after lastSystemTick.
func (ct ComponentTicks) IsAdded(lastSystemTick Tick) bool {
	return ct.Added.IsNewerThan(lastSystemTick)
}

// IsChanged reports whether the component was mutated (or added) after
// lastSystemTick. Because Changed ≥ Added always holds, a freshly added
// component is also reported as changed — matching the L1 contract that
// Changed includes additions.
func (ct ComponentTicks) IsChanged(lastSystemTick Tick) bool {
	return ct.Changed.IsNewerThan(lastSystemTick)
}

// SetChanged advances the Changed tick to changeTick. Added is left intact
// so first-insertion time remains observable for the lifetime of the
// component instance.
func (ct *ComponentTicks) SetChanged(changeTick Tick) {
	ct.Changed = changeTick
}

// ColumnTicks is the per-column aggregate that lets a Changed/Added query
// skip an entire archetype column in O(1): it is the running maximum of
// every ComponentTicks.Changed / .Added folded into the column.
type ColumnTicks struct {
	ColumnChangedTick Tick // max ComponentTicks.Changed across the column
	ColumnAddedTick   Tick // max ComponentTicks.Added across the column
}

// Observe folds one component's ticks into the column aggregate. It only
// ever raises the maxima, so removing a row never lowers the aggregate —
// the aggregate is intentionally conservative (it may report a column as
// possibly-changed when it is not; it never reports a changed column as
// unchanged, which would lose updates).
func (c *ColumnTicks) Observe(ct ComponentTicks) {
	if ct.Changed > c.ColumnChangedTick {
		c.ColumnChangedTick = ct.Changed
	}
	if ct.Added > c.ColumnAddedTick {
		c.ColumnAddedTick = ct.Added
	}
}

// Reset clears the aggregate back to the zero tick. Used when a column's
// storage is fully cleared.
func (c *ColumnTicks) Reset() { *c = ColumnTicks{} }

// MayHaveChanged reports whether any component in the column could have
// changed after lastSystemTick. A false result means the whole column can
// be skipped safely.
func (c ColumnTicks) MayHaveChanged(lastSystemTick Tick) bool {
	return c.ColumnChangedTick.IsNewerThan(lastSystemTick)
}

// MayHaveAdded reports whether any component in the column could have been
// added after lastSystemTick. A false result means the whole column can be
// skipped safely.
func (c ColumnTicks) MayHaveAdded(lastSystemTick Tick) bool {
	return c.ColumnAddedTick.IsNewerThan(lastSystemTick)
}
