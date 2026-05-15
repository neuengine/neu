package entity

import (
	"sync"
	"testing"
)

func TestEntityAllocator_ConcurrentAllocateUniqueIDs(t *testing.T) {
	t.Parallel()

	const goroutines = 8
	const perGoroutine = 1024

	a := NewEntityAllocator(goroutines * perGoroutine)
	results := make([][]Entity, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := range goroutines {
		go func(g int) {
			defer wg.Done()
			local := make([]Entity, perGoroutine)
			for i := range perGoroutine {
				local[i] = a.Allocate()
			}
			results[g] = local
		}(g)
	}
	wg.Wait()

	seen := make(map[EntityID]struct{}, goroutines*perGoroutine)
	for _, group := range results {
		for _, e := range group {
			if _, dup := seen[e.ID()]; dup {
				t.Fatalf("duplicate entity ID issued under concurrency: %v", e)
			}
			seen[e.ID()] = struct{}{}
			if !a.IsAlive(e) {
				t.Fatalf("entity not alive after concurrent allocate: %v", e)
			}
		}
	}
	if a.Len() != goroutines*perGoroutine {
		t.Fatalf("Len = %d, want %d", a.Len(), goroutines*perGoroutine)
	}
}

func TestEntityAllocator_ConcurrentAllocateAndFree(t *testing.T) {
	t.Parallel()

	a := NewEntityAllocator(0)

	// Producer/consumer: half goroutines Allocate, half Free.
	const writers = 4
	const ops = 2000

	allocated := make(chan Entity, writers*ops)
	var wg sync.WaitGroup

	wg.Add(writers)
	for range writers {
		go func() {
			defer wg.Done()
			for range ops {
				allocated <- a.Allocate()
			}
		}()
	}

	wg.Add(writers)
	for range writers {
		go func() {
			defer wg.Done()
			for range ops {
				e := <-allocated
				a.Free(e)
			}
		}()
	}

	wg.Wait()
	if a.Len() != 0 {
		t.Fatalf("Len after balanced alloc/free = %d, want 0", a.Len())
	}
}

func TestEntityAllocator_ConcurrentReadsDuringWrites(t *testing.T) {
	t.Parallel()

	a := NewEntityAllocator(0)
	// Pre-populate.
	pre := a.AllocateMany(64)

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// Reader goroutines hammer IsAlive.
	for range 4 {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
					for _, e := range pre {
						_ = a.IsAlive(e)
					}
				}
			}
		})
	}

	// Writer goroutine adds and removes entities.
	for range 1000 {
		e := a.Allocate()
		a.Free(e)
	}
	close(stop)
	wg.Wait()
}
