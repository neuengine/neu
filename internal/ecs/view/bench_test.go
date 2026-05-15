package view_test

import (
	"testing"

	"github.com/teratron/boltengine/internal/ecs/component"
	"github.com/teratron/boltengine/internal/ecs/view"
	"github.com/teratron/boltengine/internal/ecs/world"
)

// BenchmarkView_Entities measures range-over-func iteration over a view of
// 1000 entities living in a single archetype. After warm-up the iterator is
// allocation-free.
func BenchmarkView_Entities(b *testing.B) {
	w := world.NewWorld()
	for range 1000 {
		w.Spawn(component.Data{Value: pos{}})
	}
	v, err := view.Requiring(w, view.TagOf[pos](w))
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		var sum uint32
		for e := range v.Entities(w) {
			sum += e.Index()
		}
		_ = sum
	}
}

// BenchmarkView_Count measures O(K_archetypes) sum over a view that spans
// 4 distinct archetype shapes, each with 250 entities.
func BenchmarkView_Count(b *testing.B) {
	w := world.NewWorld()
	for range 250 {
		w.Spawn(component.Data{Value: pos{}})
	}
	for range 250 {
		w.Spawn(component.Data{Value: pos{}}, component.Data{Value: vel{}})
	}
	for range 250 {
		w.Spawn(component.Data{Value: pos{}}, component.Data{Value: tag{}})
	}
	for range 250 {
		w.Spawn(component.Data{Value: pos{}}, component.Data{Value: vel{}}, component.Data{Value: tag{}})
	}
	v, err := view.Requiring(w, view.TagOf[pos](w))
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = v.Count(w)
	}
}

// BenchmarkView_Contains measures the linear membership scan over the
// matched-archetypes list.
func BenchmarkView_Contains(b *testing.B) {
	w := world.NewWorld()
	var pivot = w.Spawn(component.Data{Value: pos{}, ID: 0})
	for range 999 {
		w.Spawn(component.Data{Value: pos{}})
	}
	v, err := view.Requiring(w, view.TagOf[pos](w))
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = v.Contains(w, pivot)
	}
}

// BenchmarkTagger_MaskOf measures the type→ID + Mask construction cost.
// First call registers; subsequent calls hit the cached path.
func BenchmarkTagger_MaskOf(b *testing.B) {
	w := world.NewWorld()
	_ = view.MaskOf[pos](w) // warm

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = view.MaskOf[pos](w)
	}
}

// BenchmarkArchetypeStore_Listener measures the cost of listener fan-out on
// new archetype creation. After warm-up no allocations expected.
func BenchmarkArchetypeStore_Listener(b *testing.B) {
	w := world.NewWorld()
	_ = w.Archetypes().OnArchetypeCreated(func(*world.Archetype) {})
	_ = w.Archetypes().OnArchetypeCreated(func(*world.Archetype) {})

	// We benchmark spawn-into-existing-archetype, which does NOT fire the
	// listener — measuring the hot-path cost when the listener list isn't
	// touched per spawn.
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		w.Spawn(component.Data{Value: pos{}})
	}
}
