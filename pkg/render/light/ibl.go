package light

import (
	"github.com/neuengine/neu/pkg/asset"
	pkgmath "github.com/neuengine/neu/pkg/math"
	renderimage "github.com/neuengine/neu/pkg/render/image"
)

// EnvironmentMapLight provides image-based lighting via a pair of pre-filtered
// cube maps (L1 §4.5):
//   - Diffuse:  irradiance-convolved cube (diffuse ambient term)
//   - Specular: pre-filtered cube with per-mip roughness levels (specular reflections)
//
// Both handles may be zero (Bootstrap); the lighting pass treats zero handles as
// "no IBL" and skips the contribution.
type EnvironmentMapLight struct {
	Diffuse  asset.Handle[renderimage.Image]
	Specular asset.Handle[renderimage.Image]
}

// IrradianceVolume provides baked global illumination via a 3-D grid of spherical-
// harmonic (SH) probes (L1 §4.5). Entities sample the nearest probe(s) based on
// world position.
//
// Probes holds GridSize[0]*GridSize[1]*GridSize[2] entries. Each probe is encoded
// as 9 consecutive pkgmath.Vec4 values (L0+L1 SH basis, RGB channels packed).
// The lighting system validates the total probe count on upload.
type IrradianceVolume struct {
	GridSize [3]uint32
	Probes   []pkgmath.Vec4 // 9 Vec4 SH coefficients per probe
}

// ProbeCount returns the expected number of probes for this grid (GridSize product).
func (iv IrradianceVolume) ProbeCount() uint32 {
	return iv.GridSize[0] * iv.GridSize[1] * iv.GridSize[2]
}
