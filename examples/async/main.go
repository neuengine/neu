// Package main demonstrates parallel iteration determinism using the Neu task pool.
//
// It runs ForBatched over 1 000 integer slots, accumulating squares via atomic
// operations, then checks the total against the closed-form n*(n-1)*(2n-1)/6.
// The output is stable across runs (the mathematical invariant serves as the
// golden check — no PRNG state needed).
//
// Run:  go run ./examples/async
// Test: go test -race ./examples/async
package main

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/neuengine/neu/pkg/task"
)

const n = 1_000

func main() {
	pool, _ := task.NewTaskPools(task.TaskPoolConfig{})
	defer pool.Shutdown(context.Background()) //nolint

	// Build the work slice: integers [0, n).
	items := make([]int, n)
	for i := range items {
		items[i] = i
	}

	var sum atomic.Int64
	task.ForBatched(pool, items, 64, func(batch []int) {
		var local int64
		for _, v := range batch {
			local += int64(v) * int64(v)
		}
		sum.Add(local)
	})

	got := sum.Load()
	// Closed-form: Σ i² for i in [0, n) = (n-1)*n*(2n-1)/6
	want := int64(n-1) * int64(n) * int64(2*n-1) / 6
	if got != want {
		fmt.Printf("FAIL: parallel sum-of-squares = %d, want %d\n", got, want)
		return
	}
	fmt.Printf("PASS: parallel sum-of-squares = %d (n=%d, deterministic)\n", got, n)
}
