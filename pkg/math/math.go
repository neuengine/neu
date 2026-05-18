// Package math provides spatial math primitives for the Neu engine.
// All public types use value semantics (value receivers, return new values) and
// carry zero ECS dependencies. Base scalar type is float32 throughout.
//
// Type overview:
//   - Vec2, Vec3, Vec4             — vector algebra (vec.go)
//   - Mat3, Mat4                   — column-major matrix algebra (mat.go)
//   - Quat, EulerOrder             — unit quaternion rotation (quat.go)
//   - Affine3A                     — column-matrix affine transform (affine.go)
//   - Affine3                      — TRS affine transform (affine.go)
//   - Dir2, Dir3, Rot2             — unit-direction and 2D rotation types (isometry.go)
//   - Isometry2D, Isometry3D       — rigid-body transforms (isometry.go)
//   - Ray2D, Ray3D, AABB, Sphere   — geometric primitives (primitives.go)
//   - Plane, Frustum               — geometric primitives (primitives.go)
//   - LinearRgba, SrgbRgba, HslaColor, HsvaColor — color types (color.go)
//   - CubicBezierVec3, HermiteSpline, NewCatmullRomSpline — curves (curve.go)
//   - TransformInterpolator        — transform blending (interp.go)
package math
