package main

import (
	"sync/atomic"
	"testing"

	"github.com/neuengine/neu/pkg/task"
)

// TestParallelSumDeterminism runs the same parallel sum 50 times and verifies
// the result is always equal to the closed-form reference (race-clean via -race).
func TestParallelSumDeterminism(t *testing.T) {
	want := int64(n-1) * int64(n) * int64(2*n-1) / 6
	pool, _ := task.NewTaskPools(task.TaskPoolConfig{})
	t.Cleanup(func() { pool.Shutdown(t.Context()) }) //nolint

	items := make([]int, n)
	for i := range items {
		items[i] = i
	}

	for range 50 {
		var sum atomic.Int64
		task.ForBatched(pool, items, 64, func(batch []int) {
			var local int64
			for _, v := range batch {
				local += int64(v) * int64(v)
			}
			sum.Add(local)
		})
		if got := sum.Load(); got != want {
			t.Fatalf("run got %d, want %d (non-deterministic or wrong result)", got, want)
		}
	}
}
