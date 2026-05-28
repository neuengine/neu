// Package light provides pure-data light components for the render pipeline
// (l2-materials-and-lighting-go.md, Bootstrap 0.1.0). Light components are
// attached to entities; the lighting pass clusters them per tile/frustum.
// No fixed light-count cap is imposed at the data model level (L1 §2).
package light

import pkgmath "github.com/neuengine/neu/pkg/math"

// PointLight emits omnidirectionally from the entity's world position.
// Shadow is optional; nil disables shadow casting.
type PointLight struct {
	Color     pkgmath.LinearRgba
	Intensity float32 // luminous intensity (lux)
	Radius    float32 // attenuation cutoff radius (metres)
	Shadow    *CubeShadow
}

// SpotLight emits a cone of light from the entity's world position.
// InnerAngle and OuterAngle are in radians; intensity falls off smoothly between them.
// Shadow is optional; nil disables shadow casting.
type SpotLight struct {
	Color                  pkgmath.LinearRgba
	Intensity              float32
	InnerAngle, OuterAngle float32 // radians; OuterAngle >= InnerAngle
	Shadow                 *SingleShadow
}

// DirectionalLight emits parallel rays (e.g. sun); the entity's position is ignored.
// Cascades controls cascaded shadow maps; nil disables shadow casting.
type DirectionalLight struct {
	Color     pkgmath.LinearRgba
	Intensity float32
	Cascades  *CascadeShadowConfig
}

// AmbientLight adds a flat additive color term to all fragments — no directionality.
type AmbientLight struct{ Color pkgmath.LinearRgba }
