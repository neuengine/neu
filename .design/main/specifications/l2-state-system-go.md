# State System — Go Implementation

**Version:** 0.1.0
**Status:** Stable
**Layer:** go
**Implements:** [state-system.md](l1-state-system.md)

## Overview

This specification defines the Go implementation of the state system described in the L1 concept spec. The state system provides hierarchical finite state machines for controlling application flow. States are `comparable` types (typically `uint8` iota enums) stored as ECS resources. The system supports state-keyed schedules (`OnEnter`, `OnExit`, `OnTransition`), SubStates dependent on parent state values, ComputedStates derived from other states, and automatic entity despawning on state exit. All types live in the `internal/state` package with a dependency on `internal/ecs`.

## Related Specifications

- [state-system.md](l1-state-system.md) — L1 concept specification (parent)

## 1. Motivation

The Go implementation of the State system provides a hierarchical, type-safe finite state machine (FSM) for controlling high-level application flow. It ensures:

- Clear separation of concerns via state-keyed schedules (`OnEnter`, `OnExit`).
- Robust management of dependent logic via `SubStates` and `ComputedStates`.
- Automated lifecycle management through `DespawnOnExit` markers.
- Efficient state gating using `InState` run conditions.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: Relies on generics for type-safe state resources and `comparable` constraints for transition checks.
- **Single Transition**: Only one transition per state type can be pending at a time (last one wins).
- **Schedule Flush**: Transitions are processed at explicit synchronization points in the main schedule.

## 3. Core Invariants

> [!NOTE]
> See [state-system.md §3](l1-state-system.md) for technology-agnostic invariants.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: Exclusive States | `State[S]` resource holds only one `current` value of type `S`. |
| **INV-2**: Transition Guards | `state_transition` system enforces the `OnExit -> Swap -> OnEnter` sequence. |
| **INV-3**: Hierarchical State | `SubState` activation/deactivation is driven by parent state transitions. |
| **INV-4**: Reactive States | `ComputedState` is re-evaluated immediately after its source state transitions. |
| **INV-5**: Automatic Cleanup | `DespawnOnExit` triggers immediate recursive despawning of marked entities. |

## Go Package

```
internal/state/
```

All types in this spec belong to package `state`. The package imports `internal/ecs` for World, resource, and schedule access.

## Type Definitions

### State[S]

```go
// State is a resource holding the currently active value of state type S.
// Read-only for user systems — transitions go through NextState.
// S must be comparable for use as map keys and equality checks.
type State[S comparable] struct {
    current S
}

// Current returns the active state value.
func (s *State[S]) Current() S
```

### NextState[S]

```go
// NextState is a resource used to request state transitions. A nil value
// pointer means no transition is pending. Systems write to this resource
// to trigger a transition during the next StateTransition schedule.
type NextState[S comparable] struct {
    value *S
}

// Set queues a transition to the given state value.
func (n *NextState[S]) Set(next S)

// Get returns the pending state value, or nil if no transition is pending.
func (n *NextState[S]) Get() *S

// Clear removes the pending transition.
func (n *NextState[S]) Clear()

// IsPending reports whether a transition is pending.
func (n *NextState[S]) IsPending() bool
```

### TransitionTo

```go
// TransitionTo is a convenience function that sets NextState for state type S.
func TransitionTo[S comparable](world *ecs.World, next S)
```

### State-Keyed Schedule Labels

```go
// OnEnter returns the schedule label for the OnEnter schedule of the given
// state value. Format: "OnEnter:{TypeName}:{value}"
func OnEnter[S comparable](value S) string

// OnExit returns the schedule label for the OnExit schedule of the given
// state value. Format: "OnExit:{TypeName}:{value}"
func OnExit[S comparable](value S) string

// OnTransition returns the schedule label for any transition of state type S.
// Format: "OnTransition:{TypeName}"
func OnTransition[S comparable]() string
```

### SubState[S]

```go
// SubState is an interface for states that are active only when a parent
// state has a qualifying value. When the parent transitions away from
// qualifying values, the SubState is automatically exited.
type SubState[S comparable] interface {
    // SourceStates returns the parent state values for which this SubState
    // is active. If the parent is not in one of these values, the SubState
    // is inactive (exited).
    SourceStates() []S
}
```

### ComputedState[S]

```go
// ComputedState is an interface for states whose value is derived
// automatically from other state resources. ComputedStates cannot be set
// directly — they are recalculated after every source state transition.
type ComputedState[S comparable] interface {
    // Compute derives the current state value from the world's state
    // resources. Returns a pointer to the computed value, or nil if the
    // computed state should be inactive.
    Compute(world *ecs.World) *S
}
```

### DespawnOnExit[S]

```go
// DespawnOnExit is a marker component. Entities with this component are
// automatically despawned when the state exits the specified value.
// S identifies the state type, and the State field holds the value to
// watch for exit.
type DespawnOnExit[S comparable] struct {
    State S
}
```

### Run Conditions

```go
// InState returns a run condition that is true when State[S].Current
// equals the target value. Used to gate system execution to specific states.
//
// Usage:
//   app.AddSystem(Update, movePlayer, ecs.RunIf(InState[GameState](GameStatePlaying)))
func InState[S comparable](target S) ecs.RunCondition

// StateChanged returns a run condition that is true when State[S]
// transitioned this frame (i.e., the StateTransition schedule processed
// a pending NextState for this type).
func StateChanged[S comparable]() ecs.RunCondition

// StateExists returns a run condition that is true when the State[S]
// resource exists in the World. This is false for inactive SubStates.
func StateExists[S comparable]() ecs.RunCondition
```

### StateConfig

```go
// StateConfig holds registration metadata for a state type. Used during
// plugin initialization to register state resources and schedules.
type StateConfig[S comparable] struct {
    InitialValue S
    SubStates    []any // registered SubState types dependent on this state
    Computed     []any // registered ComputedState types derived from this state
}
```

### StatePlugin

```go
// StatePlugin registers the StateTransition system and provides the
// infrastructure for state-keyed schedules.
type StatePlugin struct{}

// Build registers the StateTransition schedule in the main schedule order
// and provides helper methods on App for state registration.
func (p StatePlugin) Build(app *app.App)
```

## Key Methods

### State Transition Flow

The `StateTransition` schedule processes all pending transitions once per frame:

```
SYSTEM state_transition(world):
  FOR EACH registered state type S (in dependency order):
    next = world.Resource(NextState[S])
    IF NOT next.IsPending():
      CONTINUE

    new_value = *next.Get()
    state = world.Resource(State[S])
    old_value = state.Current()

    IF new_value == old_value:
      next.Clear()
      CONTINUE

    // Step 1: Run OnExit for old state
    RunSchedule(OnExit[S](old_value))

    // Step 2: Despawn marked entities
    FOR EACH entity WITH DespawnOnExit[S] WHERE component.State == old_value:
      DespawnRecursive(entity)  // includes children via hierarchy

    // Step 3: Swap current state
    state.current = new_value

    // Step 4: Run OnTransition
    RunSchedule(OnTransition[S]())

    // Step 5: Run OnEnter for new state
    RunSchedule(OnEnter[S](new_value))

    // Step 6: Clear pending transition
    next.Clear()

    // Step 7: Process SubStates
    process_substates(world, S, old_value, new_value)

    // Step 8: Recompute ComputedStates
    recompute_computed_states(world, S)
```

### SubState Processing

```
FUNCTION process_substates(world, parent_type, old_parent, new_parent):
  FOR EACH SubState SS registered under parent_type:
    was_active = old_parent IN SS.SourceStates()
    is_active = new_parent IN SS.SourceStates()

    IF was_active AND NOT is_active:
      // Parent left qualifying value — exit SubState
      sub_state = world.Resource(State[SS])
      RunSchedule(OnExit[SS](sub_state.Current()))
      Despawn entities with DespawnOnExit[SS]
      REMOVE State[SS] resource from world  // SubState becomes inactive

    ELSE IF NOT was_active AND is_active:
      // Parent entered qualifying value — enter SubState with default
      INSERT State[SS]{current: SS.DefaultValue} into world
      INSERT NextState[SS]{} into world
      RunSchedule(OnEnter[SS](SS.DefaultValue))
```

### ComputedState Recomputation

```
FUNCTION recompute_computed_states(world, source_type):
  FOR EACH ComputedState CS that depends on source_type:
    old_value = world.Resource(State[CS]).Current()  // may not exist
    new_ptr = CS.Compute(world)

    IF new_ptr == nil AND State[CS] exists:
      // Computed state became inactive
      RunSchedule(OnExit[CS](old_value))
      REMOVE State[CS] from world

    ELSE IF new_ptr != nil AND State[CS] does not exist:
      // Computed state became active
      INSERT State[CS]{current: *new_ptr} into world
      RunSchedule(OnEnter[CS](*new_ptr))

    ELSE IF new_ptr != nil AND *new_ptr != old_value:
      // Computed state value changed
      RunSchedule(OnExit[CS](old_value))
      State[CS].current = *new_ptr
      RunSchedule(OnEnter[CS](*new_ptr))
```

### State Registration on App

```
FUNCTION App.InitState[S](initial S):
  world.InsertResource(State[S]{current: initial})
  world.InsertResource(NextState[S]{})

  // Register OnEnter/OnExit schedules for each known value.
  // For iota enums, the user must register values explicitly or use
  // a registration helper.

  // Register state in the StateTransition processing order.
  stateRegistry.Register(typeOf(S), initial)
```

### Recommended State Type Pattern

Go does not have sum types. States should be defined as `uint8` iota enums:

```go
// Example (user code, not engine code):
type GameState uint8

const (
    GameStateMenu    GameState = iota
    GameStateLoading
    GameStatePlaying
    GameStatePaused
    GameStateGameOver
)
```

This satisfies `comparable`, is efficient as a map key, and works well with `iota`.

## Performance Strategy

- **State resources are value types**: `State[S]` and `NextState[S]` are tiny structs. Access is a direct resource lookup (O(1) by type ID), no allocation.
- **Transition check is a nil pointer check**: `NextState.value == nil` — single comparison per state type per frame.
- **`comparable` constraint**: States as `uint8` enums are cheap to compare and use as map keys. No reflection needed at runtime.
- **DespawnOnExit query**: Uses a standard ECS query with a filter. Executed only during transitions (not every frame).
- **Schedule labels as strings**: `OnEnter`/`OnExit` labels are computed once during registration and stored. The format `"OnEnter:GameState:2"` uses `fmt.Sprintf` at registration time, not at runtime.
- **Dependency order**: State types are sorted topologically (parent before child) once at registration. Transition processing iterates a pre-sorted slice.

## Error Handling

- **Transition to current state**: No-op. `NextState` is cleared without running OnExit/OnEnter. This is intentional — not an error.
- **Transition during OnEnter/OnExit**: Deferred to the next `StateTransition` cycle (INV-6 from L1). The transition is queued in `NextState` and picked up on the next frame.
- **Invalid SubState source**: If a SubState declares source states that include values the parent type does not have, this is a programming error. Detected at registration with a panic.
- **Nil ComputedState.Compute**: A `Compute` returning nil means the computed state is inactive. This is valid behavior, not an error.
- **Missing State resource**: `InState` run condition returns false if `State[S]` resource does not exist (inactive SubState).
- **Duplicate state registration**: `App.InitState` called twice for the same type logs a warning and is a no-op.

## Testing Strategy

- **Unit tests**: Register state, verify `State[S].Current()` returns initial value. Set `NextState`, run `StateTransition`, verify current updates.
- **OnEnter/OnExit**: Register counting systems on `OnEnter` and `OnExit`. Transition states, verify each fires exactly once.
- **Transition order**: Verify OnExit(old) runs before OnEnter(new). Use a shared log to record execution order.
- **DespawnOnExit**: Spawn entities with `DespawnOnExit[GameState]{State: Menu}`. Transition from Menu to Playing. Verify entities are despawned.
- **SubState activation**: Register SubState active when parent is Playing. Transition parent to Playing, verify SubState enters default. Transition parent away, verify SubState exits.
- **ComputedState**: Register ComputedState that is active when parent is Playing. Verify it activates/deactivates with parent transitions.
- **No-op transition**: Set NextState to current value, verify no OnExit/OnEnter fires.
- **InState run condition**: Register system with `InState(Playing)` condition. Verify it runs only when state is Playing.
- **Integration**: Register StatePlugin in an App, init state, add OnEnter systems, run app for 2 frames with a transition queued.
- **Benchmarks**: `BenchmarkStateTransition` with 5 state types, 3 with SubStates. Target: negligible overhead on frames with no transitions.

## 7. Drawbacks & Alternatives

- **Drawback**: State-keyed schedules can lead to complex "hidden" logic if too many transitions are chained.
- **Alternative**: Centralized switch-based update logic.
- **Decision**: State-keyed schedules are preferred for modularity and are safer when using visual debugging tools.

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
