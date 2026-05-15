package command

import (
	"github.com/teratron/boltengine/internal/ecs/component"
	"github.com/teratron/boltengine/internal/ecs/entity"
)

// Commands is the system-parameter facade over a [CommandBuffer].
// Each system receives its own Commands instance backed by a dedicated buffer.
// The API exposes a builder-style surface for common mutations without leaking
// the buffer internals into system code.
type Commands struct {
	buffer *CommandBuffer
}

// NewCommands wraps buf in a Commands facade.
func NewCommands(buf *CommandBuffer) *Commands {
	return &Commands{buffer: buf}
}

// Buffer returns the underlying [CommandBuffer].
func (c *Commands) Buffer() *CommandBuffer { return c.buffer }

// SpawnEmpty reserves an entity ID and enqueues a [SpawnEmptyCommand].
// The returned Entity is usable as a stable reference in subsequent commands
// within the same system; it has no archetype record until Apply is called.
func (c *Commands) SpawnEmpty() entity.Entity {
	e := c.buffer.ReserveEntity()
	c.buffer.Push(&SpawnEmptyCommand{entity: e})
	return e
}

// Spawn reserves an entity ID, copies data, and enqueues a [SpawnCommand].
// Returns the pre-reserved Entity immediately.
func (c *Commands) Spawn(data ...component.Data) entity.Entity {
	e := c.buffer.ReserveEntity()
	cp := make([]component.Data, len(data))
	copy(cp, data)
	c.buffer.Push(&SpawnCommand{entity: e, data: cp})
	return e
}

// Despawn enqueues a [DespawnCommand] for e.
func (c *Commands) Despawn(e entity.Entity) {
	c.buffer.Push(&DespawnCommand{entity: e})
}

// Entity returns an [EntityCommands] builder targeting e.
func (c *Commands) Entity(e entity.Entity) *EntityCommands {
	return &EntityCommands{entity: e, buffer: c.buffer}
}

// Add enqueues an arbitrary [Command].
func (c *Commands) Add(cmd Command) {
	c.buffer.Push(cmd)
}

// EntityCommands provides a fluent API for mutating a single entity via
// queued commands. All methods enqueue commands; none apply them immediately.
type EntityCommands struct {
	entity entity.Entity
	buffer *CommandBuffer
}

// Entity returns the target entity.
func (ec *EntityCommands) Entity() entity.Entity { return ec.entity }

// Insert enqueues an [InsertCommand] to add or overwrite a component.
// Returns self for chaining.
func (ec *EntityCommands) Insert(data component.Data) *EntityCommands {
	ec.buffer.Push(&InsertCommand{entity: ec.entity, data: data})
	return ec
}

// Remove enqueues a [RemoveCommand] to strip component id.
// Returns self for chaining.
func (ec *EntityCommands) Remove(id component.ID) *EntityCommands {
	ec.buffer.Push(&RemoveCommand{entity: ec.entity, id: id})
	return ec
}

// Despawn enqueues a [DespawnCommand] for the target entity.
func (ec *EntityCommands) Despawn() {
	ec.buffer.Push(&DespawnCommand{entity: ec.entity})
}

// WithChildren calls fn with a [ChildSpawner] scoped to this entity.
// Child entities spawned via the spawner are automatically linked to this
// entity as their parent once the ChildOf component type is available
// (Track I). Returns self for chaining.
func (ec *EntityCommands) WithChildren(fn func(spawner *ChildSpawner)) *EntityCommands {
	if fn != nil {
		fn(&ChildSpawner{parent: ec.entity, buffer: ec.buffer})
	}
	return ec
}

// ChildSpawner creates entities that are parented to a specific owner.
// The ChildOf component link is injected in Track I; for Phase 1 the
// spawner enqueues plain SpawnCommands without the parent link.
type ChildSpawner struct {
	parent entity.Entity
	buffer *CommandBuffer
}

// Parent returns the owner entity this spawner is scoped to.
func (cs *ChildSpawner) Parent() entity.Entity { return cs.parent }

// Spawn reserves a child entity, copies data, and enqueues a [SpawnCommand].
func (cs *ChildSpawner) Spawn(data ...component.Data) entity.Entity {
	e := cs.buffer.ReserveEntity()
	cp := make([]component.Data, len(data))
	copy(cp, data)
	cs.buffer.Push(&SpawnCommand{entity: e, data: cp})
	return e
}
