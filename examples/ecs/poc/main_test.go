package main

import (
	"testing"

	ecs "github.com/neuengine/neu/pkg/ecs"
)

// TestPOCDeterminism runs two identical worlds through the full tick loop and
// verifies the entity counts and event totals are identical, confirming
// deterministic execution (T-1T05 acceptance criterion 3 + 4).
func TestPOCDeterminism(t *testing.T) {
	t.Parallel()
	r1 := runPOC(t)
	r2 := runPOC(t)
	if r1 != r2 {
		t.Fatalf("non-deterministic output: run1=%+v run2=%+v", r1, r2)
	}
}

// TestPOCEntityCount verifies that the initial 10 000 entities span exactly
// 3 archetypes (T-1T05 acceptance criterion 1).
func TestPOCEntityCount(t *testing.T) {
	t.Parallel()
	w := ecs.NewWorldWithCapacity(10100, 8)
	for range 3_000 {
		_ = w.Spawn(ecs.Data{Value: Position{}}, ecs.Data{Value: Velocity{}})
	}
	for range 2_000 {
		_ = w.Spawn(ecs.Data{Value: Position{}}, ecs.Data{Value: Health{}})
	}
	for range 5_000 {
		_ = w.Spawn(
			ecs.Data{Value: Position{}},
			ecs.Data{Value: Velocity{}},
			ecs.Data{Value: Health{}},
		)
	}
	// Empty archetype (index 0) + 3 component sets = 4 total archetypes.
	// Archetypes are: {} empty, {P,V}, {P,H}, {P,V,H}.
	got := w.Archetypes().Len()
	if got != 4 {
		t.Fatalf("expected 4 archetypes (including empty), got %d", got)
	}
	q, _ := ecs.NewQuery1[Position](w)
	if n := q.Count(w); n != 10_000 {
		t.Fatalf("expected 10000 entities with Position, got %d", n)
	}
}

// TestPOCCommandRoundTrip verifies that a despawn command applied at the tick
// boundary takes effect before the next tick (T-1T05 acceptance criterion 3).
func TestPOCCommandRoundTrip(t *testing.T) {
	t.Parallel()
	w := ecs.NewWorld()

	e := w.Spawn(ecs.Data{Value: Health{HP: 1}})
	if !w.Contains(e) {
		t.Fatal("entity must be alive after Spawn")
	}

	buf := ecs.AcquireBuffer(w)
	buf.RegisterWith(w)
	cmds := ecs.NewCommands(buf)

	sched := ecs.NewSchedule("Update")
	sched.AddSystem(ecs.NewFuncSystem("kill", func(world *ecs.World) {
		cmds.Despawn(e)
	}))
	if err := sched.Build(); err != nil {
		t.Fatal(err)
	}

	// Run one tick: command is applied in w.ApplyDeferred() at the end.
	if err := sched.Run(w); err != nil {
		t.Fatal(err)
	}
	w.ResetDeferredFlushers()
	ecs.ReleaseBuffer(buf)

	// Entity must be dead after the tick.
	if w.Contains(e) {
		t.Fatal("entity must be dead after command apply-point")
	}
}

// TestPOCEventRoundTrip verifies that a MoveEvent sent in tick N is readable in
// tick N+1 after SwapAll (T-1T05 acceptance criterion 3).
func TestPOCEventRoundTrip(t *testing.T) {
	t.Parallel()
	w := ecs.NewWorld()
	ecs.RegisterEvent[MoveEvent](w)

	writer := ecs.NewEventWriter[MoveEvent](w)
	reader := ecs.NewEventReaderAt[MoveEvent](w)

	// Tick N: send an event.
	writer.Send(MoveEvent{Count: 42})

	// Tick boundary.
	ecs.SwapAll(w)

	// Tick N+1: reader sees the event from tick N.
	got := 0
	for e := range reader.All() {
		if e.Count != 42 {
			t.Fatalf("event.Count = %d, want 42", e.Count)
		}
		got++
	}
	if got != 1 {
		t.Fatalf("expected 1 event, got %d", got)
	}
}

// TestPOCScheduleDAG verifies that the 3 systems run in Movement→Combat→Collector
// order, as declared by Before constraints (T-1T05 acceptance criterion 2).
func TestPOCScheduleDAG(t *testing.T) {
	t.Parallel()
	var order []string

	w := ecs.NewWorld()
	sched := ecs.NewSchedule("Update")
	sched.AddSystem(ecs.NewFuncSystem("MovementSystem", func(_ *ecs.World) {
		order = append(order, "move")
	})).Before("CombatSystem")
	sched.AddSystem(ecs.NewFuncSystem("CombatSystem", func(_ *ecs.World) {
		order = append(order, "combat")
	})).Before("CollectorSystem")
	sched.AddSystem(ecs.NewFuncSystem("CollectorSystem", func(_ *ecs.World) {
		order = append(order, "collect")
	}))
	if err := sched.Build(); err != nil {
		t.Fatal(err)
	}
	if err := sched.Run(w); err != nil {
		t.Fatal(err)
	}
	want := []string{"move", "combat", "collect"}
	for i, v := range want {
		if order[i] != v {
			t.Fatalf("system[%d] = %q, want %q", i, order[i], v)
		}
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

type pocResult struct {
	ticks      int
	moved      int
	eventsRecv int
}

func runPOC(t *testing.T) pocResult {
	t.Helper()
	const (
		archPVCount  = 3_000
		archPHCount  = 2_000
		archPVHCount = 5_000
		ticks        = 100
	)

	w := ecs.NewWorldWithCapacity(archPVCount+archPHCount+archPVHCount+ticks+16, 8)
	ecs.RegisterEvent[MoveEvent](w)
	ecs.SetResource(w, Stats{})

	for range archPVCount {
		_ = w.Spawn(ecs.Data{Value: Position{}}, ecs.Data{Value: Velocity{DX: 0.1, DY: 0.05}})
	}
	for range archPHCount {
		_ = w.Spawn(ecs.Data{Value: Position{}}, ecs.Data{Value: Health{HP: 100}})
	}
	for range archPVHCount {
		_ = w.Spawn(
			ecs.Data{Value: Position{}},
			ecs.Data{Value: Velocity{DX: 0.05, DY: 0.1}},
			ecs.Data{Value: Health{HP: 50}},
		)
	}

	qPV, _ := ecs.NewQuery2[Position, Velocity](w)
	qH, _ := ecs.NewQuery1[Health](w)
	reader := ecs.NewEventReaderAt[MoveEvent](w)

	combatBuf := ecs.AcquireBuffer(w)
	combatBuf.RegisterWith(w)
	combatCmds := ecs.NewCommands(combatBuf)

	moveSys := ecs.NewFuncSystem("MovementSystem", func(world *ecs.World) {
		writer := ecs.NewEventWriter[MoveEvent](world)
		moved := 0
		for _, tup := range qPV.All(world) {
			tup.A.X += tup.B.DX
			tup.A.Y += tup.B.DY
			moved++
		}
		writer.Send(MoveEvent{Count: moved})
		if s, ok := ecs.Resource[Stats](world); ok {
			s.Moved += moved
		}
	})
	combatSys := ecs.NewFuncSystem("CombatSystem", func(world *ecs.World) {
		for e, h := range qH.All(world) {
			h.HP--
			if h.HP <= 0 {
				combatCmds.Despawn(e)
				_ = combatCmds.Spawn(ecs.Data{Value: Health{HP: 10}})
			}
		}
	})
	collectSys := ecs.NewFuncSystem("CollectorSystem", func(world *ecs.World) {
		for range reader.All() {
			if s, ok := ecs.Resource[Stats](world); ok {
				s.EventsRecv++
			}
		}
	})

	sched := ecs.NewSchedule("Update")
	sched.AddSystem(moveSys).Before("CombatSystem")
	sched.AddSystem(combatSys).Before("CollectorSystem")
	sched.AddSystem(collectSys)
	if err := sched.Build(); err != nil {
		t.Fatal(err)
	}

	for tick := range ticks {
		ecs.SwapAll(w)
		if err := sched.Run(w); err != nil {
			t.Fatalf("tick %d: %v", tick, err)
		}
		if s, ok := ecs.Resource[Stats](w); ok {
			s.Ticks++
		}
	}

	w.ResetDeferredFlushers()
	ecs.ReleaseBuffer(combatBuf)

	s, _ := ecs.Resource[Stats](w)
	return pocResult{
		ticks:      s.Ticks,
		moved:      s.Moved,
		eventsRecv: s.EventsRecv,
	}
}
