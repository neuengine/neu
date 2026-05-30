package window

import (
	"errors"

	"github.com/neuengine/neu/pkg/ecs"
)

// ErrMultiplePrimary signals that more than one entity holds the PrimaryWindow
// marker, violating INV-1 (exactly one primary at any time).
var ErrMultiplePrimary = errors.New("window: more than one PrimaryWindow exists (INV-1)")

// PrimaryWindowRes caches the primary window entity for O(1) lookup. Set is
// false when no primary exists (headless, or after the primary is despawned).
type PrimaryWindowRes struct {
	Entity ecs.Entity
	Set    bool
}

// CheckSinglePrimary enforces INV-1 given the current count of PrimaryWindow
// markers. Zero (headless) or one is valid; more than one is a developer error.
func CheckSinglePrimary(count int) error {
	if count > 1 {
		return ErrMultiplePrimary
	}
	return nil
}

// SetPrimary records the primary window entity.
func (r *PrimaryWindowRes) SetPrimary(e ecs.Entity) {
	r.Entity = e
	r.Set = true
}

// Clear marks the primary as absent (e.g. after despawn), so a close handler
// knows the primary is gone.
func (r *PrimaryWindowRes) Clear() {
	r.Entity = ecs.Entity{}
	r.Set = false
}

// IsPrimary reports whether e is the currently-tracked primary window.
func (r *PrimaryWindowRes) IsPrimary(e ecs.Entity) bool {
	return r.Set && r.Entity == e
}
