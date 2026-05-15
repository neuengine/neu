package component

import (
	"reflect"
	"sync"
	"testing"
	"unsafe"
)

type Position struct{ X, Y, Z float32 }
type Velocity struct{ DX, DY, DZ float32 }
type Health struct{ HP int32 }
type EnemyTag struct{} // zero-size

func TestNewRegistryIsEmpty(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	if r.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", r.Len())
	}
	if id, ok := r.Lookup(reflect.TypeFor[Position]()); ok || id != 0 {
		t.Fatalf("Lookup on empty registry returned (%d, %v)", id, ok)
	}
}

func TestRegisterAssignsSequentialIDs(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	idP := RegisterType[Position](r)
	idV := RegisterType[Velocity](r)
	idH := RegisterType[Health](r)

	if idP != 1 || idV != 2 || idH != 3 {
		t.Fatalf("expected sequential IDs 1,2,3 — got %d,%d,%d", idP, idV, idH)
	}
	if r.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", r.Len())
	}
}

func TestRegisterIsIdempotent(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	a := RegisterType[Position](r)
	b := RegisterType[Position](r)

	if a != b {
		t.Fatalf("re-registering Position must yield same ID; got %d then %d", a, b)
	}
	if r.Len() != 1 {
		t.Fatalf("Len() = %d after duplicate register, want 1", r.Len())
	}
}

func TestRegisterPanicsOnNilType(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	defer func() {
		if recover() == nil {
			t.Fatal("Register(Info{Type:nil}) must panic")
		}
	}()
	r.Register(Info{Type: nil})
}

func TestRegisterPanicsOnNameCollision(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	r.Register(Info{Type: reflect.TypeFor[Position](), Name: "shared"})
	defer func() {
		if recover() == nil {
			t.Fatal("name collision must panic")
		}
	}()
	r.Register(Info{Type: reflect.TypeFor[Velocity](), Name: "shared"})
}

func TestInfoMetadataDerivedFromReflection(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	id := RegisterType[Position](r)
	info := r.Info(id)

	if info.ID != id {
		t.Fatalf("Info.ID = %d, want %d", info.ID, id)
	}
	if info.Type != reflect.TypeFor[Position]() {
		t.Fatalf("Info.Type mismatch")
	}
	if info.Size != unsafe.Sizeof(Position{}) {
		t.Fatalf("Info.Size = %d, want %d", info.Size, unsafe.Sizeof(Position{}))
	}
	if info.Storage != StorageTable {
		t.Fatalf("default storage must be StorageTable; got %d", info.Storage)
	}
	if info.Name == "" {
		t.Fatal("Info.Name must be populated by Register")
	}
}

func TestZeroSizedComponentRegisters(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	id := RegisterType[EnemyTag](r)
	info := r.Info(id)
	if !info.IsZeroSized() {
		t.Fatalf("EnemyTag must be zero-sized; size=%d", info.Size)
	}
}

func TestInfoPanicsOnInvalidID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		id   ID
	}{
		{"zero_sentinel", 0},
		{"out_of_range", 99},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := NewRegistry()
			RegisterType[Position](r)
			defer func() {
				if recover() == nil {
					t.Fatalf("Info(%d) must panic", tc.id)
				}
			}()
			_ = r.Info(tc.id)
		})
	}
}

func TestLookupByName(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	id := RegisterType[Position](r)
	info := r.Info(id)

	got, ok := r.LookupByName(info.Name)
	if !ok || got != id {
		t.Fatalf("LookupByName(%q) = (%d, %v), want (%d, true)", info.Name, got, ok, id)
	}
	if _, ok := r.LookupByName("nope"); ok {
		t.Fatal("LookupByName on absent name must return false")
	}
}

func TestEachVisitsAllInIDOrder(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	want := []ID{
		RegisterType[Position](r),
		RegisterType[Velocity](r),
		RegisterType[Health](r),
	}

	got := make([]ID, 0, len(want))
	r.Each(func(i *Info) bool {
		got = append(got, i.ID)
		return true
	})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Each visited %v, want %v", got, want)
	}
}

func TestEachStopsWhenCallbackReturnsFalse(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	RegisterType[Position](r)
	RegisterType[Velocity](r)

	count := 0
	r.Each(func(*Info) bool {
		count++
		return false
	})
	if count != 1 {
		t.Fatalf("Each must stop on first false; visited %d", count)
	}
}

func TestDeterministicOrderingAcrossRegistries(t *testing.T) {
	t.Parallel()

	// Two independent registries that receive identical registration
	// sequences must produce identical IDs — required for archetype hashing
	// (T-1C03).
	r1 := NewRegistry()
	r2 := NewRegistry()

	if RegisterType[Position](r1) != RegisterType[Position](r2) {
		t.Fatal("identical registration sequence must yield identical Position IDs")
	}
	if RegisterType[Velocity](r1) != RegisterType[Velocity](r2) {
		t.Fatal("identical registration sequence must yield identical Velocity IDs")
	}
}

func TestRegisterIgnoresCallerSuppliedID(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	id := r.Register(Info{ID: 999, Type: reflect.TypeFor[Position]()})
	if id != 1 {
		t.Fatalf("Register must override caller-supplied ID; got %d", id)
	}
}

func TestQualifiedTypeNameForAnonymous(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	id := r.Register(Info{Type: reflect.TypeFor[struct{ X int }]()})
	if r.Info(id).Name == "" {
		t.Fatal("anonymous type must still receive a non-empty Name")
	}
}

func TestConcurrentReadsAfterRegistrationAreSafe(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	idP := RegisterType[Position](r)
	idV := RegisterType[Velocity](r)

	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			if got, _ := r.Lookup(reflect.TypeFor[Position]()); got != idP {
				t.Errorf("concurrent Lookup(Position) = %d, want %d", got, idP)
			}
			if got, _ := r.Lookup(reflect.TypeFor[Velocity]()); got != idV {
				t.Errorf("concurrent Lookup(Velocity) = %d, want %d", got, idV)
			}
			if r.Len() != 2 {
				t.Errorf("concurrent Len = %d, want 2", r.Len())
			}
		})
	}
	wg.Wait()
}

func TestIDIsValid(t *testing.T) {
	t.Parallel()

	if ID(0).IsValid() {
		t.Fatal("ID(0) must be invalid")
	}
	if !ID(1).IsValid() {
		t.Fatal("ID(1) must be valid")
	}
}

func BenchmarkRegisterType(b *testing.B) {
	for i := 0; i < b.N; i++ {
		r := NewRegistry()
		_ = RegisterType[Position](r)
		_ = RegisterType[Velocity](r)
		_ = RegisterType[Health](r)
	}
}

func BenchmarkLookup(b *testing.B) {
	r := NewRegistry()
	_ = RegisterType[Position](r)
	t := reflect.TypeFor[Position]()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = r.Lookup(t)
	}
}

func FuzzRegisterOrderingIsDeterministic(f *testing.F) {
	f.Add(uint8(0b00000111))
	f.Add(uint8(0b10101010))
	f.Add(uint8(0xff))

	types := []reflect.Type{
		reflect.TypeFor[Position](),
		reflect.TypeFor[Velocity](),
		reflect.TypeFor[Health](),
		reflect.TypeFor[EnemyTag](),
	}

	f.Fuzz(func(t *testing.T, mask uint8) {
		// Build a registration order from the bitmask: include each type
		// only if its bit is set, in canonical index order.
		order := make([]reflect.Type, 0, len(types))
		for i, ty := range types {
			if mask&(1<<uint(i)) != 0 {
				order = append(order, ty)
			}
		}
		if len(order) == 0 {
			return
		}

		r1 := NewRegistry()
		r2 := NewRegistry()
		for _, ty := range order {
			r1.Register(Info{Type: ty})
			r2.Register(Info{Type: ty})
		}
		for _, ty := range order {
			id1, _ := r1.Lookup(ty)
			id2, _ := r2.Lookup(ty)
			if id1 != id2 {
				t.Fatalf("nondeterministic ID for %s: %d vs %d", ty, id1, id2)
			}
		}
	})
}
