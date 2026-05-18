# Hierarchy System — Go Implementation

**Version:** 0.1.0
**Status:** Stable
**Layer:** go
**Implements:** [hierarchy-system.md](l1-hierarchy-system.md)

## Overview

This specification defines the Go implementation of the hierarchy system described in the L1 concept spec. The hierarchy provides parent-child relationships between entities via a `ChildOf` relationship component, automatic `Children` maintenance, transform propagation from parent to child, cycle detection, and tree traversal utilities. All hierarchy types live in the `internal/hierarchy` package with dependencies on `internal/ecs` and `internal/math`.

## Related Specifications

- [hierarchy-system.md](l1-hierarchy-system.md) — L1 concept specification (parent)

## 1. Motivation

The Go implementation of the Hierarchy system enables complex entity relationships and spatial parent-child propagation. It ensures:

- Robust tree-invariant maintenance (single parent per child).
- Efficient world-space transform propagation from root to leaves.
- High-performance traversal utilities using Go 1.23+ `iter`.
- Safety via built-in cycle detection and recursive lifecycle management.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: Relies on generics for relationship components and `iter` for tree walks.
- **Single Parent**: Each entity can have at most one `ChildOf` component.
- **Affine Math**: Relies on `internal/math` for high-performance SIMD-friendly transform calculations.

## 3. Core Invariants

> [!NOTE]
> See [hierarchy-system.md §3](l1-hierarchy-system.md) for technology-agnostic invariants.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: Tree Structure | `ChildOf` component hook prevents DAGs by enforcing a single parent. |
| **INV-2**: Cycle Prevention | `ON INSERT ChildOf` walks the ancestor chain to reject self-referential links. |
| **INV-3**: Lifecycle Sync | `DespawnRecursive` ensures child entities are cleaned up when a parent is removed. |
| **INV-4**: Propagation | `propagate_transforms` system recomputes `GlobalTransform` in `PostUpdate`. |
| **INV-5**: Traversal | `Descendants` and `Ancestors` provide zero-allocation iterators for tree walks. |

## Go Package

```
internal/hierarchy/
```

All types in this spec belong to package `hierarchy`. The package imports `internal/ecs` for entity and world access and `internal/math` for spatial types.

## Type Definitions

### ChildOf

```go
// ChildOf is a relationship component placed on a child entity to declare its parent.
// An entity may have at most one ChildOf component (tree invariant, not DAG).
type ChildOf struct {
    Parent ecs.Entity
}
```

### Children

```go
// Children is an automatically-maintained component on parent entities.
// It holds an ordered list of child entities. Read-only for user systems —
// mutations go through hierarchy commands or ChildOf manipulation.
type Children struct {
    entities []ecs.Entity
}

// Slice returns a read-only copy of the child entity list.
func (c *Children) Slice() []ecs.Entity

// Len returns the number of direct children.
func (c *Children) Len() int

// Contains reports whether the given entity is a direct child.
func (c *Children) Contains(entity ecs.Entity) bool
```

### Transform

```go
// Transform represents local position, rotation, and scale relative to the
// parent entity (or world origin if the entity has no parent).
// Users mutate this component directly.
type Transform struct {
    Translation math.Vec3
    Rotation    math.Quat
    Scale       math.Vec3
}

// NewTransform returns a Transform at the origin with identity rotation and
// uniform scale of 1.
func NewTransform() Transform

// FromTranslation creates a Transform with the given position, identity
// rotation, and uniform scale of 1.
func FromTranslation(translation math.Vec3) Transform

// FromRotation creates a Transform at the origin with the given rotation
// and uniform scale of 1.
func FromRotation(rotation math.Quat) Transform

// ToAffine3A converts this local transform into an Affine3A matrix.
func (t Transform) ToAffine3A() math.Affine3A

// LookAt returns a Transform that faces the target position from the given
// eye position, using the provided up vector.
func LookAt(eye, target, up math.Vec3) Transform
```

### GlobalTransform

```go
// GlobalTransform is the computed world-space affine transform. It is
// read-only for user systems — the propagation system writes it each frame.
// Adding Transform to an entity automatically adds GlobalTransform
// (required component pattern).
type GlobalTransform struct {
    matrix math.Affine3A
}

// NewGlobalTransform returns an identity GlobalTransform.
func NewGlobalTransform() GlobalTransform

// FromAffine3A wraps an existing affine matrix.
func FromAffine3A(m math.Affine3A) GlobalTransform

// Affine3A returns the underlying affine matrix.
func (g GlobalTransform) Affine3A() math.Affine3A

// Translation extracts the world-space translation vector.
func (g GlobalTransform) Translation() math.Vec3

// Right returns the local X-axis direction in world space.
func (g GlobalTransform) Right() math.Vec3

// Up returns the local Y-axis direction in world space.
func (g GlobalTransform) Up() math.Vec3

// Forward returns the local negative-Z-axis direction in world space.
func (g GlobalTransform) Forward() math.Vec3

// Mul combines two GlobalTransforms: parent * child.
func (g GlobalTransform) Mul(child GlobalTransform) GlobalTransform

// MulTransform applies a local Transform to this GlobalTransform:
// GlobalTransform(child) = GlobalTransform(parent) * Transform(child)
func (g GlobalTransform) MulTransform(local Transform) GlobalTransform
```

### Hierarchy Commands

```go
// AddChild adds a child entity to a parent. Deferred — executed during
// command flush. Inserts ChildOf{Parent} on child and updates parent's Children.
func AddChild(cmds *ecs.Commands, parent, child ecs.Entity)

// SetParent changes the parent of an entity. If the entity already has a
// parent, removes it from the old parent's Children first.
func SetParent(cmds *ecs.Commands, child, newParent ecs.Entity)

// RemoveParent removes the ChildOf component from an entity and removes it
// from the parent's Children list. The entity becomes a root.
func RemoveParent(cmds *ecs.Commands, child ecs.Entity)

// DespawnRecursive despawns an entity and all of its descendants in
// depth-first order (leaves first, then ancestors).
func DespawnRecursive(cmds *ecs.Commands, entity ecs.Entity)
```

### Traversal Functions

```go
// ChildrenOf returns an iterator over the direct children of the given entity.
// Returns an empty iterator if the entity has no Children component.
func ChildrenOf(world *ecs.World, entity ecs.Entity) iter.Seq[ecs.Entity]

// Descendants returns a depth-first iterator over all descendants of the
// given entity (does not include the entity itself).
func Descendants(world *ecs.World, entity ecs.Entity) iter.Seq[ecs.Entity]

// Ancestors returns an iterator that walks from the given entity's parent
// up to the root (does not include the entity itself).
func Ancestors(world *ecs.World, entity ecs.Entity) iter.Seq[ecs.Entity]

// Root returns the root ancestor of the given entity. If the entity has no
// parent, returns the entity itself.
func Root(world *ecs.World, entity ecs.Entity) ecs.Entity

// IsDescendantOf reports whether entity is a descendant of ancestor.
func IsDescendantOf(world *ecs.World, entity, ancestor ecs.Entity) bool
```

## Key Methods

### Cycle Detection

When `ChildOf` is inserted (via component hook or command), the system walks the ancestor chain from the proposed parent upward:

```
ON INSERT ChildOf{Parent: P} on entity C:
  current = P
  WHILE current has ChildOf:
    IF current == C:
      REJECT insertion — cycle detected
    current = ChildOf(current).Parent
  ACCEPT — no cycle
```

Time complexity: O(depth) where depth is the tree height. Typical game hierarchies are shallow (depth < 20).

### Children Auto-Maintenance

Component hooks on `ChildOf` drive `Children` updates:

```
ON INSERT ChildOf{Parent: P} on C:
  IF P has no Children component:
    ADD Children{} to P
  APPEND C to P.Children.entities

ON REMOVE ChildOf{Parent: P} from C:
  REMOVE C from P.Children.entities
  IF P.Children.entities is empty:
    REMOVE Children from P   // optional — keep or remove empty Children
```

### Transform Propagation System

Runs in `PostUpdate` schedule. Depth-first walk from root entities:

```
SYSTEM propagate_transforms(world):
  // Phase 1: Update root entities (no ChildOf)
  FOR EACH entity WITH (Transform, GlobalTransform) WITHOUT ChildOf:
    IF Transform is Changed OR GlobalTransform is Added:
      GlobalTransform = GlobalTransform.FromAffine3A(Transform.ToAffine3A())
      propagate_to_children(world, entity)

FUNCTION propagate_to_children(world, parent):
  parent_global = GlobalTransform(parent)
  FOR EACH child IN Children(parent):
    child_transform = Transform(child)
    child_global = parent_global.MulTransform(child_transform)
    SET GlobalTransform(child) = child_global
    propagate_to_children(world, child)  // recurse
```

### Dirty Flag Optimization

Skip subtrees where no `Transform` has changed:

```
FUNCTION propagate_to_children(world, parent, parent_changed):
  parent_global = GlobalTransform(parent)
  FOR EACH child IN Children(parent):
    child_changed = parent_changed OR Transform(child).IsChanged()
    IF child_changed:
      child_global = parent_global.MulTransform(Transform(child))
      SET GlobalTransform(child) = child_global
    propagate_to_children(world, child, child_changed)
```

Uses change detection ticks from `internal/ecs` — integer comparison, zero allocation.

### Recursive Despawn

When a parent is despawned (INV-3 from L1), all descendants are despawned recursively:

```
ON DESPAWN entity:
  IF entity HAS Children:
    FOR EACH child IN Children(entity) (reversed):
      DESPAWN child  // triggers recursive despawn on child's children
```

### HierarchyPlugin

```go
// HierarchyPlugin registers all hierarchy components, systems, and hooks.
type HierarchyPlugin struct{}

// Build registers ChildOf, Children, Transform, GlobalTransform components,
// component hooks for auto-maintenance, the propagation system in PostUpdate,
// and the recursive despawn observer.
func (p HierarchyPlugin) Build(app *app.App)
```

## Performance Strategy

- **Depth-first walk**: Cache-friendly traversal of the tree. Children stored contiguously in the `Children` slice.
- **Dirty flag skip**: Subtrees with no changed `Transform` are skipped entirely using tick-based change detection (integer comparison, zero allocation).
- **Parallel subtrees**: Independent root subtrees can be processed concurrently (future optimization). Each root's subtree has no data dependencies on other roots.
- **No allocations in propagation**: `Affine3A` multiplication is stack-allocated. The propagation function uses recursion bounded by tree depth (typically < 20).
- **`Children` slice reuse**: The `entities` slice is reused across frames, only growing when new children are added.
- **Cycle detection is O(depth)**: Walks at most `depth` ancestors. For typical hierarchies (< 20 deep), this is negligible.

## Error Handling

- **Cycle detection**: Inserting `ChildOf` that would create a cycle returns an error via the command system. In debug builds (`ecsdebug` tag), a descriptive panic with the cycle path.
- **Invalid parent**: Setting `ChildOf` to a dead entity logs a warning via `log/slog` and is rejected.
- **Re-parenting**: Setting `ChildOf` on an entity that already has a parent removes the old relationship first. No error — this is the expected use case for `SetParent`.
- **Despawn non-existent**: Despawning an entity that is already dead is a no-op.
- **Traversal on dead entity**: Returns empty iterator, no panic.

## Testing Strategy

- **Unit tests**: Insert/remove ChildOf, verify Children auto-update, cycle detection rejection, multi-level hierarchy construction.
- **Transform propagation**: Build 3-level hierarchy, mutate root Transform, verify GlobalTransform at each level matches expected matrix math.
- **Dirty flag**: Change one leaf's Transform, verify only that subtree recomputes GlobalTransform.
- **Recursive despawn**: Despawn root, verify all descendants are also despawned.
- **Re-parenting**: Move a child from parent A to parent B, verify both Children lists are correct.
- **Traversal**: Verify `Descendants`, `Ancestors`, `Root`, `IsDescendantOf` on a known tree.
- **Benchmarks**: `BenchmarkPropagate1K` (1000 entities, 10 roots), `BenchmarkPropagate10K` (10,000 entities). Target: zero allocations per frame when no hierarchy changes occur.
- **Fuzz tests**: Random sequences of AddChild/RemoveParent/Despawn, verify tree invariants hold.

## 7. Drawbacks & Alternatives

- **Drawback**: Deep hierarchies can cause stack overflow if propagation is recursive.
- **Alternative**: Iterative propagation using a queue/stack.
- **Decision**: Recursive propagation is simpler and safe for typical game hierarchy depths (< 64). Iterative fallback will be added if needed.

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
| 0.1.0 | 2026-03-26 | Initial L2 draft |
