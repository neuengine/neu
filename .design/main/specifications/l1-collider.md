# Collider

**Version:** 0.1.0
**Status:** Draft
**Layer:** concept

## Overview

The `Collider` component defines the collision shape of an entity. It can live on the same entity as `RigidBody` (simple body) or on a child entity (compound body). The Physics Server collects all `Collider` components reachable from a `RigidBody` — directly or through the child hierarchy — and registers them as shapes attached to that body's backend handle. Shape geometry, physical material, sensor mode, and collision filtering are all declared here. Runtime state (active contacts, normal forces) is never stored in this component.

## Related Specifications

- [physics-system.md](l1-physics-system.md) — Server that reads Collider and calls backend.AddCollider
- [rigid-body.md](l1-rigid-body.md) — Parent body that owns colliders
- [hierarchy-system.md](l1-hierarchy-system.md) — Child entity colliders traversed via ChildOf
- [component-system.md](l1-component-system.md) — OnAdd/OnRemove hooks, required components
- [math-system.md](l1-math-system.md) — Vec3, Quat, AABB, Sphere primitives

## 1. Motivation

Separating shape from body is necessary for three reasons. First, a single rigid body often needs multiple shapes — a car chassis with four wheel colliders, a character capsule with a feet sphere for stairs. Bundling all shapes into one component would require a dynamic array and make individual shape mutation awkward. Second, child-entity colliders let designers position and rotate each shape independently using the standard `Transform` system rather than a bespoke offset API. Third, the same `Collider` component can exist on entities that have no `RigidBody` at all — static trigger zones, one-sided walls, and sensor areas only need a shape and filtering rules.

## 2. The Two Approaches — Analysis

This section is the architectural decision point. Both models are described fully before the recommendation.

### 2.1 Model A — Sibling Component (flat)

`Collider` lives on the same entity as `RigidBody`. Offset and rotation are explicit fields on the component.

```plaintext
// Entity: player
//   RigidBody { body_type: Dynamic }
//   Collider  { shape: Capsule{r:0.4, h:1.8}, offset: Vec3{0,0.9,0} }
```

Compound shapes require multiple `Collider` components on the same entity. Since ECS typically allows only one component per type, this forces a workaround: either an index slot (`Collider1`, `Collider2` — ugly), a dynamic array inside one component (`Collider { shapes: []ShapeEntry }` — breaks the ECS single-responsibility principle), or a dedicated list component (`ColliderSet`).

**Pros:**

- Simple common case — one entity, one shape, no hierarchy needed.
- All physics data visible on one entity in the inspector.
- No child entity management overhead.

**Cons:**

- Compound shapes are awkward — no natural way to have multiple `Collider` components on one entity in standard ECS.
- Offset is a physics-only transform, disconnected from the visual transform.
- Animating a collider offset requires updating a field, not moving an entity.

### 2.2 Model B — Child Entity (hierarchical) ✓ Recommended

Each `Collider` lives on its own entity, parented to the `RigidBody` entity via `ChildOf`. The shape's world offset is its `Transform` relative to the parent.

```plaintext
// Entity: player (RigidBody)
//   └── Entity: body_collider (Collider { shape: Capsule{r:0.4, h:1.8} })
//   └── Entity: feet_collider  (Collider { shape: Sphere{r:0.35} }, Transform offset -0.9Y)
```

The Physics Server traverses the `RigidBody` entity's child hierarchy, collects all `Collider` children, and registers each as an attached shape with its local transform as the offset.

**Pros:**

- Compound shapes are natural — just add more child entities.
- Offsets use the standard `Transform` system, consistent with the rest of the engine.
- Collider children can be added/removed at runtime like any entity.
- Animating a collider position is just animating a `Transform`.
- Editor can visualise and manipulate each shape independently.

**Cons:**

- Simple case (one body, one shape) requires spawning a child entity — more boilerplate.
- Traversal cost: Sync phase must walk the child hierarchy each step to detect new or removed colliders.
- Slightly more complex inspector view — physics body spans two or more entities.

### 2.3 Recommendation — Model B with a convenience bundle

**Model B (child entity) is the recommended architecture.**

The core reason: compound shapes are not an edge case in games. Characters, vehicles, and interactive props almost always need at least two shapes. Model A has no clean answer for compound shapes within standard ECS conventions, while Model B handles them naturally.

The boilerplate objection is solved by a `ColliderBundle` that spawns both the `RigidBody` entity and a single child `Collider` entity in one command, making the simple case as terse as Model A:

```plaintext
commands.Spawn(
    RigidBodyBundle::dynamic()
        .with_collider(Collider { shape: Capsule{r:0.4, h:1.8} })
)
// Internally spawns:
//   Entity A: RigidBody, Transform, PhysicsTransform
//   Entity B: Collider, Transform, ChildOf{A}
```

The traversal cost objection is addressed by the AssociatedDataMap: collider handles are cached per entity and only recomputed when the child set changes (detected via change ticks on `Children`).

This model also aligns with how Godot and Unreal handle compound shapes (child nodes/components) and is consistent with how the engine already treats visual meshes and audio emitters as child entities.

## 3. Constraints & Assumptions

- A `Collider` without a `RigidBody` ancestor is a static trigger volume. It participates in overlap queries but generates no contact forces.
- A `Collider` with `sensor: true` generates `TriggerEntered`/`TriggerExited` events but no contact response.
- `Collider` requires `Transform` (for its local offset relative to the body).
- Shape geometry is immutable after creation. Changing `shape` triggers collider recreation in the backend.
- A `RigidBody` with no reachable `Collider` in its subtree generates a logged warning and participates in simulation with no collision response (ghost body).
- Maximum compound shape count per body is backend-defined; the default impulse solver supports up to 32 shapes per body.

## 4. Core Invariants

- **INV-1**: `Collider` is always a leaf in the physics hierarchy. A `Collider` entity must not itself be a `RigidBody`.
- **INV-2**: The local transform of a `Collider` entity (relative to its `RigidBody` ancestor) is the shape offset inside the body. It is baked in at Sync time — runtime `Transform` changes on collider entities are picked up each step.
- **INV-3**: `sensor: true` colliders generate events but zero contact impulse. They never affect the velocity of any body.
- **INV-4**: Removing a `Collider` entity (or its `ChildOf` link) removes the shape from the backend body in the next Sync phase. No dangling shape handles.
- **INV-5**: `density` on a `Collider` feeds into `MassProperties::Auto` on the parent `RigidBody`. Changing density triggers mass recomputation.

## 5. Detailed Design

### 5.1 Collider Component

```plaintext
Collider
  shape:              ColliderShape       // geometry (see §5.2)
  sensor:             bool                // true = trigger only, no contact response
  density:            float32             // kg/m³, used for Auto mass computation, default 1000.0
  friction:           float32             // surface friction coefficient, default 0.5
  restitution:        float32             // bounciness (0 = inelastic, 1 = perfectly elastic)
  friction_combine:   CombineRule         // how friction blends with the other body's value
  restitution_combine:CombineRule         // how restitution blends
  collision_groups:   CollisionGroups     // membership and filter mask (see §5.5)
  contact_force_threshold: float32        // min impulse to emit ContactForceEvent, default 0 (disabled)
```

### 5.2 ColliderShape

```plaintext
ColliderShape (enum):

  Sphere     { radius: float32 }

  Box        { half_extents: Vec3 }
             // axis-aligned box in local space

  Capsule    { radius: float32, half_height: float32 }
             // along local Y axis; total height = 2*half_height + 2*radius

  Cylinder   { radius: float32, half_height: float32 }
             // along local Y axis

  Cone       { radius: float32, half_height: float32 }
             // tip at +Y, base at -Y

  ConvexHull { points: []Vec3 }
             // convex hull computed from point cloud at registration time
             // original points not stored after hull is built

  TriMesh    { vertices: []Vec3, indices: []uint32 }
             // non-convex static mesh, only valid on Static or Sensor bodies
             // Dynamic bodies with TriMesh are rejected at Sync time

  HeightField { heights: [][]float32, scale: Vec3 }
               // grid-based terrain shape, Static only

  Compound   // not a shape itself — signals that children provide all shapes
             // used on a RigidBody that has no own Collider, only child Colliders
```

Shape selection guidelines:

| Shape | Best for | Notes |
| :--- | :--- | :--- |
| Sphere | Balls, projectiles, sensors | Cheapest intersection test |
| Box | Crates, walls, platforms | SAT-based, efficient |
| Capsule | Characters, barrels | Standard character collider |
| Cylinder | Wheels, pillars | Slightly more expensive than Capsule |
| ConvexHull | Vehicles, irregular props | Auto-computed from mesh, max 255 verts |
| TriMesh | Terrain, complex static geometry | Static/Sensor only |
| HeightField | Outdoor terrain | Most efficient for height-mapped ground |

### 5.3 CombineRule

Controls how two colliding surfaces blend their friction or restitution values:

```plaintext
CombineRule:
  Average    — (a + b) / 2          // default for friction
  Minimum    — min(a, b)
  Maximum    — max(a, b)            // default for restitution
  Multiply   — a * b
```

If the two bodies use different `CombineRule` values for the same property, the rule with higher precedence wins: `Maximum > Multiply > Minimum > Average`.

### 5.4 ColliderShape Offset

The shape's position and rotation inside the body are the `Transform` of the `Collider` entity relative to its `RigidBody` ancestor:

```plaintext
shape_offset   = Transform.translation   // local position offset
shape_rotation = Transform.rotation      // local rotation
```

No separate `offset` field exists on `Collider`. This keeps offsets in the standard transform system — the editor, gizmo system, and animation system all work without physics-specific code.

### 5.5 CollisionGroups

Bitfield-based filtering that controls which pairs of colliders can interact:

```plaintext
CollisionGroups
  membership: uint32   // which groups this collider belongs to
  filter:     uint32   // which groups this collider can interact with

Interaction rule:
  can_interact(a, b) =
    (a.membership & b.filter) != 0
    AND
    (b.membership & a.filter) != 0
    // both sides must agree
```

Predefined group constants (bit positions 0–31 available to users):

```plaintext
GROUP_DEFAULT  = 0x0001   // all bodies by default
GROUP_PLAYER   = 0x0002   // suggested — not enforced by engine
GROUP_ENEMY    = 0x0004
GROUP_TERRAIN  = 0x0008
GROUP_TRIGGER  = 0x0010
GROUP_DEBRIS   = 0x0020
// bits 6–31 available for game-specific groups
```

A collider with `membership = GROUP_PLAYER, filter = GROUP_TERRAIN | GROUP_ENEMY` will collide with terrain and enemies but pass through other players and debris.

### 5.6 Contact Force Events

When `contact_force_threshold > 0`, the backend emits a `ContactForceEvent` whenever the resolved impulse on this collider exceeds the threshold:

```plaintext
ContactForceEvent
  entity_self:  Entity
  entity_other: Entity
  total_force:  Vec3      // world-space resultant force this frame
  max_impulse:  float32   // peak impulse across all contact points
```

This enables gameplay reactions to hard collisions (play a crunch sound, apply damage) without polling every frame. Setting `contact_force_threshold = 0` (default) disables the event entirely for performance.

### 5.7 Collider Lifecycle Hooks

`OnAdd` (on Collider entity):

- Find the nearest `RigidBody` ancestor via `Ancestors()` traversal.
- If found: enqueue `AddCollider(body_handle, ColliderDescriptor)` in `PhysicsCommandQueue`.
- If not found: register as a standalone static collider (trigger zone).

`OnRemove` (on Collider entity):

- Enqueue `RemoveCollider(collider_handle)`.
- Remove `ColliderHandle` from parent body's `PhysicsBodyData.collider_handles`.

`ChildOf` change (collider re-parented):

- Remove from old body, add to new body. Treated as remove + add.

Shape change (`ColliderShape` field mutated):

- Detected in Sync phase via change tick.
- Enqueue `RemoveCollider` + `AddCollider` with new descriptor.

### 5.8 Compound Body Example

```plaintext
// A car body with a box chassis and four wheel spheres:

car = commands.SpawnEmpty(RigidBody { body_type: Dynamic })

commands.Entity(car).WithChildren(func(b):
    // chassis
    b.Spawn(
        Transform { translation: Vec3{0, 0.5, 0} },
        Collider { shape: Box{ half_extents: Vec3{1.0, 0.4, 2.0} } },
    )
    // front-left wheel
    b.Spawn(
        Transform { translation: Vec3{-1.1, 0, 1.2} },
        Collider { shape: Sphere{ radius: 0.4 }, friction: 0.9 },
    )
    // front-right wheel
    b.Spawn(
        Transform { translation: Vec3{ 1.1, 0, 1.2} },
        Collider { shape: Sphere{ radius: 0.4 }, friction: 0.9 },
    )
    // ... rear wheels
)
```

Each child `Collider` entity has its own `Transform` for offset. The parent `RigidBody` entity has no `Collider` of its own. This is a valid configuration — the server traverses children and registers all four shapes as belonging to the car body.

### 5.9 Standalone Collider (No RigidBody)

A `Collider` entity without a `RigidBody` ancestor is registered as a static trigger:

```plaintext
// Invisible pickup trigger zone:
commands.Spawn(
    Transform { translation: Vec3{5, 1, 3} },
    Collider {
        shape: Sphere{ radius: 1.5 },
        sensor: true,
        collision_groups: CollisionGroups {
            membership: GROUP_TRIGGER,
            filter: GROUP_PLAYER,
        },
    },
)
```

No `RigidBody` needed. The server creates a static sensor body internally, owned by the collider entity itself.

### 5.10 ColliderBundle Convenience

For the common single-shape case, a bundle spawns the body and child collider together:

```plaintext
RigidBodyBundle
  body:     RigidBody
  shape:    ColliderShape      // spawned as child Collider entity automatically
  material: ColliderMaterial   // friction + restitution shorthand

// Usage:
commands.Spawn(
    RigidBodyBundle {
        body:  RigidBody { body_type: Dynamic },
        shape: ColliderShape::Capsule{ radius: 0.4, half_height: 0.9 },
        material: ColliderMaterial { friction: 0.3, restitution: 0.1 },
    }
)
// Equivalent to manually spawning RigidBody + child Collider entity.
```

The bundle is pure ergonomics — it does not introduce a new storage model. The child `Collider` entity always exists in the World.

## 6. Open Questions

- Should `TriMesh` be allowed on `Kinematic` bodies for one-sided walls and portals? Most backends support this but it is unusual.
- `ConvexHull` max vertex count: 255 is a common backend limit (Rapier, Bullet). Should the engine enforce this at bundle construction time with an explicit error, or silently downsample?
- Should `CollisionGroups` constants be defined in the engine or left entirely to game code? Predefined names risk conflicting with game-specific group semantics.
- `contact_force_threshold`: per-collider (as specified) or per-body? Per-collider is more flexible; per-body is simpler.
- How should collider scaling work when a parent entity's `Transform.scale` is non-uniform? Propagate scale into shape dimensions at Sync time, or forbid non-uniform scale on `RigidBody` ancestors?

### 5.11 Collision Interaction Matrix (Architecture)

To decouple systems from specific tags, the engine uses a **Collision Interaction Matrix**. Instead of simply tagging objects as "Player" or "Wall", the architecture defines how these groups interact:

- **Interaction Layers**: A central resource (typically a `uint32` bitfield) where each bit represents a conceptual layer.
- **Masking Logic**: A collider only interacts with another if `(A.membership & B.filter) != 0 AND (B.membership & A.filter) != 0`.
- **Systemic Filtering**: Broad-phase algorithms (like Grids or Quadtrees) use these masks to skip narrow-phase checks entirely for non-interacting layers, significantly reducing CPU load.

### 5.12 Composite Collider Composition

Entities may require complex physical representation that a single primitive cannot provide. The engine supports **Composite Colliders** through the ECS hierarchy:

- **Atomic Primitives**: Each child entity holds a single `Collider` component with a primitive shape (Box, Sphere, Capsule).
- **Local Transformation**: The `Transform` component on each child entity defines the offset and rotation of that primitive relative to the root `RigidBody`.
- **Unified Inertia**: The Physics Server treats the collection of child colliders as a single compound shape for mass and inertia tensor calculations (if `MassProperties::Auto` is enabled).

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
| 0.1.0 | 2026-03-27 | Initial draft — architectural model B (child entity), shape set, collision groups, lifecycle hooks | [examples/physics](file:///d:/Projects/src/github.com/teratron/boltengine/examples/physics) |
| 0.1.1 | 2026-03-27 | Added Collision Interaction Matrix and Composite Collider patterns | [examples/physics](file:///d:/Projects/src/github.com/teratron/boltengine/examples/physics) |
