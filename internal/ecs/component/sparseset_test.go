package component

import (
	"reflect"
	"slices"
	"testing"
	"unsafe"

	"github.com/teratron/boltengine/internal/ecs/entity"
)

func newPositionSpec(t *testing.T) ColumnSpec {
	t.Helper()
	r := NewRegistry()
	id := RegisterType[Position](r)
	return ColumnSpecFromInfo(r.Info(id))
}

func newTagSpec(t *testing.T) ColumnSpec {
	t.Helper()
	r := NewRegistry()
	id := RegisterType[EnemyTag](r)
	return ColumnSpecFromInfo(r.Info(id))
}

func TestSparseSetAddGetRemove(t *testing.T) {
	t.Parallel()

	s := NewSparseSet(newPositionSpec(t))
	e1 := entity.NewEntity(1, 1)
	e2 := entity.NewEntity(2, 1)

	s.Add(e1, Position{X: 1, Y: 2, Z: 3})
	s.Add(e2, Position{X: 4, Y: 5, Z: 6})

	if s.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", s.Len())
	}
	if !s.Has(e1) || !s.Has(e2) {
		t.Fatal("both entities must be present")
	}

	ptr, ok := s.Get(e1)
	if !ok {
		t.Fatal("Get(e1) must succeed")
	}
	got := *(*Position)(ptr)
	if got != (Position{1, 2, 3}) {
		t.Fatalf("Get(e1) = %+v, want {1,2,3}", got)
	}

	if !s.Remove(e1) {
		t.Fatal("Remove(e1) must succeed")
	}
	if s.Has(e1) {
		t.Fatal("e1 must be gone after Remove")
	}
	if s.Remove(e1) {
		t.Fatal("double Remove must return false")
	}
	if !s.Has(e2) {
		t.Fatal("e2 must remain after removing e1")
	}
}

func TestSparseSetSwapAndPopPreservesData(t *testing.T) {
	t.Parallel()

	s := NewSparseSet(newPositionSpec(t))
	entities := []entity.Entity{
		entity.NewEntity(10, 1),
		entity.NewEntity(20, 1),
		entity.NewEntity(30, 1),
	}
	values := []Position{{1, 1, 1}, {2, 2, 2}, {3, 3, 3}}

	for i, e := range entities {
		s.Add(e, values[i])
	}

	// Remove the first entity — last (30) gets swapped into slot 0.
	s.Remove(entities[0])

	for i := 1; i < 3; i++ {
		ptr, ok := s.Get(entities[i])
		if !ok {
			t.Fatalf("entity %d must still be present", entities[i].Index())
		}
		got := *(*Position)(ptr)
		if got != values[i] {
			t.Fatalf("entity %d data mismatch after swap-and-pop: got %+v, want %+v",
				entities[i].Index(), got, values[i])
		}
	}
}

func TestSparseSetOverwriteOnReadd(t *testing.T) {
	t.Parallel()

	s := NewSparseSet(newPositionSpec(t))
	e := entity.NewEntity(5, 1)
	s.Add(e, Position{1, 1, 1})
	s.Add(e, Position{9, 9, 9})

	if s.Len() != 1 {
		t.Fatalf("re-Add must overwrite, not append; Len() = %d", s.Len())
	}
	ptr, _ := s.Get(e)
	got := *(*Position)(ptr)
	if got != (Position{9, 9, 9}) {
		t.Fatalf("overwrite failed: got %+v, want {9,9,9}", got)
	}
}

func TestSparseSetRejectsNullEntity(t *testing.T) {
	t.Parallel()

	s := NewSparseSet(newPositionSpec(t))
	s.Add(entity.Entity{}, Position{})
	if s.Len() != 0 {
		t.Fatal("Add(null) must be a no-op")
	}
	if s.Has(entity.Entity{}) {
		t.Fatal("Has(null) must return false")
	}
	if _, ok := s.Get(entity.Entity{}); ok {
		t.Fatal("Get(null) must return false")
	}
}

func TestSparseSetTypeMismatchPanics(t *testing.T) {
	t.Parallel()

	s := NewSparseSet(newPositionSpec(t))
	defer func() {
		if recover() == nil {
			t.Fatal("type-mismatched Add must panic")
		}
	}()
	s.Add(entity.NewEntity(1, 1), Velocity{})
}

func TestSparseSetZeroSizedComponent(t *testing.T) {
	t.Parallel()

	s := NewSparseSet(newTagSpec(t))
	e1 := entity.NewEntity(1, 1)
	e2 := entity.NewEntity(2, 1)

	s.Add(e1, EnemyTag{})
	s.Add(e2, EnemyTag{})

	if s.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", s.Len())
	}
	ptr, ok := s.Get(e1)
	if !ok || ptr != nil {
		t.Fatalf("zero-size Get must return (nil, true); got (%v, %v)", ptr, ok)
	}

	// Type-check is bypassed for zero-size columns; any value accepted.
	s.Add(entity.NewEntity(3, 1), 42)

	s.Remove(e1)
	if s.Len() != 2 {
		t.Fatalf("Len() after Remove = %d, want 2", s.Len())
	}
}

func TestSparseSetIterCoversAllInDenseOrder(t *testing.T) {
	t.Parallel()

	s := NewSparseSet(newPositionSpec(t))
	for i := uint32(1); i <= 4; i++ {
		s.Add(entity.NewEntity(i, 1), Position{X: float32(i)})
	}
	visited := make([]uint32, 0, 4)
	values := make([]float32, 0, 4)
	s.Iter(func(e entity.Entity, p unsafe.Pointer) {
		visited = append(visited, e.Index())
		values = append(values, (*Position)(p).X)
	})
	if len(visited) != 4 {
		t.Fatalf("Iter visited %d, want 4", len(visited))
	}
	// Iteration order is dense-insertion order, but check via sort to be
	// resilient to swap-and-pop reordering after future Removes.
	slices.Sort(visited)
	for i, idx := range visited {
		if idx != uint32(i+1) {
			t.Fatalf("visited[%d] = %d, want %d", i, idx, i+1)
		}
	}
}

func TestSparseSetClear(t *testing.T) {
	t.Parallel()

	s := NewSparseSet(newPositionSpec(t))
	for i := uint32(1); i <= 5; i++ {
		s.Add(entity.NewEntity(i, 1), Position{})
	}
	s.Clear()
	if s.Len() != 0 {
		t.Fatalf("Len() after Clear = %d, want 0", s.Len())
	}
	for i := uint32(1); i <= 5; i++ {
		if s.Has(entity.NewEntity(i, 1)) {
			t.Fatalf("entity %d must be gone after Clear", i)
		}
	}
	// Reuse after Clear must work.
	s.Add(entity.NewEntity(7, 1), Position{X: 1})
	if s.Len() != 1 {
		t.Fatalf("Len() after re-add = %d, want 1", s.Len())
	}
}

func TestSparseSetSpecAccessor(t *testing.T) {
	t.Parallel()

	spec := newPositionSpec(t)
	s := NewSparseSet(spec)
	if s.Spec().Type != reflect.TypeFor[Position]() {
		t.Fatalf("Spec() type mismatch")
	}
}

func BenchmarkSparseSetAdd(b *testing.B) {
	b.StopTimer()
	r := NewRegistry()
	id := RegisterType[Position](r)
	spec := ColumnSpecFromInfo(r.Info(id))
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		s := NewSparseSet(spec)
		s.Add(entity.NewEntity(uint32(i+1), 1), Position{X: 1})
	}
}

func BenchmarkSparseSetGet(b *testing.B) {
	r := NewRegistry()
	id := RegisterType[Position](r)
	spec := ColumnSpecFromInfo(r.Info(id))
	s := NewSparseSet(spec)
	e := entity.NewEntity(1, 1)
	s.Add(e, Position{X: 1, Y: 2, Z: 3})

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = s.Get(e)
	}
}
