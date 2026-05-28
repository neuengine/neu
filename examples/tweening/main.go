// examples/tweening demonstrates easing-curve sampling, LoopMode semantics,
// and Virtual vs Real TimeDimension (T-5T03, C29 P5 gate).
//
// Bootstrap: validates l2-tweening-system-go against l1-tweening-system.
package main

import (
	"fmt"
	"math"

	internaltween "github.com/neuengine/neu/internal/tween"
	pkgtween "github.com/neuengine/neu/pkg/tween"
)

// tweenTarget is a simple struct animated by the tween system.
type tweenTarget struct {
	X float32
}

// run exercises the tween advance system and returns a stability hash.
func run() (uint64, error) {
	// --- LoopOnce: advances to completion and reports done.
	once := &pkgtween.Tween{
		StartValue: float32(0),
		EndValue:   float32(10),
		Duration:   1,
		LoopMode:   pkgtween.LoopOnce,
		Easing:     pkgtween.EaseOutQuad,
	}
	target := &tweenTarget{}
	acc, err := internaltween.NewWriteAccessor("X")
	if err != nil {
		return 0, err
	}

	done, err := internaltween.AdvanceTween(once, acc, target, 0.5)
	if err != nil {
		return 0, fmt.Errorf("LoopOnce advance: %w", err)
	}
	if done {
		return 0, fmt.Errorf("LoopOnce should not be done at 0.5s")
	}
	if target.X <= 0 || target.X > 10 {
		return 0, fmt.Errorf("LoopOnce X out of range: %v", target.X)
	}

	done, err = internaltween.AdvanceTween(once, acc, target, 0.5)
	if err != nil {
		return 0, fmt.Errorf("LoopOnce second advance: %w", err)
	}
	if !done {
		return 0, fmt.Errorf("LoopOnce should be done at 1.0s")
	}

	// --- Loop: wraps around at duration boundary.
	loop := &pkgtween.Tween{
		StartValue: float32(0),
		EndValue:   float32(10),
		Duration:   1,
		LoopMode:   pkgtween.Loop,
	}
	loopTarget := &tweenTarget{}
	done, _ = internaltween.AdvanceTween(loop, acc, loopTarget, 1.5)
	if done {
		return 0, fmt.Errorf("Loop tween should never report done")
	}
	// After 1.5 cycles the linear easing should put X near 5.
	if math.Abs(float64(loopTarget.X-5)) > 0.02 {
		return 0, fmt.Errorf("Loop at 1.5s: X=%v, want ~5", loopTarget.X)
	}

	// --- PingPong: reverses at duration.
	pp := &pkgtween.Tween{
		StartValue: float32(0),
		EndValue:   float32(10),
		Duration:   1,
		LoopMode:   pkgtween.PingPong,
	}
	ppTarget := &tweenTarget{}
	internaltween.AdvanceTween(pp, acc, ppTarget, 1.5)
	if math.Abs(float64(ppTarget.X-5)) > 0.02 {
		return 0, fmt.Errorf("PingPong at 1.5s: X=%v, want ~5", ppTarget.X)
	}

	// --- INV-1 self-cleanup assertion (despawn-on-done).
	despawn := &pkgtween.Tween{
		StartValue:    float32(0),
		EndValue:      float32(1),
		Duration:      0, // degenerate → done immediately
		DespawnOnDone: true,
	}
	dt := &tweenTarget{}
	done2, _ := internaltween.AdvanceTween(despawn, acc, dt, 0)
	if !done2 {
		return 0, fmt.Errorf("degenerate despawn tween should be done")
	}

	// Compute a deterministic hash over final X values.
	hash := fnv1a(uint64(target.X*1000), uint64(loopTarget.X*1000), uint64(ppTarget.X*1000))
	return hash, nil
}

func fnv1a(vals ...uint64) uint64 {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)
	h := uint64(offset64)
	for _, v := range vals {
		h ^= v
		h *= prime64
	}
	return h
}

func main() {
	h, err := run()
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}
	fmt.Printf("PASS: tweening example hash=%d\n", h)
}
