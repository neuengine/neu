package task

import (
	"context"
	"sync/atomic"
	"testing"
)

func TestRunScopeJoinsAllChildren(t *testing.T) {
	cp, _ := NewTaskPools(TaskPoolConfig{ComputeThreads: new(uint(4))})
	defer func() { _ = cp.Shutdown(context.Background()) }()

	const n = 5000
	var done atomic.Int64
	RunScope(cp, func(s *Scope) {
		for range n {
			s.Spawn(func() { done.Add(1) })
		}
	})
	// RunScope must not return until every child finished (INV-2).
	if got := done.Load(); got != n {
		t.Fatalf("RunScope returned with %d/%d children done", got, n)
	}
}

// TestRunScopeBorrowVisible asserts a stack slice mutated by children is fully
// visible after RunScope returns (the borrow-safety contract).
func TestRunScopeBorrowVisible(t *testing.T) {
	cp, _ := NewTaskPools(TaskPoolConfig{ComputeThreads: new(uint(4))})
	defer func() { _ = cp.Shutdown(context.Background()) }()

	buf := make([]int, 1000) // borrowed stack-ish slice
	RunScope(cp, func(s *Scope) {
		for i := range buf {
			s.Spawn(func() { buf[i] = i * i })
		}
	})
	for i := range buf {
		if buf[i] != i*i {
			t.Fatalf("buf[%d]=%d, want %d (write not visible after RunScope)", i, buf[i], i*i)
		}
	}
}

func TestRunScopeNestedNoDeadlock(t *testing.T) {
	cp, _ := NewTaskPools(TaskPoolConfig{ComputeThreads: new(uint(2))})
	defer func() { _ = cp.Shutdown(context.Background()) }()

	var leaves int64
	RunScope(cp, func(outer *Scope) {
		for range 8 {
			outer.Spawn(func() {
				// Nested RunScope from inside a worker must not deadlock
				// (cooperative wait).
				RunScope(cp, func(inner *Scope) {
					for range 8 {
						inner.Spawn(func() { atomic.AddInt64(&leaves, 1) })
					}
				})
			})
		}
	})
	if leaves != 64 {
		t.Fatalf("nested scopes produced %d leaves, want 64", leaves)
	}
}

func TestParChunkMap(t *testing.T) {
	cp, _ := NewTaskPools(TaskPoolConfig{ComputeThreads: new(uint(4))})
	defer func() { _ = cp.Shutdown(context.Background()) }()

	in := make([]int, 1000)
	for i := range in {
		in[i] = i
	}
	out := ParChunkMap(cp, in, 64, func(v int) int { return v * 2 })
	for i, v := range out {
		if v != i*2 {
			t.Fatalf("out[%d]=%d, want %d", i, v, i*2)
		}
	}
}

func TestParSplatMap(t *testing.T) {
	cp, _ := NewTaskPools(TaskPoolConfig{ComputeThreads: new(uint(4))})
	defer func() { _ = cp.Shutdown(context.Background()) }()

	in := make([]int, 333)
	for i := range in {
		in[i] = i + 1
	}
	out := ParSplatMap(cp, in, 8, func(v int) int { return v + 100 })
	if len(out) != len(in) {
		t.Fatalf("len(out)=%d, want %d", len(out), len(in))
	}
	for i, v := range out {
		if v != in[i]+100 {
			t.Fatalf("out[%d]=%d, want %d", i, v, in[i]+100)
		}
	}
}

func TestForBatchedCoversExactlyOnce(t *testing.T) {
	cp, _ := NewTaskPools(TaskPoolConfig{ComputeThreads: new(uint(4))})
	defer func() { _ = cp.Shutdown(context.Background()) }()

	const n = 10_000
	hits := make([]int32, n)
	items := make([]int, n)
	for i := range items {
		items[i] = i
	}
	ForBatched(cp, items, 128, func(batch []int) {
		for _, v := range batch {
			atomic.AddInt32(&hits[v], 1)
		}
	})
	for i := range n {
		if hits[i] != 1 {
			t.Fatalf("item %d processed %d times, want exactly 1", i, hits[i])
		}
	}
}

// TestForBatchedGoldenDeterminism: with a fixed seed and ordered reduction the
// result is identical across many runs (the dispatcher must not corrupt or
// drop work under interleaving).
func TestForBatchedGoldenDeterminism(t *testing.T) {
	cp, _ := NewTaskPools(TaskPoolConfig{ComputeThreads: new(uint(4))})
	defer func() { _ = cp.Shutdown(context.Background()) }()

	const n = 4096
	items := make([]int, n)
	for i := range items {
		items[i] = i
	}
	var golden int64
	for i := range n {
		golden += int64(i)
	}
	for run := range 100 {
		var sum int64
		ForBatched(cp, items, 97, func(batch []int) {
			var local int64
			for _, v := range batch {
				local += int64(v)
			}
			atomic.AddInt64(&sum, local)
		})
		if sum != golden {
			t.Fatalf("run %d: sum=%d, want golden=%d", run, sum, golden)
		}
	}
}

func TestTaskHandleSpawnPollBlock(t *testing.T) {
	cp, _ := NewTaskPools(TaskPoolConfig{ComputeThreads: new(uint(2))})
	defer func() { _ = cp.Shutdown(context.Background()) }()

	h := Spawn(cp, func() int { return 42 })
	got := h.BlockOn(cp)
	if got != 42 {
		t.Fatalf("BlockOn=%d, want 42", got)
	}
	if !h.IsFinished() {
		t.Fatal("IsFinished=false after BlockOn")
	}
	v, ok := h.Poll()
	if !ok || v != 42 {
		t.Fatalf("Poll=(%d,%v), want (42,true)", v, ok)
	}
}

func TestTaskHandlePanicRepanics(t *testing.T) {
	cp, _ := NewTaskPools(TaskPoolConfig{ComputeThreads: new(uint(2))})
	defer func() { _ = cp.Shutdown(context.Background()) }()

	h := Spawn(cp, func() int { panic("kaboom") })
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("BlockOn did not re-panic on a panicking task")
		}
		if _, ok := r.(*PanicError); !ok {
			t.Fatalf("re-panic value is %T, want *PanicError", r)
		}
	}()
	_ = h.BlockOn(cp)
}

// BenchmarkForBatched: one call over b.N items. Per-item/per-batch path is
// allocation-free; O(numWorkers) setup amortizes to 0 allocs/op (C-027).
func BenchmarkForBatched(b *testing.B) {
	cp, _ := NewTaskPools(TaskPoolConfig{ComputeThreads: new(uint(4))})
	defer func() { _ = cp.Shutdown(context.Background()) }()

	items := make([]int, b.N)
	var sink int64
	b.ReportAllocs()
	b.ResetTimer()
	ForBatched(cp, items, 256, func(batch []int) {
		atomic.AddInt64(&sink, int64(len(batch)))
	})
	b.StopTimer()
	if sink != int64(b.N) {
		b.Fatalf("processed %d items, want %d", sink, b.N)
	}
}
