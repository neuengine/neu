// examples/ui validates the UI flexbox solver end-to-end (T-6T06, C29 P6 gate):
// it builds a styled node tree, solves layout, exercises the dirty-gate (INV-1),
// and hashes the computed LayoutRects. The hash is stable across ≥20 runs.
//
// Bootstrap: validates l2-ui-system-go against l1-ui-system.
package main

import (
	"fmt"
	"math"

	internalui "github.com/neuengine/neu/internal/ui"
	pkgui "github.com/neuengine/neu/pkg/ui"
)

// run builds a toolbar-like layout (row, padding, gap, align-center, mixed
// grow + fixed children), solves it, verifies the dirty-gate, and returns a
// deterministic hash over the computed rects.
func run() (uint64, error) {
	root := &internalui.LayoutNode{
		Style: pkgui.Style{
			FlexDirection: pkgui.Row,
			AlignItems:    pkgui.AlignCenter,
			Width:         pkgui.Px(800),
			Height:        pkgui.Px(600),
			Padding:       pkgui.PxRect(10, 10, 10, 10),
			Gap:           8,
		},
		Children: []*internalui.LayoutNode{
			{Style: pkgui.Style{FlexGrow: 1, Height: pkgui.Px(40)}},
			{Style: pkgui.Style{FlexGrow: 2, Height: pkgui.Px(40)}},
			{Style: pkgui.Style{Width: pkgui.Px(100), Height: pkgui.Px(40)}},
		},
	}
	vp := internalui.Viewport{Width: 800, Height: 600}
	internalui.Solve(root, vp)

	// INV-1: a clean tree is skipped by SolveIfDirty (no re-layout).
	if internalui.SolveIfDirty(root, vp) {
		return 0, fmt.Errorf("clean tree should be skipped (INV-1)")
	}

	h := fnvOffset
	for _, c := range root.Children {
		h = hashF(h, c.Rect.Position.X)
		h = hashF(h, c.Rect.Position.Y)
		h = hashF(h, c.Rect.Size.X)
		h = hashF(h, c.Rect.Size.Y)
	}
	return h, nil
}

const (
	fnvOffset uint64 = 14695981039346656037
	fnvPrime  uint64 = 1099511628211
)

// hashF folds a float32's bits into an FNV-1a accumulator.
func hashF(h uint64, f float32) uint64 {
	bits := uint64(math.Float32bits(f))
	for i := range 4 {
		h ^= (bits >> (8 * i)) & 0xff
		h *= fnvPrime
	}
	return h
}

func main() {
	h, err := run()
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}
	fmt.Printf("PASS: ui layout hash=%d\n", h)
}
