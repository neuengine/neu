package command

import (
	"github.com/teratron/boltengine/internal/ecs/component"
	"github.com/teratron/boltengine/internal/ecs/entity"
	"github.com/teratron/boltengine/internal/ecs/world"
)

// SpawnEmptyCommand parks a pre-reserved entity into the World with no
// components. The entity must have been reserved via
// [CommandBuffer.ReserveEntity] before this command was pushed.
type SpawnEmptyCommand struct {
	entity entity.Entity
}

func (c *SpawnEmptyCommand) Apply(w *world.World) {
	w.SpawnWithEntity(c.entity)
}

// SpawnCommand parks a pre-reserved entity into the World with the given
// component data. The entity must have been reserved via
// [CommandBuffer.ReserveEntity] before this command was pushed.
type SpawnCommand struct {
	entity entity.Entity
	data   []component.Data
}

func (c *SpawnCommand) Apply(w *world.World) {
	w.SpawnWithEntityAndData(c.entity, c.data...)
}

// DespawnCommand destroys an entity and all its components. No-op when the
// entity is already dead (INV-4: valid-target check prevents stale writes).
type DespawnCommand struct {
	entity entity.Entity
}

func (c *DespawnCommand) Apply(w *world.World) {
	if !w.Contains(c.entity) {
		return
	}
	_ = w.Despawn(c.entity)
}

// InsertCommand adds or overwrites a component on an entity.
// No-op when the entity is not alive.
type InsertCommand struct {
	entity entity.Entity
	data   component.Data
}

func (c *InsertCommand) Apply(w *world.World) {
	if !w.Contains(c.entity) {
		return
	}
	_ = w.Insert(c.entity, c.data)
}

// RemoveCommand strips the component with the given ID from an entity.
// No-op when the entity is not alive or does not carry the component.
type RemoveCommand struct {
	entity entity.Entity
	id     component.ID
}

func (c *RemoveCommand) Apply(w *world.World) {
	_ = world.RemoveByID(w, c.entity, c.id)
}

// CustomCommand wraps a user-provided function as a [Command]. Use
// [NewCustomCommand] to construct one; passing nil panics at construction time
// rather than silently at apply time.
type CustomCommand struct {
	fn func(*world.World)
}

// NewCustomCommand returns a [Command] that calls fn when applied.
// Panics if fn is nil.
func NewCustomCommand(fn func(*world.World)) *CustomCommand {
	if fn == nil {
		panic("ecs/command: NewCustomCommand called with nil fn")
	}
	return &CustomCommand{fn: fn}
}

func (c *CustomCommand) Apply(w *world.World) {
	c.fn(w)
}
