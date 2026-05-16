# Physics System

**Version:** 0.1.0
**Status:** Draft
**Layer:** concept

## Overview

The Physics System defines how rigid body simulation integrates with the Neu engine. A dedicated Physics SubApp owns its own internal state, receives extracted transform data each frame, runs deterministic fixed-timestep simulation, and writes results back to the main World via a writeback phase. A pluggable `PhysicsBackend` interface allows swapping the underlying solver without touching game code. The default backend is a minimal impulse-based solver written in pure Go with zero external dependencies. A Rapier or Jolt backend can be substituted later via a plugin.

## Related Specifications

- [app-framework.md](l1-app-framework.md) — SubApp pattern, FixedUpdate schedule, ServiceRegistry
- [time-system.md](l1-time-system.md) — FixedTime drives the physics step rate
- [math-system.md](l1-math-system.md) — TransformInterpolator for smooth rendering between steps
- [component-system.md](l1-component-system.md) — AssociatedDataMap for PhysicsBodyHandle per entity
- [render-core.md](l1-render-core.md) — RID + command queue pattern reused for PhysicsBodyHandle
- [event-system.md](l1-event-system.md) — Collision events delivered through the standard event bus
- [hierarchy-system.md](l1-hierarchy-system.md) — Compound colliders follow entity hierarchy
- [diagnostic-system.md](l1-diagnostic-system.md) — Collider wireframes rendered as gizmos

## 1. Motivation

Physics simulation imposes unique demands the main ECS loop cannot satisfy directly. The solver must run at a fixed, deterministic rate independent of the render frame rate. Body state (positions, velocities) lives inside the solver, not in ECS tables, to preserve solver consistency. Collision detection requires spatial acceleration structures that are owned and managed by the backend, not scattered across archetype tables.

Without a dedicated server:

- Physics would pollute the main World with solver-internal state that game code should never touch.
- Fixed timestep logic would have to be reimplemented by every project.
- Swapping backends (e.g., upgrading from a simple AABB solver to a full rigid body engine) would require rewriting game code.
- Interpolating rendered positions between physics steps would have no standard solution.
- Determinism — required for networking and replay — would be impossible to guarantee.

## 2. Constraints & Assumptions

- The physics step always runs inside `FixedUpdate`. No physics computation happens in `Update`.
- The Physics SubApp is isolated: game systems read `GlobalTransform` (interpolated), not raw solver output.
- `PhysicsBodyHandle` (a `RID`) is the only reference game code holds to a physics body. No pointers into solver internals.
- Body creation and destruction are deferred via Commands and applied at the `Sync` phase boundary.
- The default backend targets determinism: same inputs, same outputs across platforms, given the same fixed timestep.
- CGo is not permitted in the default backend (C24). External backends may use CGo but must be behind a build tag.
- The physics server never calls into the renderer or audio system directly.

## 3. Core Invariants

- **INV-1**: Physics simulation runs exclusively in `FixedUpdate` at a constant timestep. Variable-rate frames never affect the simulation.
- **INV-2**: `GlobalTransform` seen by render systems is always an interpolated value between the last two physics steps, never raw solver output.
- **INV-3**: `PhysicsBodyHandle` is valid from the frame a `RigidBody` component is inserted until the frame it is removed. Access outside this window is a no-op with a logged warning.
- **INV-4**: Collision events are delivered after the physics step completes and before the next `FixedUpdate` begins. Handlers always see a consistent post-step world state.
- **INV-5**: All body creation, destruction, and property mutations originating from game systems go through the command queue. The solver never receives mutations mid-step.
- **INV-6**: The Physics SubApp has no read access to the Render SubApp and vice versa. Data flows only through the main World.

## 4. Detailed Design

### 4.1 Physics SubApp and Execution Phases

The server runs as a SubApp with its own internal world. Each fixed timestep frame executes four sequential phases:

```plaintext
Main World (Update)
      │
      ▼
Phase 1 — Sync
  Read Commands queue: create/destroy bodies, update properties.
  Extract Transform changes from main World into physics body poses.
  Apply external forces and impulses queued this frame.

Phase 2 — Step
  Broad-phase: rebuild/update spatial acceleration structure (BVH).
  Narrow-phase: compute contact manifolds for overlapping pairs.
  Solver: resolve constraints and contacts (impulse iterations).
  Integrate: advance positions and velocities.

Phase 3 — Events
  Diff contact pairs against previous step.
  Emit CollisionStarted / CollisionPersisting / CollisionEnded events.
  Emit TriggerEntered / TriggerExited events for sensor bodies.

Phase 4 — WriteBack
  Copy solver body poses into PhysicsTransform components on main World entities.
  The render pipeline reads these and applies TransformInterpolator (math-system §4.10).
```

The `FixedUpdate` schedule runs this sequence 0–N times per frame depending on accumulated time (time-system §4.3).

### 4.2 PhysicsBackend Interface

The solver is hidden behind an interface. The default implementation (§4.8) satisfies it with pure Go.

```plaintext
PhysicsBackend interface:
  Step(dt: float32)

  CreateBody(desc: BodyDescriptor) -> PhysicsBodyHandle
  DestroyBody(handle: PhysicsBodyHandle)

  SetBodyPose(handle: PhysicsBodyHandle, position: Vec3, rotation: Quat)
  GetBodyPose(handle: PhysicsBodyHandle) -> (position: Vec3, rotation: Quat)

  SetLinearVelocity(handle: PhysicsBodyHandle, velocity: Vec3)
  GetLinearVelocity(handle: PhysicsBodyHandle) -> Vec3

  SetAngularVelocity(handle: PhysicsBodyHandle, velocity: Vec3)
  GetAngularVelocity(handle: PhysicsBodyHandle) -> Vec3

  ApplyImpulse(handle: PhysicsBodyHandle, impulse: Vec3, point: Vec3)
  ApplyTorqueImpulse(handle: PhysicsBodyHandle, torque: Vec3)

  SetGravityScale(handle: PhysicsBodyHandle, scale: float32)

  AddCollider(body: PhysicsBodyHandle, desc: ColliderDescriptor) -> ColliderHandle
  RemoveCollider(handle: ColliderHandle)

  RayCast(ray: Ray3D, filter: QueryFilter) -> Option[RayCastHit]
  ShapeCast(shape: ColliderShape, pose: ShapePose, dir: Vec3, filter: QueryFilter) -> Option[ShapeCastHit]
  OverlapShape(shape: ColliderShape, pose: ShapePose, filter: QueryFilter) -> []PhysicsBodyHandle

  ContactPairs() -> []ContactPair
  Reset()
```

`PhysicsBodyHandle` is an opaque `uint64` (RID pattern from render-core §4.5). The backend owns all internal state; callers never hold pointers into solver memory.

### 4.3 BodyDescriptor

Describes the physical properties of a body at creation time:

```plaintext
BodyDescriptor
  body_type:       BodyType     // Static | Kinematic | Dynamic
  position:        Vec3
  rotation:        Quat
  linear_damping:  float32      // default 0.0
  angular_damping: float32      // default 0.05
  gravity_scale:   float32      // default 1.0
  continuous_cd:   bool         // CCD for fast-moving bodies
  locked_axes:     LockedAxes   // bitfield: LockLinearX | ... | LockAngularZ
  user_data:       uint64       // stores packed EntityID for event correlation
```

`user_data` holds the packed `EntityID` so collision event handlers can map `PhysicsBodyHandle` back to an ECS entity without a separate reverse lookup table.

### 4.4 Body Types

```plaintext
BodyType:
  Static      — infinite mass, never moves, collidable (terrain, walls)
  Kinematic   — moved by game code via SetBodyPose; velocity computed implicitly
  Dynamic     — fully simulated: mass, velocity, forces, gravity
```

Static bodies are not integrated each step — they are fixed reference points. Kinematic bodies compute velocity implicitly from the delta between their previous and current pose. Dynamic bodies are fully integrated.

### 4.5 PhysicsTransform Component

The writeback target. Written by the server each step, read by the interpolation system:

```plaintext
PhysicsTransform
  position_prev: Vec3   // position at step N-1
  rotation_prev: Quat   // rotation at step N-1
  position_curr: Vec3   // position at step N
  rotation_curr: Quat   // rotation at step N
```

The render interpolation system in `PostUpdate` computes:

```plaintext
t = FixedTime.OverstepFraction()
rendered_position = lerp(position_prev, position_curr, t)
rendered_rotation = slerp(rotation_prev, rotation_curr, t)
```

This result is written to `GlobalTransform`, ensuring smooth visuals at any render frame rate.

### 4.6 AssociatedDataMap for Body Handles

The physics sync system uses the associated data pattern (component-system §4.10) to cache `PhysicsBodyHandle` per entity without polluting the `RigidBody` component:

```plaintext
PhysicsBodyData
  handle:           PhysicsBodyHandle
  entity:           Entity
  last_body_type:   BodyType            // detect type changes requiring body recreation
  collider_handles: []ColliderHandle
```

On `RigidBody` insertion: `GenerateData` calls `backend.CreateBody`, stores the handle.
On `RigidBody` removal: `Cleanup` calls `backend.DestroyBody`, releases collider handles.
On `BodyType` field change: `IsDataValid` returns false, triggers recreation.

### 4.7 Command Queue and Sync Phase

Game systems queue mutations instead of calling backend methods directly:

```plaintext
PhysicsCommandQueue (Resource)
  commands: []PhysicsCommand

PhysicsCommand variants:
  CreateBody(entity, BodyDescriptor)
  DestroyBody(entity)
  SetPose(entity, Vec3, Quat)
  SetVelocity(entity, linear: Vec3, angular: Vec3)
  ApplyImpulse(entity, impulse: Vec3, point: Vec3)
  ApplyForce(entity, force: Vec3, point: Vec3)
  UpdateCollider(entity, ColliderDescriptor)
```

The Sync phase drains this queue before `backend.Step`. Commands are applied in FIFO order. Commands targeting a destroyed entity are silently dropped with a debug-level log.

### 4.8 Default Backend — Impulse Solver

The built-in backend targets correctness and determinism over raw performance. Suitable for games with up to ~1000 dynamic bodies.

Pipeline per step:

```plaintext
1. Integrate forces   → new velocities (semi-implicit Euler)
2. Broad-phase        → AABB BVH traversal, candidate pairs
3. Narrow-phase       → GJK + EPA for convex shapes, SAT for boxes/spheres
4. Contact solver     → sequential impulse, configurable iterations (default 10)
5. Integrate velocities → new positions
6. Update BVH leaf AABBs
```

Determinism constraints:

- No `math.Rand` inside the step. Any stochastic feature uses a per-world PRNG with explicit seed.
- Only `float32` arithmetic — no platform-specific intrinsics.
- Contact pair ordering sorted by `(handleA, handleB)` before solver iterations.

### 4.9 Gravity and Global Settings

```plaintext
PhysicsSettings (Resource)
  gravity:                 Vec3      // default (0, -9.81, 0)
  solver_iterations:       int       // default 10
  max_ccd_substeps:        int       // default 4
  sleep_linear_threshold:  float32   // default 0.01 m/s
  sleep_angular_threshold: float32   // default 0.01 rad/s
  sleep_frames_required:   int       // idle frames before sleep, default 10
```

`PhysicsSettings` is a main World resource. Changes are picked up at the next Sync phase.

### 4.10 Physics Service

The server registers in `ServiceRegistry` (app-framework §4.12) for cross-system access without package coupling:

```plaintext
services.Register[PhysicsServer](server)

// From any system:
physics := services.Get[PhysicsServer]()
hit = physics.RayCast(ray, filter)
```

In headless builds where `PhysicsPlugin` is absent, `Get[PhysicsServer]` returns `false` gracefully.

### 4.11 Collision Events

After each step the server diffs the current contact set against the previous step and emits typed events:

```plaintext
CollisionStarted   { entity_a, entity_b: Entity, manifold: ContactManifold }
CollisionPersisting{ entity_a, entity_b: Entity, manifold: ContactManifold }
CollisionEnded     { entity_a, entity_b: Entity }
TriggerEntered     { entity_sensor, entity_other: Entity }
TriggerExited      { entity_sensor, entity_other: Entity }

ContactManifold
  contact_points: []ContactPoint
  normal:         Vec3        // world-space, points from B toward A

ContactPoint
  position: Vec3
  depth:    float32           // penetration depth
  impulse:  float32           // resolved impulse magnitude
```

Events are sent via `EventWriter` in the Events phase and readable by game systems in the following `FixedUpdate` or `Update`.

### 4.12 PhysicsPlugin

Registers all components, resources, systems, and the SubApp:

```plaintext
PhysicsPlugin
  backend: PhysicsBackend   // default: ImpulseBackend{}

Build(app):
  app.InsertResource(PhysicsSettings{...})
  app.InsertResource(PhysicsCommandQueue{})
  app.AddEvent[CollisionStarted]()
  app.AddEvent[CollisionPersisting]()
  app.AddEvent[CollisionEnded]()
  app.AddEvent[TriggerEntered]()
  app.AddEvent[TriggerExited]()
  app.RegisterComponent[RigidBody]()
  app.RegisterComponent[PhysicsTransform]()
  app.InsertSubApp("physics", PhysicsSubApp{backend})
  app.AddSystems(PostUpdate, interpolate_physics_transforms)
  services.Register[PhysicsServer](server)
```

## 5. Open Questions

- Should the default backend support a 2D mode (locked Z-axis, 2D broadphase), or is a separate `Physics2DPlugin` the right boundary?
- Joints (Fixed, Revolute, Spring, Distance) — part of this spec or a dedicated `joints.md`?
- Networking and rollback: should `PhysicsBackend` expose `SaveState() / RestoreState()` snapshots for deterministic rollback, or is that a separate layer above the server?
- Should `gravity` be overridable per-body via a `GravityOverride` component, or is `gravity_scale` (already in BodyDescriptor) sufficient?
- CCD implementation strategy: swept-sphere approximation or full shape cast per substep?

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
| 0.1.0 | 2026-03-27 | Initial draft — SubApp phases, backend interface, impulse solver, interpolation, collision events | [examples/physics](file:///d:/Projects/src/github.com/neuengine/neu/examples/physics) |
