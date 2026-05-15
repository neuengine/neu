package component

import (
	"reflect"
	"testing"
)

// flatPair returns Position + Velocity, no nesting.
type flatPair struct {
	registry *Registry
}

func (b flatPair) Components() []Data {
	return []Data{
		NewData(b.registry, Position{X: 1}),
		NewData(b.registry, Velocity{DX: 2}),
	}
}

// nestedBundle wraps two flatPair-like emitters and returns their union.
type nestedBundle struct {
	registry *Registry
}

func (n nestedBundle) Components() []Data {
	inner := flatPair(n)
	out := []Data{NewData(n.registry, Health{HP: 99})}
	out = append(out, inner.Components()...)
	return out
}

func TestDataIsValid(t *testing.T) {
	t.Parallel()

	if (Data{}).IsValid() {
		t.Fatal("zero Data must be invalid")
	}
	if !(Data{ID: 1}).IsValid() {
		t.Fatal("Data with non-zero ID must be valid")
	}
}

func TestNewDataAutoRegistersType(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	d := NewData(r, Position{X: 5, Y: 6, Z: 7})
	if !d.IsValid() {
		t.Fatalf("NewData must return a valid Data; got %+v", d)
	}
	got, ok := r.Lookup(reflect.TypeFor[Position]())
	if !ok || got != d.ID {
		t.Fatalf("NewData must register Position; lookup=(%d,%v), data.ID=%d", got, ok, d.ID)
	}
	if v, ok := d.Value.(Position); !ok || v.X != 5 {
		t.Fatalf("Data.Value lost payload; got %+v", d.Value)
	}
}

func TestFlattenBundleFlatPair(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	got := FlattenBundle(flatPair{registry: r})
	if len(got) != 2 {
		t.Fatalf("flat bundle must yield 2 entries; got %d", len(got))
	}
	if !got[0].IsValid() || !got[1].IsValid() {
		t.Fatalf("entries must be valid; got %+v", got)
	}
}

func TestFlattenBundleNested(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	got := FlattenBundle(nestedBundle{registry: r})
	if len(got) != 3 {
		t.Fatalf("nested bundle must yield 3 entries (Health + Position + Velocity); got %d", len(got))
	}
	// First entry is Health (emitted before the nested call).
	if v, ok := got[0].Value.(Health); !ok || v.HP != 99 {
		t.Fatalf("first entry must be Health{HP:99}; got %+v", got[0])
	}
}

func TestFlattenBundleNilSafe(t *testing.T) {
	t.Parallel()

	if got := FlattenBundle(nil); got != nil {
		t.Fatalf("FlattenBundle(nil) must return nil; got %v", got)
	}
}

type pointerBundle struct{ registry *Registry }

func (p *pointerBundle) Components() []Data {
	return []Data{NewData(p.registry, Position{X: 1})}
}

func TestFlattenBundlePointerReceiver(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	got := FlattenBundle(&pointerBundle{registry: r})
	if len(got) != 1 {
		t.Fatalf("pointer-bundle flatten = %d entries, want 1", len(got))
	}
}
