package scheduler_test

import (
	"errors"
	"testing"

	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
)

func TestRunIfSkipsSystemWhenFalse(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	called := false
	s := scheduler.NewSchedule("Update")
	s.AddSystem(scheduler.NewFuncSystem("Gated", func(_ *world.World) { called = true })).
		RunIf(func(_ *world.World) bool { return false })

	if err := s.Build(); err != nil {
		t.Fatal(err)
	}
	if err := s.Run(w); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("system with false condition must not run")
	}
}

func TestRunIfRunsWhenAllTrue(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	called := 0
	s := scheduler.NewSchedule("Update")
	s.AddSystem(scheduler.NewFuncSystem("Gated", func(_ *world.World) { called++ })).
		RunIf(func(_ *world.World) bool { return true }).
		RunIf(func(_ *world.World) bool { return true })

	if err := s.Build(); err != nil {
		t.Fatal(err)
	}
	if err := s.Run(w); err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatalf("system called %d times, want 1", called)
	}
}

func TestRunIfNilIgnored(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	called := false
	s := scheduler.NewSchedule("Update")
	s.AddSystem(scheduler.NewFuncSystem("X", func(_ *world.World) { called = true })).
		RunIf(nil)

	if err := s.Build(); err != nil {
		t.Fatal(err)
	}
	if err := s.Run(w); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("nil RunIf must not gate execution")
	}
}

func TestSetMembershipApplied(t *testing.T) {
	t.Parallel()

	const Movement scheduler.SystemSet = "Movement"

	s := scheduler.NewSchedule("Update")
	a := s.AddSystem(scheduler.NewFuncSystem("A", nil)).InSet(Movement)
	b := s.AddSystem(scheduler.NewFuncSystem("B", nil)).InSet(Movement)
	if a.Err() != nil || b.Err() != nil {
		t.Fatalf("builder errs: a=%v b=%v", a.Err(), b.Err())
	}

	for _, n := range []*scheduler.SystemNodeBuilder{a, b} {
		sets := s.Node(n.ID()).Sets()
		if len(sets) != 1 || sets[0] != Movement {
			t.Fatalf("node %d sets = %v, want [Movement]", n.ID(), sets)
		}
	}
}

func TestSetInSetIdempotent(t *testing.T) {
	t.Parallel()

	const Set scheduler.SystemSet = "Group"
	s := scheduler.NewSchedule("Update")
	b := s.AddSystem(scheduler.NewFuncSystem("X", nil)).InSet(Set).InSet(Set).InSet(Set)
	if b.Err() != nil {
		t.Fatal(b.Err())
	}
	if got := s.Node(b.ID()).Sets(); len(got) != 1 {
		t.Fatalf("idempotent InSet yielded %d entries, want 1", len(got))
	}
}

func TestSetConditionAppliedToAllMembers(t *testing.T) {
	t.Parallel()

	const Movement scheduler.SystemSet = "Movement"
	w := world.NewWorld()
	gate := false
	called := []string{}

	s := scheduler.NewSchedule("Update")
	s.AddSystem(scheduler.NewFuncSystem("A", func(_ *world.World) { called = append(called, "A") })).
		InSet(Movement)
	s.AddSystem(scheduler.NewFuncSystem("B", func(_ *world.World) { called = append(called, "B") })).
		InSet(Movement)
	s.ConfigureSet(Movement).RunIf(func(_ *world.World) bool { return gate })

	if err := s.Build(); err != nil {
		t.Fatal(err)
	}

	// Gate is false — neither A nor B should run.
	if err := s.Run(w); err != nil {
		t.Fatal(err)
	}
	if len(called) != 0 {
		t.Fatalf("called = %v, want [] when set gate is false", called)
	}

	// Gate is true — both run.
	gate = true
	if err := s.Run(w); err != nil {
		t.Fatal(err)
	}
	if len(called) != 2 || called[0] != "A" || called[1] != "B" {
		t.Fatalf("called = %v, want [A B] when gate is true", called)
	}
}

func TestSetBeforeOrdersAllPairs(t *testing.T) {
	t.Parallel()

	const Spawning scheduler.SystemSet = "Spawning"
	const Movement scheduler.SystemSet = "Movement"

	w := world.NewWorld()
	var order []string
	rec := func(name string) func(*world.World) {
		return func(_ *world.World) { order = append(order, name) }
	}

	s := scheduler.NewSchedule("Update")
	s.AddSystem(scheduler.NewFuncSystem("Spawn1", rec("Spawn1"))).InSet(Spawning)
	s.AddSystem(scheduler.NewFuncSystem("Spawn2", rec("Spawn2"))).InSet(Spawning)
	s.AddSystem(scheduler.NewFuncSystem("Move1", rec("Move1"))).InSet(Movement)
	s.AddSystem(scheduler.NewFuncSystem("Move2", rec("Move2"))).InSet(Movement)
	s.ConfigureSet(Spawning).Before(Movement)

	if err := s.Build(); err != nil {
		t.Fatal(err)
	}
	if err := s.Run(w); err != nil {
		t.Fatal(err)
	}

	pos := make(map[string]int)
	for i, n := range order {
		pos[n] = i
	}
	for _, sp := range []string{"Spawn1", "Spawn2"} {
		for _, mv := range []string{"Move1", "Move2"} {
			if pos[sp] >= pos[mv] {
				t.Fatalf("set Before failed: pos[%s]=%d, pos[%s]=%d (full=%v)",
					sp, pos[sp], mv, pos[mv], order)
			}
		}
	}
}

func TestSetAfterOrdersAllPairs(t *testing.T) {
	t.Parallel()

	const Pre scheduler.SystemSet = "Pre"
	const Main scheduler.SystemSet = "Main"

	w := world.NewWorld()
	var order []string
	rec := func(name string) func(*world.World) {
		return func(_ *world.World) { order = append(order, name) }
	}

	s := scheduler.NewSchedule("Update")
	s.AddSystem(scheduler.NewFuncSystem("Pre1", rec("Pre1"))).InSet(Pre)
	s.AddSystem(scheduler.NewFuncSystem("Main1", rec("Main1"))).InSet(Main)
	s.AddSystem(scheduler.NewFuncSystem("Main2", rec("Main2"))).InSet(Main)
	s.ConfigureSet(Main).After(Pre)

	if err := s.Build(); err != nil {
		t.Fatal(err)
	}
	if err := s.Run(w); err != nil {
		t.Fatal(err)
	}
	if order[0] != "Pre1" {
		t.Fatalf("Pre1 must run first; got %v", order)
	}
}

func TestSetCycleRejected(t *testing.T) {
	t.Parallel()

	const A scheduler.SystemSet = "A"
	const B scheduler.SystemSet = "B"

	s := scheduler.NewSchedule("Update")
	s.AddSystem(scheduler.NewFuncSystem("a", nil)).InSet(A)
	s.AddSystem(scheduler.NewFuncSystem("b", nil)).InSet(B)
	s.ConfigureSet(A).Before(B)
	s.ConfigureSet(B).Before(A)

	err := s.Build()
	if !errors.Is(err, scheduler.ErrScheduleCycle) {
		t.Fatalf("err = %v, want ErrScheduleCycle", err)
	}
}

func TestSetSelfLoopRejected(t *testing.T) {
	t.Parallel()

	const Self scheduler.SystemSet = "Self"

	s := scheduler.NewSchedule("Update")
	s.AddSystem(scheduler.NewFuncSystem("a", nil)).InSet(Self)
	s.ConfigureSet(Self).Before(Self) // every member before itself
	err := s.Build()
	if !errors.Is(err, scheduler.ErrScheduleCycle) {
		t.Fatalf("err = %v, want ErrScheduleCycle", err)
	}
}

func TestSetEmptyMembersHarmless(t *testing.T) {
	t.Parallel()

	const Phantom scheduler.SystemSet = "Phantom"

	w := world.NewWorld()
	s := scheduler.NewSchedule("Update")
	s.AddSystem(scheduler.NewFuncSystem("solo", nil))
	s.ConfigureSet(Phantom).Before("OtherEmpty").RunIf(func(_ *world.World) bool { return false })

	if err := s.Build(); err != nil {
		t.Fatalf("empty-set Build err = %v", err)
	}
	if err := s.Run(w); err != nil {
		t.Fatal(err)
	}
}

func TestSetConditionsAndOwnConditionsAreAnded(t *testing.T) {
	t.Parallel()

	const G scheduler.SystemSet = "G"
	w := world.NewWorld()

	setOK := false
	ownOK := false
	called := false

	s := scheduler.NewSchedule("Update")
	s.AddSystem(scheduler.NewFuncSystem("X", func(_ *world.World) { called = true })).
		InSet(G).
		RunIf(func(_ *world.World) bool { return ownOK })
	s.ConfigureSet(G).RunIf(func(_ *world.World) bool { return setOK })

	if err := s.Build(); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		set, own bool
		want     bool
	}{
		{false, false, false},
		{true, false, false},
		{false, true, false},
		{true, true, true},
	}
	for _, c := range cases {
		setOK, ownOK = c.set, c.own
		called = false
		if err := s.Run(w); err != nil {
			t.Fatal(err)
		}
		if called != c.want {
			t.Fatalf("set=%v own=%v: called=%v, want %v", c.set, c.own, called, c.want)
		}
	}
}

func TestSetRunIfNilIgnored(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	called := false
	const G scheduler.SystemSet = "G"

	s := scheduler.NewSchedule("Update")
	s.AddSystem(scheduler.NewFuncSystem("X", func(_ *world.World) { called = true })).InSet(G)
	s.ConfigureSet(G).RunIf(nil)

	if err := s.Build(); err != nil {
		t.Fatal(err)
	}
	if err := s.Run(w); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("nil set-RunIf must not gate execution")
	}
}

func TestBuilderInSetRunIfPreservedAcrossError(t *testing.T) {
	t.Parallel()

	const G scheduler.SystemSet = "G"

	s := scheduler.NewSchedule("Update")
	s.AddSystem(scheduler.NewFuncSystem("A", nil))
	dup := s.AddSystem(scheduler.NewFuncSystem("A", nil)).
		InSet(G).
		RunIf(func(_ *world.World) bool { return true })
	if dup.Err() == nil {
		t.Fatal("duplicate must surface")
	}
	// The errored builder must NOT have mutated the schedule's nodes.
	for _, n := range []scheduler.SystemNodeID{0} {
		if got := s.Node(n).Sets(); len(got) != 0 {
			t.Fatalf("errored builder leaked sets onto node %d: %v", n, got)
		}
		if got := s.Node(n).Conditions(); len(got) != 0 {
			t.Fatalf("errored builder leaked conditions onto node %d", n)
		}
	}
}

func TestSetAfterSelfRejected(t *testing.T) {
	t.Parallel()

	const Self scheduler.SystemSet = "Self"

	s := scheduler.NewSchedule("Update")
	s.AddSystem(scheduler.NewFuncSystem("a", nil)).InSet(Self)
	s.ConfigureSet(Self).After(Self)
	if err := s.Build(); err == nil {
		t.Fatal("After(self) must be rejected as a cycle")
	}
}

func TestNodeAccessorsAfterRunIfAndInSet(t *testing.T) {
	t.Parallel()

	const Group scheduler.SystemSet = "G"

	s := scheduler.NewSchedule("Update")
	cond := func(_ *world.World) bool { return true }
	b := s.AddSystem(scheduler.NewFuncSystem("X", nil)).RunIf(cond).InSet(Group)
	if b.Err() != nil {
		t.Fatal(b.Err())
	}
	node := s.Node(b.ID())
	if len(node.Conditions()) != 1 {
		t.Fatalf("Conditions len = %d, want 1", len(node.Conditions()))
	}
	if len(node.Sets()) != 1 || node.Sets()[0] != Group {
		t.Fatalf("Sets = %v, want [G]", node.Sets())
	}
}
