package typereg_test

import (
	"reflect"
	"testing"

	"github.com/neuengine/neu/internal/ecs/typereg"
)

// BenchmarkResolveByID measures dense-slice lookup. Pure index access; no
// allocations after warm-up.
func BenchmarkResolveByID(b *testing.B) {
	r := typereg.NewTypeRegistry()
	reg := typereg.RegisterType[position](r)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = r.ResolveByID(reg.ID)
	}
}

// BenchmarkResolveByType measures map[reflect.Type]*Reg lookup. The cost is
// dominated by Go's map hash; no allocations expected.
func BenchmarkResolveByType(b *testing.B) {
	r := typereg.NewTypeRegistry()
	typereg.RegisterType[position](r)
	t := reflect.TypeFor[position]()

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = r.Resolve(t)
	}
}

// BenchmarkResolveByName measures map[string]*Reg lookup with a fully
// qualified package name as key.
func BenchmarkResolveByName(b *testing.B) {
	r := typereg.NewTypeRegistry()
	reg := typereg.RegisterType[position](r)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = r.ResolveByName(reg.Name)
	}
}

// BenchmarkFieldByName measures the lazily-built name→index map on a 3-field
// struct after warm-up.
func BenchmarkFieldByName(b *testing.B) {
	r := typereg.NewTypeRegistry()
	reg := typereg.RegisterType[position](r)
	// Warm the lazy map.
	_ = reg.FieldByName("X")

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = reg.FieldByName("Y")
	}
}

// BenchmarkRegisterType is registration-time only — not on the hot path,
// but useful for tracking init-time scaling as the registry grows.
type benchType0 struct{ A, B, C int }
type benchType1 struct{ A, B, C int }
type benchType2 struct{ A, B, C int }
type benchType3 struct{ A, B, C int }

func BenchmarkRegisterFour(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		r := typereg.NewTypeRegistry()
		typereg.RegisterType[benchType0](r)
		typereg.RegisterType[benchType1](r)
		typereg.RegisterType[benchType2](r)
		typereg.RegisterType[benchType3](r)
	}
}
