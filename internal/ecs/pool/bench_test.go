package pool_test

import (
	"testing"

	"github.com/neuengine/neu/internal/ecs/pool"
)

// BenchmarkPool_GetPut measures the steady-state cost of acquiring and
// returning a *T. After warm-up the round-trip is essentially atomic
// pointer ops — no allocations.
func BenchmarkPool_GetPut(b *testing.B) {
	p := pool.New[payload]()
	// Warm up.
	p.Put(p.Get())

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		v := p.Get()
		p.Put(v)
	}
}

// BenchmarkPool_GetPut_WithReset measures the same round-trip when a Reset
// hook runs on Put. The hook adds a function call but still 0 allocs.
// The bench avoids any caller-side allocation so the alloc-counter reflects
// only what the pool itself does.
func BenchmarkPool_GetPut_WithReset(b *testing.B) {
	p := pool.NewWithReset(func(v *payload) {
		v.ID = 0
		v.Data = v.Data[:0]
	})
	first := p.Get()
	first.Data = make([]byte, 0, 16)
	p.Put(first)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		v := p.Get()
		v.ID = 1
		p.Put(v)
	}
}

// BenchmarkSlicePool_GetPut measures cap-preserving slice recycle through the
// SliceBuf wrapper. After warm-up no allocations.
func BenchmarkSlicePool_GetPut(b *testing.B) {
	p := pool.NewSlicePool[int](64)
	p.Put(p.Get())

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		buf := p.Get()
		buf.Data = append(buf.Data, 1, 2, 3, 4, 5)
		p.Put(buf)
	}
}

// BenchmarkSlicePool_GetPut_LargeFill measures recycle with a fill that hits
// the pre-allocated capacity (no growth).
func BenchmarkSlicePool_GetPut_LargeFill(b *testing.B) {
	p := pool.NewSlicePool[int](256)
	p.Put(p.Get())

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		buf := p.Get()
		for j := range 200 {
			buf.Data = append(buf.Data, j)
		}
		p.Put(buf)
	}
}

// BenchmarkBaseline_MakeSlice is the no-pool baseline: allocate a fresh
// slice every iteration. Used for comparison against BenchmarkSlicePool_*.
func BenchmarkBaseline_MakeSlice(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		s := make([]int, 0, 64)
		s = append(s, 1, 2, 3, 4, 5)
		_ = s
	}
}
