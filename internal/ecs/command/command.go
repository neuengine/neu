// Package command provides the deferred-mutation layer for the ECS runtime.
// Command values are pushed into a CommandBuffer during system execution and
// applied atomically at synchronisation points (apply-points) by the
// scheduler. The design follows a strict FIFO contract: commands within a
// single buffer are applied in push order; buffers from multiple systems are
// flushed in system-execution order at the apply point.
//
// # Constraints
//
//   - Zero external dependencies (C24).
//   - CommandBuffer instances are recycled via [sync.Pool] to reduce per-frame
//     heap pressure (C27). [BenchmarkCommandFlush] verifies ≤1 alloc/op for
//     an Apply+Reset cycle after warm-up.
//   - The package is concurrent-read-safe but not concurrent-write-safe; each
//     system owns its buffer exclusively during execution.
package command

import "github.com/neuengine/neu/internal/ecs/world"

// Command is a single deferred mutation to a [world.World].
// Apply is invoked by [CommandBuffer.Apply] in FIFO order at the flush point.
// Implementations must be safe to call multiple times (idempotent is preferred
// but not required) because a mistaken double-flush is a recoverable error.
type Command interface {
	Apply(w *world.World)
}
