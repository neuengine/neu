// Command shader demonstrates the post-processing pipeline:
// a FullscreenMaterial custom pass inserted after SlotBloom, a canonical
// Bloom→Tonemapping→FXAA chain (no AA conflict since FXAA only), and
// a PingPongPool managing HDR/LDR intermediate targets at zero heap cost.
//
// Run:  go run ./examples/shader
// Test: go test ./examples/shader
package main

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"

	internalrender "github.com/neuengine/neu/internal/render"
	"github.com/neuengine/neu/internal/render/postpass"
	gpu "github.com/neuengine/neu/pkg/render"
	"github.com/neuengine/neu/pkg/render/postprocess"
)

// buildPostGraph constructs the post-process render graph for one frame and
// returns it together with the per-frame ping-pong pool.
func buildPostGraph() (*internalrender.RenderGraph, *postpass.PingPongPool, error) {
	g := &internalrender.RenderGraph{}
	pool := postpass.NewPingPongPool()

	// AA check: FXAA only → no conflict.
	if err := postpass.CheckAAConflict(true, false); err != nil {
		return nil, nil, fmt.Errorf("unexpected AA conflict: %w", err)
	}

	// Post-process chain: Bloom (HDR) → Tonemapping (HDR→LDR) → SpatialAA (LDR).
	sceneColor := gpu.MakeRID(gpu.KindTexture, 1, 0)
	slots := []postprocess.EffectSlot{
		postprocess.SlotBloom,
		postprocess.SlotTonemapping,
		postprocess.SlotSpatialAA,
	}
	if _, err := postpass.BuildPostChain(slots, g, sceneColor); err != nil {
		return nil, nil, fmt.Errorf("BuildPostChain: %w", err)
	}

	if err := g.Build(nil); err != nil {
		return nil, nil, fmt.Errorf("g.Build: %w", err)
	}
	return g, pool, nil
}

// frameHash hashes the barrier list of the render graph for stability testing.
func frameHash(g *internalrender.RenderGraph) uint64 {
	h := fnv.New64a()
	var buf [8]byte
	for _, b := range g.Barriers() {
		binary.LittleEndian.PutUint32(buf[:4], uint32(b.From))
		binary.LittleEndian.PutUint32(buf[4:], uint32(b.To))
		h.Write(buf[:8])
		binary.LittleEndian.PutUint64(buf[:], uint64(b.Resource))
		h.Write(buf[:8])
	}
	return h.Sum64()
}

// run builds the post-process graph, verifies the AA check, and computes the
// frame hash. Also simulates the per-frame ping-pong steady-state (C-027).
func run() (uint64, error) {
	g, pool, err := buildPostGraph()
	if err != nil {
		return 0, err
	}

	// Simulate per-frame pool reset + acquire (C-027 zero-cost steady state).
	pool.Reset()
	hdr1 := pool.AcquireHDR()
	_ = pool.AcquireHDR()
	ldr := pool.AcquireLDR()
	if hdr1 == ldr {
		return 0, fmt.Errorf("HDR and LDR targets must differ")
	}

	// Verify SMAA+FXAA conflict is detected.
	if err := postpass.CheckAAConflict(true, true); err == nil {
		return 0, fmt.Errorf("expected ErrAAConflict for FXAA+SMAA, got nil")
	}

	return frameHash(g), nil
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
		panic(fmt.Sprintf("non-deterministic shader hash: %d != %d", h1, h2))
	}
	fmt.Println("PASS")
}
