package command_test

import (
	"testing"

	"github.com/teratron/boltengine/internal/ecs/command"
	"github.com/teratron/boltengine/internal/ecs/world"
)

// noopCmd implements command.Command without any work or allocation. Used as
// a stable shared instance so that interface boxing is a pointer copy and
// pushing it into a CommandBuffer does not allocate (C27).
type noopCmd struct{}

func (n *noopCmd) Apply(_ *world.World) {}

var sharedNoop = &noopCmd{}

// BenchmarkCommandFlush measures one Push/Apply/Reset cycle on a 100-command
// buffer. After the first iteration the backing slice has been grown to
// capacity, the sync.Pool is warm, and every subsequent op should perform
// zero heap allocations — well within the C27 budget of ≤1 alloc/op.
func BenchmarkCommandFlush(b *testing.B) {
	w := world.NewWorld()
	buf := command.NewCommandBuffer(w.Entities(), 128)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		for range 100 {
			buf.Push(sharedNoop)
		}
		buf.Apply(w)
		buf.Reset()
	}
}

// BenchmarkCommandApply measures Apply alone on a pre-populated buffer.
// This is the pure flush path — no Push, no Reset, no allocations expected.
func BenchmarkCommandApply(b *testing.B) {
	w := world.NewWorld()
	buf := command.NewCommandBuffer(w.Entities(), 128)
	for range 100 {
		buf.Push(sharedNoop)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		buf.Apply(w)
	}
}

// BenchmarkAcquireRelease measures the sync.Pool round-trip cost. After
// warm-up this should be allocation-free: bufPool returns a recycled
// CommandBuffer rather than constructing a new one.
func BenchmarkAcquireRelease(b *testing.B) {
	w := world.NewWorld()
	alloc := w.Entities()

	// Warm the pool.
	command.ReleaseBuffer(command.AcquireBuffer(alloc))

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		buf := command.AcquireBuffer(alloc)
		command.ReleaseBuffer(buf)
	}
}
