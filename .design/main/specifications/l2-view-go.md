# Entity View Cache — Go Implementation

**Version:** 0.1.0
**Status:** Stable
**Layer:** go
**Implements:** [l1-ecs-lifecycle-patterns.md](l1-ecs-lifecycle-patterns.md)

## Overview

Go implementation of the reactive entity view cache (L1 §5.3). A `View` holds a stable, reactively-maintained list of `ArchetypeID`s that match a fixed `QueryState`. On construction the view performs a one-time scan of existing archetypes and registers a push-based listener with `ArchetypeStore`; new matching archetypes are appended automatically, eliminating per-frame full archetype-graph scans. A companion `tagger.go` provides type-safe helpers (`TagOf`, `MaskOf`, `MaskOfN`) that resolve component IDs and build query masks without scattering `reflect` plumbing across system code.

## Related Specifications

- [l1-ecs-lifecycle-patterns.md](l1-ecs-lifecycle-patterns.md) — L1 parent; §5.3 Entity Views (Caching) and INV-3.
- [l1-query-system.md](l1-query-system.md) — `QueryState` and `Mask` are the matching primitives this view wraps.
- [l2-query-system-go.md](l2-query-system-go.md) — provides `QueryState`, `Mask`, `ErrInvalidAccess`.
- [l2-world-system-go.md](l2-world-system-go.md) — provides `ArchetypeStore`, `OnArchetypeCreated`, `ListenerID`.
- [l2-pool-go.md](l2-pool-go.md) — transient view structs are recycled via `Pool[T]` (C27).

## 1. Motivation

Without caching, every system re-scans all archetypes each frame to find those matching its query. As archetype count grows (unique component combinations accumulate over a session), this O(K_archetypes) scan per system per frame wastes CPU even when the matching set is completely stable.

A `View` amortises the cost: it pays O(K_archetypes) once at construction, then receives zero-cost push notifications when the `ArchetypeStore` creates a new archetype. Subsequent frame iterations over matched entities are O(N_entities_in_matched_archetypes) — no wasted work on non-matching archetypes. The steady-state iteration path is allocation-free (Go 1.23 `iter.Seq`, no intermediate slice).

## 2. Constraints & Assumptions

- **Phase 1 scope (T-1I01)**: Type-erased entity iteration only. Component-bound cached queries (`Query1[T]`, `Query2[T,U]` backed by views) are deferred to Phase 2 (SystemParam fetching work).
- **View lifetime**: Views typically live as long as the World. Short-lived views MUST call `Close` to release the listener subscription; forgetting `Close` does not leak memory but will keep the matched list growing.
- **Entity IDs only**: `View.Entities()` yields `entity.Entity` identifiers — callers fetch component data via the World or the query system separately.
- **Append-only matched list**: once an archetype is added to `v.matched` it is never removed (archetypes are permanent in the current model). The list is monotonically growing.
- **O(K_matches) membership**: `Contains` iterates `v.matched` linearly. Prefer `Entities()` in tight loops over repeated `Contains` calls.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1 (Atomic Tag Updates)** | Not in scope — `view` reads archetype membership from `ArchetypeStore`; tag-update atomicity is enforced by `internal/ecs/world`. |
| **INV-2 (Deterministic Cleanup)** | `Close(w)` synchronously calls `ArchetypeStore.UnregisterListener(v.listenerID)`, zeroing the listener slot (tombstone). After `Close` the matched list is frozen. Calling `Close` more than once is a safe no-op (guarded by `listenerID == 0` sentinel). |
| **INV-3 (View Consistency)** | `New` subscribes to `ArchetypeStore.OnArchetypeCreated` before returning. Every archetype created after construction that satisfies `state.Matches(mask)` is appended via the callback. The view is never stale between an archetype-creation event and the caller's next `Entities()` call — no polling required. |

## 5. Detailed Design

### 5.1 `View` — Reactive Archetype Cache

**Project Structure:**

```plaintext
internal/ecs/view/
├── view.go      # View: New, Requiring, Close, Entities, Contains, Count, MatchedArchetypes
└── tagger.go    # TagOf[T], MaskOf[T], MaskOf2, MaskOf3, MaskOfIDs
```

**Struct (pseudo-code):**

```plaintext
View {
    state      *QueryState      // fixed match predicate — never mutated after construction
    matched    []ArchetypeID    // append-only cache; creation order preserved
    listenerID ListenerID       // 0 after Close (sentinel for idempotent Close)
}
```

**Construction flow:**

```plaintext
New(w *World, state *QueryState) *View:
  v = &View{state: state}
  store = w.Archetypes()
  store.Each(fn → v.consider(arch))        // initial scan: O(K_existing_archetypes)
  v.listenerID = store.OnArchetypeCreated(v.consider)
  return v

consider(arch *Archetype):                  // shared: scan callback + creation listener
  mask = MaskFromIDs(arch.ComponentIDs())
  if state.Matches(mask): v.matched = append(v.matched, arch.ID())
```

The same `consider` function serves both the initial scan and the push listener — no duplicated matching logic.

**Shorthand constructor:**

```plaintext
Requiring(w, ids...) (*View, error):
  state, err = NewQueryState(ids, nil, Access{})  // required set only; no excludes, no access tracking
  if err: return nil, err
  return New(w, state), nil
```

Use `Requiring` for the common "required component set" case. Use `New` only when a full `QueryState` (with excludes, access tracking, or tick filters) is needed.

### 5.2 Tagger Helpers

`tagger.go` resolves component IDs and builds query masks without `reflect` boilerplate at call sites:

| Helper | Signature | Returns | Notes |
| :--- | :--- | :--- | :--- |
| `TagOf[T]` | `(w *World) component.ID` | Stable ID for type `T` | Registers `T` via `Components().RegisterByType(reflect.TypeFor[T]())` on first call |
| `MaskOf[T]` | `(w *World) query.Mask` | Single-bit mask | Calls `TagOf[T]` internally |
| `MaskOf2[T1,T2]` | `(w *World) query.Mask` | Two-bit mask | — |
| `MaskOf3[T1,T2,T3]` | `(w *World) query.Mask` | Three-bit mask | — |
| `MaskOfIDs` | `(ids ...component.ID) query.Mask` | Mask from pre-resolved IDs | Avoids reflect when IDs are already known |

Component IDs returned by `TagOf[T]` are stable for the lifetime of the `World` — safe to cache at system-init time.

### 5.3 Iteration — Range-Over-Func

```plaintext
Entities(w *World) iter.Seq[entity.Entity]:
  store = w.Archetypes()
  return func(yield func(entity.Entity) bool):
    for each archID in v.matched:
      arch = store.At(archID)
      for each entity in arch.Entities():
        if !yield(entity): return    // early exit: caller breaks the range loop
```

- Uses Go 1.23 `iter.Seq[entity.Entity]` — compatible with `for e := range view.Entities(w)`.
- Allocation-free in steady state (no intermediate slice).
- Iteration order: archetype-creation order, then entity-insertion order within each archetype.
- Early exit (`break` in caller's range) is supported via the `yield` return value.

### 5.4 Membership Check

```plaintext
Contains(w *World, e entity.Entity) bool:
  if !w.Contains(e): return false          // dead or unspawned entity
  archID, ok = w.ArchetypeOf(e)
  if !ok: return false
  return slices.Contains(v.matched, archID) // O(K_matches): linear search
```

Suitable for sparse, per-entity lookups. Avoid in tight iteration loops — use `Entities()` instead.

### 5.5 Lifecycle and Listener Management

```plaintext
Close(w *World):
  if v.listenerID == 0: return          // already closed — no-op
  w.Archetypes().UnregisterListener(v.listenerID)
  v.listenerID = 0                      // freeze matched list

```

After `Close`, the `matched` slice is stable and read-only. The `ArchetypeStore` tombstones the listener slot; the closure is GC-eligible once the tombstone is compacted.

## 6. Implementation Notes

1. Create views at system initialization, not per-frame. The one-time scan cost at construction is paid once.
2. For systems that only need to count entities (not iterate), `Count(w)` is O(K_matches) over archetype sizes — cheaper than building an entity slice.
3. Component-bound views (where the view stores component pointers alongside entity IDs for O(1) component access) are out of T-1I01 scope. Track as Phase 2 work.
4. If a system must observe archetype removals (not currently supported — archetypes are permanent), the `consider` callback model would need a `removeListener` counterpart. File as a future extension.

## 7. Drawbacks & Alternatives

- **Polling alternative**: systems check `ArchetypeStore.Generation()` and rescan on change. Avoids listener registration overhead but re-introduces O(K_archetypes) cost on any structural change — worse for steady-state performance.
- **Component-bound cache (`Query1[T]`)**: the view could store `[]*T` component pointers alongside entity IDs for direct component access without a World lookup. Correct pointer invalidation across archetype moves requires additional bookkeeping — deferred to Phase 2.
- **Lock-free append**: the `consider` callback currently runs under `ArchetypeStore`'s internal lock. For workloads that create many archetypes in parallel, this could become a contention point — profile before optimising.

## Canonical References

<!-- Downstream agents: read ALL files below before implementing or extending the view package. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |
| `[VIEW]` | `internal/ecs/view/view.go` | Authoritative `View` implementation: reactive subscription, iterator, membership check |
| `[TAGGER]` | `internal/ecs/view/tagger.go` | Type-safe `TagOf[T]`, `MaskOf[T]`, `MaskOfN` helpers |
| `[BENCH]` | `internal/ecs/view/bench_test.go` | Steady-state iteration benchmark (zero alloc/op target) |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-28 | Initial Stable — reactive archetype cache, tagger helpers, range-over-func iteration, O(K_matches) membership check. Implements l1-ecs-lifecycle-patterns.md §5.3. T-1I01 scope: entity IDs only; component-bound caches deferred to Phase 2. |
