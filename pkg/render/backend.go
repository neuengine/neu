// Package render is the public, engine-agnostic surface of the render core
// (l2-render-core-go.md, Bootstrap 0.1.0). Callers hold opaque [RID] handles
// and a [RenderBackend]; they never touch GPU objects directly. The
// command-queue server and graph live in internal/render.
//
// Bootstrap status: l1-render-core / l2-render-core-go are Draft (C29 P4 gate
// open). Types here are contracts, not final; descriptor fields are minimal
// and will grow as backends land.
package render

// RID is an opaque 64-bit resource handle. Layout (high → low):
//
//	bits 56-63 : ResourceKind (8)
//	bits 32-55 : generation  (24)  — reuse detection (reserved; 1 in 0.1.0)
//	bits  0-31 : dense index  (32)
//
// The zero value is the nil handle ([RID.IsNil] reports true). Callers compare
// RIDs by value; they never dereference them.
type RID uint64

// ResourceKind tags what a [RID] refers to. KindNil (0) makes the zero RID nil.
type ResourceKind uint8

const (
	KindNil ResourceKind = iota
	KindBuffer
	KindTexture
	KindPipeline
	KindBindGroup
	KindScenario // self-contained 3D render world (l1-render-core §4.5)
	KindView     // a RenderView render target
)

const (
	ridIndexBits = 32
	ridGenBits   = 24
	ridGenShift  = ridIndexBits
	ridKindShift = ridIndexBits + ridGenBits
	ridIndexMask = (uint64(1) << ridIndexBits) - 1
	ridGenMask   = (uint64(1) << ridGenBits) - 1
)

// MakeRID packs a kind, dense index, and generation into an [RID]. Used by the
// server's allocator; exported so backends and tests can synthesise handles.
func MakeRID(kind ResourceKind, index uint32, generation uint32) RID {
	return RID(uint64(kind)<<ridKindShift |
		(uint64(generation)&ridGenMask)<<ridGenShift |
		uint64(index)&ridIndexMask)
}

// Kind reports the resource category this handle refers to.
func (r RID) Kind() ResourceKind { return ResourceKind(uint64(r) >> ridKindShift) }

// Index reports the dense slot index encoded in the handle.
func (r RID) Index() uint32 { return uint32(uint64(r) & ridIndexMask) }

// Generation reports the reuse counter encoded in the handle.
func (r RID) Generation() uint32 { return uint32((uint64(r) >> ridGenShift) & ridGenMask) }

// IsNil reports whether r is the nil handle (no resource).
func (r RID) IsNil() bool { return r.Kind() == KindNil }

// --- Descriptors (minimal Bootstrap surface; fields grow with backends) ---

// BufferUsage is a bitset of intended GPU buffer uses.
type BufferUsage uint16

const (
	BufVertex BufferUsage = 1 << iota
	BufIndex
	BufUniform
	BufStorage
	BufCopySrc
	BufCopyDst
)

// TextureFormat enumerates GPU-compatible texel formats (subset for 0.1.0).
type TextureFormat uint8

const (
	FmtInvalid TextureFormat = iota
	FmtRGBA8
	FmtRGBA16F
	FmtRG11B10F
	FmtDepth32F
	FmtBC7
)

type (
	// BufferDesc describes a GPU buffer to create.
	BufferDesc struct {
		Size  uint64
		Usage BufferUsage
	}
	// TextureDesc describes a GPU texture to create.
	TextureDesc struct {
		Width, Height uint32
		Format        TextureFormat
		MipLevels     uint32
	}
	// PipelineDesc describes a render pipeline. ShaderID + layout hash form the
	// specialization key (l1-render-core §4.7).
	PipelineDesc struct {
		ShaderID   uint64
		LayoutHash uint64
		Phase      uint8
	}
	// BindGroupDesc binds resources for a pipeline.
	BindGroupDesc struct {
		Layout    uint64
		Resources []RID
	}
	// RenderPassDesc opens a render pass against the given color/depth targets.
	RenderPassDesc struct {
		Color []RID
		Depth RID
	}
	// DrawCmd is a single batched draw submission.
	DrawCmd struct {
		Pipeline    RID
		BindGroups  []RID
		Vertex      RID
		Index       RID
		InstanceCnt uint32
	}
)

// RenderBackend is the single interface every GPU backend (OpenGL, Vulkan,
// WebGPU, software rasteriser) must fully implement (l1-render-core INV-5).
// Partial implementations fail to compile — the invariant is unrepresentable
// at runtime by construction.
type RenderBackend interface {
	CreateBuffer(BufferDesc) RID
	CreateTexture(TextureDesc) RID
	CreatePipeline(PipelineDesc) RID
	CreateBindGroup(BindGroupDesc) RID
	BeginRenderPass(RenderPassDesc)
	Draw(DrawCmd)
	EndRenderPass()
	Submit()
	Present()
	Destroy(RID)
}
