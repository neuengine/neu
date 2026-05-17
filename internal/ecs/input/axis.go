package input

// AxisInput tracks analog input values (gamepad sticks, triggers).
// T identifies the axis. Values are typically −1.0..1.0 for sticks
// and 0.0..1.0 for triggers.
type AxisInput[T comparable] struct {
	values map[T]float64
}

// NewAxisInput creates an empty AxisInput resource.
func NewAxisInput[T comparable]() *AxisInput[T] {
	return &AxisInput[T]{values: make(map[T]float64, 8)}
}

// Get returns the current value of the axis, or 0 if not set.
func (a *AxisInput[T]) Get(axis T) float64 { return a.values[axis] }

// Set updates the current value of axis.
func (a *AxisInput[T]) Set(axis T, value float64) { a.values[axis] = value }

// Reset sets all axis values to zero.
func (a *AxisInput[T]) Reset() { clear(a.values) }
