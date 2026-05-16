package query

import (
	"strings"
	"testing"

	"github.com/neuengine/neu/internal/ecs/component"
)

func TestAccessZeroValueEmpty(t *testing.T) {
	t.Parallel()

	var a Access
	if !a.IsEmpty() {
		t.Fatal("zero-value Access must be empty")
	}
	if a.Touches(0) || a.Touches(50) {
		t.Fatal("empty Access must not Touch any id")
	}
}

func TestAccessAddAndTouches(t *testing.T) {
	t.Parallel()

	var a Access
	a.AddRead(1)
	a.AddWrite(2)
	a.AddExclusive(3)

	if !a.Touches(1) || !a.Touches(2) || !a.Touches(3) {
		t.Fatal("Touches must report true for each declared id")
	}
	if a.Touches(4) {
		t.Fatal("Touches must report false for an undeclared id")
	}
	if a.IsEmpty() {
		t.Fatal("Access with declarations must not be Empty")
	}
}

func TestAccessConflictsReadRead(t *testing.T) {
	t.Parallel()

	var a, b Access
	a.AddRead(1)
	a.AddRead(2)
	b.AddRead(1)
	b.AddRead(2)

	if a.Conflicts(b) || b.Conflicts(a) {
		t.Fatal("Read-Read on shared components must NOT conflict")
	}
	if !a.IsDisjoint(b) {
		t.Fatal("IsDisjoint must be the inverse of Conflicts")
	}
}

func TestAccessConflictsWriteRead(t *testing.T) {
	t.Parallel()

	var w, r Access
	w.AddWrite(1)
	r.AddRead(1)

	if !w.Conflicts(r) {
		t.Fatal("Write vs Read on same id must conflict")
	}
	if !r.Conflicts(w) {
		t.Fatal("Conflicts must be symmetric")
	}
}

func TestAccessConflictsWriteWrite(t *testing.T) {
	t.Parallel()

	var a, b Access
	a.AddWrite(1)
	b.AddWrite(1)
	if !a.Conflicts(b) {
		t.Fatal("Write vs Write on same id must conflict")
	}

	var c Access
	c.AddWrite(2)
	if a.Conflicts(c) {
		t.Fatal("Writes on disjoint ids must NOT conflict")
	}
}

func TestAccessConflictsExclusive(t *testing.T) {
	t.Parallel()

	var ex, r, w, ex2 Access
	ex.AddExclusive(5)
	r.AddRead(5)
	w.AddWrite(5)
	ex2.AddExclusive(5)

	if !ex.Conflicts(r) || !r.Conflicts(ex) {
		t.Fatal("Exclusive vs Read on same id must conflict")
	}
	if !ex.Conflicts(w) || !w.Conflicts(ex) {
		t.Fatal("Exclusive vs Write on same id must conflict")
	}
	if !ex.Conflicts(ex2) {
		t.Fatal("Exclusive vs Exclusive on same id must conflict")
	}

	var other Access
	other.AddRead(7)
	other.AddWrite(8)
	if ex.Conflicts(other) {
		t.Fatal("Exclusive on id 5 must NOT conflict with access on id 7/8")
	}
}

func TestAccessMerge(t *testing.T) {
	t.Parallel()

	var a, b Access
	a.AddRead(1)
	a.AddWrite(2)
	b.AddRead(3)
	b.AddExclusive(4)

	merged := a.Merge(b)
	for _, id := range []component.ID{1, 3} {
		if !merged.Read.Has(id) {
			t.Fatalf("merged Read missing id %d", id)
		}
	}
	if !merged.Write.Has(2) {
		t.Fatal("merged Write missing id 2")
	}
	if !merged.Exclusive.Has(4) {
		t.Fatal("merged Exclusive missing id 4")
	}
}

func TestAccessValidate(t *testing.T) {
	t.Parallel()

	var ok Access
	ok.AddRead(1)
	ok.AddWrite(1) // R+W overlap is OK — write supersedes
	if err := ok.Validate(); err != nil {
		t.Fatalf("Read+Write overlap must be valid; got %v", err)
	}

	var bad Access
	bad.AddExclusive(2)
	bad.AddRead(2)
	err := bad.Validate()
	if err == nil {
		t.Fatal("Exclusive overlapping Read must fail validation")
	}
	if !strings.Contains(err.Error(), "exclusive set overlaps") {
		t.Fatalf("error must mention overlap; got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "2") {
		t.Fatalf("error must mention conflicting id; got %q", err.Error())
	}

	var bad2 Access
	bad2.AddExclusive(3)
	bad2.AddWrite(3)
	if err := bad2.Validate(); err == nil {
		t.Fatal("Exclusive overlapping Write must fail validation")
	}

	// Multi-id overlap exercises the comma branch of the error formatter.
	var bad3 Access
	bad3.AddExclusive(7)
	bad3.AddExclusive(9)
	bad3.AddRead(7)
	bad3.AddRead(9)
	err = bad3.Validate()
	if err == nil {
		t.Fatal("multi-id Exclusive/Read overlap must fail")
	}
	for _, want := range []string{"7", "9", ", "} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("multi-id error missing %q: %s", want, err.Error())
		}
	}
}

func TestAccessString(t *testing.T) {
	t.Parallel()

	var a Access
	a.AddRead(1)
	a.AddWrite(2)
	a.AddExclusive(3)
	got := a.String()
	for _, want := range []string{"Read=Mask{1}", "Write=Mask{2}", "Exclusive=Mask{3}"} {
		if !strings.Contains(got, want) {
			t.Fatalf("String %q missing %q", got, want)
		}
	}
}
