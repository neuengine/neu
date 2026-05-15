---
name: boltengine-reference
description: Architectural reference for the ECS game engine project. Use when creating or reviewing specifications.
---

# Bolt Engine Architecture Reference

Use this skill when working on Bolt engine specifications. It provides the complete
architectural map of the engine's ECS system and its Go implementation guidelines.

## When to Use

- Creating L1 (concept) or L2 (Go implementation) specifications
- Reviewing spec completeness against the engine's feature set
- Understanding how engine modules map to Go packages
- Resolving architectural questions about ECS design

## Bolt Engine Architecture

### Layer 1: ECS Core

These modules form the heart of the engine. Every game built on the engine depends on them.

#### Entity

- **Concept**: Lightweight identifier for a game object. No data, no behavior — just an ID.
- **Key Types**: `Entity` (generational index = ID + generation counter), `EntityAllocator`
- **Storage**: Generational arena — recycled IDs carry incremented generation to detect stale references
- **Collections**: `EntitySet`, `EntityHashMap`, `EntityHashSet`, `UniqueVec[Entity]`
- **Go Mapping**: `type EntityID uint64`, `type Entity struct { ID EntityID; Generation uint32 }`

#### Component

- **Concept**: Pure data attached to entities. No logic — systems process components.
- **Registration**: Components registered at runtime with metadata (storage type, hooks, required deps)
- **Storage Strategy**: Each component declares `Table` (dense, cache-friendly) or `SparseSet` (fast add/remove)
- **Required Components**: Component A can declare that adding it automatically adds Component B
- **Clone Behavior**: Per-component cloning strategies for entity duplication
- **Go Mapping**: `type Component interface{}` with struct tags for storage strategy
- **Single Responsibility**: Prefer multiple small components (e.g., `Jump`, `Walk`) over one large `Character` component.
- **Tagging**: Use zero-sized structs for filtering (e.g., `type PlayerTag struct{}`).
- **Internal Logic**: Read-only helper methods on components are acceptable for pure state queries (`IsExpired() bool`).

#### Storage

- **Concept**: The physical data layout behind ECS. Determines iteration performance.
- **Archetype**: A unique combination of component types. Entities with the same components share an archetype.
- **Table Storage**: Column-oriented (SoA). Each component type = one column. Entities = rows. Cache-friendly iteration.
- **Sparse Set**: Entity-indexed sparse array. O(1) add/remove but less cache-friendly for bulk iteration.
- **Blob Array**: Type-erased contiguous storage for heterogeneous data.
- **Go Mapping**: `[]ComponentData` slices per archetype column, `map[EntityID]int` for sparse sets

#### Query

- **Concept**: The way systems access entity data. Declarative "give me all entities with these components".
- **WorldQuery**: Interface defining what data a query fetches (component refs, entity IDs, optional components)
- **Filters**: `With[T]`, `Without[T]`, `Changed[T]`, `Added[T]`, `Or[A, B]`
- **Access Tracking**: Queries declare read/write access. Scheduler uses this for parallelism.
- **Iteration**: Sequential (`Iter()`), parallel (`ParIter()`), single-entity (`Get()`)
- **QueryState**: Cached query that tracks which archetypes match — avoids re-scanning.
- **Go Mapping**: Query builder pattern with generics, `Query[T1, T2]` via type parameters

#### System

- **Concept**: Functions that process components. All game logic lives in systems.
- **Function Systems**: Plain functions with special parameter types → auto-converted to systems
- **System Parameters**: `Query`, `Res[T]`, `ResMut[T]`, `Commands`, `EventReader[T]`, `Local[T]`
- **Exclusive Systems**: Systems with `*World` access — run alone, cannot be parallelized
- **Conditions**: `RunIf(condition)` — systems skip execution based on runtime predicates
- **Commands**: Deferred mutations (spawn, despawn, insert). Applied between system runs.
- **Go Mapping**: `type System interface { Update(world *World, dt float64) }` + param injection
- **Specialization**: Systems should depend on the minimum number of components. Split complex systems into smaller ones to allow partial reuse.
- **Graduation Pattern**: Start logic as a specific "Script" (specific entity behavior) and promote to a "System" (generic world-wide logic) when it reaches stability and reusability.

#### Schedule

- **Concept**: Orchestration layer — decides system execution order and parallelism.
- **Schedule**: Named collection of systems with ordering constraints
- **System Sets**: Named groups for applying shared configuration (ordering, conditions)
- **Ordering**: `Before()`, `After()`, `Chain()` — explicit dependency declarations
- **Executor**: Single-threaded or multi-threaded. Uses access tracking for safe parallelism.
- **Stepping**: Debug tool — step through systems one at a time
- **Auto Apply Deferred**: Deferred commands applied at synchronization points
- **Frame Delay Mitigation**: Reorder systems to avoid one-frame lag; use lifecycle hooks for immediate init; place visual finalization in `PostUpdate`.
- **Ordering Tests**: Expose `GetExecutionOrder()` and `DetectAmbiguities()` for CI-level verification of system ordering constraints.
- **Go Mapping**: `type Schedule struct { systems []SystemNode; executor Executor }`

#### World

- **Concept**: The central data store. Owns all entities, components, resources, and schedules.
- **Entity CRUD**: Spawn, despawn, insert/remove components, get/query
- **Resources**: Global singletons (not entity-attached). `Res[T]` (read), `ResMut[T]` (write)
- **Change Detection**: Tick-based. Every mutation increments a tick counter. Queries detect changes.
- **Deferred World**: Limited world access for use inside hooks/observers (prevents re-entrancy)
- **Abstract Concept Entities**: Use entities for non-visible concepts (squads, sessions, quests) when multiple instances exist or lifecycle hooks are needed. Prefer `Res[T]` only for true singletons.
- **Entity Object Pooling**: Pre-spawn and disable entities for frequently created/destroyed archetypes (projectiles, particles). Use `DisabledTag` to hide from queries; reset on acquire.
- **Anti-Patterns**: No shared mutable refs between entities (use EntityID references). No bool flags for filtering (use tag components). No god components (split by concern).
- **Go Mapping**: `type World struct { entities EntityManager; archetypes []Archetype; resources ResourceMap }`

### Layer 2: ECS Extended

#### Events & Observers

- **Events**: Typed, double-buffered event bus. `EventWriter[T]` sends, `EventReader[T]` receives.
- **Entity Events**: Events targeted at specific entities
- **Observers**: Reactive triggers — fire when component added/removed/changed
- **Messages**: System-to-system communication channel (typed, ordered)
- **Go Mapping**: `type EventBus[T any] struct{}`, channel-based or ring-buffer

#### Bundles & Spawning

- **Bundles**: Groups of components for convenient spawning `world.Spawn(PlayerBundle{...})`
- **Spawn/Despawn**: Entity lifecycle with hooks (OnAdd, OnInsert, OnRemove)
- **Lifecycle**: Component add/remove tracking, RemovedComponents iteration
- **Go Mapping**: `type Bundle interface { Components() []Component }`

#### Relationships & Hierarchy

- **ChildOf**: Built-in parent-child relationship component
- **Children**: Automatically-maintained list of child entities
- **Custom Relations**: User-defined relationships with source collections
- **Traversal**: Tree walking utilities (depth-first, breadth-first)
- **Go Mapping**: `type ChildOf struct { Parent Entity }`, relationship queries

#### Change Detection

- **Ticks**: Global tick counter incremented each system run
- **Ref[T] / Mut[T]**: Smart wrappers that track when data was last changed
- **DetectChanges**: `IsChanged()`, `IsAdded()`, `LastChanged()` methods
- **Go Mapping**: `type Tracked[T any] struct { Value T; ChangedTick uint32 }`

### Layer 3: Engine Framework

#### App Framework

- **App Builder**: `NewApp().AddPlugins(DefaultPlugins).AddSystems(Update, mySystem).Run()`
- **Plugins**: Modular engine extensions. Each plugin configures the app.
- **Plugin Groups**: Collections of plugins (e.g., `DefaultPlugins`)
- **Sub-Apps**: Isolated app instances for pipelined rendering, etc.
- **Main Schedule**: `Startup`, `PreUpdate`, `Update`, `PostUpdate`, `Last`
- **Game Loop**: Fixed-tick logic + variable render, `context.Context` for shutdown
- **Go Mapping**: Builder pattern, `type Plugin interface { Build(app *App) }`

#### Task Parallelism

- **Task Pools**: `ComputeTaskPool` (CPU), `AsyncComputeTaskPool` (background), `IoTaskPool` (IO)
- **Task**: Future-like handle for async work
- **Go Mapping**: `goroutines + errgroup` (stdlib), `context.Context` for cancellation

#### Asset Management

- **Asset Server**: Loads assets asynchronously, returns handles
- **Handles**: Strong (keep alive) and Weak (don't prevent unload)
- **Hot Reloading**: File watcher detects changes, reloads affected assets
- **Asset Processor**: Transform assets at load time (compression, format conversion)
- **Go Mapping**: `type Handle[T any] struct { ID AssetID }`, `type AssetLoader interface{}`

#### Scene Management

- **Static Scene**: Serialized entity/component snapshot
- **Dynamic Scene**: Runtime-built scene from world data
- **Scene Spawner**: Instantiates scenes into the world
- **Go Mapping**: `type Scene struct { Entities []EntityRecord }`

#### Type Registry & Reflect

- **Type Registry**: Centralized metadata store — register types, query metadata at runtime
- **Introspection**: Access struct fields by name, iterate fields, get type info
- **Serialization**: Automatic serialization/deserialization via registry metadata
- **Dynamic Types**: Runtime-constructed proxy objects for editor/scripting
- **Go Mapping**: `reflect` stdlib + `type TypeRegistry struct{}` with registered component metadata

### Layer 4: Engine Systems

#### Input

- **`ButtonInput[T]`**: Generic pressed/just_pressed/released state tracking
- **Devices**: Keyboard (`KeyCode`), Mouse (`MouseButton`, `MouseMotion`), Gamepad, Touch
- **Go Mapping**: Polling model via `type InputState struct{}`

#### Transform

- **Transform**: Local position, rotation, scale
- **GlobalTransform**: Computed world-space transform (propagated from hierarchy)
- **Propagation System**: Walks parent→child tree updating global transforms
- **Go Mapping**: `type Transform struct { Position Vec3; Rotation Quat; Scale Vec3 }`

#### Math

- **Vectors**: Vec2, Vec3, Vec4 as value types (float32)
- **Matrices**: Mat4 as [16]float32
- **Quaternions**: Quat for rotations
- **Immutable Methods**: All operations return new values, no mutation
- **Go Mapping**: `func (v Vec3) Add(u Vec3) Vec3 { return Vec3{...} }`

#### Render Pipeline

- **Render Graph**: DAG of render passes
- **Extract**: Copy data from main world to render world (Sub-App separation)
- **Prepare**: Upload GPU resources
- **Queue**: Sort and batch draw calls
- **Render Phases**: Opaque, transparent, UI — each with sort rules
- **Backend Abstraction**: Pluggable interface — `type RenderBackend interface{}`
- **Go Mapping**: Supports multiple backends (OpenGL, Vulkan, WebGPU) via interface

#### Window

- **Window Component**: Entity-based window (multiple windows = multiple entities)
- **Window Events**: Resize, close, focus, cursor enter/leave
- **Go Mapping**: `type Window struct{}` component, platform-specific backend

#### State Machine

- **States**: Enum-based app states (Menu, Playing, Paused)
- **State Transitions**: `OnEnter`, `OnExit`, `OnTransition` schedules
- **Computed States**: Derived from other states
- **Sub-States**: Hierarchical states
- **Go Mapping**: `type State interface { comparable }`, transition hooks

#### Diagnostics

- **DiagnosticStore**: Named metrics with averaging (FPS, frame time, entity count)
- **Logging**: Structured logging via `log/slog` (Go stdlib)
- **Debug Overlay**: Runtime profiling, pprof integration
- **Go Mapping**: `type Diagnostic struct { Name string; History []float64 }`

#### Audio

- **Audio Sources**: Components representing sound emitters
- **Spatial Audio**: 3D audio positioning relative to listener entity
- **Backend Abstraction**: Pluggable audio backend interface
- **Go Mapping**: `type AudioSource struct{}` component, `type AudioBackend interface{}`

#### Config

- **Engine Configuration**: Centralized settings (window size, render quality, audio volume)
- **Persistence**: Save/load via TOML or JSON
- **Runtime Overrides**: CLI flags and environment variables
- **Go Mapping**: `type Config struct{}`, `encoding/json` stdlib

## Go-Specific Conventions

When creating L2 (Go) specifications, always follow these patterns:

1. **Package Layout**: `internal/` (engine) vs `game/` (user code), never mix
2. **Naming**: lowercase packages, `New{Type}` constructors, `iota` enums with type prefix
3. **Components**: Pure data structs, no logic methods
4. **Systems**: All logic via `System` interface, iterate via `Query`
5. **Errors**: Wrap with `fmt.Errorf("context: %w", err)`, sentinel errors, `Must*` only at init
6. **Performance**: Zero-alloc hot path, `sync.Pool`, SoA layout, slice reuse
7. **Concurrency**: Sequential by default, `errgroup` for parallel systems, `context.Context` everywhere
8. **Interfaces**: Defined at consumer, 1–3 methods max
9. **Math**: Value types, immutable methods returning new values
10. **Logging**: `log/slog` (stdlib, Go 1.21+)
11. **Testing**: `_test.go` colocated, benchmarks for hot paths, mocks via interfaces
