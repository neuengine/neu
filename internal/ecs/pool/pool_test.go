package pool_test

import (
	"sync"
	"testing"

	"github.com/neuengine/neu/internal/ecs/pool"
)

type payload struct {
	Data []byte
	ID   uint32
}

// ---- Pool[T] basics ---------------------------------------------------------

func TestPool_GetReturnsZeroValueOnFirstCall(t *testing.T) {
	t.Parallel()

	p := pool.New[payload]()
	v := p.Get()
	if v == nil {
		t.Fatal("Get must never return nil")
	}
	if v.ID != 0 || v.Data != nil {
		t.Fatalf("first Get must yield zero value; got %+v", v)
	}
}

func TestPool_PutGetRoundTrip(t *testing.T) {
	t.Parallel()

	p := pool.New[payload]()
	v := p.Get()
	v.ID = 42
	v.Data = []byte("hello")
	p.Put(v)

	// sync.Pool does not guarantee retrieval, but in single-goroutine code
	// without GC pressure the same pointer is overwhelmingly likely.
	got := p.Get()
	if got == nil {
		t.Fatal("Get after Put must return a value")
	}
	// The returned value MAY be the recycled one or fresh — both are valid
	// outcomes per sync.Pool semantics. Just confirm Get works.
}

func TestPool_PutNilDropped(t *testing.T) {
	t.Parallel()

	p := pool.New[payload]()
	p.Put(nil) // must not panic
	v := p.Get()
	if v == nil {
		t.Fatal("Get after Put(nil) must still yield a value")
	}
}

func TestPool_ResetHookRunsBeforeReturn(t *testing.T) {
	t.Parallel()

	resetCalls := 0
	p := pool.NewWithReset(func(v *payload) {
		resetCalls++
		v.ID = 0
		v.Data = nil
	})
	v := p.Get()
	v.ID = 99
	v.Data = []byte{1, 2, 3}
	p.Put(v)

	if resetCalls != 1 {
		t.Fatalf("reset hook called %d times, want 1", resetCalls)
	}
	if v.ID != 0 || v.Data != nil {
		t.Fatalf("reset hook did not clear value: %+v", v)
	}
}

func TestPool_ResetHookSkippedOnNilPut(t *testing.T) {
	t.Parallel()

	resetCalls := 0
	p := pool.NewWithReset(func(*payload) { resetCalls++ })
	p.Put(nil)
	if resetCalls != 0 {
		t.Fatalf("reset hook fired on nil Put; calls=%d", resetCalls)
	}
}

func TestPool_FactoryConstructsCustomValue(t *testing.T) {
	t.Parallel()

	p := pool.NewWithFactory(func() *payload {
		return &payload{ID: 7, Data: make([]byte, 0, 16)}
	})
	v := p.Get()
	if v.ID != 7 {
		t.Fatalf("factory output not honored: ID=%d, want 7", v.ID)
	}
	if cap(v.Data) != 16 {
		t.Fatalf("factory backing capacity = %d, want 16", cap(v.Data))
	}
}

func TestPool_FactoryNilFallsBackToZeroValue(t *testing.T) {
	t.Parallel()

	p := pool.NewWithFactory[payload](nil)
	v := p.Get()
	if v == nil || v.ID != 0 {
		t.Fatalf("nil factory must fall back to zero-value New(); got %+v", v)
	}
}

// ---- SlicePool[T] basics ----------------------------------------------------

func TestSlicePool_NewWithCapacity(t *testing.T) {
	t.Parallel()

	p := pool.NewSlicePool[byte](128)
	buf := p.Get()
	if buf == nil {
		t.Fatal("Get must never return nil")
	}
	if len(buf.Data) != 0 {
		t.Fatalf("Get must return Data.len=0; got %d", len(buf.Data))
	}
	if cap(buf.Data) != 128 {
		t.Fatalf("Get must return Data.cap=128 on first call; got %d", cap(buf.Data))
	}
	if p.MinCap() != 128 {
		t.Fatalf("MinCap = %d, want 128", p.MinCap())
	}
}

func TestSlicePool_NegativeMinCapClampedToZero(t *testing.T) {
	t.Parallel()

	p := pool.NewSlicePool[int](-5)
	if p.MinCap() != 0 {
		t.Fatalf("MinCap = %d, want 0", p.MinCap())
	}
	buf := p.Get()
	if cap(buf.Data) != 0 {
		t.Fatalf("Get Data.cap = %d, want 0 for negative minCap", cap(buf.Data))
	}
}

func TestSlicePool_PutGetReusesBackingArray(t *testing.T) {
	t.Parallel()

	p := pool.NewSlicePool[int](4)
	buf := p.Get()
	buf.Data = append(buf.Data, 1, 2, 3, 4, 5, 6, 7, 8) // grow beyond initial cap
	growthCap := cap(buf.Data)
	p.Put(buf)

	got := p.Get()
	if cap(got.Data) < growthCap {
		// sync.Pool may discard entries on GC; we only log the divergence.
		t.Logf("note: pool returned cap=%d (expected ≥%d) — possible Pool churn", cap(got.Data), growthCap)
	}
	if len(got.Data) != 0 {
		t.Fatalf("recycled buf.Data len = %d, want 0", len(got.Data))
	}
}

func TestSlicePool_PutNilDropped(t *testing.T) {
	t.Parallel()

	p := pool.NewSlicePool[int](16)
	p.Put(nil) // must not panic
	buf := p.Get()
	if buf == nil {
		t.Fatal("Get after Put(nil) must yield a non-nil buffer")
	}
}

func TestSlicePool_PutClearsElementReferences(t *testing.T) {
	t.Parallel()

	type holder struct{ Data *[1024]byte }
	p := pool.NewSlicePool[*holder](8)
	buf := p.Get()

	hold := &holder{Data: new([1024]byte)}
	buf.Data = append(buf.Data, hold)
	p.Put(buf)

	// After Put, the backing array must not retain pointer references.
	got := p.Get()
	full := got.Data[:cap(got.Data)]
	for i := range full {
		if full[i] != nil {
			t.Fatalf("recycled backing array index %d retains a non-nil reference", i)
		}
	}
}

// ---- Concurrent use ---------------------------------------------------------

func TestPool_Concurrent_NoRace(t *testing.T) {
	t.Parallel()

	p := pool.New[payload]()
	const goroutines = 8
	const ops = 1000

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range ops {
				v := p.Get()
				v.ID = 1
				p.Put(v)
			}
		}()
	}
	wg.Wait()
}

func TestSlicePool_Concurrent_NoRace(t *testing.T) {
	t.Parallel()

	p := pool.NewSlicePool[int](32)
	const goroutines = 8
	const ops = 1000

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range ops {
				buf := p.Get()
				buf.Data = append(buf.Data, 1, 2, 3)
				p.Put(buf)
			}
		}()
	}
	wg.Wait()
}
