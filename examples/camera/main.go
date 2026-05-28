// Command camera demonstrates multi-camera ordering via SortedActiveCameras:
// three cameras are spawned with Orders {2, 0, 1} and verified to sort
// deterministically to {0, 1, 2} regardless of insertion order (INV-4).
//
// Run:  go run ./examples/camera
// Test: go test ./examples/camera
package main

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"

	"github.com/neuengine/neu/internal/ecs/component"
	ecworld "github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/internal/render/cameraupd"
	"github.com/neuengine/neu/pkg/render/camera"
)

func buildCameraWorld() *ecworld.World {
	w := ecworld.NewWorld()
	ecworld.RegisterComponent[camera.Camera](w)
	return w
}

// run builds a 3-camera world and verifies deterministic ordering.
// Returns the FNV-1a hash of sorted entity IDs for stability testing.
func run() (uint64, error) {
	w := buildCameraWorld()

	// Spawn cameras in non-order: {2, 0, 1}.
	for _, order := range []int32{2, 0, 1} {
		w.Spawn(component.Data{Value: camera.Camera{Order: order, IsActive: true}})
	}
	// Inactive camera — must not appear in sorted result.
	w.Spawn(component.Data{Value: camera.Camera{Order: -1, IsActive: false}})

	sorted := cameraupd.SortedActiveCameras(w)
	if len(sorted) != 3 {
		return 0, fmt.Errorf("SortedActiveCameras: expected 3, got %d", len(sorted))
	}

	// Verify ascending Order.
	for i, e := range sorted {
		cam, ok := ecworld.Get[camera.Camera](w, e)
		if !ok {
			return 0, fmt.Errorf("camera[%d] not found", i)
		}
		if cam.Order != int32(i) {
			return 0, fmt.Errorf("camera[%d].Order = %d, want %d", i, cam.Order, i)
		}
	}

	// Hash entity IDs in sorted order.
	h := fnv.New64a()
	var buf [8]byte
	for _, e := range sorted {
		binary.LittleEndian.PutUint64(buf[:], uint64(e.ID()))
		h.Write(buf[:])
	}
	return h.Sum64(), nil
}

func main() {
	h1, err := run()
	if err != nil {
		panic(fmt.Sprintf("run: %v", err))
	}
	h2, err := run()
	if err != nil {
		panic(fmt.Sprintf("run (2nd): %v", err))
	}
	if h1 != h2 {
		panic(fmt.Sprintf("non-deterministic camera hash: %d != %d", h1, h2))
	}
	fmt.Println("PASS")
}
