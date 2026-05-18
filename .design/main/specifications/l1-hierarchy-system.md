# Hierarchy System

**Version:** 0.2.0
**Status:** Stable
**Layer:** concept

## Overview

The hierarchy system provides parent-child relationships between entities, transform propagation from parent to child, and tree traversal utilities. It is the foundation for scene graphs, UI layout, skeletal animation, and any feature that requires spatial nesting.

## Related Specifications

- [entity-system.md](l1-entity-system.md) — Entities form hierarchy nodes
- [component-system.md](l1-component-system.md) — ChildOf as a relationship component
- [math-system.md](l1-math-system.md) — Transform and GlobalTransform types
- [event-system.md](l1-event-system.md) — Observers for hierarchy change notifications

## 1. Motivation

Games need entity hierarchies: a character's weapon moves with the character, UI panels contain child widgets, a vehicle's wheels follow the chassis. Without a built-in hierarchy:

- Every game would reimplement parent-child logic.
- Transform propagation would be inconsistent and error-prone.
- Systems like visibility culling and physics would lack a shared tree structure.

## 2. Constraints & Assumptions

- Hierarchy is implemented via a `ChildOf` relationship component, not a special-case system.
- An entity can have at most one parent (tree, not DAG).
- An entity can have zero or more children.
- Transform propagation runs in `PostUpdate`, producing a 1-frame delay for changes made in `Update`.

## 3. Core Invariants

- **INV-1**: Adding `ChildOf{Parent: P}` to entity C makes C a child of P and adds C to P's `Children` list.
- **INV-2**: Removing `ChildOf` from C removes C from the parent's `Children` list.
- **INV-3**: Despawning a parent despawns all descendants recursively.
- **INV-4**: GlobalTransform is always consistent with the hierarchy after the propagation system runs.
- **INV-5**: Circular parent-child relationships are forbidden and detected at insertion time.
- **INV-6**: Cascade activation utilizes a `deactivated_from_parent` flag to ensure that when a parent is re-activated, only children that were active before the parent's deactivation are restored.

## 4. Detailed Design

### 4.1 Relationship Components

- **ChildOf** — A component on the child entity pointing to the parent. `ChildOf { Parent: Entity }`
- **Children** — Automatically-maintained ordered list of child entities on the parent. Read-only for users.

Adding `ChildOf` to an entity triggers:

1. Validation that the parent exists and no cycle is formed.
2. Insertion of the child into the parent's `Children` list.
3. Component hooks fire for both `ChildOf` (on child) and `Children` (on parent).

### 4.2 Transform Components

- **Transform** — Local position, rotation, and scale relative to parent (or world origin if no parent).

  ```
  Transform { Translation: Vec3, Rotation: Quat, Scale: Vec3 }
  ```

  Users mutate this directly.

- **GlobalTransform** — Computed world-space affine transform. Read-only.

  ```
  GlobalTransform(Affine3A)
  ```

  Automatically computed by the propagation system. Provides: `Translation()`, `Right()`, `Up()`, `Forward()`.

- Adding `Transform` to an entity automatically adds `GlobalTransform` (required component pattern).

### 4.3 Transform Propagation

A system that walks the hierarchy tree and computes GlobalTransform:

```
GlobalTransform(child) = GlobalTransform(parent) * Transform(child)
```

- Runs in `PostUpdate` schedule.
- Depth-first traversal from roots (entities without `ChildOf`).
- Parallelizable across independent subtrees.
- Only recomputes subtrees where `Transform` or hierarchy changed (dirty flag optimization).

### 4.4 Hierarchy Propagation (Generic)

Beyond transforms, any component value can propagate down the tree:

- **Propagate[C]** — Marks a source entity. The value of component C is copied to all descendants.
- **PropagateOver** — Skip this entity during propagation (inherit from grandparent).
- **PropagateStop** — Stop propagation at this entity (descendants get default).

Use cases: visibility inheritance, render layers, physics groups.

### 4.5 Hierarchy Validation

A plugin that warns when hierarchy constraints are violated:

- "Entity has MeshRenderer but parent lacks Transform."
- Runs in `Last` schedule as a diagnostic.
- Configurable per component type.

### 4.6 Traversal Utilities

- `children(entity)` — Iterate direct children.
- `descendants(entity)` — Depth-first iteration of all descendants.
- `ancestors(entity)` — Walk up to root.
- `root(entity)` — Find the root ancestor.
- `is_descendant_of(entity, ancestor)` — Hierarchy membership check.

### 4.7 Hierarchy Commands

```
commands.Entity(parent).AddChild(child)
commands.Entity(parent).WithChildren(func(builder) {
    builder.Spawn(ChildBundle{...})
    builder.Spawn(ChildBundle{...})
})
commands.Entity(child).SetParent(new_parent)
commands.Entity(child).RemoveParent()
```

## 5. Open Questions

- Should the engine support multiple hierarchy types (e.g., spatial hierarchy + UI hierarchy)?
- Custom relationships beyond ChildOf — how do they interact with traversal utilities?

## Canonical References

<!-- MANDATORY for Stable status. List authoritative source files that downstream agents
     MUST read before implementing this spec. Use relative paths from project root.
     Stub state — fill with concrete files when implementation begins (Phase 1+). -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

<!-- Empty table = no canonical sources yet. Populate one row per authoritative file
     when implementation lands (Phase 1+). Stable promotion requires ≥1 row. -->

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-03-25 | Initial draft |
| 0.2.0 | 2026-04-20 | Added cascade activation edge-case invariant based on 3D Engine analysis |
| — | — | Planned examples: `examples/world/` |
