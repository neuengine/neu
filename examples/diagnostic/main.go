// examples/diagnostic validates the diagnostics subsystem end-to-end (T-6T06,
// C29 P6 gate): the reader-gate (INV-1), deterministic rolling averages, and
// immediate-mode gizmo geometry. The hash is stable across ≥20 runs.
//
// Bootstrap: validates l2-diagnostic-system-go against l1-diagnostic-system.
package main

import (
	"fmt"
	"math"

	pkgdiag "github.com/neuengine/neu/pkg/diag"
	pkgmath "github.com/neuengine/neu/pkg/math"
)

func run() (uint64, error) {
	store := pkgdiag.NewDiagnosticsStore()
	fps := pkgdiag.NewDiagnostic("engine/fps", "fps", 8)
	store.Register(fps)

	// INV-1: with no reader registered, Push is a no-op.
	store.Push("engine/fps", 60)
	if d, _ := store.Get("engine/fps"); d.Len() != 0 {
		return 0, fmt.Errorf("INV-1 violated: push with no reader recorded a sample")
	}

	// Register a reader, then push a deterministic series.
	store.AddReader("engine/fps")
	for _, v := range []float64{60, 59, 61, 60, 58, 62, 60, 60} {
		store.Push("engine/fps", v)
	}
	d, _ := store.Get("engine/fps")

	// Immediate-mode gizmo geometry (INV-2 pure visual; deterministic vertices).
	g := pkgdiag.NewGizmoBuffer()
	white := pkgmath.LinearRgba{R: 1, G: 1, B: 1, A: 1}
	g.Box(pkgmath.Vec3{}, pkgmath.Vec3{X: 1, Y: 1, Z: 1}, white)
	g.Sphere(pkgmath.Vec3{X: 2}, 0.5, white)
	g.Line(pkgmath.Vec3{}, pkgmath.Vec3{X: 5}, white)

	h := fnvOffset
	h = hashF(h, float32(d.Average()))
	h = hashF(h, float32(d.Min()))
	h = hashF(h, float32(d.Max()))
	h = hashU(h, uint64(len(g.Lines())))
	return h, nil
}

const (
	fnvOffset uint64 = 14695981039346656037
	fnvPrime  uint64 = 1099511628211
)

func hashF(h uint64, f float32) uint64 { return hashU(h, uint64(math.Float32bits(f))) }

func hashU(h, v uint64) uint64 {
	for i := range 8 {
		h ^= (v >> (8 * i)) & 0xff
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
	fmt.Printf("PASS: diagnostic hash=%d\n", h)
}
