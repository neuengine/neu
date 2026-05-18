package task

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

func TestDequeOwnerLIFO(t *testing.T) {
	d := newDeque(4)
	if _, ok := d.popBottom(); ok {
		t.Fatal("popBottom on empty deque returned ok")
	}
	const n = 100
	for i := 0; i < n; i++ {
		i := i
		d.pushBottom(func() { _ = i })
	}
	got := 0
	for {
		if _, ok := d.popBottom(); !ok {
			break
		}
		got++
	}
	if got != n {
		t.Fatalf("popBottom drained %d, want %d", got, n)
	}
}

func TestDequeGrow(t *testing.T) {
	d := newDeque(2) // 4 slots
	const n = 10_000 // forces several grows
	for i := 0; i < n; i++ {
		d.pushBottom(func() {})
	}
	got := 0
	for {
		if _, ok := d.popBottom(); !ok {
			break
		}
		got++
	}
	if got != n {
		t.Fatalf("after grow drained %d, want %d", got, n)
	}
}

// TestDequeStealStorm runs the deque under a steal storm: one owner pushing /
// popping while 8×CPU thieves steal concurrently. It asserts every task is
// retrieved exactly once (Chase-Lev CAS uniqueness) and is -race clean (C-005).
func TestDequeStealStorm(t *testing.T) {
	const n = 200_000
	d := newDeque(8)

	seen := make([]int32, n)
	mk := func(i int) task { return func() { atomic.AddInt32(&seen[i], 1) } }

	thieves := 8 * runtime.NumCPU()
	var wg sync.WaitGroup
	var stolen, popped int64
	stop := make(chan struct{})

	wg.Add(thieves)
	for g := 0; g < thieves; g++ {
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					// Final sweep so nothing is left behind.
					for {
						t, ok := d.steal()
						if !ok {
							return
						}
						atomic.AddInt64(&stolen, 1)
						t()
					}
				default:
					if tk, ok := d.steal(); ok {
						atomic.AddInt64(&stolen, 1)
						tk()
					}
				}
			}
		}()
	}

	// Owner: interleave pushes with opportunistic pops.
	for i := 0; i < n; i++ {
		d.pushBottom(mk(i))
		if i%3 == 0 {
			if tk, ok := d.popBottom(); ok {
				atomic.AddInt64(&popped, 1)
				tk()
			}
		}
	}
	for {
		tk, ok := d.popBottom()
		if !ok {
			break
		}
		atomic.AddInt64(&popped, 1)
		tk()
	}
	close(stop)
	wg.Wait()

	total := atomic.LoadInt64(&stolen) + atomic.LoadInt64(&popped)
	if total != n {
		t.Fatalf("retrieved %d tasks, want %d (stolen=%d popped=%d)",
			total, n, stolen, popped)
	}
	for i := 0; i < n; i++ {
		if c := atomic.LoadInt32(&seen[i]); c != 1 {
			t.Fatalf("task %d executed %d times, want exactly 1", i, c)
		}
	}
}

// BenchmarkDequePushPop measures the steady-state owner hot path. Depth stays
// ≤1 so the ring never grows ⇒ 0 allocs/op (C-027 / C-004).
func BenchmarkDequePushPop(b *testing.B) {
	d := newDeque(logInitialDeque)
	fn := func() {}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.pushBottom(fn)
		if _, ok := d.popBottom(); !ok {
			b.Fatal("popBottom returned !ok in steady state")
		}
	}
}
