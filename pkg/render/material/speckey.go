package material

import (
	"fmt"
	"hash/fnv"

	"github.com/neuengine/neu/pkg/asset"
	gpu "github.com/neuengine/neu/pkg/render"
	"github.com/neuengine/neu/pkg/render/mesh"
)

// SpecializationKey uniquely identifies a GPU pipeline variant in the render-core
// pipeline cache (l1-render-core §4.7, l2-render-core-go §Performance).
// A changed key triggers async pipeline recompile with a fallback pipeline meanwhile.
// The struct is comparable — safe as a map key.
type SpecializationKey struct {
	Shader uint64 // hash of shader AssetID
	Layout uint64 // mesh.VertexLayout.Hash (FNV-1a over sorted elements)
	Alpha  AlphaMode
	Phase  gpu.RenderPhase
}

// SpecKey builds the pipeline-specialization key for this material using the
// given vertex layout. The key is the bridge to render-core's pipeline cache.
// Recomputed only when shader, layout, alpha, or phase changes — not per draw.
func (m *Material) SpecKey(layout mesh.VertexLayout) SpecializationKey {
	return SpecializationKey{
		Shader: assetIDHash(m.Shader.ID()),
		Layout: layout.Hash,
		Alpha:  m.Alpha,
		Phase:  m.resolvePhase(),
	}
}

// assetIDHash derives a uint64 pipeline-cache discriminant from a shader AssetID.
// Bootstrap: fmt-encodes the opaque struct (all fields via reflect) then FNV-1a hashes.
// Zero ID (nil shader handle) fast-paths to 0 without allocations.
func assetIDHash(id asset.AssetID) uint64 {
	if !id.IsValid() {
		return 0
	}
	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "%v", id)
	return h.Sum64()
}
