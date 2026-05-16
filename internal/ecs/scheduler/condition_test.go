package scheduler_test

import (
	"testing"

	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
)

func boolCond(v bool) scheduler.RunCondition {
	return func(_ *world.World) bool { return v }
}

func TestNotInverts(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	if scheduler.Not(boolCond(true))(w) {
		t.Fatal("Not(true)(w) = true; want false")
	}
	if !scheduler.Not(boolCond(false))(w) {
		t.Fatal("Not(false)(w) = false; want true")
	}
}

func TestNotNilTreatedAsFalse(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	// Per docs: nil is "always true" so Not(nil) is "always false".
	if scheduler.Not(nil)(w) {
		t.Fatal("Not(nil)(w) = true; want false")
	}
}

func TestAndShortCircuits(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()

	// Empty list → true.
	if !scheduler.And()(w) {
		t.Fatal("And() must be true")
	}

	// Records calls so we can verify short-circuit behaviour.
	calls := 0
	track := func(v bool) scheduler.RunCondition {
		return func(_ *world.World) bool { calls++; return v }
	}
	calls = 0
	res := scheduler.And(track(true), track(false), track(true))(w)
	if res {
		t.Fatal("And with a false element must be false")
	}
	if calls != 2 {
		t.Fatalf("And called %d predicates, want 2 (short-circuit)", calls)
	}

	calls = 0
	res = scheduler.And(track(true), track(true), track(true))(w)
	if !res {
		t.Fatal("And with all-true must be true")
	}
	if calls != 3 {
		t.Fatalf("And called %d predicates, want 3", calls)
	}
}

func TestAndSkipsNil(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	if !scheduler.And(nil, boolCond(true), nil)(w) {
		t.Fatal("And must skip nils and return true when the rest is true")
	}
	if scheduler.And(nil, boolCond(false))(w) {
		t.Fatal("And must respect a non-nil false")
	}
}

func TestOrShortCircuits(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	// Empty list → false.
	if scheduler.Or()(w) {
		t.Fatal("Or() must be false")
	}

	calls := 0
	track := func(v bool) scheduler.RunCondition {
		return func(_ *world.World) bool { calls++; return v }
	}
	calls = 0
	if !scheduler.Or(track(false), track(true), track(false))(w) {
		t.Fatal("Or with a true element must be true")
	}
	if calls != 2 {
		t.Fatalf("Or called %d predicates, want 2 (short-circuit)", calls)
	}

	calls = 0
	if scheduler.Or(track(false), track(false), track(false))(w) {
		t.Fatal("Or with all-false must be false")
	}
	if calls != 3 {
		t.Fatalf("Or called %d predicates, want 3", calls)
	}
}

func TestOrNilElementShortCircuitsTrue(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	// Per doc: nil is "always true" so Or short-circuits to true on nil.
	if !scheduler.Or(boolCond(false), nil, boolCond(false))(w) {
		t.Fatal("Or with a nil element must short-circuit true")
	}
}
