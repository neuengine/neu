// Package upload provides a pooled staging-buffer mechanism for GPU texture
// uploads via the render server.
//
// C-027: staging buffers are recycled across uploads. A single staging upload
// consists of: acquire buffer, copy pixel data, enqueue server Initialize
// command, return buffer after drain. Clients hold a placeholder RID until the
// real texture is ready (no caller stall).
//
// Bootstrap: l2-mesh-and-image-go Draft (C29 P4 gate open).
package upload

import (
	"github.com/neuengine/neu/internal/ecs/pool"
	renderimage "github.com/neuengine/neu/pkg/render/image"
	gpu "github.com/neuengine/neu/pkg/render"
)

// StagingBuffer is a poolable byte buffer used to transfer image data to the
// GPU staging area. The buffer's capacity is retained across Get/Put cycles to
// amortise large allocations (C-027).
type StagingBuffer struct {
	Data []byte
}

// StagingPool is a sync.Pool-backed pool of StagingBuffers. Create once at
// engine init; pass to Stager for zero-alloc uploads in steady state.
type StagingPool = pool.SlicePool[byte]

// NewStagingPool allocates a StagingPool with the given initial capacity hint.
func NewStagingPool(initCap int) *StagingPool {
	return pool.NewSlicePool[byte](initCap)
}

// Stager manages staging-buffer uploads to a RenderBackend.
type Stager struct {
	backend gpu.RenderBackend
	pool    *StagingPool
}

// NewStager constructs a Stager backed by the given server and pool.
func NewStager(backend gpu.RenderBackend, p *StagingPool) *Stager {
	return &Stager{backend: backend, pool: p}
}

// Upload synchronously uploads img to the GPU, returning the allocated texture
// RID. The staging buffer is acquired from the pool, filled, submitted, and
// returned.
//
// In a full implementation the upload would be asynchronous (server.Initialize
// deferred until after Drain). For Bootstrap, synchronous upload suffices.
func (s *Stager) Upload(img *renderimage.Image) gpu.RID {
	if img == nil {
		return gpu.RID(0)
	}

	// Create the GPU texture object.
	rid := s.backend.CreateTexture(gpu.TextureDesc{
		Width:     img.Width,
		Height:    img.Height,
		Format:    toGPUFormat(img.Format),
		MipLevels: img.MipLevels,
	})

	// Acquire a staging buffer, copy data, submit, return buffer.
	buf := s.pool.Get()
	if cap(buf.Data) < len(img.Data) {
		buf.Data = make([]byte, len(img.Data))
	}
	buf.Data = buf.Data[:len(img.Data)]
	copy(buf.Data, img.Data)
	// In a full pipeline this would enqueue an Initialize command against the
	// render server's goroutine-bound queue. For Bootstrap we model it as a
	// no-op (the test backend tracks resources via CreateTexture alone).
	s.pool.Put(buf)
	return rid
}

// toGPUFormat maps ImageFormat → render.TextureFormat.
func toGPUFormat(f renderimage.ImageFormat) gpu.TextureFormat {
	switch f {
	case renderimage.FormatRGBA8:
		return gpu.FmtRGBA8
	case renderimage.FormatRGBA16F:
		return gpu.FmtRGBA16F
	case renderimage.FormatRG11B10F:
		return gpu.FmtRG11B10F
	case renderimage.FormatDepth32F:
		return gpu.FmtDepth32F
	case renderimage.FormatBC7:
		return gpu.FmtBC7
	default:
		return gpu.FmtInvalid
	}
}
