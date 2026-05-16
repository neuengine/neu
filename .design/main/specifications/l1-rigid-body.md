# Rigid Body

**Version:** 0.1.0
**Status:** Draft
**Layer:** concept

## Overview

The `RigidBody` component marks an entity as a participant in physics simulation. It declares the body's physical properties — mass, damping, material response, and axis constraints — without holding any solver-internal state. The Physics Server reads this component during the Sync phase to create or update the corresponding body inside the backend. The component is pure data; all runtime state (position, velocity, contact forces) lives inside the backend and is surfaced through `PhysicsTransform` and collision events.

## Related Specifications

- [physics-system.md](l1-physics-system.md) — Server that consumes RigidBody and manages PhysicsBodyHandle
- [collider.md](l1-collider.md) — Collision shape attached alongside RigidBody
- [hierarchy-system.md](l1-hierarchy-system.md) — Transform component required alongside RigidBody
- [component-system.md](l1-component-system.md) — Required component pattern, OnAdd/OnRemove hooks
- [change-detection.md](l1-change-detection.md) — Field changes detected to sync properties to backend

## 1. Motivation

A component-based rigid body model lets game code express physics intent declaratively. Adding a `RigidBody` to an entity is sufficient to bring it into the simulation — no imperative API calls, no manual handle management. Removing it cleanly exits the simulation. Field mutations (changing mass, toggling gravity) are detected via change ticks and forwarded to the backend without game code knowing whether the backend is the impulse solver or Rapier.

## 2. Constraints & Assumptions

- `RigidBody` requires `Transform` (position and rotation source) and at least one `Collider` on the same entity or a child entity.
- Adding `RigidBody` without `Transform` is a logged warning; the body is created at the world origin.
- `RigidBody` fields are authoritative at creation time. After that, the backend is authoritative for `Dynamic` bodies — game code must use velocity and impulse APIs rather than writing `Transform` directly.
- For `Kinematic` bodies `Transform` remains the authoritative input; the backend computes implicit velocity from the pose delta.
- Mass properties (center of mass, inertia tensor) are auto-computed from attached collider shapes unless explicitly overridden.

## 3. Core Invariants

- **INV-1**: One `RigidBody` per entity. A second insertion on the same entity is a hard error at component insertion time.
- **INV-2**: Removing `RigidBody` from a live entity removes it from the simulation cleanly — no dangling handles, no orphaned contact pairs.
- **INV-3**: `BodyType::Static` bodies never accumulate velocity. Impulse or force commands targeting a Static body are logged and ignored.
- **INV-4**: Axis locks are enforced at the solver level. A body with `LockLinearZ` will never acquire Z velocity regardless of forces applied.
- **INV-5**: `RigidBody` never stores solver-internal state (velocity, contact forces). Those are read-only outputs surfaced via `PhysicsServer` query APIs.

## 4. Detailed Design

### 4.1 RigidBody Component

```plaintext
RigidBody
  body_type:        BodyType          // Static | Kinematic | Dynamic
  mass:             MassProperties    // Auto | Override(MassOverride)
  linear_damping:   float32           // velocity decay per second, default 0.0
  angular_damping:  float32           // angular velocity decay per second, default 0.05
  gravity_scale:    float32           // multiplier on world gravity, default 1.0
  continuous_cd:    bool              // enable CCD for fast-moving bodies, default false
  locked_axes:      LockedAxes        // bitfield of locked degrees of freedom
  dominance:        int8              // collision dominance group (-127..127), default 0
  sleeping:         SleepMode         // Auto | ForceAwake | ForceSleep
```

### 4.2 BodyType

```plaintext
BodyType:
  Static
    — Infinite effective mass. Never moves under simulation forces.
    — Collides with Dynamic and Kinematic bodies.
    — Velocity and impulse commands: no-op with debug warning.
    — Use for: terrain, walls, platforms that never change position.

  Kinematic
    — Moved by writing Transform or via SetBodyPose command.
    — Backend computes implicit velocity = (curr_pose - prev_pose) / dt.
    — Pushes Dynamic bodies but is not pushed by them.
    — Use for: elevators, moving platforms, animated obstacles.

  Dynamic
    — Fully simulated: mass, gravity, forces, impulses, contacts.
    — Position and rotation written by backend each step into PhysicsTransform.
    — Game code must not write Transform directly for Dynamic bodies.
    — Use for: projectiles, ragdolls, physics props.
```

Changing `body_type` at runtime is supported. The AssociatedDataMap detects the mismatch via `IsDataValid`, destroys the old backend body, and creates a new one. Velocity is reset to zero on recreation.

### 4.3 MassProperties

```plaintext
MassProperties:
  Auto
    — Mass and inertia tensor computed from attached Collider shapes and
      their density field. Center of mass at geometric centroid.

  Override(MassOverride)
    — Explicit values that replace auto-computation entirely.

MassOverride
  mass:           float32   // kg, must be > 0 for Dynamic bodies
  center_of_mass: Vec3      // local space offset, default (0, 0, 0)
  inertia_tensor: Vec3      // principal moments (Ixx, Iyy, Izz)
                            // off-diagonal terms assumed zero
```

`Auto` is the recommended default. `Override` is for vehicles or weapons where the visual collider geometry does not reflect the real mass distribution.

### 4.4 LockedAxes

A bitfield restricting degrees of freedom at the solver level:

```plaintext
LockedAxes (bitfield):
  LockLinearX  = 0b000001
  LockLinearY  = 0b000010
  LockLinearZ  = 0b000100
  LockAngularX = 0b001000
  LockAngularY = 0b010000
  LockAngularZ = 0b100000

Convenience combinations:
  Lock2D  = LockLinearZ | LockAngularX | LockAngularY
            // restricts body to the XY plane — standard 2D physics mode
  LockAll = 0b111111
            // body cannot move (prefer Static instead for fixed geometry)
```

`Lock2D` is the primary real-world use case: a 3D physics world used for a 2D game. Bodies tagged with `Lock2D` behave like classic 2D rigid bodies without a separate physics pipeline.

### 4.5 SleepMode

```plaintext
SleepMode:
  Auto
    — Backend decides when to sleep based on velocity threshold and idle
      frame count (thresholds in PhysicsSettings). Woken automatically by
      nearby collisions or applied forces.

  ForceAwake
    — Body never sleeps. Use for bodies driven by game logic every frame,
      such as a platform that must respond instantly to player input.

  ForceSleep
    — Body is immediately deactivated. Useful for pre-placed props that
      should start dormant and wake on first contact.
```

### 4.6 Dominance

An `int8` in the range -127..127 controlling collision response when mass ratios are extreme:

```plaintext
dominance: int8   // default 0

Resolution rule:
  diff = body_a.dominance - body_b.dominance
  if diff > 0 : body_a treated as infinitely heavy — pushes body_b, barely moves itself
  if diff < 0 : body_b treated as infinitely heavy
  if diff == 0: standard mass-based impulse response
```

### 4.7 Required Components

`RigidBody` declares `Transform` and `PhysicsTransform` as required components (component-system §4.3):

```plaintext
RigidBody.Required():
  Transform{}          // position/rotation source for Kinematic; read by Sync phase
  PhysicsTransform{}   // writeback target from backend; read by interpolation system
```

### 4.8 Lifecycle and Change Detection

`OnAdd` hook enqueues `CreateBody` in `PhysicsCommandQueue`.
`OnRemove` hook enqueues `DestroyBody`.
Field changes are detected in the Sync phase via change ticks and mapped to targeted backend commands:

| Field changed | Command issued |
| :--- | :--- |
| `body_type` | DestroyBody + CreateBody (full recreation) |
| `mass` | UpdateMass |
| `linear_damping` / `angular_damping` | UpdateDamping |
| `gravity_scale` | SetGravityScale |
| `locked_axes` | UpdateLockedAxes |
| `continuous_cd` | UpdateCCD |
| `sleeping` | SetSleepMode |

### 4.9 Velocity and Force API

Game code does not write velocity directly to `RigidBody`. Commands are queued via `PhysicsCommandQueue` or through the `PhysicsServer` service:

```plaintext
// Via service (preferred):
physics.SetLinearVelocity(entity, Vec3{0, 5, 0})
physics.ApplyImpulse(entity, Vec3{0, 100, 0}, at_point)
physics.ApplyForce(entity, force_vec, at_point)

// Reading back:
vel = physics.GetLinearVelocity(entity)    // Vec3, zero if body not found
ang = physics.GetAngularVelocity(entity)
```

### 4.10 2D Physics Shortcut

For 2D games, a `RigidBody2D` convenience bundle pre-configures axis locks:

```plaintext
RigidBody2D = RigidBody {
  locked_axes: LockedAxes::Lock2D,
  ...other fields at user-specified defaults...
}
```

### 4.11 Sleep Thresholds

Sleep sensitivity is configured globally in `PhysicsSettings` (physics-system §4.9):

```plaintext
PhysicsSettings (additions)
  sleep_linear_threshold:  float32   // default 0.01 m/s
  sleep_angular_threshold: float32   // default 0.01 rad/s
```

## 5. Open Questions

- Should `MassOverride` expose the full 3×3 inertia tensor for irregular shapes, or is the diagonal approximation sufficient for v1?
- Should collision layer membership live on `RigidBody` (body-level filtering before shape tests) or entirely on `Collider` (shape-level)?
- How should kinematic body teleportation (large discontinuous position jump) be signalled to prevent incorrect implicit velocity spikes?
- Should a `Sensor` flag live on `RigidBody` for trigger volumes, or is it a property of `Collider`?
- `dominance` type: `int8` allows negative values for explicitly "lighter" bodies. Is this useful in practice or just confusing?

## Canonical References

<!-- MANDATORY for Stable status. List authoritative source files that downstream agents
     MUST read before implementing this spec. Use relative paths from project root.
     Stub state — fill with concrete files when implementation begins (Phase 1+). -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

<!-- Empty table = no canonical sources yet. Populate one row per authoritative file
     when implementation lands (Phase 1+). Stable promotion requires ≥1 row. -->

## Document History

| Version | Date | Description | Examples |
| :--- | :--- | :--- | :--- |
| 0.1.0 | 2026-03-27 | Initial draft — component fields, body types, mass properties, axis locks, sleep, dominance, lifecycle hooks | [examples/physics](file:///d:/Projects/src/github.com/neuengine/neu/examples/physics) |
