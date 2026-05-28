// Package mesh provides GPU-ready geometry assets: attribute-map Mesh,
// validated index/skin invariants, VertexLayout for pipeline-key hashing,
// and ECS marker components.
//
// Bootstrap: l2-mesh-and-image-go Draft (C29 P4 gate open).
package mesh

import (
	"errors"
	"fmt"
	"maps"

	"github.com/neuengine/neu/pkg/asset"
	"github.com/neuengine/neu/pkg/ecs"
)

// AttrKind identifies the semantic role of a vertex attribute.
type AttrKind uint8

const (
	AttrPosition     AttrKind = iota // vec3 world/model position (INV-1: required)
	AttrNormal                       // vec3 unit normal
	AttrUV0                          // vec2 primary texcoord
	AttrUV1                          // vec2 secondary texcoord
	AttrTangent                      // vec4 tangent + handedness
	AttrColor                        // vec4 vertex color
	AttrJointWeights                 // vec4 skin joint weights  (INV-3)
	AttrJointIndices                 // uvec4 skin joint indices (INV-3)
	attrKindCount
)

// VertexFormat enumerates per-element data types. Each format encodes its own
// byte width via [VertexFormat.Size].
type VertexFormat uint8

const (
	FormatFloat32x2 VertexFormat = iota // 8 B
	FormatFloat32x3                     // 12 B
	FormatFloat32x4                     // 16 B
	FormatUint16x4                      // 8 B (joint indices)
	FormatUint8x4                       // 4 B (normalised joint weights)
)

// Size returns the byte width of one element in this format.
func (f VertexFormat) Size() uint32 {
	switch f {
	case FormatFloat32x2:
		return 8
	case FormatFloat32x3:
		return 12
	case FormatFloat32x4:
		return 16
	case FormatUint16x4:
		return 8
	case FormatUint8x4:
		return 4
	}
	return 0
}

// PrimitiveTopology describes how vertices form primitives.
type PrimitiveTopology uint8

const (
	TopologyTriangleList PrimitiveTopology = iota
	TopologyTriangleStrip
	TopologyLineList
	TopologyLineStrip
	TopologyPointList
)

// VertexAttribute is one tightly-packed semantic stream.
// len(Data) must equal Mesh.vertexCount * Format.Size().
type VertexAttribute struct {
	Data   []byte
	Kind   AttrKind
	Format VertexFormat
}

// IndexBuffer holds index data for indexed draws.
// Wide == false → uint16 indices; Wide == true → uint32.
type IndexBuffer struct {
	Data []byte
	Wide bool
}

// SubMesh references a contiguous slice of the index buffer for multi-material draws.
type SubMesh struct{ Offset, Count uint32 }

// Sentinel errors for Validate.
var (
	ErrMeshNoPosition        = errors.New("mesh: missing position attribute")
	ErrIndexOutOfRange       = errors.New("mesh: index references out-of-range vertex")
	ErrSkinAttributeMismatch = errors.New("mesh: joint weight/index length mismatch")
)

// Mesh is an immutable, attribute-map geometry asset. Build via [NewMesh] +
// [Mesh.SetAttribute] / [Mesh.SetIndices] then call [Mesh.Validate] before
// uploading to the GPU.
type Mesh struct {
	attrs       map[AttrKind]VertexAttribute
	indices     *IndexBuffer
	subMeshes   []SubMesh
	layout      VertexLayout
	vertexCount int
	Topology    PrimitiveTopology
	layoutReady bool
}

// NewMesh allocates a Mesh with the given primitive topology.
func NewMesh(topology PrimitiveTopology) *Mesh {
	return &Mesh{
		Topology: topology,
		attrs:    make(map[AttrKind]VertexAttribute, int(attrKindCount)),
	}
}

// SetAttribute attaches an attribute stream and returns the receiver for chaining.
// If this is the first attribute, it establishes vertexCount from the data length.
// Subsequent attributes must have the same element count.
func (m *Mesh) SetAttribute(a VertexAttribute) *Mesh {
	n := int(a.Format.Size())
	if n > 0 {
		count := len(a.Data) / n
		if m.vertexCount == 0 {
			m.vertexCount = count
		}
	}
	m.attrs[a.Kind] = a
	m.layoutReady = false
	return m
}

// SetIndices attaches an index buffer and returns the receiver.
func (m *Mesh) SetIndices(ib IndexBuffer) *Mesh {
	m.indices = &ib
	return m
}

// AddSubMesh appends a sub-mesh draw range.
func (m *Mesh) AddSubMesh(s SubMesh) *Mesh {
	m.subMeshes = append(m.subMeshes, s)
	return m
}

// Validate checks all mesh invariants (INV-1, 2, 3). Call once before GPU upload.
func (m *Mesh) Validate() error {
	// INV-1: position attribute required.
	if _, ok := m.attrs[AttrPosition]; !ok {
		return ErrMeshNoPosition
	}

	// INV-2: every index must be in-range.
	if m.indices != nil {
		if err := m.validateIndices(); err != nil {
			return err
		}
	}

	// INV-3: joint weights and indices must match vertex count.
	jw, hasWeights := m.attrs[AttrJointWeights]
	ji, hasIndices := m.attrs[AttrJointIndices]
	if hasWeights || hasIndices {
		wCount := len(jw.Data) / int(jw.Format.Size())
		iCount := len(ji.Data) / int(ji.Format.Size())
		if wCount != m.vertexCount || iCount != m.vertexCount {
			return ErrSkinAttributeMismatch
		}
	}

	return nil
}

func (m *Mesh) validateIndices() error {
	ib := m.indices
	stride := 2
	if ib.Wide {
		stride = 4
	}
	count := len(ib.Data) / stride
	for i := range count {
		var idx uint32
		offset := i * stride
		if ib.Wide {
			idx = uint32(ib.Data[offset]) |
				uint32(ib.Data[offset+1])<<8 |
				uint32(ib.Data[offset+2])<<16 |
				uint32(ib.Data[offset+3])<<24
		} else {
			idx = uint32(ib.Data[offset]) | uint32(ib.Data[offset+1])<<8
		}
		if int(idx) >= m.vertexCount {
			return fmt.Errorf("%w: index %d >= vertexCount %d", ErrIndexOutOfRange, idx, m.vertexCount)
		}
	}
	return nil
}

// Layout returns the memoised VertexLayout for this mesh (pipeline-cache key fragment).
func (m *Mesh) Layout() VertexLayout {
	if !m.layoutReady {
		m.layout = buildLayout(m.attrs)
		m.layoutReady = true
	}
	return m.layout
}

// Attributes returns a copy of the internal attribute map for read-only inspection.
func (m *Mesh) Attributes() map[AttrKind]VertexAttribute {
	out := make(map[AttrKind]VertexAttribute, len(m.attrs))
	maps.Copy(out, m.attrs)
	return out
}

// Indices returns the index buffer, or nil for non-indexed draws.
func (m *Mesh) Indices() *IndexBuffer { return m.indices }

// SubMeshes returns the sub-mesh draw ranges.
func (m *Mesh) SubMeshes() []SubMesh { return m.subMeshes }

// VertexCount returns the number of vertices in the mesh.
func (m *Mesh) VertexCount() int { return m.vertexCount }

// ─── ECS marker components ────────────────────────────────────────────────────

// Mesh3D marks an entity as a 3-D renderable mesh.
type Mesh3D struct{ Handle asset.Handle[Mesh] }

// Mesh2D marks an entity as a 2-D renderable mesh.
type Mesh2D struct{ Handle asset.Handle[Mesh] }

// SkinnedMesh pairs a mesh with a skeleton joint entity list.
type SkinnedMesh struct {
	Joints []ecs.Entity
	Mesh   asset.Handle[Mesh]
}

// MorphWeights drives morph-target animation on a SkinnedMesh.
type MorphWeights struct{ Weights []float32 }
