package version

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidConstraint is returned for a malformed version constraint.
var ErrInvalidConstraint = errors.New("version: invalid version constraint")

// Constraint is a conjunction of version clauses (all must hold). It supports
// the Cargo/npm subset used by plugin manifests and engine compatibility gating:
// caret `^`, tilde `~`, comparison operators (`>=`, `>`, `<=`, `<`, `=`), and
// comma-separated ranges. The zero value (and "" / "*") matches any version.
type Constraint struct {
	clauses []clause
}

type clause struct {
	op  string // ">=", ">", "<=", "<", "="
	ver Version
}

// ParseConstraint parses an `engine_version`-style constraint string.
func ParseConstraint(s string) (Constraint, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "*" {
		return Constraint{}, nil // empty/any matches everything
	}
	var c Constraint
	for raw := range strings.SplitSeq(s, ",") {
		part := strings.TrimSpace(raw)
		if part == "" {
			continue
		}
		cls, err := parseClause(part)
		if err != nil {
			return Constraint{}, err
		}
		c.clauses = append(c.clauses, cls...)
	}
	if len(c.clauses) == 0 {
		return Constraint{}, fmt.Errorf("%w: %q", ErrInvalidConstraint, s)
	}
	return c, nil
}

// IsAny reports whether the constraint matches every version (empty/"*"/zero value).
func (c Constraint) IsAny() bool { return len(c.clauses) == 0 }

// parseClause expands one constraint term into one or two comparison clauses.
func parseClause(part string) ([]clause, error) {
	switch {
	case strings.HasPrefix(part, "^"):
		return caret(part[1:])
	case strings.HasPrefix(part, "~"):
		return tilde(part[1:])
	case strings.HasPrefix(part, ">="):
		return single(">=", part[2:])
	case strings.HasPrefix(part, "<="):
		return single("<=", part[2:])
	case strings.HasPrefix(part, ">"):
		return single(">", part[1:])
	case strings.HasPrefix(part, "<"):
		return single("<", part[1:])
	case strings.HasPrefix(part, "="):
		return single("=", part[1:])
	default:
		return single("=", part) // bare version ⇒ exact
	}
}

func single(op, verStr string) ([]clause, error) {
	v, err := Parse(verStr)
	if err != nil {
		return nil, err
	}
	return []clause{{op: op, ver: v}}, nil
}

// caret expands `^x.y.z` to `>=x.y.z, <upper` using Cargo caret semantics: the
// upper bound increments the leftmost non-zero component (so on 0.x a caret pins
// the minor — 0.y is treated as a breaking boundary).
func caret(verStr string) ([]clause, error) {
	v, err := Parse(verStr)
	if err != nil {
		return nil, err
	}
	var upper Version
	switch {
	case v.Major > 0:
		upper = Version{Major: v.Major + 1}
	case v.Minor > 0:
		upper = Version{Minor: v.Minor + 1}
	default:
		upper = Version{Patch: v.Patch + 1}
	}
	return []clause{{">=", v}, {"<", upper}}, nil
}

// tilde expands `~x.y.z` to `>=x.y.z, <x.(y+1).0`.
func tilde(verStr string) ([]clause, error) {
	v, err := Parse(verStr)
	if err != nil {
		return nil, err
	}
	upper := Version{Major: v.Major, Minor: v.Minor + 1}
	return []clause{{">=", v}, {"<", upper}}, nil
}

// Matches reports whether v satisfies every clause. An empty constraint matches
// any version.
func (c Constraint) Matches(v Version) bool {
	for _, cl := range c.clauses {
		cmp := v.Compare(cl.ver)
		ok := false
		switch cl.op {
		case ">=":
			ok = cmp >= 0
		case ">":
			ok = cmp > 0
		case "<=":
			ok = cmp <= 0
		case "<":
			ok = cmp < 0
		case "=":
			ok = cmp == 0
		}
		if !ok {
			return false
		}
	}
	return true
}
