package plugin

import "github.com/neuengine/neu/pkg/version"

// Version is the engine's semantic-version type. It is a type alias for
// [version.Version] — the canonical SemVer implementation in pkg/version — so
// plugin manifests and engine compatibility gating share one parser with no
// duplicated logic.
type Version = version.Version

// ErrInvalidVersion is returned for a malformed SemVer string. Aliased from
// pkg/version so errors.Is matches identically across both packages.
var ErrInvalidVersion = version.ErrInvalidVersion

// ParseVersion parses "MAJOR.MINOR.PATCH" (a leading 'v' and any pre-release or
// build suffix are tolerated and ignored for comparison). It delegates to
// [version.Parse].
func ParseVersion(s string) (Version, error) { return version.Parse(s) }
