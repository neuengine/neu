# Physics Materials

**Version:** 0.1.0
**Status:** Draft
**Layer:** concept

## Overview

A physics material defines the surface response properties of a collider — how much it resists sliding (friction) and how much kinetic energy it returns after impact (restitution). Materials are either declared inline on the `Collider` component or referenced as shared `PhysicsMaterial` assets. When two colliders meet, the backend combines their values using configurable blend rules.

## Related Specifications

- [collider.md](l1-collider.md) — Collider carries inline material fields or a Handle to PhysicsMaterial
- [physics-system.md](l1-physics-system.md) — Backend applies combined material values during contact solving
- [asset-system.md](../main/specifications/asset-system.md) — PhysicsMaterial loaded as an asset

## 1. Motivation

Surface feel is critical for game "juice". Ice feels different from rubber. A shared material system allows changing "ice friction" in one place to update every icy surface in the game instantly, including via hot-reload, rather than tuning hundreds of individual colliders.

## 2. Constraints & Assumptions

- Material values are applied per contact pair.
- `friction` and `restitution` ranges are `[0.0, ∞)` and `[0.0, 1.0]` respectively.
- Inline material fields on `Collider` take priority over a referenced `PhysicsMaterial` asset.
- Default material: `friction: 0.5`, `restitution: 0.0`, `combine: Average/Maximum`.

## 3. Core Invariants

- **INV-1**: Every contact pair has exactly one effective friction and restitution value, computed by blending.
- **INV-2**: Blend rule precedence: `Maximum > Multiply > Minimum > Average`. When rules differ, the higher-precedence rule wins.
- **INV-3**: `restitution` is always clamped to `[0.0, 1.0]` before being passed to the solver.
- **INV-4**: `PhysicsMaterial` asset updates propagate to all referencing colliders within the same Sync phase.

## 4. Detailed Design

### 4.1 Inline Material Fields on Collider

For rapid prototyping or one-off surfaces:

```plaintext
Collider
  ...
  friction:            float32       // default 0.5
  restitution:         float32       // default 0.0
  friction_combine:    CombineRule   // default Average
  restitution_combine: CombineRule   // default Maximum
```

### 4.2 PhysicsMaterial Asset

Reusable surface definition (`.phymat.json`). Referencing from a collider uses `material: Handle<PhysicsMaterial>`. If set, inline fields are ignored.

### 4.3 CombineRule

Controls value blending at contact:

- `Average`: `(a + b) / 2`
- `Minimum`: `min(a, b)`
- `Maximum`: `max(a, b)`
- `Multiply`: `a * b`

**Precedence**: `Maximum > Multiply > Minimum > Average`.
Example: Rubber (Maximum) vs Concrete (Average) -> Rubber wins. Ball on Ice (Minimum) vs Wood (Average) -> Ice wins.

### 4.4 Predefined Material Presets

Engine-provided materials (`vfs:///engine/physics/materials/`):

- `Default`, `Ice`, `Rubber`, `Metal`, `Wood`, `Concrete`, `Glass`, `Mud`.

Accessed via constants like `PhysicsMaterials::ICE`.

### 4.5 Per-Pair Material Override

Optional `MaterialOverrideTable` resource maps `(EntityCategory, EntityCategory)` to specific friction/restitution values, allowing special behavior for specific pairs (e.g., character feet on specific terrain).

### 4.6 Surface Tags (SFX/VFX Hints)

`PhysicsMaterial` carries a `surface_tag: StringName` (e.g., "ice", "metal").
The engine does **not** interpret this tag. Game code reads it from collision events to select appropriate sound or particle effects.

### 4.7 Hot-Reload

Modification of a `.phymat.json` file triggers an asset reload. The Physics Sync system catches `AssetEvent::Modified`, marks referencing colliders as dirty, and updates the backend shapes in the next Sync phase.

## 5. Open Questions

- Should `density` be part of `PhysicsMaterial` or strictly stay on `Collider`?
- Should `surface_tag` be a StringName or a TypeID pointing to a tag component?

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
| 0.1.0 | 2026-03-27 | Initial draft — inline vs asset, combine rules, presets, surface tags | [examples/physics](file:///d:/Projects/src/github.com/teratron/boltengine/examples/physics) |
