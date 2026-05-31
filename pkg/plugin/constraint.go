package plugin

import "github.com/neuengine/neu/pkg/version"

// Constraint is a conjunction of SemVer clauses (the Cargo/npm subset used by a
// manifest's engine_version: caret `^`, tilde `~`, comparison operators, and
// comma-separated ranges). It is a type alias for [version.Constraint] so the
// grammar has a single implementation shared with engine compatibility gating.
type Constraint = version.Constraint

// ErrInvalidConstraint is returned for a malformed version constraint. Aliased
// from pkg/version.
var ErrInvalidConstraint = version.ErrInvalidConstraint

// ParseConstraint parses a manifest engine_version constraint string. It
// delegates to [version.ParseConstraint].
func ParseConstraint(s string) (Constraint, error) { return version.ParseConstraint(s) }
