// Package light provides pure-data light components for the render pipeline
// (l2-materials-and-lighting-go.md, Bootstrap 0.1.0). Light components are
// attached to entities; the lighting pass clusters them per tile/frustum.
// No fixed light-count cap is imposed at the data model level (L1 §2).
package light

import pkgmath "github.com/neuengine/neu/pkg/math"

// PointLight emits omnidirectionally from the entity's world position.
// Shadow is optional; nil disables shadow casting.
type PointLight struct {
	Shadow    *CubeShadow
	Color     pkgmath.LinearRgba
	Intensity float32
	Radius    float32
}

// SpotLight emits a cone of light from the entity's world position.
// InnerAngle and OuterAngle are in radians; intensity falls off smoothly between them.
// Shadow is optional; nil disables shadow casting.
type SpotLight struct {
	Shadow     *SingleShadow
	Color      pkgmath.LinearRgba
	Intensity  float32
	InnerAngle float32
	OuterAngle float32
}

// DirectionalLight emits parallel rays (e.g. sun); the entity's position is ignored.
// Cascades controls cascaded shadow maps; nil disables shadow casting.
type DirectionalLight struct {
	Cascades  *CascadeShadowConfig
	Color     pkgmath.LinearRgba
	Intensity float32
}

// AmbientLight adds a flat additive color term to all fragments — no directionality.
type AmbientLight struct{ Color pkgmath.LinearRgba }
