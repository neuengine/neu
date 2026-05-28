package material

import (
	"github.com/neuengine/neu/pkg/asset"
	pkgmath "github.com/neuengine/neu/pkg/math"
	renderimage "github.com/neuengine/neu/pkg/render/image"
)

// StandardPBR returns a Material pre-populated with PBR metallic-roughness defaults.
// All values satisfy INV-2 (physically valid) without requiring an initial Sanitize call.
//   - base_color: (1, 1, 1, 1) — white, fully opaque
//   - metallic:   0.0          — dielectric
//   - roughness:  0.5          — mid-roughness
//   - occlusion:  1.0          — fully lit
//   - emissive:   (0, 0, 0, 0) — no self-emission
func StandardPBR() *Material {
	return &Material{
		Alpha: AlphaOpaque,
		Params: MaterialParameters{
			Floats: map[string]float32{
				"metallic":  0.0,
				"roughness": 0.5,
				"occlusion": 1.0,
			},
			Colors: map[string]pkgmath.LinearRgba{
				"base_color": {R: 1, G: 1, B: 1, A: 1},
				"emissive":   {},
			},
		},
	}
}

// SetFloat sets a named float parameter, returning m for chaining.
// PBR scalars (metallic, roughness, occlusion) are clamped to [0, 1] (INV-2).
func (m *Material) SetFloat(name string, v float32) *Material {
	if m.Params.Floats == nil {
		m.Params.Floats = make(map[string]float32)
	}
	switch name {
	case "metallic", "roughness", "occlusion":
		v = clamp01(v)
	}
	m.Params.Floats[name] = v
	return m
}

// SetColor sets a named color parameter, clamping all components to ≥ 0 (INV-2).
// Returns m for chaining.
func (m *Material) SetColor(name string, c pkgmath.LinearRgba) *Material {
	if m.Params.Colors == nil {
		m.Params.Colors = make(map[string]pkgmath.LinearRgba)
	}
	m.Params.Colors[name] = pkgmath.LinearRgba{
		R: max32(0, c.R),
		G: max32(0, c.G),
		B: max32(0, c.B),
		A: max32(0, c.A),
	}
	return m
}

// SetTexture sets a named texture parameter. Returns m for chaining.
func (m *Material) SetTexture(name string, h asset.Handle[renderimage.Image]) *Material {
	if m.Params.Textures == nil {
		m.Params.Textures = make(map[string]asset.Handle[renderimage.Image])
	}
	m.Params.Textures[name] = h
	return m
}
