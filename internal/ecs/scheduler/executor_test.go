package scheduler_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/teratron/boltengine/internal/ecs/scheduler"
	"github.com/teratron/boltengine/internal/ecs/world"
)

func TestSequentialExecutorEmptySchedule(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	s := scheduler.NewSchedule("Update")
	if err := s.Build(); err != nil {
		t.Fatal(err)
	}
	if err := scheduler.NewSequentialExecutor().Run(s, w); err != nil {
		t.Fatalf("empty schedule Run err = %v, want nil", err)
	}
}

func TestSequentialExecutorNilScheduleRejected(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	err := scheduler.NewSequentialExecutor().Run(nil, w)
	if err == nil {
		t.Fatal("nil schedule must yield error")
	}
}

func TestSequentialExecutorUnbuiltScheduleRejected(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	s := scheduler.NewSchedule("Update")
	s.AddSystem(scheduler.NewFuncSystem("A", nil))
	// No Build() called.
	err := scheduler.NewSequentialExecutor().Run(s, w)
	if !errors.Is(err, scheduler.ErrScheduleNotBuilt) {
		t.Fatalf("err = %v, want ErrScheduleNotBuilt", err)
	}
}

func TestSequentialExecutorRunsInTopologicalOrder(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	var order []string
	record := func(name string) func(*world.World) {
		return func(_ *world.World) { order = append(order, name) }
	}

	s := scheduler.NewSchedule("Update")
	s.AddSystem(scheduler.NewFuncSystem("Renderer", record("Renderer"))).After("Movement")
	s.AddSystem(scheduler.NewFuncSystem("Movement", record("Movement"))).After("Spawner")
	s.AddSystem(scheduler.NewFuncSystem("Spawner", record("Spawner")))

	if err := s.Build(); err != nil {
		t.Fatal(err)
	}
	if err := s.Run(w); err != nil {
		t.Fatal(err)
	}

	want := []string{"Spawner", "Movement", "Renderer"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i, n := range want {
		if order[i] != n {
			t.Fatalf("order[%d] = %s, want %s (full=%v)", i, order[i], n, order)
		}
	}
}

func TestSequentialExecutorSchedulRunConvenience(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	called := 0
	s := scheduler.NewSchedule("Update")
	s.AddSystem(scheduler.NewFuncSystem("Counter", func(_ *world.World) { called++ }))
	if err := s.Build(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := s.Run(w); err != nil {
			t.Fatal(err)
		}
	}
	if called != 3 {
		t.Fatalf("Counter ran %d times, want 3", called)
	}
}

func TestSequentialExecutorRecoversPanic(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	s := scheduler.NewSchedule("Update")
	ranAfter := false
	s.AddSystem(scheduler.NewFuncSystem("Boom", func(_ *world.World) {
		panic("kaboom")
	}))
	s.AddSystem(scheduler.NewFuncSystem("AfterBoom", func(_ *world.World) {
		ranAfter = true
	})).After("Boom")

	if err := s.Build(); err != nil {
		t.Fatal(err)
	}
	err := s.Run(w)
	if !errors.Is(err, scheduler.ErrSystemPanic) {
		t.Fatalf("err = %v, want ErrSystemPanic", err)
	}
	if !strings.Contains(err.Error(), "Boom") {
		t.Fatalf("error must name the offending system; got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "kaboom") {
		t.Fatalf("error must include the panic value; got %q", err.Error())
	}
	if ranAfter {
		t.Fatal("schedule must halt on panic — AfterBoom must not run")
	}
}

func TestSequentialExecutorPropagatesWorldMutation(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	s := scheduler.NewSchedule("Update")
	s.AddSystem(scheduler.NewFuncSystem("Tick", func(w *world.World) {
		w.IncrementChangeTick()
	}))
	if err := s.Build(); err != nil {
		t.Fatal(err)
	}
	startTick := w.ChangeTick()
	for i := 0; i < 5; i++ {
		if err := s.Run(w); err != nil {
			t.Fatal(err)
		}
	}
	if w.ChangeTick() != startTick+5 {
		t.Fatalf("ChangeTick = %d, want %d", w.ChangeTick(), startTick+5)
	}
}
