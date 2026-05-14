package typereg_test

import (
	"errors"
	"testing"

	"github.com/teratron/ecs-engine/internal/ecs/typereg"
)

// ---- fixtures ----------------------------------------------------------------

type playerStats struct {
	HP     int     `editor:"label:Hit Points" range:"0,100"`
	Speed  float32 `editor:"label:Speed"`
	Name   string
	Hidden bool `editor:"hidden"`
}

type emptyStruct struct{}

// ---- TypeHooks ---------------------------------------------------------------

func TestRegisterTypeWithHooks_StoresHooks(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()

	cloneCalled := false
	hooks := typereg.TypeHooks{
		Clone: func(src any) any {
			cloneCalled = true
			return src
		},
	}
	reg := typereg.RegisterTypeWithHooks[playerStats](r, hooks)
	if reg.Hooks.Clone == nil {
		t.Fatal("Clone hook must be stored on the TypeRegistration")
	}
	// Invoke the hook to verify it's the right function.
	reg.Hooks.Clone(nil)
	if !cloneCalled {
		t.Fatal("Clone hook was not called")
	}
}

func TestRegisterTypeWithHooks_Idempotent(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()

	hooks := typereg.TypeHooks{Default: func() any { return playerStats{} }}
	reg1 := typereg.RegisterTypeWithHooks[playerStats](r, hooks)
	reg2 := typereg.RegisterTypeWithHooks[playerStats](r, hooks)
	if reg1 != reg2 {
		t.Fatal("RegisterTypeWithHooks must be idempotent for the same type")
	}
}

// ---- SerializeValue ----------------------------------------------------------

func TestSerializeValue_WithHook_CallsHook(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()

	hooks := typereg.TypeHooks{
		Serialize: func(value any) ([]byte, error) {
			return []byte("ok"), nil
		},
	}
	typereg.RegisterTypeWithHooks[playerStats](r, hooks)

	data, err := r.SerializeValue(playerStats{HP: 10})
	if err != nil {
		t.Fatalf("SerializeValue: %v", err)
	}
	if string(data) != "ok" {
		t.Fatalf("expected %q, got %q", "ok", data)
	}
}

func TestSerializeValue_NoHook_ReturnsError(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	typereg.RegisterType[playerStats](r)

	_, err := r.SerializeValue(playerStats{})
	if err == nil {
		t.Fatal("SerializeValue without hook must return an error")
	}
}

func TestSerializeValue_UnregisteredType_ReturnsError(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()

	_, err := r.SerializeValue(playerStats{})
	if err == nil {
		t.Fatal("SerializeValue for unregistered type must return an error")
	}
	if !errors.Is(err, typereg.ErrTypeNotRegistered) {
		t.Fatalf("expected ErrTypeNotRegistered, got %v", err)
	}
}

func TestSerializeValue_Nil_ReturnsError(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()

	_, err := r.SerializeValue(nil)
	if err == nil {
		t.Fatal("SerializeValue(nil) must return an error")
	}
}

// ---- DeserializeValue --------------------------------------------------------

func TestDeserializeValue_WithHook_CallsHook(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()

	hooks := typereg.TypeHooks{
		Deserialize: func(data []byte) (any, error) {
			return playerStats{HP: int(data[0])}, nil
		},
	}
	reg := typereg.RegisterTypeWithHooks[playerStats](r, hooks)

	got, err := r.DeserializeValue(reg.ID, []byte{42})
	if err != nil {
		t.Fatalf("DeserializeValue: %v", err)
	}
	ps, ok := got.(playerStats)
	if !ok || ps.HP != 42 {
		t.Fatalf("unexpected result: %v", got)
	}
}

func TestDeserializeValue_NoHook_ReturnsError(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	reg := typereg.RegisterType[playerStats](r)

	_, err := r.DeserializeValue(reg.ID, []byte{})
	if err == nil {
		t.Fatal("DeserializeValue without hook must return an error")
	}
}

func TestDeserializeValue_UnknownID_ReturnsError(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()

	_, err := r.DeserializeValue(typereg.TypeID(999), nil)
	if err == nil {
		t.Fatal("DeserializeValue for unknown TypeID must return an error")
	}
	if !errors.Is(err, typereg.ErrTypeNotRegistered) {
		t.Fatalf("expected ErrTypeNotRegistered, got %v", err)
	}
}

// ---- NewDynamicObject --------------------------------------------------------

func TestNewDynamicObject_Success(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	typereg.RegisterType[playerStats](r)

	ps := &playerStats{HP: 5}
	d, err := typereg.NewDynamicObject(r, ps)
	if err != nil {
		t.Fatalf("NewDynamicObject: %v", err)
	}
	if d == nil {
		t.Fatal("DynamicObject must not be nil")
	}
}

func TestNewDynamicObject_Nil_ReturnsError(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	_, err := typereg.NewDynamicObject(r, nil)
	if !errors.Is(err, typereg.ErrNotPointer) {
		t.Fatalf("expected ErrNotPointer, got %v", err)
	}
}

func TestNewDynamicObject_NonPointer_ReturnsError(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	typereg.RegisterType[playerStats](r)

	_, err := typereg.NewDynamicObject(r, playerStats{}) // value, not pointer
	if !errors.Is(err, typereg.ErrNotPointer) {
		t.Fatalf("expected ErrNotPointer, got %v", err)
	}
}

func TestNewDynamicObject_UnregisteredType_ReturnsError(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()

	ps := &playerStats{HP: 1}
	_, err := typereg.NewDynamicObject(r, ps)
	if !errors.Is(err, typereg.ErrTypeNotRegistered) {
		t.Fatalf("expected ErrTypeNotRegistered, got %v", err)
	}
}

// ---- NewDynamicObjectByID ----------------------------------------------------

func TestNewDynamicObjectByID_Success(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	reg := typereg.RegisterType[playerStats](r)

	d, err := typereg.NewDynamicObjectByID(r, reg.ID)
	if err != nil {
		t.Fatalf("NewDynamicObjectByID: %v", err)
	}
	if d.TypeID() != reg.ID {
		t.Fatalf("TypeID = %v, want %v", d.TypeID(), reg.ID)
	}
}

func TestNewDynamicObjectByID_UnknownID_ReturnsError(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()

	_, err := typereg.NewDynamicObjectByID(r, typereg.TypeID(999))
	if !errors.Is(err, typereg.ErrTypeNotRegistered) {
		t.Fatalf("expected ErrTypeNotRegistered, got %v", err)
	}
}

func TestNewDynamicObjectByID_ZeroValues(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	reg := typereg.RegisterType[playerStats](r)

	d, err := typereg.NewDynamicObjectByID(r, reg.ID)
	if err != nil {
		t.Fatalf("NewDynamicObjectByID: %v", err)
	}
	val, err := d.Get("HP")
	if err != nil {
		t.Fatalf("Get HP: %v", err)
	}
	if val.(int) != 0 {
		t.Fatalf("expected zero HP, got %v", val)
	}
}

// ---- DynamicObject.Get / Set -------------------------------------------------

func TestDynamicObject_GetSet_RoundTrip(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	typereg.RegisterType[playerStats](r)

	ps := &playerStats{HP: 10, Speed: 1.5, Name: "hero"}
	d, _ := typereg.NewDynamicObject(r, ps)

	// Get
	hp, err := d.Get("HP")
	if err != nil {
		t.Fatalf("Get HP: %v", err)
	}
	if hp.(int) != 10 {
		t.Fatalf("HP = %v, want 10", hp)
	}

	// Set
	if err := d.Set("HP", 99); err != nil {
		t.Fatalf("Set HP: %v", err)
	}
	if ps.HP != 99 {
		t.Fatalf("underlying struct HP = %d, want 99", ps.HP)
	}
}

func TestDynamicObject_Get_UnknownField_ReturnsError(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	typereg.RegisterType[playerStats](r)

	d, _ := typereg.NewDynamicObject(r, &playerStats{})
	_, err := d.Get("Nonexistent")
	if !errors.Is(err, typereg.ErrFieldNotFound) {
		t.Fatalf("expected ErrFieldNotFound, got %v", err)
	}
}

func TestDynamicObject_Set_UnknownField_ReturnsError(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	typereg.RegisterType[playerStats](r)

	d, _ := typereg.NewDynamicObject(r, &playerStats{})
	err := d.Set("Nonexistent", 1)
	if !errors.Is(err, typereg.ErrFieldNotFound) {
		t.Fatalf("expected ErrFieldNotFound, got %v", err)
	}
}

func TestDynamicObject_Set_TypeMismatch_ReturnsError(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	typereg.RegisterType[playerStats](r)

	d, _ := typereg.NewDynamicObject(r, &playerStats{})
	err := d.Set("HP", "not an int") // HP is int, value is string
	if !errors.Is(err, typereg.ErrFieldTypeMismatch) {
		t.Fatalf("expected ErrFieldTypeMismatch, got %v", err)
	}
}

func TestDynamicObject_Set_AllFieldTypes(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	typereg.RegisterType[playerStats](r)

	ps := &playerStats{}
	d, _ := typereg.NewDynamicObject(r, ps)

	tests := []struct {
		field string
		value any
	}{
		{"HP", 42},
		{"Speed", float32(3.14)},
		{"Name", "warrior"},
		{"Hidden", true},
	}
	for _, tc := range tests {
		if err := d.Set(tc.field, tc.value); err != nil {
			t.Fatalf("Set %q: %v", tc.field, err)
		}
	}
	if ps.HP != 42 || ps.Speed != 3.14 || ps.Name != "warrior" || !ps.Hidden {
		t.Fatalf("unexpected values after Set: %+v", ps)
	}
}

// ---- DynamicObject.Fields ----------------------------------------------------

func TestDynamicObject_Fields_IteratesAll(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	typereg.RegisterType[playerStats](r)

	ps := &playerStats{HP: 7, Speed: 2.0, Name: "mage"}
	d, _ := typereg.NewDynamicObject(r, ps)

	names := make(map[string]bool)
	for fi, _ := range d.Fields() {
		names[fi.Name] = true
	}
	for _, want := range []string{"HP", "Speed", "Name", "Hidden"} {
		if !names[want] {
			t.Fatalf("Fields() did not yield field %q", want)
		}
	}
}

func TestDynamicObject_Fields_EarlyBreak(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	typereg.RegisterType[playerStats](r)

	d, _ := typereg.NewDynamicObject(r, &playerStats{})
	count := 0
	for range d.Fields() {
		count++
		break // early exit
	}
	if count != 1 {
		t.Fatalf("expected 1 iteration with early break, got %d", count)
	}
}

// ---- DynamicObject.Value / TypeID / Registration ----------------------------

func TestDynamicObject_Value_ReturnsPointer(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	typereg.RegisterType[playerStats](r)

	ps := &playerStats{HP: 3}
	d, _ := typereg.NewDynamicObject(r, ps)

	v := d.Value()
	got, ok := v.(*playerStats)
	if !ok || got != ps {
		t.Fatal("Value() must return the original pointer")
	}
}

func TestDynamicObject_Registration(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	reg := typereg.RegisterType[playerStats](r)

	d, _ := typereg.NewDynamicObject(r, &playerStats{})
	if d.Registration() != reg {
		t.Fatal("Registration() must return the same *TypeRegistration as RegisterType")
	}
}

// ---- Benchmark ---------------------------------------------------------------

func BenchmarkDynamicObjectGet(b *testing.B) {
	r := typereg.NewTypeRegistry()
	typereg.RegisterType[playerStats](r)
	ps := &playerStats{HP: 1}
	d, _ := typereg.NewDynamicObject(r, ps)
	b.ResetTimer()
	for range b.N {
		_, _ = d.Get("HP")
	}
}

func BenchmarkDynamicObjectSet(b *testing.B) {
	r := typereg.NewTypeRegistry()
	typereg.RegisterType[playerStats](r)
	ps := &playerStats{}
	d, _ := typereg.NewDynamicObject(r, ps)
	b.ResetTimer()
	for range b.N {
		_ = d.Set("HP", 42)
	}
}

func BenchmarkDynamicObjectFieldIteration(b *testing.B) {
	r := typereg.NewTypeRegistry()
	typereg.RegisterType[playerStats](r)
	ps := &playerStats{HP: 1, Speed: 1.0}
	d, _ := typereg.NewDynamicObject(r, ps)
	b.ResetTimer()
	for range b.N {
		for range d.Fields() {
		}
	}
}

func BenchmarkNewDynamicObjectByID(b *testing.B) {
	r := typereg.NewTypeRegistry()
	reg := typereg.RegisterType[playerStats](r)
	b.ResetTimer()
	for range b.N {
		_, _ = typereg.NewDynamicObjectByID(r, reg.ID)
	}
}
