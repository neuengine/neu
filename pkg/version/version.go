// Package version implements the engine's semantic-versioning policy: a parsed
// SemVer [Version], a Cargo/npm-subset [Constraint] grammar (caret/tilde/range/
// exact), and the engine's Go-toolchain compatibility policy. It is the single
// source of truth reused by plugin manifests (pkg/plugin) and engine
// compatibility gating (Track J) — there is exactly one SemVer implementation to
// keep correct, so the two consumers can never drift apart.
//
// Bootstrap: l1-compatibility-policy (governance L1; Track J).
package version

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrInvalidVersion is returned for a malformed SemVer string.
var ErrInvalidVersion = errors.New("version: invalid semantic version")

// Version is a parsed semantic version (major.minor.patch). Pre-release and
// build metadata are accepted in the string form but compared only on the
// numeric core — sufficient for engine + plugin compatibility gating.
type Version struct {
	Major, Minor, Patch uint64
}

// Parse parses "MAJOR.MINOR.PATCH". A leading 'v' and any pre-release/build
// suffix (after '-' or '+') are tolerated and ignored for comparison. Fewer than
// three components default the omitted trailing fields to zero ("1" ⇒ 1.0.0).
func Parse(s string) (Version, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	// Drop pre-release/build metadata for the numeric comparison.
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return Version{}, fmt.Errorf("%w: %q", ErrInvalidVersion, s)
	}
	var v Version
	dst := []*uint64{&v.Major, &v.Minor, &v.Patch}
	for i, p := range parts {
		n, err := strconv.ParseUint(p, 10, 64)
		if err != nil {
			return Version{}, fmt.Errorf("%w: %q", ErrInvalidVersion, s)
		}
		*dst[i] = n
	}
	return v, nil
}

// Compare returns -1, 0, or +1 as v is less than, equal to, or greater than o.
func (v Version) Compare(o Version) int {
	switch {
	case v.Major != o.Major:
		return cmpU(v.Major, o.Major)
	case v.Minor != o.Minor:
		return cmpU(v.Minor, o.Minor)
	default:
		return cmpU(v.Patch, o.Patch)
	}
}

func cmpU(a, b uint64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// String renders the version as "major.minor.patch".
func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}
