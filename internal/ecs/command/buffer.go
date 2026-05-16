package command

import (
	"sync"

	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/world"
)

const defaultCommandBufferCap = 64

// bufPool recycles *CommandBuffer values to reduce per-frame heap pressure
// (C27). AcquireBuffer / ReleaseBuffer are the public entry points.
var bufPool = sync.Pool{
	New: func() any {
		return &CommandBuffer{commands: make([]Command, 0, defaultCommandBufferCap)}
	},
}

// CommandBuffer is a per-system, append-only queue of [Command] values.
// Commands are applied in FIFO order when [CommandBuffer.Apply] is called.
// The backing slice is retained across [CommandBuffer.Reset] calls so that
// typical frame-level usage stays within the pre-allocated capacity and
// produces zero heap allocations on the hot path.
//
// CommandBuffer is not thread-safe; each system must own its buffer
// exclusively during execution.
type CommandBuffer struct {
	entities *entity.EntityAllocator
	commands []Command
}

// NewCommandBuffer allocates a CommandBuffer with the given initial capacity.
// Prefer [AcquireBuffer] for temporary (per-frame) buffers to benefit from
// pool reuse.
func NewCommandBuffer(alloc *entity.EntityAllocator, initialCap int) *CommandBuffer {
	if initialCap <= 0 {
		initialCap = defaultCommandBufferCap
	}
	return &CommandBuffer{
		commands: make([]Command, 0, initialCap),
		entities: alloc,
	}
}

// AcquireBuffer returns a CommandBuffer from the package-level pool
// configured with alloc. The caller must call [ReleaseBuffer] when the
// buffer is no longer needed to return it for reuse.
func AcquireBuffer(alloc *entity.EntityAllocator) *CommandBuffer {
	buf := bufPool.Get().(*CommandBuffer)
	buf.entities = alloc
	return buf
}

// ReleaseBuffer resets buf and returns it to the pool.
func ReleaseBuffer(buf *CommandBuffer) {
	buf.Reset()
	buf.entities = nil
	bufPool.Put(buf)
}

// Push appends cmd to the buffer. Panics on a nil command — pushing nil is
// always a programming error caught at development time.
func (cb *CommandBuffer) Push(cmd Command) {
	if cmd == nil {
		panic("ecs/command: Push called with nil Command")
	}
	cb.commands = append(cb.commands, cmd)
}

// Apply executes all buffered commands in FIFO order against w.
// The buffer is NOT reset after Apply; call [CommandBuffer.Reset] explicitly.
// Apply itself performs zero heap allocations after warm-up.
func (cb *CommandBuffer) Apply(w *world.World) {
	for _, cmd := range cb.commands {
		cmd.Apply(w)
	}
}

// Reset clears all pending commands and releases their interface references
// for the garbage collector. The backing slice capacity is preserved so the
// next fill cycle reuses the same memory.
func (cb *CommandBuffer) Reset() {
	for i := range cb.commands {
		cb.commands[i] = nil
	}
	cb.commands = cb.commands[:0]
}

// Len returns the number of pending commands.
func (cb *CommandBuffer) Len() int { return len(cb.commands) }

// ReserveEntity allocates a new Entity from the buffer's entity allocator.
// The entity is immediately alive in the allocator but has no archetype
// record in the World until the corresponding SpawnCommand is applied.
//
// Thread-safe (T-1F02): the underlying [entity.EntityAllocator] serialises
// concurrent reservations on its internal RWMutex, so multiple systems may
// reserve from the same buffer pool without external coordination.
func (cb *CommandBuffer) ReserveEntity() entity.Entity {
	return cb.entities.Allocate()
}

// Flush is shorthand for Apply(w) followed by Reset(). Used as the default
// flusher when a CommandBuffer is registered with a [world.World] via
// [CommandBuffer.RegisterWith].
func (cb *CommandBuffer) Flush(w *world.World) {
	cb.Apply(w)
	cb.Reset()
}

// RegisterWith installs cb's [CommandBuffer.Flush] as a deferred flusher on w.
// Every subsequent call to [world.World.ApplyDeferred] will Apply this buffer
// then Reset it. Intended for long-lived per-system buffers — pool-rented
// buffers (Acquire/Release) must NOT be registered, since pool reuse would
// dangle the registration.
func (cb *CommandBuffer) RegisterWith(w *world.World) {
	w.RegisterDeferredFlusher(cb.Flush)
}

// ApplyDeferredCommands flushes every buffer in execution order. nil entries
// in buffers are skipped. Each non-nil buffer is Applied then Reset.
//
// This top-level helper is the explicit alternative to the world-registered
// flusher path: callers that manage their own buffer slice (e.g. ad-hoc tests
// or specialised executors) can flush them in a single call without going
// through [world.World.RegisterDeferredFlusher].
func ApplyDeferredCommands(w *world.World, buffers []*CommandBuffer) {
	for _, buf := range buffers {
		if buf == nil {
			continue
		}
		buf.Flush(w)
	}
}
