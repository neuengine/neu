package typereg_test

import (
	"reflect"
	"testing"

	"github.com/teratron/boltengine/internal/ecs/typereg"
)

// ---- fixtures ---------------------------------------------------------------

type position struct {
	X, Y, Z float32
}

type velocity struct {
	DX, DY float32
}

type playerTag struct{}

type sparseHealth struct {
	_   struct{} `ecs:"storage:sparse"`
	HP  int      `editor:"label:Hit Points" range:"0,100"`
	Max int      `ecs:"ignore" editor:"hidden"`
}

type nestedRef struct {
	Pos position
	Vel velocity
}

type unexportedFields struct {
	Public  int
	private int //nolint:unused // intentional for test
}

// ---- TypeID -----------------------------------------------------------------

func TestTypeID_IsValid(t *testing.T) {
	t.Parallel()
	if typereg.TypeID(0).IsValid() {
		t.Fatal("TypeID(0) must be invalid")
	}
	if !typereg.TypeID(1).IsValid() {
		t.Fatal("TypeID(1) must be valid")
	}
}

// ---- registry basics --------------------------------------------------------

func TestNewTypeRegistry_EmptyButLenZero(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	if r.Len() != 0 {
		t.Fatalf("Len = %d, want 0", r.Len())
	}
	if r.ResolveByID(0) != nil {
		t.Fatal("ResolveByID(0) must yield nil sentinel")
	}
}

func TestRegisterType_AssignsID(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	a := typereg.RegisterType[position](r)
	b := typereg.RegisterType[velocity](r)
	if a.ID == 0 || b.ID == 0 {
		t.Fatal("registered types must have non-zero IDs")
	}
	if a.ID == b.ID {
		t.Fatalf("distinct types share ID %d", a.ID)
	}
	if r.Len() != 2 {
		t.Fatalf("Len = %d, want 2", r.Len())
	}
}

func TestRegisterType_Idempotent(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	a := typereg.RegisterType[position](r)
	b := typereg.RegisterType[position](r)
	if a != b {
		t.Fatal("re-registration must return the same registration")
	}
	if r.Len() != 1 {
		t.Fatalf("Len = %d, want 1 (idempotent)", r.Len())
	}
}

func TestRegisterByType_NonGenericPath(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	reg := r.RegisterByType(reflect.TypeFor[position]())
	if reg == nil || reg.Type != reflect.TypeFor[position]() {
		t.Fatal("RegisterByType must populate the registration")
	}
}

// ---- Resolve* ---------------------------------------------------------------

func TestResolve_RoundTrips(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	reg := typereg.RegisterType[position](r)

	if got := r.Resolve(reflect.TypeFor[position]()); got != reg {
		t.Fatal("Resolve(reflect.Type) must yield the registration")
	}
	if got := r.ResolveByName(reg.Name); got != reg {
		t.Fatal("ResolveByName must yield the registration")
	}
	if got := r.ResolveByID(reg.ID); got != reg {
		t.Fatal("ResolveByID must yield the registration")
	}
}

func TestResolve_UnknownReturnsNil(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	if got := r.Resolve(reflect.TypeFor[position]()); got != nil {
		t.Fatal("Resolve on unregistered type must return nil")
	}
	if got := r.ResolveByName("nonexistent"); got != nil {
		t.Fatal("ResolveByName on unknown name must return nil")
	}
	if got := r.ResolveByID(99); got != nil {
		t.Fatal("ResolveByID on out-of-range id must return nil")
	}
}

func TestMustResolve_PanicOnUnknown(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("MustResolve must panic on unregistered type")
		}
	}()
	r := typereg.NewTypeRegistry()
	_ = typereg.MustResolve[position](r)
}

func TestMustResolve_ReturnsRegistration(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	want := typereg.RegisterType[position](r)
	got := typereg.MustResolve[position](r)
	if got != want {
		t.Fatal("MustResolve must return the registered registration")
	}
}

// ---- TypeRegistration size/align --------------------------------------------

func TestRegistration_SizeAlign(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	reg := typereg.RegisterType[position](r)
	want := reflect.TypeFor[position]()
	if reg.Size != want.Size() {
		t.Fatalf("Size = %d, want %d", reg.Size, want.Size())
	}
	if reg.Align != uintptr(want.Align()) {
		t.Fatalf("Align = %d, want %d", reg.Align, want.Align())
	}
}

func TestRegistration_FieldByName(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	reg := typereg.RegisterType[position](r)
	x := reg.FieldByName("X")
	if x == nil || x.Name != "X" {
		t.Fatalf("FieldByName(X) = %v", x)
	}
	if missing := reg.FieldByName("nonexistent"); missing != nil {
		t.Fatal("FieldByName must return nil for unknown field")
	}
}

func TestRegistration_FieldByName_NoFields(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	reg := typereg.RegisterType[playerTag](r)
	if reg.FieldByName("Anything") != nil {
		t.Fatal("zero-field registration must yield nil for any name lookup")
	}
}

// ---- Field extraction -------------------------------------------------------

func TestExtractFields_PrimitiveStruct(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	reg := typereg.RegisterType[position](r)

	if len(reg.Fields) != 3 {
		t.Fatalf("len(Fields) = %d, want 3", len(reg.Fields))
	}
	for i, f := range reg.Fields {
		want := []string{"X", "Y", "Z"}[i]
		if f.Name != want {
			t.Fatalf("Fields[%d].Name = %q, want %q", i, f.Name, want)
		}
		if f.Index != i {
			t.Fatalf("Fields[%d].Index = %d, want %d", i, f.Index, i)
		}
		if !f.Exported {
			t.Fatalf("Fields[%d] must be exported", i)
		}
	}
}

func TestExtractFields_OffsetsMatchReflect(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	reg := typereg.RegisterType[position](r)
	rt := reflect.TypeFor[position]()
	for i, f := range reg.Fields {
		if f.Offset != rt.Field(i).Offset {
			t.Fatalf("Fields[%d].Offset = %d, want %d", i, f.Offset, rt.Field(i).Offset)
		}
	}
}

func TestExtractFields_SkipsMetaUnderscore(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	reg := typereg.RegisterType[sparseHealth](r)
	for _, f := range reg.Fields {
		if f.Name == "_" {
			t.Fatal("the meta `_` field must be skipped from FieldInfo")
		}
	}
}

func TestExtractFields_Unexported(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	reg := typereg.RegisterType[unexportedFields](r)
	if len(reg.Fields) != 2 {
		t.Fatalf("len(Fields) = %d, want 2 (both Public and private recorded)", len(reg.Fields))
	}
	for _, f := range reg.Fields {
		if f.Name == "Public" && !f.Exported {
			t.Fatal("Public must be marked exported")
		}
		if f.Name == "private" && f.Exported {
			t.Fatal("private must be marked unexported")
		}
	}
}

func TestExtractFields_NonStructIsEmpty(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	reg := typereg.RegisterType[int](r)
	if len(reg.Fields) != 0 {
		t.Fatal("non-struct types must yield no fields")
	}
}

// ---- Type tags --------------------------------------------------------------

func TestExtractTypeTags_StorageSparse(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	reg := typereg.RegisterType[sparseHealth](r)
	if reg.Tags.Storage != typereg.StorageSparseSet {
		t.Fatalf("Tags.Storage = %v, want sparse", reg.Tags.Storage)
	}
}

func TestExtractTypeTags_DefaultTable(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	reg := typereg.RegisterType[position](r)
	if reg.Tags.Storage != typereg.StorageTable {
		t.Fatalf("Tags.Storage = %v, want table", reg.Tags.Storage)
	}
}

// ---- Field tags -------------------------------------------------------------

func TestFieldTags_RangeAndLabel(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	reg := typereg.RegisterType[sparseHealth](r)
	hp := reg.FieldByName("HP")
	if hp == nil {
		t.Fatal("HP missing")
	}
	if !hp.Tags.HasRange || hp.Tags.RangeMin != 0 || hp.Tags.RangeMax != 100 {
		t.Fatalf("HP range tags = %+v", hp.Tags)
	}
	if hp.Tags.Label != "Hit Points" {
		t.Fatalf("HP label = %q, want %q", hp.Tags.Label, "Hit Points")
	}
}

func TestFieldTags_IgnoreAndHidden(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	reg := typereg.RegisterType[sparseHealth](r)
	mx := reg.FieldByName("Max")
	if mx == nil {
		t.Fatal("Max missing")
	}
	if !mx.Tags.Ignore {
		t.Fatal("Max must have Ignore=true")
	}
	if !mx.Tags.Hidden {
		t.Fatal("Max must have Hidden=true")
	}
}

func TestFieldTags_AllStorageAliases(t *testing.T) {
	t.Parallel()
	type aliasA struct {
		_ struct{} `ecs:"storage:table"`
		V int
	}
	type aliasB struct {
		_ struct{} `ecs:"storage:sparseset"`
		V int
	}
	type aliasC struct {
		_ struct{} `ecs:"storage:sparse_set"`
		V int
	}
	r := typereg.NewTypeRegistry()
	a := typereg.RegisterType[aliasA](r)
	b := typereg.RegisterType[aliasB](r)
	c := typereg.RegisterType[aliasC](r)
	if a.Tags.Storage != typereg.StorageTable {
		t.Fatal("storage:table must yield StorageTable")
	}
	if b.Tags.Storage != typereg.StorageSparseSet {
		t.Fatal("storage:sparseset must yield StorageSparseSet")
	}
	if c.Tags.Storage != typereg.StorageSparseSet {
		t.Fatal("storage:sparse_set must yield StorageSparseSet")
	}
}

func TestFieldTags_RangeMalformedIgnored(t *testing.T) {
	t.Parallel()
	type bad struct {
		A int `range:"foo"`
		B int `range:"1,abc"`
		C int `range:"1,2,3"`
	}
	r := typereg.NewTypeRegistry()
	reg := typereg.RegisterType[bad](r)
	for _, f := range reg.Fields {
		if f.Tags.HasRange {
			t.Fatalf("malformed range tag must be ignored on field %q", f.Name)
		}
	}
}

func TestFieldTags_ReadOnly(t *testing.T) {
	t.Parallel()
	type ro struct {
		Locked int `editor:"readonly"`
	}
	r := typereg.NewTypeRegistry()
	reg := typereg.RegisterType[ro](r)
	if !reg.Fields[0].Tags.ReadOnly {
		t.Fatal("readonly tag must produce Tags.ReadOnly=true")
	}
}

func TestFieldTags_RawPreserved(t *testing.T) {
	t.Parallel()
	type x struct {
		V int `custom:"foo" json:"v"`
	}
	r := typereg.NewTypeRegistry()
	reg := typereg.RegisterType[x](r)
	if reg.Fields[0].Tags.Raw == "" {
		t.Fatal("Raw must preserve the original tag")
	}
	if v, ok := reg.Fields[0].Tags.Raw.Lookup("custom"); !ok || v != "foo" {
		t.Fatal("Raw tag must remain reflect.StructTag-queryable")
	}
}

// ---- Late-bound TypeID ------------------------------------------------------

func TestField_TypeID_LateBoundOnPostRegistration(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	// Register the parent FIRST. nestedRef.Pos and .Vel are not yet known.
	parent := typereg.RegisterType[nestedRef](r)
	for _, f := range parent.Fields {
		if f.TypeID != 0 {
			t.Fatalf("field %q TypeID = %d before inner registration; want 0", f.Name, f.TypeID)
		}
	}
	// Register the inner types and rebind.
	pos := typereg.RegisterType[position](r)
	vel := typereg.RegisterType[velocity](r)
	r.BindFieldTypeIDs()

	if got := parent.FieldByName("Pos").TypeID; got != pos.ID {
		t.Fatalf("Pos.TypeID = %d, want %d", got, pos.ID)
	}
	if got := parent.FieldByName("Vel").TypeID; got != vel.ID {
		t.Fatalf("Vel.TypeID = %d, want %d", got, vel.ID)
	}
}

func TestField_TypeID_AutoBoundOnLaterRegistration(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	pos := typereg.RegisterType[position](r)
	parent := typereg.RegisterType[nestedRef](r)
	if got := parent.FieldByName("Pos").TypeID; got != pos.ID {
		t.Fatalf("Pos.TypeID = %d, want %d (registered before parent)", got, pos.ID)
	}
}

// ---- Name collisions --------------------------------------------------------

func TestTypeName_AnonymousFallsBackToString(t *testing.T) {
	t.Parallel()
	r := typereg.NewTypeRegistry()
	reg := typereg.RegisterType[struct{ V int }](r)
	if reg.Name == "" {
		t.Fatal("anonymous types must still have a non-empty Name")
	}
}

func TestStorageStrategy_String(t *testing.T) {
	t.Parallel()
	if got := typereg.StorageTable.String(); got != "table" {
		t.Fatalf("StorageTable.String() = %q, want \"table\"", got)
	}
	if got := typereg.StorageSparseSet.String(); got != "sparse" {
		t.Fatalf("StorageSparseSet.String() = %q, want \"sparse\"", got)
	}
}
