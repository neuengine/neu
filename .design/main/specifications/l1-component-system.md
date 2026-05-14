# Component System

**Version:** 0.4.0
**Status:** Draft
**Layer:** concept

## Overview

Components are pure data attached to entities. They carry no logic — systems process components. The component system manages registration, storage strategy selection, lifecycle hooks, required component dependencies, clone behavior, and the type registry that maps runtime type information to component metadata.

## Related Specifications

- [entity-system.md](l1-entity-system.md) — Components attached to entities
- [query-system.md](l1-query-system.md) — Systems access components via queries
- [change-detection.md](l1-change-detection.md) — Component mutations tracked by ticks

## 1. Motivation

Entities are meaningless without data attached to them. The Component System is the mechanism by which arbitrary, user-defined data is associated with entities in a way that supports both cache-friendly iteration (hot inner loops) and ergonomic structural changes (add/remove at runtime). A naive solution — a map of entity to list of components — sacrifices performance. A monolithic array-of-struct layout sacrifices flexibility. The Component System resolves this tension by giving each component type its own storage strategy chosen at registration time.

Key requirements:

- Efficient memory layout for cache-friendly iteration.
- Flexible storage: dense iteration vs. fast add/remove tradeoff.
- Automatic dependency management (adding Transform auto-adds GlobalTransform).
- Lifecycle hooks for cross-cutting concerns (physics body registration on add, cleanup on remove).

## 2. Constraints & Assumptions

- Components are plain data structures with no methods (logic lives in systems).
- Each component type has a unique ComponentID assigned at registration time.
- Component registration is one-time and immutable after first use.
- A component cannot be both Table-stored and SparseSet-stored — the choice is per-type.

## 3. Core Invariants

- **INV-1**: Each component type has exactly one storage strategy, declared at registration.
- **INV-2**: Required components are transitively resolved — if A requires B, and B requires C, then adding A adds B and C.
- **INV-3**: Component hooks (OnAdd, OnInsert, OnReplace, OnRemove) fire exactly once per event, in deterministic order.
- **INV-4**: Immutable components cannot be mutated after insertion. Attempting mutation is an error.
- **INV-5**: The type registry is the single source of truth for component metadata at runtime.

## 4. Detailed Design

### 4.1 Storage Strategies

#### Table Storage (Default)

- Column-oriented (Structure of Arrays): each component type = one contiguous array.
- Entities with the same archetype share a table.
- **Pros**: Excellent cache locality for iteration, SIMD-friendly.
- **Cons**: Moving an entity between archetypes (add/remove component) requires data copy.

#### Sparse Set Storage

- Entity-indexed sparse array with a dense data array.
- **Pros**: O(1) add/remove without archetype changes, good for frequently toggled components.
- **Cons**: Iteration less cache-friendly (random access by entity index).

Selection guideline: Use Table (default) for most components. Use SparseSet for components that are frequently added/removed (e.g., status effects, temporary markers).

### 4.2 Component Registration

Every component type must be registered before use:

```
ComponentDescriptor {
    Name:             string
    TypeID:           unique type identifier
    Size:             bytes
    Alignment:        bytes
    StorageType:      Table | SparseSet
    RequiredComponents: []ComponentID
    CloneBehavior:    Clone | Ignore | Custom
    Hooks:            ComponentHooks
    Immutable:        bool
}
```

Registration happens automatically on first use or explicitly during plugin setup.

### 4.3 Required Components

A component can declare dependencies:

```
Transform requires GlobalTransform
Mesh3D requires Transform, Visibility
Camera requires Transform, GlobalTransform, Frustum
```

When component A with requirements is inserted on an entity, the engine:

1. Resolves the full transitive dependency set.
2. Inserts any missing required components with their default values.
3. Fires hooks for all inserted components in dependency order.

Circular dependencies are forbidden and detected at registration time.

### 4.4 Component Hooks

Lifecycle callbacks invoked during component mutations:

| Hook | Trigger | DeferredWorld Access |
| :--- | :--- | :--- |
| **OnAdd** | First time component type appears on entity | Yes |
| **OnInsert** | Every insertion (including overwrite) | Yes |
| **OnReplace** | Component value overwritten (old value available) | Yes |
| **OnRemove** | Component about to be removed | Yes |

Hooks receive a DeferredWorld — they can read/write other components but cannot perform structural changes (spawn/despawn/add/remove). Structural changes must go through Commands.

### 4.5 Immutable Components

Components marked as immutable cannot be mutated after insertion:

- `Query<&mut ImmutableComponent>` is a compile-time or registration-time error.
- The only way to change the value is to remove and re-insert.
- Use case: entity identifiers, UUID components, archetype-defining tags.

### 4.6 Clone Behavior

When duplicating entities, each component defines how it is cloned:

- **Clone** (default): Deep copy of the component data.
- **Ignore**: Component is not copied to the new entity.
- **Custom**: User-defined clone function (e.g., assign new unique ID).

### 4.7 Bundles

Bundles are groups of components for convenient spawning:

```
PlayerBundle = { Transform, Velocity, Health, Player }
world.Spawn(PlayerBundle{...})
```

- A bundle is NOT a component — it dissolves into individual components on spawn.
- Bundles can contain other bundles (flattened recursively).
- Spawning a bundle triggers all individual component hooks.

### 4.8 Type Registry

The type registry maps runtime type information to engine metadata:

- ComponentID → ComponentDescriptor
- Type name → ComponentID (for serialization, debug, editor)
- Reflection data for dynamic component access (scene loading, inspector)

The registry is a World resource, populated during plugin initialization.

### 4.9 Component Attachment Validation

The component collection enforces structural rules at insertion time, preventing invalid states before they reach systems:

```plaintext
Validation Rules (checked on Add/Insert):
  1. SingleInstancePerType  — only one component of a given type per entity
                              (unless type declares AllowMultipleComponents)
  2. NoDualAttach           — a component instance cannot be attached to two entities
                              simultaneously; detach from old entity first
  3. NoCyclicParent         — when adding a hierarchy component (e.g., ChildOf),
                              verify no ancestor cycle exists
  4. EntityOwnership        — the component stores a back-reference to its owning
                              entity; the collection updates this on attach/detach
```

Rule 1 uses an opt-out attribute: most components are single-instance (Position, Health), but some (e.g., multiple AudioPlayer sources on one entity) can declare `AllowMultipleComponents`. The default is single-instance because it enables fast `Get[T]()` lookups without iteration.

Rule 2 prevents accidental data sharing — a component struct is owned by exactly one entity. If user code needs the same data on two entities, it must clone explicitly (see §4.6 Clone Behavior).

Systems often need to cache preprocessed or runtime-specific data for each entity they process, without adding fields to the component itself:

```plaintext
AssociatedDataMap[TComponent, TData]
  data: map[ComponentID]TData      // dictionary keyed by internal component index

  // Each frame (or as needed):
  Get(entity: Entity, component: TComponent) -> TData:
    if existing := data.get(component.Id):
        if IsDataValid(entity, component, existing):
            return existing
        else:
            Cleanup(existing)

    newData = GenerateData(entity, component)
    data.put(component.Id, newData)
    return newData
```

**Key Mechanisms:**

- **Separation of Concerns**: Components remain "blind" data (POD structs). Systems own the technical "handles" or "buffers" needed for their operation.
- **Fast Lookup**: Using internal non-generational `ComponentID` as a key allows O(1) array or sparse-set indexing for maximal performance.
- **Validation**: `IsDataValid` checks if environmental factors (like a change in `Transform` or `GlobalTransform` tick) require a full data rebuild.
- **Automatic Lifecycle**: When a component is removed from an entity, its `OnRemove` hook triggers the cleanup of all associated data in registered maps.

For example, a **RenderMeshSystem** caches GPU buffer handles per `MeshComponent`. If the `MeshComponent`'s source path changes, `IsDataValid` returns false, and the GPU handles are re-initialized for the new asset.

### 4.11 Design Best Practices

To maintain a scalable and performant ECS architecture, follow these guidelines:

1. **Component Granularity (Single Responsibility)**: Favor small, specialized components over monolithic "God" components.
   - *Bad*: `CharacterComponent { pos, vel, health, jumpSpeed, isGrounded, ... }`
   - *Good*: `Transform`, `Velocity`, `Health`, `Jump { speed }`, `GroundedTag`.
   - *Benefit*: Better reuse and composition (e.g., a static trap with `Health` but no `Velocity`).

2. **Tag Components for Filtering**: Use empty (zero-sized) structs as "Tag" components to mark entities for specific system processing or exclusion.
   - *Example*: `DisabledTag`, `PlayerTag`, `EnemyTag`, `MainCameraTag`.
   - *Advantage*: Queries like `With[PlayerTag]` are significantly faster and cleaner than checking a `bool IsPlayer` field inside a larger component.

3. **Command Components for Reactive Logic**: Use components as one-time "signals" to trigger cross-system logic.
   - *Example*: Adding a `PlaySoundEffect { id }` component to an entity. The `AudioSystem` processes it and then removes the component.

4. **Data Aggregation vs. Multiple Components**: If an entity needs multiple instances of similar data (e.g., multiple status effects), use a single component containing a list/slice of structs rather than attempting to attach multiple components of the same type.
   - *Example*: `AbilitiesComponent { list []Ability }` instead of multiple `AbilityComponent` instances.

5. **Read-Only Helper Methods**: While components must be pure data, providing simple read-only methods for state queries is acceptable and improves readability.
   - *Example*: `func (c *Cooldown) IsReady() bool { return c.Remaining <= 0 }`.

6. **Graduation Workflow**: When unsure where logic belongs, start by implementing it as a specific "Script" (associated with a specific entity/prefab). As the logic stabilizes or requires reuse, "graduate" it: move data to new components and logic to a generic system.

### 4.12 Anti-Patterns

The following patterns are known to cause subtle bugs and architectural decay. Avoid them.

1. **Shared Mutable References Between Entities**: Never share a pointer/reference to the same mutable object across multiple entities' components.
   - *Bad*: `entity1.Squad = entity2.Squad` (same pointer — mutation from one entity silently affects the other).
   - *Good*: Create a dedicated `SquadEntity` with a `SquadComponent`, and have member entities reference it via `SquadRef { entityID EntityID }`.
   - *Why*: Shared mutable state breaks ECS determinism, makes debugging nearly impossible, and prevents safe parallel iteration. Dedicated entities also allow attaching debug/diagnostic components to abstract concepts.

2. **Boolean Flags Instead of Tag Components**: Avoid adding `bool` fields to components for filtering purposes.
   - *Bad*: `MovementComponent { speed float64; isDisabled bool }` — every system iterating `MovementComponent` must check `isDisabled`.
   - *Good*: Use a `DisabledTag` component and filter with `Without[DisabledTag]` in queries.
   - *Why*: Tag-based filtering is resolved at the archetype level (O(1) per query), while boolean checks require per-entity branching inside hot loops.

3. **God Components**: A component whose fields have obvious prefixes grouping unrelated concerns.
   - *Smell*: Fields like `jumpSpeed`, `jumpCooldown`, `walkSpeed`, `walkAccel` in one `CharacterComponent`.
   - *Fix*: Extract into `JumpComponent { speed, cooldown }` and `WalkComponent { speed, accel }`.
   - *Why*: God components force systems to depend on data they don't need, bloating cache lines and preventing partial entity composition.

4. **Systems That Process Unrelated Data**: A single system touching component combinations that serve different purposes.
   - *Smell*: `GameplaySystem` that handles both health regeneration and input processing.
   - *Fix*: Split into `HealthRegenSystem` and `InputSystem`, each depending on the minimum set of components.
   - *Why*: Smaller systems enable partial entity matching, better parallelism, and easier reordering.

## 5. Open Questions

- Should components support inheritance / composition beyond required components?
- Maximum component count per archetype — is there a practical limit to enforce?

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
| 0.2.0 | 2026-03-26 | Added component attachment validation rules, associated data pattern |
| 0.3.0 | 2026-03-28 | Added anti-patterns section (shared mutable refs, bool flags, god components, monolithic systems) |
| 0.4.0 | 2026-04-19 | Added `## 1. Motivation` section (RULES §5 compliance) |
| 0.5.0 | 2026-05-14 | Phase 1 implementation complete; validated by `examples/ecs/poc` |
