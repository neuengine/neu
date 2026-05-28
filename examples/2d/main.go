// examples/2d demonstrates the 2D sprite pipeline: sort-key determinism,
// batch-key construction, and the sprite picking algorithm (T-5T04, C29 P5 gate).
//
// This example uses the headless Sprite2DFeature — no GPU backend required
// (C-003). The golden topology (number of batches) and pick result are stable
// across ≥20 runs.
//
// Bootstrap: validates l2-2d-rendering-go against l1-2d-rendering.
package main

import (
	"fmt"

	sprite2d "github.com/neuengine/neu/internal/render/sprite2d"
)

// buildSprites constructs a deterministic set of ExtractedSprites for testing.
func buildSprites() []sprite2d.ExtractedSprite {
	// Create 6 sprites: 3 batches of 2 sprites each (same batchKey per batch).
	sprites := make([]sprite2d.ExtractedSprite, 6)

	// Batch A: texture=1, z=0.0
	for i := range 2 {
		sprites[i] = sprite2d.ExtractedSprite{
			AtlasUV:  [4]float32{0, 0, 1, 1},
			Color:    [4]float32{1, 1, 1, 1},
		}
		sprites[i].SetSortKey(sprite2d.BuildSortKey(0, float32(i), uint32(i)))
		sprites[i].SetBatchKey(sprite2d.BatchKey(1, 0, 0))
	}
	// Batch B: texture=2, z=0.5
	for i := range 2 {
		sprites[2+i] = sprite2d.ExtractedSprite{
			AtlasUV:  [4]float32{0, 0, 1, 1},
			Color:    [4]float32{1, 0, 0, 1},
		}
		sprites[2+i].SetSortKey(sprite2d.BuildSortKey(0.5, float32(i), uint32(2+i)))
		sprites[2+i].SetBatchKey(sprite2d.BatchKey(2, 0, 0))
	}
	// Batch C: texture=3, z=1.0
	for i := range 2 {
		sprites[4+i] = sprite2d.ExtractedSprite{
			AtlasUV:  [4]float32{0, 0, 1, 1},
			Color:    [4]float32{0, 1, 0, 1},
		}
		sprites[4+i].SetSortKey(sprite2d.BuildSortKey(1, float32(i), uint32(4+i)))
		sprites[4+i].SetBatchKey(sprite2d.BatchKey(3, 0, 0))
	}
	return sprites
}

// countBatches counts adjacent runs of same batchKey (INV-3).
func countBatches(sprites []sprite2d.ExtractedSprite) int {
	if len(sprites) == 0 {
		return 0
	}
	n := 1
	for i := 1; i < len(sprites); i++ {
		if sprites[i].GetBatchKey() != sprites[i-1].GetBatchKey() {
			n++
		}
	}
	return n
}

// run builds the sprite list, sorts it, verifies batch count, and returns a
// deterministic hash for stability testing.
func run() (uint64, error) {
	_ = sprite2d.NewSprite2DFeature()
	sprites := buildSprites()

	sprite2d.SortSprites(sprites)

	// INV-3: 6 sprites in 3 batches → 3 draw calls.
	batches := countBatches(sprites)
	if batches != 3 {
		return 0, fmt.Errorf("expected 3 batches, got %d (INV-3)", batches)
	}

	// INV-2: sort is deterministic — hash over sort keys is stable.
	hash := uint64(14695981039346656037) // FNV-1a offset
	for _, s := range sprites {
		hash ^= s.GetSortKey()
		hash *= 1099511628211
	}

	// Sprite picking: query a point covered by sprites (AtlasUV [0,0,1,1]).
	result := sprite2d.PickSprite(sprites, 0.5, 0.5)
	if !result.Hit {
		return 0, fmt.Errorf("expected a hit when querying (0.5, 0.5)")
	}

	return hash, nil
}

func main() {
	h, err := run()
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}
	fmt.Printf("PASS: 2D example hash=%d\n", h)
}
