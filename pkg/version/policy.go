package version

import "strings"

// Engine compatibility policy (Track J / l1-compatibility-policy).
//
// The engine versions itself with Cargo-subset SemVer (see [Constraint]):
//   - MAJOR — removals or behavioural breaks in the public `pkg/` surface.
//   - MINOR — backward-compatible additions.
//   - PATCH — backward-compatible fixes.
//
// Pre-1.0 (0.x), a caret constraint treats each MINOR bump as a breaking
// boundary (`^0.4` ⇒ `>=0.4.0, <0.5.0`), matching Cargo. Plugin manifests
// declare their `engine_version` requirement with this grammar and the engine
// evaluates it via [Constraint.Matches].

// MinGoToolchain is the minimum Go toolchain the engine supports. It mirrors the
// `go` directive in go.mod; building on an older toolchain is unsupported.
const MinGoToolchain = "1.26.3"

// GoToolchainConstraint returns the constraint a Go toolchain must satisfy to
// build the engine: `>= MinGoToolchain`. It dogfoods the SemVer machinery so the
// toolchain policy uses exactly the same evaluator as plugin compatibility.
func GoToolchainConstraint() Constraint {
	c, err := ParseConstraint(">=" + MinGoToolchain)
	if err != nil {
		// MinGoToolchain is a compile-time constant; a parse failure is a bug.
		panic("version: invalid MinGoToolchain constant: " + err.Error())
	}
	return c
}

// IsGoToolchainSupported reports whether goVersion satisfies the engine's
// minimum-toolchain policy. It accepts the forms returned by runtime.Version()
// and the `go` directive alike (e.g. "go1.26.3", "1.26.3", "v1.27").
func IsGoToolchainSupported(goVersion string) (bool, error) {
	v, err := Parse(strings.TrimPrefix(strings.TrimSpace(goVersion), "go"))
	if err != nil {
		return false, err
	}
	return GoToolchainConstraint().Matches(v), nil
}
