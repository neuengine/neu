package scene

import (
	"encoding/json"
	"testing"
)

// ─── StaticScene ─────────────────────────────────────────────────────────────

type testPayload struct {
	Name   string
	Values []int
}

func TestStaticSceneRoundtrip(t *testing.T) {
	original := testPayload{Name: "level-1", Values: []int{1, 2, 3, 42}}

	sc, err := Capture(original)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}

	var got testPayload
	if err := sc.Restore(&got); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if got.Name != original.Name {
		t.Errorf("Name: got %q want %q", got.Name, original.Name)
	}
	if len(got.Values) != len(original.Values) {
		t.Errorf("Values len: got %d want %d", len(got.Values), len(original.Values))
	}
	for i, v := range original.Values {
		if got.Values[i] != v {
			t.Errorf("Values[%d]: got %d want %d", i, got.Values[i], v)
		}
	}
}

func TestStaticSceneMarshalUnmarshalRoundtrip(t *testing.T) {
	original := testPayload{Name: "checkpoint-42", Values: []int{7, 8, 9}}

	sc, err := Capture(original)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	data, err := MarshalStatic(sc)
	if err != nil {
		t.Fatalf("MarshalStatic: %v", err)
	}
	sc2, err := UnmarshalStatic(data)
	if err != nil {
		t.Fatalf("UnmarshalStatic: %v", err)
	}
	var got testPayload
	if err := sc2.Restore(&got); err != nil {
		t.Fatalf("Restore after marshal: %v", err)
	}
	if got.Name != original.Name || len(got.Values) != len(original.Values) {
		t.Errorf("marshal roundtrip mismatch: got %+v want %+v", got, original)
	}
}

func TestStaticSceneVersionMismatch(t *testing.T) {
	original := testPayload{Name: "mismatch-test"}
	sc, err := Capture(original)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	// Corrupt the build hash by temporarily replacing the global.
	saved := buildHash
	buildHash = buildHash ^ 0xDEADBEEF
	defer func() { buildHash = saved }()

	var got testPayload
	err = sc.Restore(&got)
	if err == nil {
		t.Fatal("Restore must fail when build hash mismatches")
	}
}

// ─── SerializedScene query API ───────────────────────────────────────────────

func makeTestScene() SerializedScene {
	return SerializedScene{
		Names:    []string{"Transform", "Velocity", "x", "y", "z", "dx", "dy", "dz"},
		Variants: []any{float64(0), float64(1), float64(-1)},
		Entities: []SerializedEntity{
			{
				NameIdx: 0, // entity type = "Transform"
				Components: []SerializedComponent{
					{TypeIdx: 0, Props: [][2]int{{2, 0}, {3, 0}, {4, 1}}}, // Transform: x=0 y=0 z=1
					{TypeIdx: 1, Props: [][2]int{{5, 1}, {6, 0}, {7, 2}}}, // Velocity: dx=1 dy=0 dz=-1
				},
			},
		},
	}
}

func TestSerializedSceneEntityCount(t *testing.T) {
	sc := makeTestScene()
	if sc.EntityCount() != 1 {
		t.Fatalf("EntityCount: got %d want 1", sc.EntityCount())
	}
}

func TestSerializedSceneComponentType(t *testing.T) {
	sc := makeTestScene()
	if got := sc.ComponentType(0, 0); got != "Transform" {
		t.Errorf("ComponentType(0,0): got %q want %q", got, "Transform")
	}
	if got := sc.ComponentType(0, 1); got != "Velocity" {
		t.Errorf("ComponentType(0,1): got %q want %q", got, "Velocity")
	}
}

func TestSerializedScenePropertyValue(t *testing.T) {
	sc := makeTestScene()
	v := sc.PropertyValue(0, 0, 2) // Transform.z = Variants[1] = 1.0
	if v != float64(1) {
		t.Errorf("PropertyValue(0,0,2): got %v want 1.0", v)
	}
}

func TestSerializedSceneOutOfBounds(t *testing.T) {
	sc := makeTestScene()
	if sc.ComponentType(-1, 0) != "" {
		t.Error("ComponentType(-1,0) must return empty string")
	}
	if sc.PropertyValue(0, 0, 100) != nil {
		t.Error("PropertyValue out-of-range must return nil")
	}
}

// ─── Interned binary codec ───────────────────────────────────────────────────

func TestBinaryRoundtrip(t *testing.T) {
	original := makeTestScene()
	encoded, err := MarshalBinary(&original)
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	got, err := UnmarshalBinary(encoded)
	if err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
	// Structural equality checks.
	if len(got.Names) != len(original.Names) {
		t.Errorf("Names len: got %d want %d", len(got.Names), len(original.Names))
	}
	for i, n := range original.Names {
		if got.Names[i] != n {
			t.Errorf("Names[%d]: got %q want %q", i, got.Names[i], n)
		}
	}
	if len(got.Entities) != len(original.Entities) {
		t.Errorf("Entities len: got %d want %d", len(got.Entities), len(original.Entities))
	}
	if len(got.Entities) > 0 {
		oe, ge := original.Entities[0], got.Entities[0]
		if ge.NameIdx != oe.NameIdx {
			t.Errorf("Entity[0].NameIdx: got %d want %d", ge.NameIdx, oe.NameIdx)
		}
		for j, oc := range oe.Components {
			gc := ge.Components[j]
			if gc.TypeIdx != oc.TypeIdx {
				t.Errorf("Entity[0].Comp[%d].TypeIdx: got %d want %d", j, gc.TypeIdx, oc.TypeIdx)
			}
			for k, op := range oc.Props {
				gp := gc.Props[k]
				if gp != op {
					t.Errorf("Entity[0].Comp[%d].Props[%d]: got %v want %v", j, k, gp, op)
				}
			}
		}
	}
}

// TestInternedSmallerThanNaive verifies the size invariant on a 1000-entity
// golden fixture: interned binary must be smaller than naive JSON (where each
// entity repeats the component-type strings in full).
func TestInternedSmallerThanNaive(t *testing.T) {
	sc := build1kScene()

	interned, err := MarshalBinary(&sc)
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	// Naive = JSON where each entity encodes component names inline.
	type naiveComp struct {
		TypeName string   `json:"typeName"`
		Props    [][2]any `json:"props"`
	}
	type naiveEnt struct {
		EntityName string      `json:"entityName"`
		Components []naiveComp `json:"components"`
	}
	type naiveScene struct {
		Entities []naiveEnt `json:"entities"`
	}
	ns := naiveScene{Entities: make([]naiveEnt, len(sc.Entities))}
	for i, ent := range sc.Entities {
		name := ""
		if ent.NameIdx < len(sc.Names) {
			name = sc.Names[ent.NameIdx]
		}
		comps := make([]naiveComp, len(ent.Components))
		for j, comp := range ent.Components {
			typeName := ""
			if comp.TypeIdx < len(sc.Names) {
				typeName = sc.Names[comp.TypeIdx]
			}
			props := make([][2]any, len(comp.Props))
			for k, p := range comp.Props {
				pName := ""
				if p[0] < len(sc.Names) {
					pName = sc.Names[p[0]]
				}
				var pVal any
				if p[1] < len(sc.Variants) {
					pVal = sc.Variants[p[1]]
				}
				props[k] = [2]any{pName, pVal}
			}
			comps[j] = naiveComp{TypeName: typeName, Props: props}
		}
		ns.Entities[i] = naiveEnt{EntityName: name, Components: comps}
	}
	naive, err := json.Marshal(ns)
	if err != nil {
		t.Fatalf("naive JSON: %v", err)
	}

	t.Logf("interned binary: %d bytes, naive JSON: %d bytes (ratio %.2f×)",
		len(interned), len(naive), float64(len(naive))/float64(len(interned)))

	if len(interned) >= len(naive) {
		t.Errorf("interned binary (%d B) must be smaller than naive JSON (%d B)",
			len(interned), len(naive))
	}
}

// build1kScene creates a 1000-entity scene where every entity has
// "Transform" and "Velocity" components — heavy string repetition in the naive
// format, fully interned in ours.
func build1kScene() SerializedScene {
	names := []string{"Entity", "Transform", "Velocity", "x", "y", "z", "dx", "dy", "dz"}
	variants := []any{float64(0), float64(1), float64(-1), float64(0.5)}

	entities := make([]SerializedEntity, 1000)
	for i := range entities {
		entities[i] = SerializedEntity{
			NameIdx: 0, // "Entity"
			Components: []SerializedComponent{
				{
					TypeIdx: 1, // "Transform"
					Props: [][2]int{
						{3, 0}, // x=0
						{4, 1}, // y=1
						{5, 3}, // z=0.5
					},
				},
				{
					TypeIdx: 2, // "Velocity"
					Props: [][2]int{
						{6, 2}, // dx=-1
						{7, 0}, // dy=0
						{8, 1}, // dz=1
					},
				},
			},
		}
	}
	return SerializedScene{Names: names, Variants: variants, Entities: entities}
}

func TestBinaryBadMagic(t *testing.T) {
	_, err := UnmarshalBinary([]byte("BADM\x01\x00"))
	if err == nil {
		t.Fatal("bad magic must return error")
	}
}

func TestBinaryFutureVersion(t *testing.T) {
	// Craft a buffer with version 99.
	data, _ := MarshalBinary(&SerializedScene{})
	// Bytes 4-5 are the version (little-endian uint16).
	data[4] = 99
	data[5] = 0
	_, err := UnmarshalBinary(data)
	if err == nil {
		t.Fatal("future version must return error")
	}
}

// ─── JSON codec ──────────────────────────────────────────────────────────────

func TestJSONRoundtrip(t *testing.T) {
	original := makeTestScene()
	data, err := MarshalJSON(&original)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	got, err := UnmarshalJSON(data)
	if err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
	if len(got.Names) != len(original.Names) {
		t.Errorf("JSON roundtrip Names len: got %d want %d", len(got.Names), len(original.Names))
	}
	if got.ComponentType(0, 0) != "Transform" {
		t.Errorf("JSON roundtrip ComponentType: got %q", got.ComponentType(0, 0))
	}
}
