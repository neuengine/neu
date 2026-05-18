# State System

**Version:** 0.1.0
**Status:** Stable
**Layer:** concept

## Overview

The state system provides hierarchical finite state machines for controlling application flow. States are enum-based types that partition engine behavior into discrete modes (e.g., Menu, Playing, Paused). The system supports SubStates that depend on parent state values, ComputedStates derived automatically from other states, state-keyed schedules (OnEnter, OnExit, OnTransition), and convenience features like automatic entity despawning on state exit.

## Related Specifications

- [system-scheduling.md](l1-system-scheduling.md) — State transitions run in the StateTransition schedule
- [app-framework.md](l1-app-framework.md) — States registered as App resources; StateTransition is a main schedule
- [entity-system.md](l1-entity-system.md) — DespawnOnExit marker component targets entities for cleanup
- [definition-system.md](l1-definition-system.md) — Flow definitions describe state graphs declaratively in JSON

## 1. Motivation

Games are inherently state-driven. A main menu behaves differently from gameplay, which behaves differently from a pause screen. Without a state system:

- Systems use ad-hoc boolean flags to guard execution, leading to tangled conditions.
- Entering and exiting states requires manual setup and teardown scattered across systems.
- Hierarchical states (Playing contains Exploring, Combat, Inventory) are implemented inconsistently.
- Entity cleanup on state exit is forgotten, causing resource leaks.

A first-class state system makes state transitions explicit, provides lifecycle hooks, and automates common patterns like entity cleanup.

## 2. Constraints & Assumptions

- States are comparable types (typically enums via `iota` constants).
- State transitions are deferred — requested during a frame, applied during the StateTransition schedule.
- Only one value of each state type is active at a time.
- SubStates are only active when their parent state has a qualifying value.
- State transition schedules run in a deterministic order within the StateTransition schedule.
- The state system is part of the engine core and has zero external dependencies (C24).

## 3. Core Invariants

- **INV-1**: At most one value per state type is active at any time.
- **INV-2**: OnExit for the old state runs before OnEnter for the new state.
- **INV-3**: SubStates are automatically exited when their parent state transitions to a non-qualifying value.
- **INV-4**: ComputedStates are updated after all source states have been processed in the current transition.
- **INV-5**: Entities marked with `DespawnOnExit[S]` are despawned immediately when state S exits, before OnEnter of the new state runs.
- **INV-6**: State transitions requested during OnEnter/OnExit are deferred to the next StateTransition cycle.

## 4. Detailed Design

### 4.1 States Trait

Any type used as a state must satisfy:

```
States interface:
  - Comparable (supports == and !=)
  - Has a defined set of valid values
  - Provides a default/initial value
```

Example conceptual state definition:

```
AppState: Menu | Playing | Paused | GameOver

SubState of Playing:
  PlayingState: Exploring | Combat | Inventory

ComputedState derived from AppState and PlayingState:
  InCombat: true when AppState == Playing AND PlayingState == Combat
```

### 4.2 State Resources

Each registered state type creates two resources in the World:

```
Resource: State[S]
  - Current  S          // the active state value (read-only to user systems)

Resource: NextState[S]
  - Pending  Option[S]  // None if no transition requested, Some(value) if queued
```

Systems request transitions by writing to `NextState[S]`:

```
system request_pause(next: ResMut[NextState[AppState]], input: Res[ButtonInput[KeyCode]]):
    if input.JustPressed(KeyEscape):
        next.Set(AppState.Paused)
```

### 4.3 State Transition Schedule

The StateTransition schedule runs once per frame (see [app-framework.md](l1-app-framework.md)) and processes all pending transitions:

```
For each registered state type S (in dependency order):
  1. Check NextState[S].Pending
  2. If pending is Some(new_value) and new_value != current:
     a. Run OnExit[old_value] schedule
     b. Despawn entities with DespawnOnExit[S] component
     c. Update State[S].Current = new_value
     d. Run OnTransition[S] schedule
     e. Run OnEnter[new_value] schedule
     f. Clear NextState[S].Pending
  3. Process SubStates of S (see 4.4)
  4. Recompute ComputedStates that depend on S (see 4.5)
```

### 4.4 SubStates

A SubState declares a dependency on a parent state and the parent values for which it is active:

```
PlayingState is SubState of AppState:
  Active when: AppState == Playing
  Values: Exploring | Combat | Inventory
  Default when activated: Exploring
```

When the parent transitions to a qualifying value, the SubState enters its default value (triggering OnEnter). When the parent transitions away, the SubState exits (triggering OnExit) and becomes inactive.

SubStates can themselves have SubStates, forming a hierarchy:

```
AppState (root)
  └── PlayingState (active when Playing)
       └── CombatPhase (active when Combat)
            Values: PlayerTurn | EnemyTurn | Resolution
```

### 4.5 ComputedStates

A ComputedState derives its value from one or more source states via a pure function:

```
ComputedState: IsPaused
  Sources: AppState
  Compute: func(app AppState) -> Option[IsPaused]:
    if app == Paused: return Some(IsPaused.Yes)
    return None  // computed state is inactive
```

ComputedStates cannot be set directly. They are recalculated after every source state transition. They support OnEnter/OnExit schedules just like regular states.

### 4.6 State-Keyed Schedules

Each state value gets its own schedule labels:

```
OnEnter[AppState.Playing]   — runs once when entering Playing
OnExit[AppState.Playing]    — runs once when leaving Playing
OnTransition[AppState]      — runs on any AppState transition (has access to old and new values)
```

Systems registered in these schedules run only during their associated transition:

```
app.AddSystems(OnEnter[AppState.Playing], spawn_player, spawn_level)
app.AddSystems(OnExit[AppState.Playing], save_progress)
```

### 4.7 DespawnOnExit[S] Marker Component

A marker component that tags entities for automatic cleanup:

```
Component: DespawnOnExit[S]
  - S: the state type to bind to

When State[S] exits its current value:
  - Query all entities with DespawnOnExit[S]
  - Despawn each (including children via hierarchy)
```

This eliminates the need to manually track and clean up state-specific entities:

```
// Spawning a menu entity that auto-cleans when leaving Menu state
commands.Spawn(MenuButton{...}, DespawnOnExit[AppState]{State: AppState.Menu})
```

### 4.8 Run Conditions

The state system provides built-in run conditions for gating system execution:

```
in_state(value S) -> bool
  Returns true if State[S].Current == value

state_changed[S]() -> bool
  Returns true if State[S] transitioned this frame

state_exists[S]() -> bool
  Returns true if State[S] resource exists in the World (SubState is active)
```

Usage:

```
app.AddSystems(Update, move_player.RunIf(in_state(AppState.Playing)))
app.AddSystems(Update, animate_menu.RunIf(in_state(AppState.Menu)))
```

## 5. Open Questions

- Should state transitions support transition guards (conditions that can reject a transition)?
- Should there be a state history / stack for "return to previous state" patterns (e.g., unpause returns to Playing)?
- How should state transitions interact with the fixed timestep loop — can transitions happen mid-FixedUpdate?
- Should OnTransition schedules receive both old and new state values as resources?

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
| — | — | Planned examples: `examples/state/` |
