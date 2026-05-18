# Scene System — Go Implementation

**Version:** 0.1.0
**Status:** Stable
**Layer:** go
**Implements:** [l1-scene-system.md](l1-scene-system.md)

## Overview

Go-level design for scene capture/restore: `StaticScene` (a `gob` World blob,
fast/opaque) and `DynamicScene` (reflection-based via the type registry,
human-editable JSON or compact interned binary). Covers the
`DynamicSceneBuilder` extraction pipeline, `SceneFilter`, the mandatory
two-pass instantiation with an entity-remap table, `SceneSpawner` instance
grouping, interned-array binary serialization, and the symmetric pack/unpack
query API.

## Related Specifications

- [l1-scene-system.md](l1-scene-system.md) — L1 concept specification (parent)
- [l2-type-registry-go.md](l2-type-registry-go.md) — reflection source for `DynamicScene`
- [l2-asset-system-go.md](l2-asset-system-go.md) — scenes load as assets
- [l2-world-system-go.md](l2-world-system-go.md) — extract from / instantiate into a `World`

## 1. Motivation

Saving, level streaming, and editor workflows need a subset of the World
serialized to a portable format and re-instantiated with fresh entity IDs and
correctly remapped cross-references. The Go implementation must guarantee the
L1 two-pass loading invariant so intra-scene entity references never dangle.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: generics where useful; `reflect` for `DynamicScene` field
  walk via the type registry.
- **C-003 — stdlib only**: JSON via `encoding/json`, binary via `encoding/gob`
  (StaticScene) and a hand-rolled interned codec (DynamicScene binary). No
  third-party serializers.
- **StaticScene is not version-portable** (L1 §2): `gob` blob tagged with the
  engine build hash; load rejects a mismatched hash.
- **Instantiation always allocates new entity IDs** — never overwrites
  (L1 INV-1).
- **No `Option[T]`**: `Assets`/lookup returns map to `(T, bool)`.

## 3. Core Invariants

> [!NOTE]
> See [l1-scene-system.md §3](l1-scene-system.md) for technology-agnostic
> invariants INV-1…INV-5. Go-specific compliance is tabulated in §4.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: Entity uniqueness | Pass 1 calls `world.SpawnEmpty()` for every `DynamicEntity`; new generational IDs are allocated by `EntityAllocator` — existing live IDs are structurally unreachable. |
| **INV-2**: Reference integrity | A `remap map[entity.EntityID]entity.EntityID` is built in Pass 1; Pass 2 invokes each component type's registered `RemapEntities(visitor)` (from the type registry) to rewrite stored IDs. |
| **INV-3**: Filter consistency | `SceneFilter` is a value (allow/deny `bitset` over `TypeID`); the *same* filter value is applied in `DynamicSceneBuilder.Build` and in `Spawn`, so the component set is identical by construction. |
| **INV-4**: Asset identity | `DynamicScene` is immutable after build; `SceneSpawner.Spawn` reads it without mutation, so one handle spawns N independent instances. |
| **INV-5**: Two-pass loading | `instantiate()` is split into `pass1CreateEntities` (spawn + insert components *without* binding) then `pass2Rebind` (apply remap visitor) before firing `SceneInstanceReady`. The two functions are private and only ever called in this order. |

## Go Package

```
pkg/scene/
  static.go    // StaticScene (gob blob + build-hash guard)
  dynamic.go   // DynamicScene, DynamicEntity, ReflectedComponent
  builder.go   // DynamicSceneBuilder, SceneFilter
  spawn.go     // SceneSpawner, two-pass instantiate, remap
  codec.go     // JSON + interned-binary, SerializedScene pack/unpack
```

## Type Definitions

```go
type StaticScene struct {
    buildHash uint64 // engine build identity; load rejects on mismatch
    blob      []byte // gob-encoded archived World subset
}

type DynamicScene struct {
    entities []DynamicEntity // immutable after Build (INV-4)
}
type DynamicEntity struct {
    ID         entity.EntityID        // original (pre-remap)
    Components []ReflectedComponent
}
type ReflectedComponent struct {
    TypeName string
    Value    reflect.Value // field tree from the type registry
}

type SceneFilter struct {
    allow, deny bitset // over component TypeID; deny wins (L1 §4.4)
}

type SceneSpawner struct { // World resource
    instances map[InstanceID][]entity.EntityID
    next      InstanceID
}
type InstanceID uint32

// Interned binary form (L1 §4.10/§4.13).
type SerializedScene struct {
    Names    []string
    Variants []any
    Entities []SerializedEntity
}
type SerializedEntity struct {
    NameIdx    int
    Components []SerializedComponent
}
type SerializedComponent struct {
    TypeIdx int
    Props   [][2]int // (nameIdx, valueIdx) pairs
}
```

## Key Methods

```
// Extraction.
func NewBuilder(w *world.World) *DynamicSceneBuilder
func (b *DynamicSceneBuilder) ExtractEntity(e entity.EntityID) *DynamicSceneBuilder
func (b *DynamicSceneBuilder) ExtractEntities(it iter.Seq[entity.EntityID]) *DynamicSceneBuilder
func (b *DynamicSceneBuilder) WithFilter(f SceneFilter) *DynamicSceneBuilder
func (b *DynamicSceneBuilder) Build() DynamicScene

// Instantiation (INV-5 two-pass; private split, public entry).
func (s *SceneSpawner) Spawn(w *world.World, sc *DynamicScene, ctx SceneLoadContext) InstanceID
func (s *SceneSpawner) SpawnStatic(w *world.World, ss *StaticScene) InstanceID
func (s *SceneSpawner) DespawnInstance(w *world.World, id InstanceID)
func (s *SceneSpawner) InstanceEntities(id InstanceID) []entity.EntityID

// Codec.
func MarshalJSON(sc *DynamicScene) ([]byte, error)
func UnmarshalJSON(data []byte) (DynamicScene, error)
func MarshalBinary(sc *DynamicScene) ([]byte, error)   // interned
func UnmarshalBinary(data []byte) (DynamicScene, error)

// Symmetric pack/unpack query API (L1 §4.13) — inspect without instantiating.
func (s *SerializedScene) EntityCount() int
func (s *SerializedScene) ComponentType(entityIdx, compIdx int) string
func (s *SerializedScene) PropertyValue(entityIdx, compIdx, propIdx int) any

type SceneLoadContext uint8
const (
    CtxRuntime SceneLoadContext = iota // no editor meta, no undo
    CtxEditorPreview
    CtxEditorMain                      // full meta + undo tracking
)
```

## Performance Strategy

- **Interned arrays** (L1 §4.10): `"Transform"` stored once across N entities;
  binary form stores `int` indices, not repeated strings — large scenes shrink
  ~5–10×.
- **Two-pass without re-scan**: the remap table is built during Pass 1's spawn
  loop (no separate traversal); Pass 2 is a single visitor walk.
- **`StaticScene` fast path**: `gob` decode + bulk archetype insert, no
  per-field reflection — used for runtime saves where portability is not needed.
- **Type-registry cache**: component reflectors resolved once per type per
  build, not per entity.

## Error Handling

| Condition | Behavior |
| :--- | :--- |
| `StaticScene` build-hash mismatch | `Load` returns `ErrSceneVersionMismatch` with both hashes; no partial spawn |
| Unknown component type on load | Skip that component, record into `SceneInstanceReady.skipped`, continue (L1 §5 Q3 forward-compat) |
| `format_version` newer than reader | `ErrSceneFromFuture` with a clear range message; reject whole file |
| Remap visitor missing for a ref-bearing component | `ErrUnremappedReference` — fail the spawn (INV-2 cannot be weakened) |

```go
var (
    ErrSceneVersionMismatch = errors.New("scene: StaticScene built by a different engine version")
    ErrSceneFromFuture      = errors.New("scene: format_version newer than this build supports")
    ErrUnremappedReference  = errors.New("scene: ref-bearing component has no remap visitor")
)
```

## Testing Strategy

- **Two-pass invariant (INV-5)**: a scene where Entity 5 references Entity 3;
  assert post-spawn the reference points to 3's *remapped* ID, never the raw ID.
- **Round-trip per format-version** (L1 §4.12): serialize → deserialize →
  deep-equal; one case per format version (mandatory on every bump).
- **Filter consistency (INV-3)**: same `SceneFilter` on extract and spawn ⇒
  identical component set.
- **Prefab independence (INV-4)**: spawn one handle twice; mutate instance A;
  assert instance B unaffected.
- **Fuzz** (C-028): fuzz the interned binary decoder against malformed index
  tables (out-of-range `nameIdx`/`valueIdx` must error, never panic).

## 7. Drawbacks & Alternatives

- **Drawback**: `gob` `StaticScene` is not portable across builds.
  **Alternative**: always use `DynamicScene`.
  **Decision**: L1 §2 explicitly accepts this — `StaticScene` is the fast
  runtime-save path; the build-hash guard makes the limitation safe, not silent.
- **Drawback**: reflection-based `DynamicScene` is slower than codegen.
  **Alternative**: generated per-type marshalers.
  **Decision**: reflection reuses the existing type registry (no new codegen
  surface); revisit via `l2-codegen-tools` only if profiling shows scene I/O
  is hot.
- **Drawback**: partial-merge spawn unsupported.
  **Decision**: L1 Open Question Q1 —
  <!-- TBD (L1 §5 Q1): merge-into-existing vs. full-spawn-only. 0.1.0 is
       full-spawn-only (INV-1 keeps it simple). Merge needs an identity
       strategy (GUID, §4.15) — defer to editor kickoff (Phase 4, C-002). -->

## Canonical References

<!-- MANDATORY for Stable. Stub — populate at implementation (P3). Stable
     blocked: (1) L1 parent Draft; (2) C29 — no validating examples/ yet. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-17 | Initial L2 draft — Go translation of l1-scene-system v0.4.0. gob StaticScene + reflection DynamicScene, two-pass remap instantiation (INV-5), interned binary codec, symmetric pack/unpack query API, SceneLoadContext. L1 Q1 carried as TBD. |
