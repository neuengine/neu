package command_test

import (
	"testing"

	"github.com/neuengine/neu/internal/ecs/command"
	"github.com/neuengine/neu/internal/ecs/world"
)

func TestCommandBuffer_Flush_AppliesAndResets(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	buf := command.NewCommandBuffer(w.Entities(), 4)
	called := 0
	buf.Push(command.NewCustomCommand(func(_ *world.World) { called++ }))
	buf.Push(command.NewCustomCommand(func(_ *world.World) { called++ }))

	buf.Flush(w)

	if called != 2 {
		t.Fatalf("called = %d, want 2 after Flush", called)
	}
	if buf.Len() != 0 {
		t.Fatalf("Len = %d after Flush, want 0 (Flush must reset)", buf.Len())
	}

	// Second Flush is a no-op (buffer was reset).
	buf.Flush(w)
	if called != 2 {
		t.Fatalf("called = %d after second Flush, want 2 (must not re-execute)", called)
	}
}

func TestCommandBuffer_RegisterWith_FlushedByApplyDeferred(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	buf := command.NewCommandBuffer(w.Entities(), 4)
	buf.RegisterWith(w)

	called := 0
	buf.Push(command.NewCustomCommand(func(_ *world.World) { called++ }))
	w.ApplyDeferred()

	if called != 1 {
		t.Fatalf("registered buffer must be flushed by ApplyDeferred (called=%d)", called)
	}
	if buf.Len() != 0 {
		t.Fatal("registered buffer must be reset after ApplyDeferred")
	}

	// Push again, ApplyDeferred again — flush continues to work across ticks.
	buf.Push(command.NewCustomCommand(func(_ *world.World) { called++ }))
	w.ApplyDeferred()
	if called != 2 {
		t.Fatalf("registered buffer must remain registered across flushes (called=%d)", called)
	}
}

func TestApplyDeferredCommands_FlushesInOrder(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	buf1 := command.NewCommandBuffer(w.Entities(), 4)
	buf2 := command.NewCommandBuffer(w.Entities(), 4)

	var order []int
	buf1.Push(command.NewCustomCommand(func(_ *world.World) { order = append(order, 1) }))
	buf2.Push(command.NewCustomCommand(func(_ *world.World) { order = append(order, 2) }))

	command.ApplyDeferredCommands(w, []*command.CommandBuffer{buf1, buf2})

	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Fatalf("order = %v, want [1 2]", order)
	}
	if buf1.Len() != 0 || buf2.Len() != 0 {
		t.Fatal("ApplyDeferredCommands must Reset each buffer")
	}
}

func TestApplyDeferredCommands_NilBufferSkipped(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	buf := command.NewCommandBuffer(w.Entities(), 4)
	called := false
	buf.Push(command.NewCustomCommand(func(_ *world.World) { called = true }))

	// nil entries must be silently skipped.
	command.ApplyDeferredCommands(w, []*command.CommandBuffer{nil, buf, nil})

	if !called {
		t.Fatal("non-nil buffer must still be flushed when interleaved with nil entries")
	}
}

func TestWorld_ApplyDeferred_NoFlushers(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	// Must not panic when nothing is registered.
	w.ApplyDeferred()
}

func TestWorld_RegisterDeferredFlusher_NilIgnored(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	w.RegisterDeferredFlusher(nil) // must not panic
	w.ApplyDeferred()              // must not panic / call nil
}

func TestWorld_DeferredFlushers_FIFOOrder(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	var order []int
	w.RegisterDeferredFlusher(func(_ *world.World) { order = append(order, 1) })
	w.RegisterDeferredFlusher(func(_ *world.World) { order = append(order, 2) })
	w.RegisterDeferredFlusher(func(_ *world.World) { order = append(order, 3) })

	w.ApplyDeferred()

	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Fatalf("flushers must run in registration order; got %v", order)
	}
}

func TestWorld_ResetDeferredFlushers(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	called := false
	w.RegisterDeferredFlusher(func(_ *world.World) { called = true })
	w.ResetDeferredFlushers()
	w.ApplyDeferred()
	if called {
		t.Fatal("ResetDeferredFlushers must drop all registered flushers")
	}
}
