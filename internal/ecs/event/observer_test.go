package event_test

import (
	"testing"

	"github.com/neuengine/neu/internal/ecs/command"
	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/event"
	"github.com/neuengine/neu/internal/ecs/world"
)

// ---- helpers -----------------------------------------------------------------

func newTestWorld() *world.World {
	return world.NewWorld()
}

func spawnEntity(w *world.World) entity.Entity {
	return w.SpawnEmpty()
}

var testTrigger = event.TriggerType{Kind: event.TriggerOnAdd, TypeID: 1}

// ---- EnsureObserverRegistry / LookupObserverRegistry -------------------------

func TestEnsureObserverRegistry_CreatesOnce(t *testing.T) {
	t.Parallel()
	w := newTestWorld()
	r1 := event.EnsureObserverRegistry(w)
	r2 := event.EnsureObserverRegistry(w)
	if r1 != r2 {
		t.Fatal("EnsureObserverRegistry must return the same registry on every call")
	}
}

func TestLookupObserverRegistry_NilWhenNotCreated(t *testing.T) {
	t.Parallel()
	w := newTestWorld()
	if event.LookupObserverRegistry(w) != nil {
		t.Fatal("LookupObserverRegistry must return nil before any observer is registered")
	}
}

func TestLookupObserverRegistry_ReturnsSameAfterAdd(t *testing.T) {
	t.Parallel()
	w := newTestWorld()
	event.AddObserver(w, testTrigger, func(*event.ObserverContext) {})
	if event.LookupObserverRegistry(w) == nil {
		t.Fatal("LookupObserverRegistry must return a registry after AddObserver")
	}
}

// ---- AddObserver / RemoveObserver (global) -----------------------------------

func TestAddObserver_ReturnsUniqueIDs(t *testing.T) {
	t.Parallel()
	w := newTestWorld()
	id1 := event.AddObserver(w, testTrigger, func(*event.ObserverContext) {})
	id2 := event.AddObserver(w, testTrigger, func(*event.ObserverContext) {})
	if id1 == id2 {
		t.Fatal("AddObserver must assign unique ObserverIDs")
	}
}

func TestRemoveObserver_Global_RemovesCallback(t *testing.T) {
	t.Parallel()
	w := newTestWorld()
	called := 0
	id := event.AddObserver(w, testTrigger, func(*event.ObserverContext) { called++ })

	e := spawnEntity(w)
	event.TriggerObservers(w.NewDeferred(), testTrigger, e, nil)
	if called != 1 {
		t.Fatalf("expected 1 call before removal, got %d", called)
	}

	event.RemoveObserver(w, id)
	event.TriggerObservers(w.NewDeferred(), testTrigger, e, nil)
	if called != 1 {
		t.Fatalf("expected no calls after removal, got %d", called)
	}
}

func TestRemoveObserver_UnknownID_NoOp(t *testing.T) {
	t.Parallel()
	w := newTestWorld()
	// Should not panic.
	event.RemoveObserver(w, event.ObserverID(999))
}

func TestRemoveObserver_EmptyRegistry_NoOp(t *testing.T) {
	t.Parallel()
	w := newTestWorld()
	// No registry created yet — must be a no-op.
	event.RemoveObserver(w, event.ObserverID(1))
}

// ---- Observe / RemoveObserver (entity-bound) ---------------------------------

func TestObserve_FiresOnlyForTargetEntity(t *testing.T) {
	t.Parallel()
	w := newTestWorld()
	e1 := spawnEntity(w)
	e2 := spawnEntity(w)

	calls := 0
	event.Observe(w, e1, testTrigger, func(*event.ObserverContext) { calls++ })

	// Fire on e1 — observer should fire.
	event.TriggerObservers(w.NewDeferred(), testTrigger, e1, nil)
	if calls != 1 {
		t.Fatalf("expected 1 call for e1, got %d", calls)
	}

	// Fire on e2 — observer must NOT fire.
	event.TriggerObservers(w.NewDeferred(), testTrigger, e2, nil)
	if calls != 1 {
		t.Fatalf("entity-bound observer must not fire for unrelated entity, calls=%d", calls)
	}
}

func TestObserve_RemoveObserver_EntityBound(t *testing.T) {
	t.Parallel()
	w := newTestWorld()
	e := spawnEntity(w)

	calls := 0
	id := event.Observe(w, e, testTrigger, func(*event.ObserverContext) { calls++ })
	event.TriggerObservers(w.NewDeferred(), testTrigger, e, nil)
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}

	event.RemoveObserver(w, id)
	event.TriggerObservers(w.NewDeferred(), testTrigger, e, nil)
	if calls != 1 {
		t.Fatalf("expected no calls after removal, got %d", calls)
	}
}

// ---- TriggerObservers — basic dispatch ---------------------------------------

func TestTriggerObservers_NoRegistry_NoOp(t *testing.T) {
	t.Parallel()
	w := newTestWorld()
	e := spawnEntity(w)
	// Must not panic when no registry exists.
	event.TriggerObservers(w.NewDeferred(), testTrigger, e, nil)
}

func TestTriggerObservers_GlobalAndEntityBound_BothFire(t *testing.T) {
	t.Parallel()
	w := newTestWorld()
	e := spawnEntity(w)

	var order []string
	event.Observe(w, e, testTrigger, func(*event.ObserverContext) { order = append(order, "entity") })
	event.AddObserver(w, testTrigger, func(*event.ObserverContext) { order = append(order, "global") })

	event.TriggerObservers(w.NewDeferred(), testTrigger, e, nil)

	if len(order) != 2 {
		t.Fatalf("expected 2 callbacks, got %d: %v", len(order), order)
	}
	// Entity-bound must fire before global.
	if order[0] != "entity" || order[1] != "global" {
		t.Fatalf("wrong order: %v", order)
	}
}

func TestTriggerObservers_MultipleGlobalObservers(t *testing.T) {
	t.Parallel()
	w := newTestWorld()
	e := spawnEntity(w)

	calls := 0
	for range 3 {
		event.AddObserver(w, testTrigger, func(*event.ObserverContext) { calls++ })
	}
	event.TriggerObservers(w.NewDeferred(), testTrigger, e, nil)
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestTriggerObservers_DifferentTriggerType_DoesNotFire(t *testing.T) {
	t.Parallel()
	w := newTestWorld()
	e := spawnEntity(w)

	otherTrigger := event.TriggerType{Kind: event.TriggerOnRemove, TypeID: 1}
	calls := 0
	event.AddObserver(w, testTrigger, func(*event.ObserverContext) { calls++ })
	event.TriggerObservers(w.NewDeferred(), otherTrigger, e, nil)
	if calls != 0 {
		t.Fatalf("observer must not fire for a different trigger type, calls=%d", calls)
	}
}

// ---- TriggerObservers — entity event bubbling --------------------------------

func TestTriggerObservers_Bubbling_ParentReceivesEvent(t *testing.T) {
	t.Parallel()
	w := newTestWorld()

	parent := spawnEntity(w)
	child := spawnEntity(w)

	// Link child → parent via ChildOf.
	if err := w.Insert(child, component.Data{Value: entity.ChildOf{Parent: parent}}); err != nil {
		t.Fatalf("Insert ChildOf: %v", err)
	}

	var fired []string
	event.AddObserver(w, testTrigger, func(ctx *event.ObserverContext) {
		if ctx.Entity() == child {
			fired = append(fired, "child")
		} else if ctx.Entity() == parent {
			fired = append(fired, "parent")
		}
	})

	event.TriggerObservers(w.NewDeferred(), testTrigger, child, nil)

	if len(fired) != 2 {
		t.Fatalf("expected bubbling to parent, got events: %v", fired)
	}
	if fired[0] != "child" || fired[1] != "parent" {
		t.Fatalf("wrong bubbling order: %v", fired)
	}
}

func TestTriggerObservers_Bubbling_ThreeLevels(t *testing.T) {
	t.Parallel()
	w := newTestWorld()

	grandparent := spawnEntity(w)
	parent := spawnEntity(w)
	child := spawnEntity(w)

	if err := w.Insert(parent, component.Data{Value: entity.ChildOf{Parent: grandparent}}); err != nil {
		t.Fatalf("Insert ChildOf parent: %v", err)
	}
	if err := w.Insert(child, component.Data{Value: entity.ChildOf{Parent: parent}}); err != nil {
		t.Fatalf("Insert ChildOf child: %v", err)
	}

	count := 0
	event.AddObserver(w, testTrigger, func(*event.ObserverContext) { count++ })
	event.TriggerObservers(w.NewDeferred(), testTrigger, child, nil)
	if count != 3 {
		t.Fatalf("expected 3 bubbled calls (child, parent, grandparent), got %d", count)
	}
}

// ---- StopPropagation ---------------------------------------------------------

func TestTriggerObservers_StopPropagation_PreventsBubbling(t *testing.T) {
	t.Parallel()
	w := newTestWorld()

	parent := spawnEntity(w)
	child := spawnEntity(w)

	if err := w.Insert(child, component.Data{Value: entity.ChildOf{Parent: parent}}); err != nil {
		t.Fatalf("Insert ChildOf: %v", err)
	}

	parentCalled := false
	event.AddObserver(w, testTrigger, func(ctx *event.ObserverContext) {
		if ctx.Entity() == child {
			ctx.StopPropagation()
		}
		if ctx.Entity() == parent {
			parentCalled = true
		}
	})

	event.TriggerObservers(w.NewDeferred(), testTrigger, child, nil)
	if parentCalled {
		t.Fatal("parent observer must not fire after StopPropagation")
	}
}

func TestTriggerObservers_StopPropagation_AllCurrentLevelObserversFire(t *testing.T) {
	t.Parallel()
	w := newTestWorld()

	parent := spawnEntity(w)
	child := spawnEntity(w)

	if err := w.Insert(child, component.Data{Value: entity.ChildOf{Parent: parent}}); err != nil {
		t.Fatalf("Insert ChildOf: %v", err)
	}

	calls := 0
	event.AddObserver(w, testTrigger, func(ctx *event.ObserverContext) {
		calls++
		if ctx.Entity() == child {
			ctx.StopPropagation()
		}
	})
	// Second global observer — must still fire for child even after StopPropagation.
	event.AddObserver(w, testTrigger, func(*event.ObserverContext) { calls++ })

	event.TriggerObservers(w.NewDeferred(), testTrigger, child, nil)
	// 2 global observers × 1 entity (child) = 2 calls; parent is skipped.
	if calls != 2 {
		t.Fatalf("expected 2 calls (both observers at child level), got %d", calls)
	}
}

// ---- ObserverContext helpers -------------------------------------------------

func TestObserverContext_Entity(t *testing.T) {
	t.Parallel()
	w := newTestWorld()
	e := spawnEntity(w)

	var seen entity.Entity
	event.AddObserver(w, testTrigger, func(ctx *event.ObserverContext) {
		seen = ctx.Entity()
	})
	event.TriggerObservers(w.NewDeferred(), testTrigger, e, nil)
	if seen != e {
		t.Fatalf("ctx.Entity() = %v, want %v", seen, e)
	}
}

func TestObserverContext_World(t *testing.T) {
	t.Parallel()
	w := newTestWorld()
	e := spawnEntity(w)

	var gotDW *world.DeferredWorld
	event.AddObserver(w, testTrigger, func(ctx *event.ObserverContext) {
		gotDW = ctx.World()
	})
	event.TriggerObservers(w.NewDeferred(), testTrigger, e, nil)
	if gotDW == nil {
		t.Fatal("ctx.World() must not be nil")
	}
}

func TestObserverContextEvent_TypeAssertion(t *testing.T) {
	t.Parallel()
	w := newTestWorld()
	e := spawnEntity(w)

	type myEvent struct{ Value int }
	payload := myEvent{Value: 42}

	var got myEvent
	event.AddObserver(w, testTrigger, func(ctx *event.ObserverContext) {
		got = event.ObserverContextEvent[myEvent](ctx)
	})
	event.TriggerObservers(w.NewDeferred(), testTrigger, e, payload)
	if got.Value != 42 {
		t.Fatalf("ObserverContextEvent got %v, want 42", got)
	}
}

func TestObserverContextEvent_WrongType_ReturnsZero(t *testing.T) {
	t.Parallel()
	w := newTestWorld()
	e := spawnEntity(w)

	type myEvent struct{ Value int }

	var got string
	event.AddObserver(w, testTrigger, func(ctx *event.ObserverContext) {
		got = event.ObserverContextEvent[string](ctx) // payload is myEvent, not string
	})
	event.TriggerObservers(w.NewDeferred(), testTrigger, e, myEvent{Value: 7})
	if got != "" {
		t.Fatalf("expected empty string for wrong type assertion, got %q", got)
	}
}

// ---- ObserverContext.Commands ------------------------------------------------

func TestObserverContext_Commands_EnqueuesAndApplies(t *testing.T) {
	t.Parallel()
	w := newTestWorld()
	e := spawnEntity(w)

	var spawned entity.Entity
	event.AddObserver(w, testTrigger, func(ctx *event.ObserverContext) {
		cmds := ctx.Commands()
		spawned = cmds.SpawnEmpty()
	})

	event.TriggerObservers(w.NewDeferred(), testTrigger, e, nil)

	// After TriggerObservers returns, commands should have been applied.
	if !w.Contains(spawned) {
		t.Fatal("entity spawned inside observer must be alive after TriggerObservers returns")
	}
}

// ---- Panic recovery ----------------------------------------------------------

func TestTriggerObservers_PanicInCallback_Recovered(t *testing.T) {
	t.Parallel()
	w := newTestWorld()
	e := spawnEntity(w)

	afterPanic := false
	event.AddObserver(w, testTrigger, func(*event.ObserverContext) {
		panic("intentional panic in observer test")
	})
	event.AddObserver(w, testTrigger, func(*event.ObserverContext) {
		afterPanic = true
	})

	// Must not propagate the panic to the caller.
	event.TriggerObservers(w.NewDeferred(), testTrigger, e, nil)

	if !afterPanic {
		t.Fatal("observer after panicking observer must still be called")
	}
}

// ---- Depth limit -------------------------------------------------------------

func TestTriggerObservers_DepthLimit_Panics(t *testing.T) {
	t.Parallel()
	w := newTestWorld()

	// Build a chain of 65 entities (exceeds maxObserverDepth=64).
	const chainLen = 66
	entities := make([]entity.Entity, chainLen)
	for i := range chainLen {
		entities[i] = spawnEntity(w)
	}
	for i := 1; i < chainLen; i++ {
		if err := w.Insert(entities[i], component.Data{Value: entity.ChildOf{Parent: entities[i-1]}}); err != nil {
			t.Fatalf("Insert ChildOf[%d]: %v", i, err)
		}
	}

	// Register a global observer so the dispatch actually walks the chain.
	event.AddObserver(w, testTrigger, func(*event.ObserverContext) {})

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for depth > maxObserverDepth")
		}
	}()
	// Trigger from the deepest entity (entities[chainLen-1]).
	event.TriggerObservers(w.NewDeferred(), testTrigger, entities[chainLen-1], nil)
}

// ---- Race detection ----------------------------------------------------------

func TestTriggerObservers_RaceClean(t *testing.T) {
	// Uses -race flag from CI; sequential here, just verify no data races
	// with multiple AddObserver + TriggerObservers in the same goroutine.
	w := newTestWorld()
	e := spawnEntity(w)

	for range 10 {
		event.AddObserver(w, testTrigger, func(*event.ObserverContext) {})
	}
	for range 20 {
		event.TriggerObservers(w.NewDeferred(), testTrigger, e, nil)
	}
}

// ---- Benchmark ---------------------------------------------------------------

func BenchmarkAddObserver(b *testing.B) {
	w := newTestWorld()
	cb := func(*event.ObserverContext) {}
	b.ResetTimer()
	for range b.N {
		event.AddObserver(w, testTrigger, cb)
	}
}

func BenchmarkTriggerObservers_NoHierarchy(b *testing.B) {
	w := newTestWorld()
	e := spawnEntity(w)
	event.AddObserver(w, testTrigger, func(*event.ObserverContext) {})
	dw := w.NewDeferred()
	b.ResetTimer()
	for range b.N {
		event.TriggerObservers(dw, testTrigger, e, nil)
	}
}

func BenchmarkTriggerObservers_WithCommands(b *testing.B) {
	w := newTestWorld()
	e := spawnEntity(w)
	event.AddObserver(w, testTrigger, func(ctx *event.ObserverContext) {
		_ = ctx.Commands()
	})
	// Register a long-lived buffer so applied commands are flushed properly.
	buf := command.NewCommandBuffer(w.Entities(), 16)
	buf.RegisterWith(w)
	dw := w.NewDeferred()
	b.ResetTimer()
	for range b.N {
		event.TriggerObservers(dw, testTrigger, e, nil)
	}
}
