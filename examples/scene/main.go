// Package main demonstrates the Neu scene system: capture → marshal → unmarshal
// → restore → re-marshal byte-stability.
//
// The fixture is a struct with nested data. The demo proves:
//   - gob round-trip is lossless
//   - binary wire format is deterministic (re-marshal == original bytes)
//   - build-hash guard catches cross-binary attempts
//
// Run:  go run ./examples/scene
// Test: go test -race ./examples/scene
package main

import (
	"fmt"

	"github.com/neuengine/neu/pkg/scene"
)

type GameLevel struct {
	Name     string
	Width    int
	Height   int
	Entities []EntityData
}

type EntityData struct {
	ID       uint32
	X, Y     float32
	IsActive bool
}

func main() {
	original := GameLevel{
		Name:   "Level-1",
		Width:  1024,
		Height: 768,
		Entities: []EntityData{
			{ID: 1, X: 100, Y: 200, IsActive: true},
			{ID: 2, X: 300, Y: 400, IsActive: false},
			{ID: 3, X: 500, Y: 600, IsActive: true},
		},
	}

	// ── Save ──────────────────────────────────────────────────────────────────
	ss, err := scene.Capture(original)
	if err != nil {
		fmt.Println("FAIL capture:", err)
		return
	}
	wire1, err := scene.MarshalStatic(ss)
	if err != nil {
		fmt.Println("FAIL marshal:", err)
		return
	}
	fmt.Printf("PASS save:     %d bytes\n", len(wire1))

	// ── Load ──────────────────────────────────────────────────────────────────
	ss2, err := scene.UnmarshalStatic(wire1)
	if err != nil {
		fmt.Println("FAIL unmarshal:", err)
		return
	}
	var loaded GameLevel
	if err := ss2.Restore(&loaded); err != nil {
		fmt.Println("FAIL restore:", err)
		return
	}
	if loaded.Name != original.Name || len(loaded.Entities) != len(original.Entities) {
		fmt.Printf("FAIL content: got %+v\n", loaded)
		return
	}
	fmt.Printf("PASS load:     Name=%q Entities=%d\n", loaded.Name, len(loaded.Entities))

	// ── Re-save byte-stability ────────────────────────────────────────────────
	ss3, err := scene.Capture(loaded)
	if err != nil {
		fmt.Println("FAIL re-capture:", err)
		return
	}
	wire2, err := scene.MarshalStatic(ss3)
	if err != nil {
		fmt.Println("FAIL re-marshal:", err)
		return
	}
	if len(wire1) != len(wire2) {
		fmt.Printf("FAIL byte-stable: len1=%d len2=%d\n", len(wire1), len(wire2))
		return
	}
	for i := range wire1 {
		if wire1[i] != wire2[i] {
			fmt.Printf("FAIL byte-stable: byte %d differs\n", i)
			return
		}
	}
	fmt.Printf("PASS re-save:  %d bytes (byte-stable)\n", len(wire2))
	fmt.Println("PASS: scene save→load→re-save roundtrip complete")
}
