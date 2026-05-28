// Package material defines surface materials and their GPU-pipeline specialisation
// keys (l2-materials-and-lighting-go.md, Bootstrap 0.1.0).
//
// Material is a shader handle plus a typed MaterialParameters bag and an AlphaMode
// that deterministically maps to a render phase (INV-5). PBR metallic-roughness is
// the default shading model; MaterialParameters.Sanitize clamps values to
// physically valid ranges (INV-2).
package material

import (
	"errors"
	"log/slog"

	"github.com/neuengine/neu/pkg/asset"
	pkgmath "github.com/neuengine/neu/pkg/math"
	gpu "github.com/neuengine/neu/pkg/render"
	renderimage "github.com/neuengine/neu/pkg/render/image"
)

// ErrMaterialNoShader is returned by Validate when no shader handle has been set.
var ErrMaterialNoShader = errors.New("material: nil shader handle")

// Shader is an opaque compiled GPU shader asset.
// Bootstrap stub — full type lives in pkg/render/shader once that package lands.
type Shader struct{}

// AlphaMode controls how the fragment's alpha value is interpreted during rendering.
type AlphaMode uint8

const (
	AlphaOpaque        AlphaMode = iota // fully opaque; alpha ignored
	AlphaMask                           // fragment-discard threshold (cutout)
	AlphaBlend                          // standard over-blending
	AlphaPremultiplied                  // premultiplied alpha
	AlphaAdditive                       // additive blending
)

// Phase returns the fixed render-phase bucket for this alpha mode (INV-5: total map,
// non-overridable at the bucket level). PhaseHint may reorder within the bucket.
func (a AlphaMode) Phase() gpu.RenderPhase {
	switch a {
	case AlphaOpaque:
		return gpu.PhaseOpaque
	case AlphaMask:
		return gpu.PhaseAlphaMask
	case AlphaBlend, AlphaPremultiplied, AlphaAdditive:
		return gpu.PhaseTransparent
	default:
		return gpu.PhaseOpaque
	}
}

// MaterialParameters holds all typed shader inputs for a Material.
// The zero value is valid (empty parameter sets).
type MaterialParameters struct {
	Textures map[string]asset.Handle[renderimage.Image]
	Floats   map[string]float32
	Vectors  map[string]pkgmath.Vec4
	Colors   map[string]pkgmath.LinearRgba
}

// Sanitize clamps PBR values to physically valid ranges (INV-2). Idempotent.
//   - metallic, roughness, occlusion → [0, 1]
//   - all color components           → ≥ 0
func (p *MaterialParameters) Sanitize() {
	for _, name := range []string{"metallic", "roughness", "occlusion"} {
		if v, ok := p.Floats[name]; ok {
			if c := clamp01(v); c != v {
				slog.Debug("material: clamped PBR scalar", "name", name, "from", v, "to", c)
				p.Floats[name] = c
			}
		}
	}
	for name, c := range p.Colors {
		clamped := pkgmath.LinearRgba{
			R: max32(0, c.R),
			G: max32(0, c.G),
			B: max32(0, c.B),
			A: max32(0, c.A),
		}
		if clamped != c {
			slog.Debug("material: clamped color", "name", name)
			p.Colors[name] = clamped
		}
	}
}

// Material is a surface description: shader + typed parameters + alpha mode.
// The zero value is invalid; build via StandardPBR() or populate Shader directly.
type Material struct {
	Params      MaterialParameters
	PhaseHint   *gpu.RenderPhase
	Shader      asset.Handle[Shader]
	Alpha       AlphaMode
	DoubleSided bool
}

// Validate checks material invariants before pipeline build (INV-1).
func (m *Material) Validate() error {
	if m.Shader.IsWeak() {
		return ErrMaterialNoShader
	}
	return nil
}

// resolvePhase applies PhaseHint only when it stays on the same opaque/transparent
// side as the AlphaMode-mapped phase. Crossing the boundary silently returns the
// mapped phase (INV-5).
func (m *Material) resolvePhase() gpu.RenderPhase {
	mapped := m.Alpha.Phase()
	if m.PhaseHint == nil {
		return mapped
	}
	hint := *m.PhaseHint
	if isOpaqueSide(mapped) != isOpaqueSide(hint) {
		return mapped
	}
	return hint
}

// isOpaqueSide reports whether phase belongs to the opaque render bucket
// (PhaseOpaque or PhaseAlphaMask), as opposed to the transparent bucket.
func isOpaqueSide(p gpu.RenderPhase) bool {
	return p == gpu.PhaseOpaque || p == gpu.PhaseAlphaMask
}

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
