# Joints

**Version:** 0.1.0
**Status:** Draft
**Layer:** concept

## Overview

Joints constrain the relative motion between two rigid bodies. Each joint type restricts a specific subset of the six degrees of freedom (three translational, three rotational) while allowing the rest to move freely. Joints are declared as ECS components on a dedicated joint entity that references the two bodies being connected. The Physics Server reads joint components during the Sync phase and maintains corresponding constraint objects inside the backend. Breaking forces, motor drives, and soft limits are all configured on the component.

## Related Specifications

- [physics-system.md](l1-physics-system.md) — Server Sync phase that creates and destroys joint constraints
- [rigid-body.md](l1-rigid-body.md) — The two bodies connected by a joint
- [component-system.md](l1-component-system.md) — OnAdd/OnRemove hooks, required components
- [event-system.md](l1-event-system.md) — JointBroken event delivered through the event bus
- [math-system.md](l1-math-system.md) — Vec3, Quat, coordinate frames

## 1. Motivation

Many game objects require constrained motion between two bodies. A door swings on a hinge. A vehicle's wheels rotate around an axle. A rope segment connects to the next. A crane lifts a crate on a cable. Without joints, these behaviours require manual velocity manipulation every frame — fragile, non-deterministic, and divergent under high loads. A constraint solver handles all of this correctly as part of the normal physics step.

## 2. Constraints & Assumptions

- Every joint connects exactly two bodies: `body_a` and `body_b`. One-body joints (anchored to the world) are expressed by setting `body_b = Entity::PLACEHOLDER`; the backend treats this as a connection to a static world anchor.
- Joint anchors are expressed in the local space of each body.
- Joints are broken (removed) when the constraint force exceeds `break_force` or `break_torque`. A `JointBroken` event is emitted. The joint entity is not despawned automatically.
- A joint between two `Static` bodies is a configuration error — logged and ignored.

## 3. Core Invariants

- **INV-1**: A joint entity must reference two distinct live entities via `body_a` and `body_b`.
- **INV-2**: When either referenced body is despawned, the joint is automatically removed from the backend and a `JointBroken` event is emitted with `reason: BodyDespawned`.
- **INV-3**: Joint limits are always expressed such that `lower_limit <= upper_limit`.
- **INV-4**: A motor and a position target on the same axis follow a priority rule: the motor takes precedence; position target is ignored.
- **INV-5**: Joint entities live independently of the bodies they connect, allowing for addition of SFX/VFX components to the joint itself.

## 4. Detailed Design

### 4.1 Joint Entity Pattern

Joints live on their own dedicated entities — they are not components on either body:

```plaintext
hinge_entity = commands.Spawn(
    Transform {},                         // world-space anchor position
    RevoluteJoint {
        body_a:       door_frame,
        body_b:       door,
        anchor_a:     Vec3{1.0, 1.0, 0},
        anchor_b:     Vec3{-0.5, 1.0, 0},
        axis:         Vec3{0, 1, 0},
    },
)
```

### 4.2 Joint Types

| Type | Locked DOF | Free DOF | Best For |
| :--- | :--- | :--- | :--- |
| **Fixed** | 6 (all) | 0 | Welding, compound structures |
| **Revolute** | 5 | 1 (rot) | Doors, wheels, hinges |
| **Prismatic** | 5 | 1 (trans) | Pistons, elevators, sliders |
| **Spherical** | 3 (trans) | 3 (rot) | Ragdoll joints, ropes, chains |
| **Distance** | 1 (dist) | 5 | Bungee cords, tethers, springs |
| **Generic** | Custom | Custom | Complex linkages, vehicle suspension |

### 4.3 JointMotor

A motor drives a joint axis toward a target velocity or position:

```plaintext
MotorMode:
  VelocityDrive   — maintains target_velocity; used for wheels, fans.
  PositionDrive   — moves toward target_position using PD controller; used for robotic arms.
```

`JointMotor` configuration: `mode`, `target_velocity`, `target_position`, `max_force`, `stiffness`, `damping`.

### 4.4 Soft Limits

When `stiffness > 0` on a limit, the joint becomes a spring rather than a hard stop. This avoids jitter at limits for heavy bodies and produces natural motion for suspension.

### 4.5 JointBroken Event

Emitted when constraints fail or bodies are despawned:
`JointBroken { joint_entity, body_a, body_b, break_force, reason: BreakReason }`

### 4.6 World Anchor

Setting `body_b = Entity::PLACEHOLDER` anchors `body_a` to a fixed world point. The `anchor_b` field is then interpreted as a world-space position.

## 5. Open Questions

- Should `RevoluteJoint` support a second motor for universal joints?
- Stabilisation: Baumgarte vs pseudo-velocity correction (backend detail).
- `max_force` default value: infinity vs a safe finite limit.

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
| 0.1.0 | 2026-03-27 | Initial draft — joint types (6), motors, soft limits, world anchor | [examples/physics](file:///d:/Projects/src/github.com/teratron/boltengine/examples/physics) |
