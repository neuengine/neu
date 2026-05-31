package version

import (
	"errors"
	"testing"
)

func TestParse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in              string
		want            Version
		wantErr         bool
	}{
		{"1.4.2", Version{1, 4, 2}, false},
		{"v1.4.2-rc1+build", Version{1, 4, 2}, false}, // 'v' + pre-release/build stripped
		{" 2.0.0 ", Version{2, 0, 0}, false},          // surrounding space trimmed
		{"1", Version{1, 0, 0}, false},                // trailing components default to 0
		{"1.5", Version{1, 5, 0}, false},
		{"0.0.0", Version{}, false},
		{"1.x.0", Version{}, true},
		{"1.2.3.4", Version{}, true}, // too many components
		{"", Version{}, true},        // empty → not a number
		{"abc", Version{}, true},
	}
	for _, tc := range tests {
		got, err := Parse(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("Parse(%q) expected error, got %+v", tc.in, got)
			} else if !errors.Is(err, ErrInvalidVersion) {
				t.Errorf("Parse(%q) err = %v, want ErrInvalidVersion", tc.in, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("Parse(%q) unexpected error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("Parse(%q) = %+v, want %+v", tc.in, got, tc.want)
		}
	}
}

func TestCompareAndString(t *testing.T) {
	t.Parallel()
	a, b := Version{1, 4, 0}, Version{1, 5, 0}
	if a.Compare(b) != -1 || b.Compare(a) != 1 || a.Compare(a) != 0 {
		t.Errorf("Compare ordering wrong: a<b=%d b>a=%d a==a=%d", a.Compare(b), b.Compare(a), a.Compare(a))
	}
	// Each component dominates in order.
	if (Version{2, 0, 0}).Compare(Version{1, 9, 9}) != 1 {
		t.Error("major should dominate minor/patch")
	}
	if (Version{1, 2, 0}).Compare(Version{1, 1, 9}) != 1 {
		t.Error("minor should dominate patch")
	}
	if (Version{1, 2, 3}).Compare(Version{1, 2, 4}) != -1 {
		t.Error("patch compare wrong")
	}
	if got := (Version{1, 2, 3}).String(); got != "1.2.3" {
		t.Errorf("String() = %q, want 1.2.3", got)
	}
}

func TestConstraintMatrix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		constraint string
		version    string
		want       bool
	}{
		// caret: next non-zero leftmost component.
		{"^1.2.3", "1.2.3", true},
		{"^1.2.3", "1.9.9", true},
		{"^1.2.3", "2.0.0", false},
		{"^1.2.3", "1.2.2", false},
		// caret on 0.x pins the minor (0.y is a breaking boundary).
		{"^0.1.5", "0.1.9", true},
		{"^0.1.5", "0.2.0", false},
		{"^0.0.3", "0.0.3", true},
		{"^0.0.3", "0.0.4", false},
		// tilde: pins the minor.
		{"~1.4.0", "1.4.9", true},
		{"~1.4.0", "1.5.0", false},
		// comparison operators.
		{">=1.0.0", "1.0.0", true},
		{">=1.0.0", "0.9.9", false},
		{">1.0.0", "1.0.0", false},
		{"<=2.0.0", "2.0.0", true},
		{"<2.0.0", "2.0.0", false},
		{"=1.2.3", "1.2.3", true},
		{"=1.2.3", "1.2.4", false},
		{"1.2.3", "1.2.3", true}, // bare ⇒ exact
		// comma-separated range (conjunction).
		{">=1.0.0, <2.0.0", "1.5.0", true},
		{">=1.0.0, <2.0.0", "2.0.0", false},
		// empty/any.
		{"", "9.9.9", true},
		{"*", "0.0.1", true},
	}
	for _, tc := range tests {
		c, err := ParseConstraint(tc.constraint)
		if err != nil {
			t.Fatalf("ParseConstraint(%q): %v", tc.constraint, err)
		}
		v, err := Parse(tc.version)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tc.version, err)
		}
		if got := c.Matches(v); got != tc.want {
			t.Errorf("%q matches %q = %v, want %v", tc.constraint, tc.version, got, tc.want)
		}
	}
}

func TestConstraintErrors(t *testing.T) {
	t.Parallel()
	// Bad version inside a clause → ErrInvalidVersion (propagated through the parser).
	if _, err := ParseConstraint(">=bad"); !errors.Is(err, ErrInvalidVersion) {
		t.Errorf("ParseConstraint(>=bad) err = %v, want ErrInvalidVersion", err)
	}
	if _, err := ParseConstraint("^x"); !errors.Is(err, ErrInvalidVersion) {
		t.Errorf("ParseConstraint(^x) err = %v", err)
	}
	if _, err := ParseConstraint("~y"); !errors.Is(err, ErrInvalidVersion) {
		t.Errorf("ParseConstraint(~y) err = %v", err)
	}
	// A constraint that trims to nothing but is non-empty (only commas) → invalid.
	if _, err := ParseConstraint(",,"); !errors.Is(err, ErrInvalidConstraint) {
		t.Errorf("ParseConstraint(,,) err = %v, want ErrInvalidConstraint", err)
	}
}

func TestConstraintIsAny(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"", "*", "  "} {
		c, err := ParseConstraint(s)
		if err != nil {
			t.Fatalf("ParseConstraint(%q): %v", s, err)
		}
		if !c.IsAny() {
			t.Errorf("ParseConstraint(%q).IsAny() = false, want true", s)
		}
	}
	c, _ := ParseConstraint(">=1.0.0")
	if c.IsAny() {
		t.Error(">=1.0.0 should not be IsAny")
	}
	// Zero value matches everything.
	if !(Constraint{}).Matches(Version{9, 9, 9}) {
		t.Error("zero-value Constraint should match any version")
	}
}

func TestGoToolchainPolicy(t *testing.T) {
	t.Parallel()
	// The minimum toolchain itself is supported.
	if ok, err := IsGoToolchainSupported(MinGoToolchain); err != nil || !ok {
		t.Errorf("IsGoToolchainSupported(%q) = %v,%v; want true,nil", MinGoToolchain, ok, err)
	}
	// runtime.Version()-style "go" prefix is accepted.
	if ok, _ := IsGoToolchainSupported("go1.27.0"); !ok {
		t.Error("go1.27.0 should be supported (>= min)")
	}
	// Below the minimum is unsupported.
	if ok, _ := IsGoToolchainSupported("1.25.0"); ok {
		t.Error("1.25.0 should be unsupported (< min)")
	}
	// Malformed toolchain string surfaces the parse error.
	if _, err := IsGoToolchainSupported("not-a-version"); !errors.Is(err, ErrInvalidVersion) {
		t.Errorf("IsGoToolchainSupported(garbage) err = %v, want ErrInvalidVersion", err)
	}
	// The constraint is well-formed and not "any".
	if GoToolchainConstraint().IsAny() {
		t.Error("GoToolchainConstraint() should be a real >= bound, not any")
	}
}
