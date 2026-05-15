package query

import (
	"strings"
	"testing"

	"github.com/teratron/boltengine/internal/ecs/component"
)

func TestNewQueryStateAutoRead(t *testing.T) {
	t.Parallel()

	q, err := NewQueryState([]component.ID{1, 2}, nil, Access{})
	if err != nil {
		t.Fatalf("NewQueryState failed: %v", err)
	}
	a := q.Access()
	if !a.Read.Has(1) || !a.Read.Has(2) {
		t.Fatalf("required ids must be auto-added to Read; got %s", a.Read)
	}
	if !a.Write.IsZero() || !a.Exclusive.IsZero() {
		t.Fatal("Write and Exclusive must remain empty when not explicitly set")
	}
}

func TestNewQueryStateRespectsExplicitWrite(t *testing.T) {
	t.Parallel()

	var explicit Access
	explicit.AddWrite(1)
	q, err := NewQueryState([]component.ID{1, 2}, nil, explicit)
	if err != nil {
		t.Fatalf("NewQueryState failed: %v", err)
	}
	a := q.Access()
	if a.Read.Has(1) {
		t.Fatal("explicit Write must NOT also become Read")
	}
	if !a.Read.Has(2) {
		t.Fatal("required id 2 must be auto-added to Read")
	}
	if !a.Write.Has(1) {
		t.Fatal("explicit Write must be preserved")
	}
}

func TestNewQueryStateRespectsExplicitExclusive(t *testing.T) {
	t.Parallel()

	var explicit Access
	explicit.AddExclusive(1)
	q, err := NewQueryState([]component.ID{1}, nil, explicit)
	if err != nil {
		t.Fatalf("NewQueryState failed: %v", err)
	}
	a := q.Access()
	if a.Read.Has(1) {
		t.Fatal("explicit Exclusive must NOT also become Read")
	}
	if !a.Exclusive.Has(1) {
		t.Fatal("explicit Exclusive must be preserved")
	}
}

func TestNewQueryStateValidationError(t *testing.T) {
	t.Parallel()

	// Caller declares id 1 as Exclusive AND id 1 also in Read explicitly —
	// validation must reject because Exclusive ∩ Read ≠ ∅.
	var bad Access
	bad.AddExclusive(1)
	bad.AddRead(1)
	_, err := NewQueryState(nil, nil, bad)
	if err == nil {
		t.Fatal("invalid Access must propagate from NewQueryState")
	}
	if !strings.Contains(err.Error(), "exclusive") {
		t.Fatalf("error must mention exclusive overlap; got %q", err.Error())
	}
}

func TestQueryStateMatchesRequired(t *testing.T) {
	t.Parallel()

	q, err := NewQueryState([]component.ID{1, 2}, nil, Access{})
	if err != nil {
		t.Fatal(err)
	}
	if !q.Matches(NewMask(1, 2, 3)) {
		t.Fatal("archetype with all required ids must match")
	}
	if !q.Matches(NewMask(1, 2)) {
		t.Fatal("exact-match archetype must match")
	}
	if q.Matches(NewMask(1)) {
		t.Fatal("archetype missing required id 2 must NOT match")
	}
	if q.Matches(NewMask()) {
		t.Fatal("empty archetype must NOT match a non-empty required set")
	}
}

func TestQueryStateMatchesExcluded(t *testing.T) {
	t.Parallel()

	q, err := NewQueryState([]component.ID{1}, []component.ID{5}, Access{})
	if err != nil {
		t.Fatal(err)
	}
	if !q.Matches(NewMask(1, 2)) {
		t.Fatal("archetype with required, no excluded must match")
	}
	if q.Matches(NewMask(1, 5)) {
		t.Fatal("archetype carrying excluded id must NOT match")
	}
	if q.Matches(NewMask(1, 2, 5)) {
		t.Fatal("archetype carrying any excluded id must NOT match")
	}
}

func TestQueryStateEmptyRequiredMatchesAll(t *testing.T) {
	t.Parallel()

	q, err := NewQueryState(nil, nil, Access{})
	if err != nil {
		t.Fatal(err)
	}
	if !q.Matches(NewMask()) {
		t.Fatal("empty required must match empty archetype")
	}
	if !q.Matches(NewMask(1, 2, 3)) {
		t.Fatal("empty required must match any archetype")
	}
}

func TestQueryStateMatchesIDs(t *testing.T) {
	t.Parallel()

	q, err := NewQueryState([]component.ID{1, 2}, []component.ID{5}, Access{})
	if err != nil {
		t.Fatal(err)
	}
	if !q.MatchesIDs([]component.ID{1, 2, 3}) {
		t.Fatal("MatchesIDs must agree with Matches on the same set")
	}
	if q.MatchesIDs([]component.ID{1, 2, 5}) {
		t.Fatal("MatchesIDs must reject excluded id presence")
	}
	if q.MatchesIDs([]component.ID{1}) {
		t.Fatal("MatchesIDs must reject missing required id")
	}
}

func TestQueryStateAccessors(t *testing.T) {
	t.Parallel()

	var explicit Access
	explicit.AddWrite(1)
	q, err := NewQueryState([]component.ID{1, 2}, []component.ID{5}, explicit)
	if err != nil {
		t.Fatal(err)
	}

	if !q.Required().Equal(NewMask(1, 2)) {
		t.Fatalf("Required = %s, want Mask{1,2}", q.Required())
	}
	if !q.Excluded().Equal(NewMask(5)) {
		t.Fatalf("Excluded = %s, want Mask{5}", q.Excluded())
	}
	if !q.Access().Write.Has(1) {
		t.Fatal("Access().Write must include id 1")
	}
	if !q.Access().Read.Has(2) {
		t.Fatal("Access().Read must include id 2 (auto-added)")
	}
}
