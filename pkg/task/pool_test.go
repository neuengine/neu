package task

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

//go:fix inline
func uptr(v uint) *uint { return new(v) }

func TestConfigResolution(t *testing.T) {
	pct := 0.5
	cases := []struct {
		name string
		cfg  TaskPoolConfig
		want func(int) bool
	}{
		{"auto", TaskPoolConfig{}, func(n int) bool { return n == max(1, runtime.NumCPU()-1) }},
		{"explicit", TaskPoolConfig{ComputeThreads: uptr(3)}, func(n int) bool { return n == 3 }},
		{"zero-clamped", TaskPoolConfig{ComputeThreads: uptr(0)}, func(n int) bool { return n == 1 }},
		{"percent", TaskPoolConfig{PercentOfCores: &pct}, func(n int) bool { return n == max(1, runtime.NumCPU()/2) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.cfg.resolveCompute(); !c.want(got) {
				t.Fatalf("resolveCompute()=%d unexpected", got)
			}
		})
	}
}

// TestPoolFixedWorkerCount asserts INV-1: the goroutine count attributable to
// the pool stays exactly NumWorkers under heavy load.
func TestPoolFixedWorkerCount(t *testing.T) {
	base := runtime.NumGoroutine()
	cp, io := NewTaskPools(TaskPoolConfig{ComputeThreads: uptr(4)})
	if cp.NumWorkers() != 4 {
		t.Fatalf("NumWorkers=%d, want 4", cp.NumWorkers())
	}

	var ran int64
	const n = 50_000
	for range n {
		if err := cp.submit(func() { atomic.AddInt64(&ran, 1) }); err != nil {
			t.Fatalf("submit: %v", err)
		}
	}
	// Under sustained load the worker goroutine count must not grow.
	peak := runtime.NumGoroutine()
	if peak-base > 4+2 { // 4 workers + small slack for the test/runtime
		t.Fatalf("goroutine growth under load: base=%d peak=%d", base, peak)
	}

	deadline := time.After(10 * time.Second)
	for atomic.LoadInt64(&ran) < n {
		select {
		case <-deadline:
			t.Fatalf("only %d/%d tasks ran before timeout", atomic.LoadInt64(&ran), n)
		default:
			runtime.Gosched()
		}
	}

	io.Shutdown()
	if err := cp.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	// Workers must have exited — no leak (INV-4).
	settle := time.After(2 * time.Second)
	for runtime.NumGoroutine() > base+1 {
		select {
		case <-settle:
			t.Fatalf("goroutine leak after Shutdown: base=%d now=%d",
				base, runtime.NumGoroutine())
		default:
			runtime.Gosched()
		}
	}
}

// TestPoolDrainsOnShutdown verifies INV-4: queued work runs during Shutdown
// rather than being dropped.
func TestPoolDrainsOnShutdown(t *testing.T) {
	cp, _ := NewTaskPools(TaskPoolConfig{ComputeThreads: uptr(2)})
	var ran int64
	const n = 10_000
	for range n {
		_ = cp.submit(func() { atomic.AddInt64(&ran, 1) })
	}
	if err := cp.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if got := atomic.LoadInt64(&ran); got != n {
		t.Fatalf("drained %d/%d tasks on shutdown", got, n)
	}
	if err := cp.submit(func() {}); err != ErrPoolClosed {
		t.Fatalf("submit after Shutdown = %v, want ErrPoolClosed", err)
	}
}

func TestPoolPanicIsolation(t *testing.T) {
	cp, _ := NewTaskPools(TaskPoolConfig{ComputeThreads: uptr(2)})
	defer cp.Shutdown(context.Background())

	var after int64
	_ = cp.submit(func() { panic("boom") })
	_ = cp.submit(func() { atomic.AddInt64(&after, 1) })

	deadline := time.After(5 * time.Second)
	for atomic.LoadInt64(&after) == 0 {
		select {
		case <-deadline:
			t.Fatal("worker did not survive a panicking task")
		default:
			runtime.Gosched()
		}
	}
}

func TestIOPoolBounded(t *testing.T) {
	_, io := NewTaskPools(TaskPoolConfig{IOMaxThreads: 4})
	var inflight, peak int64
	var wg sync.WaitGroup
	const n = 200
	wg.Add(n)
	for range n {
		err := io.Go(func() {
			defer wg.Done()
			cur := atomic.AddInt64(&inflight, 1)
			for {
				p := atomic.LoadInt64(&peak)
				if cur <= p || atomic.CompareAndSwapInt64(&peak, p, cur) {
					break
				}
			}
			time.Sleep(time.Millisecond)
			atomic.AddInt64(&inflight, -1)
		})
		if err != nil {
			t.Fatalf("io.Go: %v", err)
		}
	}
	wg.Wait()
	if peak > 4 {
		t.Fatalf("IO concurrency peak=%d exceeded bound 4", peak)
	}
	io.Shutdown()
	if err := io.Go(func() {}); err != ErrPoolClosed {
		t.Fatalf("io.Go after Shutdown = %v, want ErrPoolClosed", err)
	}
}
