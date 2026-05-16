package scheduler_test

import (
	"testing"

	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
)

func TestSequentialExecutor_CallsApplyDeferredAfterRun(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	var order []string

	// One flusher registered; it should run AFTER the system.
	w.RegisterDeferredFlusher(func(_ *world.World) { order = append(order, "flush") })

	s := scheduler.NewSchedule("Update")
	s.AddSystem(scheduler.NewFuncSystem("Sys", func(_ *world.World) { order = append(order, "sys") }))

	if err := s.Build(); err != nil {
		t.Fatal(err)
	}
	if err := s.Run(w); err != nil {
		t.Fatal(err)
	}

	if len(order) != 2 || order[0] != "sys" || order[1] != "flush" {
		t.Fatalf("order = %v, want [sys flush]", order)
	}
}

func TestSequentialExecutor_SkipsApplyDeferredOnPanic(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	flushed := false
	w.RegisterDeferredFlusher(func(_ *world.World) { flushed = true })

	s := scheduler.NewSchedule("Update")
	s.AddSystem(scheduler.NewFuncSystem("Boom", func(_ *world.World) { panic("boom") }))

	if err := s.Build(); err != nil {
		t.Fatal(err)
	}
	if err := s.Run(w); err == nil {
		t.Fatal("expected panic-derived error from Run")
	}
	if flushed {
		t.Fatal("ApplyDeferred must NOT run after a panicking system")
	}
}

func TestSequentialExecutor_ApplyDeferredWithNoFlushers(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	called := false
	s := scheduler.NewSchedule("Update")
	s.AddSystem(scheduler.NewFuncSystem("Sys", func(_ *world.World) { called = true }))

	if err := s.Build(); err != nil {
		t.Fatal(err)
	}
	if err := s.Run(w); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("system should still run even with no flushers")
	}
}
