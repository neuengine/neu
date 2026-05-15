# Character Controller

**Version:** 0.1.0
**Status:** Draft
**Layer:** concept

## Overview

The character controller provides kinematic movement for player characters and NPCs that need precise, game-feel-driven motion rather than fully simulated rigid body dynamics. It wraps a capsule collider in a high-level API: move by velocity, detect floors, handle step-up, slide along walls, and snap to slopes.

## Related Specifications

- [physics-system.md](l1-physics-system.md) — Backend executes the movement sweep
- [rigid-body.md](l1-rigid-body.md) — Contrast: RigidBody::Kinematic for animated objects
- [collider.md](l1-collider.md) — Capsule shape and density settings
- [physics-query.md](l1-physics-query.md) — Uses ShapeCast internally for sweeping
- [collision-events.md](l1-collision-events.md) — CharacterCollision events emitted per move

## 1. Motivation

A fully simulated dynamic body makes a poor player character. They tumble when hit, jitter on stairs, and slide off ledges. Game characters need instant velocity changes, predictable floor snapping, and step-up over kerbs. A first-class controller provides these as a tested building block.

## 2. Constraints & Assumptions

- Uses a **capsule** shape exclusively for reliable slope/step detection.
- Movement runs in `FixedUpdate` (60 Hz). Rendering interpolates via `PhysicsTransform`.
- The controller is **kinematic**: it does not receive forces or impulses. External pushes must be handled by game code writing to `CharacterController.velocity`.
- Interacts with dynamic bodies by applying force based on `push_strength`.

## 3. Core Invariants

- **INV-1**: `CharacterController.velocity` is the authoritative input. The controller does **not** apply gravity or acceleration; that is the caller's responsibility.
- **INV-2**: The controller never penetrates solid geometry. Moves are clipped if they would result in penetration deeper than `skin_width`.
- **INV-3**: `is_grounded` reflects the state **after** the move is completed.
- **INV-4**: `step_height` applies only to horizontal movement toward an obstacle.

## 4. Detailed Design

### 4.1 CharacterController Component

Defines collision shape (`radius`, `height`), movement rules (`velocity`, `up`), floor detection (`max_slope_angle`, `step_height`, `snap_to_floor`), and collision response (`skin_width`, `push_strength`, `slide_on_walls`). Output fields like `is_grounded` and `ground_normal` are updated by the system.

### 4.2 Movement Algorithm (Iterative Sweep)

The controller uses an **iterative depenetration-based sweep** (up to 4 iterations):

1. **ShapeCast**: Cast the capsule in the direction of `velocity * dt`.
2. **Move**: Advance the position to the hit point minus `skin_width`.
3. **Project**: If blocked, project the remaining velocity onto the hit surface normal (sliding).
4. **Repeat**: Try the move again with the projected velocity.

### 4.3 Step-Up Logic (Pseudo-code)

Allows climbing low obstacles (kerbs/stairs) without jumping:

1. **Lift**: Perform a vertical `ShapeCast` up by `step_height`.
2. **Advance**: From the raised position, attempt a horizontal `ShapeCast`.
3. **Snap Down**: If clear, perform a vertical `ShapeCast` down to find the new surface.
4. **Accept**: If the new surface is walkable, move to the final position.

### 4.4 Gravity Ownership

The controller does **not** handle gravity. This allows game code to implement custom feel:

- Coyote time (jumping shortly after leaving a ledge).
- Variable jump height (longer press = higher jump).
- Custom gravity directions for planet surfaces.

### 4.5 Slope Behaviour

Slopes steeper than `max_slope_angle` are treated as walls. The character will slide down or be stopped by them depending on `slide_on_walls`. For walkable slopes, the velocity is projected onto the slope plane for smooth ascent/descent.

### 4.6 CharacterCollision Event

Emitted for every surface hit during a move. Includes `hit_entity`, `normal`, `point`, and `collision_type` (`Wall`, `Floor`, `Ceiling`, `StepUp`).

## 5. Open Questions

- Interaction with moving kinematic platforms (velocity inheritance).
- Support for multiple capsule shapes (crouching vs standing).

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
| 0.1.0 | 2026-03-27 | Initial draft — capsule controller, iterative sweep, step-up, external gravity | [examples/physics](file:///d:/Projects/src/github.com/teratron/boltengine/examples/physics) |
