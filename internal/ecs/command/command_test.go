package command_test

import (
	"reflect"
	"testing"

	"github.com/teratron/boltengine/internal/ecs/command"
	"github.com/teratron/boltengine/internal/ecs/component"
	"github.com/teratron/boltengine/internal/ecs/entity"
	"github.com/teratron/boltengine/internal/ecs/world"
)

// ---- helpers ----------------------------------------------------------------

type posComp struct{ X, Y float32 }
type tagComp struct{}

func newWorld() *world.World { return world.NewWorld() }

// ---- CommandBuffer ----------------------------------------------------------

func TestCommandBuffer_NewWithCap(t *testing.T) {
	t.Parallel()
	w := newWorld()
	buf := command.NewCommandBuffer(w.Entities(), 32)
	if buf.Len() != 0 {
		t.Fatalf("Len = %d, want 0", buf.Len())
	}
}

func TestCommandBuffer_NewDefaultCap(t *testing.T) {
	t.Parallel()
	w := newWorld()
	buf := command.NewCommandBuffer(w.Entities(), 0)
	if buf.Len() != 0 {
		t.Fatal("Len must be 0 after construction")
	}
}

func TestCommandBuffer_PushLen(t *testing.T) {
	t.Parallel()
	w := newWorld()
	buf := command.NewCommandBuffer(w.Entities(), 8)
	for i := range 5 {
		buf.Push(command.NewCustomCommand(func(_ *world.World) {}))
		if buf.Len() != i+1 {
			t.Fatalf("Len after push %d = %d, want %d", i+1, buf.Len(), i+1)
		}
	}
}

func TestCommandBuffer_PushNilPanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil Push")
		}
	}()
	w := newWorld()
	buf := command.NewCommandBuffer(w.Entities(), 4)
	buf.Push(nil)
}

func TestCommandBuffer_FIFOOrder(t *testing.T) {
	t.Parallel()
	w := newWorld()
	buf := command.NewCommandBuffer(w.Entities(), 8)
	var order []int
	for i := range 5 {
		i := i
		buf.Push(command.NewCustomCommand(func(_ *world.World) { order = append(order, i) }))
	}
	buf.Apply(w)
	for i, v := range order {
		if v != i {
			t.Fatalf("order[%d] = %d, want %d (FIFO violated)", i, v, i)
		}
	}
}

func TestCommandBuffer_Reset(t *testing.T) {
	t.Parallel()
	w := newWorld()
	buf := command.NewCommandBuffer(w.Entities(), 4)
	buf.Push(command.NewCustomCommand(func(_ *world.World) {}))
	buf.Push(command.NewCustomCommand(func(_ *world.World) {}))
	buf.Reset()
	if buf.Len() != 0 {
		t.Fatalf("Len after Reset = %d, want 0", buf.Len())
	}
}

func TestCommandBuffer_ApplyThenReset(t *testing.T) {
	t.Parallel()
	w := newWorld()
	buf := command.NewCommandBuffer(w.Entities(), 4)
	calls := 0
	buf.Push(command.NewCustomCommand(func(_ *world.World) { calls++ }))
	buf.Apply(w)
	if calls != 1 {
		t.Fatalf("calls = %d, want 1 after Apply", calls)
	}
	buf.Reset()
	buf.Apply(w)
	if calls != 1 {
		t.Fatal("Apply after Reset must not re-execute commands")
	}
}

// ---- AcquireBuffer / ReleaseBuffer ------------------------------------------

func TestAcquireReleaseBuffer(t *testing.T) {
	t.Parallel()
	w := newWorld()
	buf := command.AcquireBuffer(w.Entities())
	if buf == nil {
		t.Fatal("AcquireBuffer returned nil")
	}
	buf.Push(command.NewCustomCommand(func(_ *world.World) {}))
	command.ReleaseBuffer(buf)
}

func TestAcquireBufferReuse(t *testing.T) {
	t.Parallel()
	w := newWorld()
	buf1 := command.AcquireBuffer(w.Entities())
	command.ReleaseBuffer(buf1)
	buf2 := command.AcquireBuffer(w.Entities())
	if buf2 == nil {
		t.Fatal("second AcquireBuffer returned nil")
	}
	if buf2.Len() != 0 {
		t.Fatalf("reused buffer Len = %d, want 0", buf2.Len())
	}
	command.ReleaseBuffer(buf2)
}

// ---- ReserveEntity ----------------------------------------------------------

func TestCommandBuffer_ReserveEntity(t *testing.T) {
	t.Parallel()
	w := newWorld()
	buf := command.NewCommandBuffer(w.Entities(), 4)
	e := buf.ReserveEntity()
	if !e.IsValid() {
		t.Fatal("ReserveEntity returned invalid entity")
	}
}

// ---- DespawnCommand ---------------------------------------------------------

func TestDespawnCommand_LiveEntity(t *testing.T) {
	t.Parallel()
	w := newWorld()
	e := w.SpawnEmpty()
	buf := command.NewCommandBuffer(w.Entities(), 4)
	cmds := command.NewCommands(buf)
	cmds.Despawn(e)
	buf.Apply(w)
	if w.Contains(e) {
		t.Fatal("entity must be dead after DespawnCommand.Apply")
	}
}

func TestDespawnCommand_DeadEntityNoOp(t *testing.T) {
	t.Parallel()
	w := newWorld()
	e := w.SpawnEmpty()
	_ = w.Despawn(e)
	buf := command.NewCommandBuffer(w.Entities(), 4)
	cmds := command.NewCommands(buf)
	cmds.Despawn(e)
	// Must not panic.
	buf.Apply(w)
}

// ---- SpawnEmptyCommand ------------------------------------------------------

func TestSpawnEmptyCommand_EntityInWorld(t *testing.T) {
	t.Parallel()
	w := newWorld()
	buf := command.NewCommandBuffer(w.Entities(), 4)
	cmds := command.NewCommands(buf)
	e := cmds.SpawnEmpty()
	if !e.IsValid() {
		t.Fatal("SpawnEmpty must return a valid entity before Apply")
	}
	buf.Apply(w)
	if !w.Contains(e) {
		t.Fatal("entity must be alive in the World after SpawnEmptyCommand.Apply")
	}
}

// ---- SpawnCommand -----------------------------------------------------------

func TestSpawnCommand_EntityWithComponents(t *testing.T) {
	t.Parallel()
	w := newWorld()
	buf := command.NewCommandBuffer(w.Entities(), 4)
	cmds := command.NewCommands(buf)
	e := cmds.Spawn(component.Data{Value: posComp{X: 1, Y: 2}})
	buf.Apply(w)
	if !w.Contains(e) {
		t.Fatal("spawned entity must be alive after Apply")
	}
	ptr, ok := world.Get[posComp](w, e)
	if !ok {
		t.Fatal("component not found after SpawnCommand.Apply")
	}
	if ptr == nil || ptr.X != 1 || ptr.Y != 2 {
		t.Fatalf("unexpected component value: %+v", ptr)
	}
}

func TestSpawnCommand_ZeroData(t *testing.T) {
	t.Parallel()
	w := newWorld()
	buf := command.NewCommandBuffer(w.Entities(), 4)
	cmds := command.NewCommands(buf)
	e := cmds.Spawn()
	buf.Apply(w)
	if !w.Contains(e) {
		t.Fatal("spawned entity (no data) must be alive after Apply")
	}
}

// ---- InsertCommand ----------------------------------------------------------

func TestInsertCommand_AddsComponent(t *testing.T) {
	t.Parallel()
	w := newWorld()
	e := w.SpawnEmpty()
	buf := command.NewCommandBuffer(w.Entities(), 4)
	cmds := command.NewCommands(buf)
	cmds.Entity(e).Insert(component.Data{Value: posComp{X: 3, Y: 4}})
	buf.Apply(w)
	ptr, ok := world.Get[posComp](w, e)
	if !ok || ptr == nil {
		t.Fatal("InsertCommand.Apply must add component")
	}
	if ptr.X != 3 || ptr.Y != 4 {
		t.Fatalf("unexpected value: %+v", ptr)
	}
}

func TestInsertCommand_DeadEntityNoOp(t *testing.T) {
	t.Parallel()
	w := newWorld()
	e := w.SpawnEmpty()
	_ = w.Despawn(e)
	buf := command.NewCommandBuffer(w.Entities(), 4)
	cmds := command.NewCommands(buf)
	cmds.Entity(e).Insert(component.Data{Value: posComp{}})
	buf.Apply(w) // must not panic
}

// ---- RemoveCommand ----------------------------------------------------------

func TestRemoveCommand_RemovesComponent(t *testing.T) {
	t.Parallel()
	w := newWorld()
	e := w.Spawn(component.Data{Value: posComp{X: 5}})
	// Look up the component ID.
	id, ok := w.Components().Lookup(reflect.TypeFor[posComp]())
	if !ok {
		t.Fatal("posComp not registered after Spawn")
	}
	buf := command.NewCommandBuffer(w.Entities(), 4)
	cmds := command.NewCommands(buf)
	cmds.Entity(e).Remove(id)
	buf.Apply(w)
	_, has := world.Get[posComp](w, e)
	if has {
		t.Fatal("component must be absent after RemoveCommand.Apply")
	}
}

func TestRemoveCommand_DeadEntityNoOp(t *testing.T) {
	t.Parallel()
	w := newWorld()
	e := w.Spawn(component.Data{Value: posComp{}})
	id, _ := w.Components().Lookup(reflect.TypeFor[posComp]())
	_ = w.Despawn(e)
	buf := command.NewCommandBuffer(w.Entities(), 4)
	cmds := command.NewCommands(buf)
	cmds.Entity(e).Remove(id)
	buf.Apply(w) // must not panic
}

// ---- CustomCommand ----------------------------------------------------------

func TestCustomCommand_Executed(t *testing.T) {
	t.Parallel()
	w := newWorld()
	called := false
	buf := command.NewCommandBuffer(w.Entities(), 4)
	buf.Push(command.NewCustomCommand(func(_ *world.World) { called = true }))
	buf.Apply(w)
	if !called {
		t.Fatal("CustomCommand.Apply must invoke fn")
	}
}

func TestCustomCommand_NilPanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil fn")
		}
	}()
	command.NewCustomCommand(nil)
}

// ---- Commands facade --------------------------------------------------------

func TestCommands_Add(t *testing.T) {
	t.Parallel()
	w := newWorld()
	buf := command.NewCommandBuffer(w.Entities(), 4)
	cmds := command.NewCommands(buf)
	called := false
	cmds.Add(command.NewCustomCommand(func(_ *world.World) { called = true }))
	buf.Apply(w)
	if !called {
		t.Fatal("Commands.Add must enqueue the command")
	}
}

func TestCommands_Buffer(t *testing.T) {
	t.Parallel()
	w := newWorld()
	buf := command.NewCommandBuffer(w.Entities(), 4)
	cmds := command.NewCommands(buf)
	if cmds.Buffer() != buf {
		t.Fatal("Commands.Buffer must return the underlying buffer")
	}
}

// ---- EntityCommands ---------------------------------------------------------

func TestEntityCommands_Entity(t *testing.T) {
	t.Parallel()
	w := newWorld()
	e := w.SpawnEmpty()
	buf := command.NewCommandBuffer(w.Entities(), 4)
	cmds := command.NewCommands(buf)
	ec := cmds.Entity(e)
	if ec.Entity() != e {
		t.Fatal("EntityCommands.Entity() must return target entity")
	}
}

func TestEntityCommands_Chaining(t *testing.T) {
	t.Parallel()
	w := newWorld()
	e := w.SpawnEmpty()
	buf := command.NewCommandBuffer(w.Entities(), 8)
	cmds := command.NewCommands(buf)
	cmds.Entity(e).
		Insert(component.Data{Value: posComp{X: 7}}).
		Insert(component.Data{Value: tagComp{}})
	buf.Apply(w)
	ptr, ok := world.Get[posComp](w, e)
	if !ok || ptr == nil || ptr.X != 7 {
		t.Fatal("chained Insert must add posComp")
	}
}

func TestEntityCommands_Despawn(t *testing.T) {
	t.Parallel()
	w := newWorld()
	e := w.SpawnEmpty()
	buf := command.NewCommandBuffer(w.Entities(), 4)
	cmds := command.NewCommands(buf)
	cmds.Entity(e).Despawn()
	buf.Apply(w)
	if w.Contains(e) {
		t.Fatal("EntityCommands.Despawn must despawn the entity")
	}
}

// ---- WithChildren / ChildSpawner --------------------------------------------

func TestWithChildren_SpawnsChildren(t *testing.T) {
	t.Parallel()
	w := newWorld()
	parent := w.SpawnEmpty()
	buf := command.NewCommandBuffer(w.Entities(), 8)
	cmds := command.NewCommands(buf)
	var child1, child2 entity.Entity
	cmds.Entity(parent).WithChildren(func(cs *command.ChildSpawner) {
		if cs.Parent() != parent {
			t.Error("ChildSpawner.Parent must be the owner entity")
		}
		child1 = cs.Spawn()
		child2 = cs.Spawn(component.Data{Value: posComp{X: 9}})
	})
	buf.Apply(w)
	if !w.Contains(child1) {
		t.Fatal("child1 must be alive after Apply")
	}
	if !w.Contains(child2) {
		t.Fatal("child2 must be alive after Apply")
	}
}

func TestWithChildren_NilFnNoOp(t *testing.T) {
	t.Parallel()
	w := newWorld()
	e := w.SpawnEmpty()
	buf := command.NewCommandBuffer(w.Entities(), 4)
	cmds := command.NewCommands(buf)
	cmds.Entity(e).WithChildren(nil) // must not panic
	buf.Apply(w)
}

// ---- Multiple buffers applied in order -------------------------------------

func TestMultipleBuffersAppliedInOrder(t *testing.T) {
	t.Parallel()
	w := newWorld()
	buf1 := command.NewCommandBuffer(w.Entities(), 4)
	buf2 := command.NewCommandBuffer(w.Entities(), 4)
	var order []int
	buf1.Push(command.NewCustomCommand(func(_ *world.World) { order = append(order, 1) }))
	buf2.Push(command.NewCustomCommand(func(_ *world.World) { order = append(order, 2) }))
	buf1.Apply(w)
	buf2.Apply(w)
	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Fatalf("order = %v, want [1 2]", order)
	}
}
