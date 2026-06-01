package scene

import (
	"bytes"
	"strings"
	"testing"

	"github.com/neuengine/neu/pkg/asset"
	enginescene "github.com/neuengine/neu/pkg/scene"
)

// sampleScene builds a small valid SerializedScene: one entity with a Transform
// component carrying x=1, y=2 (interned).
func sampleScene() enginescene.SerializedScene {
	return enginescene.SerializedScene{
		Names:    []string{"game.Transform", "x", "y"},
		Variants: []any{float64(1), float64(2)},
		Entities: []enginescene.SerializedEntity{
			{NameIdx: 0, Components: []enginescene.SerializedComponent{
				{TypeIdx: 0, Props: [][2]int{{1, 0}, {2, 1}}},
			}},
		},
	}
}

func TestSceneRoundTripByteStable(t *testing.T) {
	t.Parallel()
	sc := sampleScene()
	wire1, err := enginescene.MarshalJSON(&sc)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	got, err := SceneJSONLoader{}.Load(bytes.NewReader(wire1), LoadSettings{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.EntityCount() != 1 {
		t.Fatalf("EntityCount = %d, want 1", got.EntityCount())
	}
	if ct := got.ComponentType(0, 0); ct != "game.Transform" {
		t.Errorf("ComponentType = %q, want game.Transform", ct)
	}
	if v := got.PropertyValue(0, 0, 1); v != float64(2) {
		t.Errorf("PropertyValue(y) = %v, want 2", v)
	}

	// decode → re-serialize must be byte-stable.
	wire2, err := enginescene.MarshalJSON(&got)
	if err != nil {
		t.Fatalf("re-MarshalJSON: %v", err)
	}
	if !bytes.Equal(wire1, wire2) {
		t.Errorf("round-trip not byte-stable:\nwire1=%s\nwire2=%s", wire1, wire2)
	}
}

func TestSceneDecodeErrors(t *testing.T) {
	t.Parallel()
	// Malformed JSON.
	if _, err := Decode(strings.NewReader("{not json")); err == nil {
		t.Error("malformed JSON should error")
	}

	// Out-of-range interning indices (validate, since UnmarshalJSON doesn't check).
	bad := []enginescene.SerializedScene{
		{Names: []string{"T"}, Entities: []enginescene.SerializedEntity{{NameIdx: 5}}},                                                                                                           // nameIdx OOR
		{Names: []string{"T"}, Entities: []enginescene.SerializedEntity{{NameIdx: 0, Components: []enginescene.SerializedComponent{{TypeIdx: 9}}}}},                                              // typeIdx OOR
		{Names: []string{"T"}, Variants: []any{1}, Entities: []enginescene.SerializedEntity{{NameIdx: 0, Components: []enginescene.SerializedComponent{{TypeIdx: 0, Props: [][2]int{{0, 7}}}}}}}, // valueIdx OOR
	}
	for i, sc := range bad {
		data, err := enginescene.MarshalJSON(&sc)
		if err != nil {
			t.Fatalf("case %d marshal: %v", i, err)
		}
		if _, err := Decode(bytes.NewReader(data)); err == nil {
			t.Errorf("case %d: out-of-range indices should error", i)
		}
	}
}

func TestSceneExtensions(t *testing.T) {
	t.Parallel()
	exts := SceneJSONLoader{}.Extensions()
	if len(exts) != 1 || exts[0] != ".scene.json" {
		t.Errorf("Extensions = %v, want [.scene.json]", exts)
	}
}

func TestSceneRegisterAll(t *testing.T) {
	t.Parallel()
	if err := RegisterAll(nil); err == nil {
		t.Error("RegisterAll(nil) should error")
	}
	srv := asset.NewAssetServer(asset.NewVFS(), nil)
	if err := RegisterAll(srv); err != nil {
		t.Errorf("RegisterAll(server) = %v", err)
	}
}
