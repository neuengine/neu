package main

import (
	"testing"

	"github.com/neuengine/neu/pkg/scene"
)

func TestSceneSaveLoadByteStable(t *testing.T) {
	orig := GameLevel{
		Name: "test", Width: 800, Height: 600,
		Entities: []EntityData{{ID: 42, X: 1, Y: 2, IsActive: true}},
	}

	ss, err := scene.Capture(orig)
	if err != nil {
		t.Fatal("capture:", err)
	}
	wire1, err := scene.MarshalStatic(ss)
	if err != nil {
		t.Fatal("marshal:", err)
	}

	ss2, err := scene.UnmarshalStatic(wire1)
	if err != nil {
		t.Fatal("unmarshal:", err)
	}
	var loaded GameLevel
	if err := ss2.Restore(&loaded); err != nil {
		t.Fatal("restore:", err)
	}
	if loaded.Name != orig.Name || len(loaded.Entities) != len(orig.Entities) {
		t.Fatalf("content mismatch: got %+v", loaded)
	}

	ss3, _ := scene.Capture(loaded)
	wire2, _ := scene.MarshalStatic(ss3)
	if string(wire1) != string(wire2) {
		t.Error("save→load→re-save is not byte-stable")
	}
}
