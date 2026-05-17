# Math System — Go Implementation

**Version:** 0.1.0
**Status:** Draft
**Layer:** go
**Implements:** [l1-math-system.md](l1-math-system.md)

## Overview

Go-level design for the pure math library: vector, matrix, quaternion, affine
transform, direction, isometry, geometric primitive, color, and curve types.
The package has zero ECS dependencies and zero external Go dependencies — it
relies only on the standard library `math` package. All public types use value
semantics: every method has a value receiver and returns a new value rather than
mutating in place.

## Related Specifications

- [l1-math-system.md](l1-math-system.md) — L1 concept specification (parent)
- [l1-hierarchy-system.md](l1-hierarchy-system.md) — consumer: Transform components use `Vec3`, `Quat`, `Affine3`

## 1. Motivation

Every engine subsystem (render, physics, hierarchy, animation) depends on math
primitives. A dedicated Go package must:

- Provide correct, allocation-free implementations of common 3D math operations.
- Use value semantics so math values are stack-allocated and free of aliasing
  bugs in concurrent systems.
- Stay decoupled from ECS so it is usable in standalone tools, benchmarks, and
  tests.
- Honor C-003 (engine core has zero external Go dependencies) — stdlib `math`
  only, no third-party linear-algebra packages.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: generics power the curve types (`CubicSegment[P]`) and the
  point constraint interface.
- **Base type `float32`**: all types are single-precision unless an explicit
  suffix indicates otherwise. Internal trig/sqrt helpers wrap stdlib `math`
  (`float64`) and convert back to `float32` at the boundary.
- **No `Option[T]` type**: Go's idiom for fallible construction is the
  comma-ok pattern. Constructors that can fail return `(T, bool)`; the L1
  `Option[Self]` inverse/intersection returns map to `(T, bool)`.
- **Column-major storage**: `Mat3` is `[3]Vec3` and `Mat4` is `[4]Vec4`,
  where each element is a *column*. This matches GPU upload conventions and
  satisfies L1 INV-4.
- **Value receivers only**: no method takes a pointer receiver on the public
  math types — enforces L1 INV-1 (immutability) at the API surface.
- **Package boundary**: pure math types live in `pkg/math` (public, reusable).
  L1 §4.12 (Batch Transform Processing) and §4.13 (Post-Transform Operations)
  describe ECS-coupled *components and systems* — those are implemented in the
  hierarchy package ([l2-hierarchy-system-go.md](l2-hierarchy-system-go.md)) and
  merely *consume* `Affine3`/`Mat4`/`TransformInterpolator` from here. This L2
  specifies only the pure primitives plus `TransformInterpolator` (pure, no ECS).
- **No `sync.Pool` (C-004 N/A)**: math values are value types returned by copy;
  they never escape to the heap on the hot path, so the C-004 pooling mandate
  does not apply. Zero-allocation is achieved structurally, not via pooling.

## 3. Core Invariants

> [!NOTE]
> See [l1-math-system.md §3](l1-math-system.md) for the technology-agnostic
> invariants. Go-specific compliance is tabulated in §4.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: Immutability | All public methods use value receivers and return new values. No exported pointer-receiver mutators. Vet rule + review gate enforce this. |
| **INV-2**: Quaternion normalization | `Quat` constructors (`QuatFromAxisAngle`, `QuatFromEuler`, `QuatFromRotationArc`) call internal `normalize()` before return. `Mul`/`Slerp` re-normalize their result. The zero value is *not* a valid `Quat`; use `QuatIdentity()`. |
| **INV-3**: Direction normalization | `Dir2`/`Dir3` wrap a private vector field. The only constructors are `NewDir3(v) (Dir3, bool)` (normalizes; `false` if zero-length) and `NewDir3Unchecked(v)` (caller-asserted). No exported field access — invariant cannot be broken externally. |
| **INV-4**: Column-major layout | `Mat3 [3]Vec3`, `Mat4 [4]Vec4`. Index `m[c]` is column `c`. `ColMajorArray()` returns the backing `[16]float32` directly for GPU upload with no transpose. |

## 5. Detailed Design

### 5.1 Vector Types

```go
type Vec2 struct{ X, Y float32 }
type Vec3 struct{ X, Y, Z float32 }
type Vec4 struct{ X, Y, Z, W float32 }
```

Methods (value receiver, return new value) mirror L1 §4.1:

```
func (v Vec3) Add(o Vec3) Vec3
func (v Vec3) Sub(o Vec3) Vec3
func (v Vec3) Mul(s float32) Vec3      // scalar
func (v Vec3) MulComp(o Vec3) Vec3     // component-wise (Hadamard)
func (v Vec3) Div(s float32) Vec3
func (v Vec3) Dot(o Vec3) float32
func (v Vec3) Cross(o Vec3) Vec3       // Vec3 only
func (v Vec3) Length() float32
func (v Vec3) LengthSquared() float32  // alloc-free, avoids sqrt
func (v Vec3) Normalize() Vec3         // returns ZERO if length == 0
func (v Vec3) Lerp(o Vec3, t float32) Vec3
func (v Vec3) DistanceTo(o Vec3) float32
func (v Vec3) Min(o Vec3) Vec3
func (v Vec3) Max(o Vec3) Vec3
func (v Vec3) Clamp(lo, hi Vec3) Vec3
```

`Vec2`/`Vec4` expose the same set minus `Cross`. `LengthSquared` is added over
L1 as a Go-idiomatic alloc-free fast path for distance comparisons.

#### Named Constants (L1 §4.11)

Go has no struct constants; named vectors are exposed as zero-arg constructor
functions returning the value (the compiler inlines and stack-allocates them):

```
func Vec3Zero() Vec3     // {0,0,0}
func Vec3One() Vec3      // {1,1,1}
func Vec3Up() Vec3       // {0,1,0}
func Vec3Down() Vec3
func Vec3Left() Vec3
func Vec3Right() Vec3
func Vec3Forward() Vec3  // {0,0,-1} (right-handed, -Z forward)
func Vec3Back() Vec3
```

<!-- TBD (L1 §5 Q1): float64 variants (Vec3D, etc.) for editor-precision use.
     Deferred — not in 0.1.0 scope. Revisit when editor tooling needs it. -->

### 5.2 Matrix Types

```go
type Mat3 [3]Vec3  // 3 columns
type Mat4 [4]Vec4  // 4 columns
```

```
func (m Mat4) MulMat(o Mat4) Mat4
func (m Mat4) MulVec4(v Vec4) Vec4
func (m Mat4) TransformPoint(v Vec3) Vec3   // implicit w=1
func (m Mat4) TransformVector(v Vec3) Vec3  // implicit w=0
func (m Mat4) Transpose() Mat4
func (m Mat4) Inverse() (Mat4, bool)        // false if singular (det≈0)
func (m Mat4) Determinant() float32
func (m Mat4) ColMajorArray() [16]float32   // GPU upload, no transpose

func Mat4FromScale(s Vec3) Mat4
func Mat4FromTranslation(t Vec3) Mat4
func Mat4Perspective(fovYRad, aspect, near, far float32) Mat4
func Mat4Orthographic(l, r, b, t, near, far float32) Mat4
func Mat4Identity() Mat4
```

Singularity threshold: `|det| < 1e-6` → `(zero, false)`. This is the Go
mapping of L1's `Option[Self]` inverse.

### 5.3 Quaternion

```go
type Quat struct{ X, Y, Z, W float32 } // INV-2: always unit; zero value invalid
```

```
func QuatIdentity() Quat
func QuatFromAxisAngle(axis Vec3, angleRad float32) Quat
func QuatFromEuler(order EulerOrder, a, b, c float32) Quat
func QuatFromRotationArc(from, to Vec3) Quat

func (q Quat) Mul(o Quat) Quat              // compose; re-normalizes
func (q Quat) MulVec3(v Vec3) Vec3          // rotate vector
func (q Quat) Slerp(o Quat, t float32) Quat // re-normalizes
func (q Quat) ToEuler(order EulerOrder) (a, b, c float32)
func (q Quat) Inverse() Quat                // conjugate (unit ⇒ conj == inv)
func (q Quat) AngleBetween(o Quat) float32

type EulerOrder uint8
const (
    EulerXYZ EulerOrder = iota
    EulerXZY
    EulerYXZ
    EulerYZX
    EulerZXY
    EulerZYX
)
```

### 5.4 Affine Transforms

L1 names this `Affine3A` (the `A` = SIMD-aligned storage). Go cannot portably
guarantee SIMD alignment without `unsafe`/assembly, so the type is named
`Affine3`; alignment is treated as a future internal optimization.

```go
type Affine3 struct {
    Translation Vec3
    Rotation    Quat
    Scale       Vec3
}
```

```
func Affine3Identity() Affine3
func (a Affine3) TransformPoint(p Vec3) Vec3
func (a Affine3) TransformVector(v Vec3) Vec3   // ignores translation
func (a Affine3) Mul(o Affine3) Affine3         // compose
func (a Affine3) ToMat4() Mat4
func Affine3FromMat4(m Mat4) Affine3

// Split inversion (L1 §4.4):
func (a Affine3) Inverse() Affine3        // fast: assumes orthonormal + uniform scale
func (a Affine3) AffineInverse() Affine3  // general: handles non-uniform scale/shear
```

`Inverse` is documented as undefined-behavior for non-uniform scale; callers
with non-uniform scale MUST use `AffineInverse`. The godoc carries an
`AI-Meta` stability note so downstream agents pick the correct one.

<!-- TBD (L1 §5 Q2): expose SIMD as an explicit design layer vs. keep it an
     internal optimization detail behind Affine3/Mat4. Current decision:
     internal-only; revisit if profiling shows transform composition is hot. -->

### 5.5 Direction and Rotation Types

```go
type Dir2 struct{ v Vec2 } // unexported field ⇒ INV-3 unbreakable externally
type Dir3 struct{ v Vec3 }

func NewDir2(v Vec2) (Dir2, bool)     // false if |v| < epsilon
func NewDir2Unchecked(v Vec2) Dir2    // caller guarantees unit length
func (d Dir2) Vec() Vec2

func NewDir3(v Vec3) (Dir3, bool)
func NewDir3Unchecked(v Vec3) Dir3
func (d Dir3) Vec() Vec3

type Rot2 struct{ angleRad float32 }
func Rot2FromDegrees(deg float32) Rot2
func Rot2FromRadians(rad float32) Rot2
func (r Rot2) Rotate(v Vec2) Vec2
```

### 5.6 Isometry Types

```go
type Isometry2D struct {
    Rotation    Rot2
    Translation Vec2
}
type Isometry3D struct {
    Rotation    Quat
    Translation Vec3
}
```

```
func (i Isometry3D) TransformPoint(p Vec3) Vec3
func (i Isometry3D) Inverse() Isometry3D
func (i Isometry3D) Mul(o Isometry3D) Isometry3D
```

`Isometry2D` exposes the same three methods over `Vec2`/`Rot2`.

### 5.7 Geometric Primitives

```go
type Ray2D   struct{ Origin Vec2; Direction Dir2 }
type Ray3D   struct{ Origin Vec3; Direction Dir3 }
type AABB    struct{ Min, Max Vec3 }
type Sphere  struct{ Center Vec3; Radius float32 }
type Plane   struct{ Normal Dir3; Distance float32 }
type Frustum struct{ Planes [6]Plane }
```

Intersection tests map L1's `Option[f32]` → `(float32, bool)`:

```
func RayAABB(r Ray3D, b AABB) (float32, bool)    // hit distance
func RaySphere(r Ray3D, s Sphere) (float32, bool)
func AABBAABB(a, b AABB) bool
func SphereSphere(a, b Sphere) bool
func FrustumAABB(f Frustum, b AABB) bool          // visibility culling
func AABBContains(b AABB, p Vec3) bool
```

### 5.8 Color Types

```go
type Color interface {
    LinearRgba() LinearRgba // canonical conversion target
}

type Srgba      struct{ R, G, B, A uint8 }
type LinearRgba struct{ R, G, B, A float32 }
type Hsla       struct{ H, S, L, A float32 }
type Hsva       struct{ H, S, V, A float32 }
```

```
func (c Srgba) LinearRgba() LinearRgba       // gamma decode
func (c LinearRgba) Srgba() Srgba            // gamma encode
func (c LinearRgba) Hsla() Hsla
func (c LinearRgba) Hsva() Hsva
func (c Hsla) LinearRgba() LinearRgba
func (c Hsva) LinearRgba() LinearRgba
func (c LinearRgba) Lerp(o LinearRgba, t float32) LinearRgba // linear-space
```

All blending happens in `LinearRgba`; sRGB encode is the final output stage.

<!-- TBD (L1 §5 Q3): additional color spaces (OKLCH, CIE-Lab). Out of 0.1.0
     scope — the Color interface is open for extension without breaking callers. -->

### 5.9 Curves

```go
// Point constrains the element type of a curve to types that support the
// vector-space operations the evaluators need.
type Point interface {
    ~struct{ X, Y float32 } | ~struct{ X, Y, Z float32 } | ~float32
}
```

> The exact constraint set is provisional — Go type sets cannot name `Vec2`/
> `Vec3` structurally and via `~` simultaneously in all cases.
> <!-- TBD: finalize the Point constraint. Options: (a) method-based interface
>      { Add(P) P; Scale(float32) P }, (b) generic over a small vector-ops
>      adapter. Decide during implementation; (a) is the leading candidate. -->

```go
type CubicSegment[P any] struct { /* 4 control coefficients */ }
func (s CubicSegment[P]) Position(t float32) P
func (s CubicSegment[P]) Velocity(t float32) P     // 1st derivative
func (s CubicSegment[P]) Acceleration(t float32) P // 2nd derivative

type RationalSegment[P any] struct { /* weighted control points */ }
func (s RationalSegment[P]) Position(t float32) P

type CubicCurve[P any] struct{ Segments []CubicSegment[P] }
func (c CubicCurve[P]) Position(t float32) P              // t spans full curve
func (c CubicCurve[P]) IterPositions(steps uint) iter.Seq[P] // Go 1.23+ range-over-func
```

`IterPositions` returns an `iter.Seq[P]` so callers can `range` it directly
with zero intermediate slice allocation.

### 5.10 Transform Interpolation (pure, no ECS)

Two-phase design from L1 §4.10. Lives in `pkg/math` because it operates purely
on `Affine3` — the *system* that calls it per frame lives in the hierarchy
package.

```go
type InterpMethod uint8
const (
    InterpLerp        InterpMethod = iota // translation-only delta
    InterpSlerp                            // rotation delta, uniform scale
    InterpScaledSlerp                      // rotation + non-uniform scale delta
)

type InterpolationParams struct {
    from, to Affine3
    method   InterpMethod
}

// Phase 1 — once per physics tick: analyze keyframes, pick method.
func PrepareInterpolation(from, to Affine3) InterpolationParams

// Phase 2 — every render frame: apply pre-selected method at t ∈ [0,1].
func (p InterpolationParams) Interpolate(t float32) Affine3
```

### 5.11 Internal Float Helpers

To keep the API `float32` while using stdlib `math` (`float64`), an unexported
helper layer wraps the conversions in one place:

```go
func sqrt(x float32) float32  { return float32(math.Sqrt(float64(x))) }
func sin(x float32) float32   { return float32(math.Sin(float64(x))) }
func cos(x float32) float32   { return float32(math.Cos(float64(x))) }
// ... acos, atan2, abs, etc.
```

<!-- TBD (L1 §5 Q4): build-time precision switch (build tag swapping the base
     float type float32↔float64) vs. separate suffixed float64 types. Leaning
     toward separate suffixed types (explicit, no hidden ABI change); a build
     tag would silently change struct sizes and GPU upload layout. Final call
     deferred to when float64 demand is concrete. -->

## 6. Implementation Notes

1. **Vec/Mat/Quat first** — every other type composes them. Land §5.1–§5.3
   with full benchmark coverage before anything else.
2. **Affine3 + primitives** — depend on Vec/Quat; parallel with Color.
3. **Color** — independent of the geometry types; can land in parallel with 2.
4. **Curves + TransformInterpolator** — depend on Vec/Affine3; land last.
5. The `Point` constraint (§5.9) is the single open design decision that may
   reshape the curve API — prototype it early to de-risk.

## 7. Drawbacks & Alternatives

- **Drawback**: value semantics copy structs by value; a `Mat4` is 64 bytes and
  is copied on every method call.
  **Alternative**: pointer-receiver in-place mutators.
  **Decision**: keep value semantics — it eliminates aliasing bugs in the
  parallel transform/physics systems (L1 INV-1, CLAUDE.md concurrency rules),
  and 64-byte copies stay in registers/stack; escape analysis keeps them
  off-heap. Revisit only if profiling proves it a hot spot.
- **Drawback**: `float32` base loses precision for large world coordinates.
  **Alternative**: `float64` everywhere.
  **Decision**: `float32` matches GPU upload and halves bandwidth; the float64
  question is tracked as a TBD (§5.1, §5.4, §5.11), not resolved here.
- **Drawback**: dropping the `A` (SIMD-aligned) suffix loses an explicit
  performance signal at the type name.
  **Alternative**: `unsafe`-aligned struct + assembly kernels.
  **Decision**: defer SIMD to an internal optimization (§5.4 TBD) — premature
  given no profiling data and the C-003 zero-external-dep constraint.

## Canonical References

<!-- MANDATORY for Stable status. List authoritative source files that downstream
     agents MUST read before implementing this spec. Use relative paths from
     project root. Stub state — fill with concrete files when implementation
     begins (Phase 3+, P3 Assets & Math). -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

<!-- Empty table = no canonical sources yet. Populate one row per authoritative
     file when implementation lands. Stable promotion requires ≥1 row AND a
     Stable L1 parent (l1-math-system.md is currently Draft). -->

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-17 | Initial L2 draft — Go translation of l1-math-system v0.3.0 (§4.1–§4.13). Maps Option[T]→(T,bool), value-receiver immutability, column-major Mat layout. Open L1 questions Q1–Q4 carried as TBD markers. |
