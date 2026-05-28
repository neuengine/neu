package postpass

import (
	"fmt"

	gpu "github.com/neuengine/neu/pkg/render"
	"github.com/neuengine/neu/pkg/render/postprocess"
)

// PingPongPool manages two HDR and two LDR intermediate render-target RIDs
// per camera (C-027). Targets are pre-allocated once; per-frame acquisition
// rotates an index — zero heap allocations in steady state.
//
// Call Reset() each frame before BuildPostChain, then call AcquireHDR /
// AcquireLDR to hand targets to each post-process pass.
type PingPongPool struct {
	hdr    [2]gpu.RID
	ldr    [2]gpu.RID
	hdrCur int
	ldrCur int
}

// pingpongBase is the dense-index base for pooled ping-pong texture RIDs.
// Offset from intermRIDBase so the two families never collide.
const pingpongBase = 0x0002_0000

// NewPingPongPool allocates a PingPongPool with synthetic pre-allocated RIDs.
// In a real backend the RIDs would be created via RenderBackend.CreateTexture.
func NewPingPongPool() *PingPongPool {
	p := &PingPongPool{}
	for i := range 2 {
		p.hdr[i] = gpu.MakeRID(gpu.KindTexture, uint32(pingpongBase+i), 0)
		p.ldr[i] = gpu.MakeRID(gpu.KindTexture, uint32(pingpongBase+2+i), 0)
	}
	return p
}

// Reset resets the acquisition indices for the next frame (C-027: no
// deallocation — indices just wrap back to 0).
func (p *PingPongPool) Reset() {
	p.hdrCur = 0
	p.ldrCur = 0
}

// AcquireHDR returns the next available HDR intermediate target (linear space).
func (p *PingPongPool) AcquireHDR() gpu.RID {
	rid := p.hdr[p.hdrCur%2]
	p.hdrCur++
	return rid
}

// AcquireLDR returns the next available LDR intermediate target (encoded).
func (p *PingPongPool) AcquireLDR() gpu.RID {
	rid := p.ldr[p.ldrCur%2]
	p.ldrCur++
	return rid
}

// CheckAAConflict returns ErrAAConflict when incompatible AA settings are
// both enabled on the same camera. SMAA is the preferred operator — when
// both are present SMAA wins and FXAA is silently dropped (per Error Handling
// table). The caller is responsible for acting on the returned error.
func CheckAAConflict(hasFXAA, hasSMAA bool) error {
	if hasFXAA && hasSMAA {
		return fmt.Errorf("%w: SMAA preferred; FXAA dropped", postprocess.ErrAAConflict)
	}
	return nil
}
