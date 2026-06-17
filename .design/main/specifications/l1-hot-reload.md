# Hot-Reload System

**Version:** 0.2.0
**Status:** Stable
**Layer:** concept

## Overview

The Hot-Reload System enables rapid iteration during development by allowing code, data, and shader changes to take effect without manually restarting the engine. For data assets (JSON definitions, textures, audio), the engine reloads files in-place via the Asset System's file watcher. For Go code, the system uses a **process-restart with state snapshot** strategy — serializing the World state, rebuilding the binary, and restoring state into the new process. For shaders, the render pipeline recompiles and hot-swaps shader programs without interrupting the frame loop.

This specification covers the Go code reload pipeline, shader hot-swap, and the orchestration layer that ties these mechanisms together in the editor workflow.

## Related Specifications

- [asset-system.md](l1-asset-system.md) — File watcher and hot-reload for data assets (§4.8 in definition-system)
- [definition-system.md](l1-definition-system.md) — JSON definition hot-reload workflow (§4.8)
- [scene-system.md](l1-scene-system.md) — World serialization and entity remapping used for state snapshots
- [app-framework.md](l1-app-framework.md) — Plugin lifecycle, SubApp isolation, RunMode
- [scripting-system.md](l1-scripting-system.md) — Lua bridge provides an alternative hot-reload path for gameplay logic
- [render-core.md](l1-render-core.md) — Shader compilation and render pipeline stages
- [diagnostic-system.md](l1-diagnostic-system.md) — Reload metrics and error overlay
- [profiling-protocol.md](l1-profiling-protocol.md) — Reload cycle timing spans

## 1. Motivation

The edit-compile-run cycle in Go is fast (sub-second for incremental builds), but restarting the engine loses all runtime state: entity positions, component values, camera angles, debug UI, and the current game flow state. For a game developer iterating on gameplay code, this means:

- Navigate back to the scene under test after every code change.
- Re-trigger the game state that exposed the bug.
- Lose any debug breakpoints or inspector state in the editor.

Hot-reload eliminates this friction by preserving the World across code changes. The developer edits a system function, saves, and sees the new behavior applied to the current scene within 1-2 seconds.

For shaders, the problem is even more acute — shader iteration requires visual feedback. Restarting the engine to see a material tweak is unacceptable. Shader hot-swap provides immediate visual feedback without interrupting the frame loop.

## 2. Constraints & Assumptions

- Go does not support in-process code replacement. Hot-reload of Go code requires a new process.
- The `plugin` package (`.so` shared objects) is Linux-only, does not support unloading, and breaks on type changes. It is NOT used as a hot-reload mechanism.
- State snapshot relies on the engine's serialization infrastructure (TypeRegistry + scene serialization). Components that are not serializable cannot survive a reload.
- Hot-reload is a **development-only** feature. It is excluded from production builds via `//go:build editor` tags.
- The editor orchestrates the reload cycle. Headless/standalone builds do not include hot-reload infrastructure.
- Shader hot-reload operates in-process and does not require a process restart.
- The file watcher (from the Asset System) is the sole trigger mechanism — no polling.

## 3. Core Invariants

- **INV-1**: A hot-reload cycle must not corrupt World state. If deserialization fails, the engine falls back to a clean restart with an error overlay.
- **INV-2**: Non-serializable components are explicitly dropped during reload. The system logs which components were lost so the developer can add serialization support.
- **INV-3**: Shader hot-swap must not produce GPU errors. If compilation fails, the previous shader remains active and the error is displayed in the diagnostic overlay.
- **INV-4**: Hot-reload infrastructure adds zero overhead to production builds (fully excluded by build tags).
- **INV-5**: The reload cycle preserves entity IDs. After reload, `Entity(42, gen=7)` in the old process maps to `Entity(42, gen=7)` in the new process.

## 4. Detailed Design

### 4.1 Reload Strategy Overview

Three hot-reload mechanisms, each targeting a different asset type:

```plaintext
Mechanism             Target           Process Impact    Latency
-------------------------------------------------------------------
Asset Hot-Reload      JSON, textures   In-process        < 100ms
                      audio, configs
Shader Hot-Swap       GLSL/SPIR-V      In-process        < 500ms
Code Hot-Restart      Go source files  New process        1-3s
```

Asset hot-reload is already covered by the Asset System and Definition System. This spec focuses on **Code Hot-Restart** and **Shader Hot-Swap**.

### 4.2 Code Hot-Restart Pipeline

The code hot-restart cycle is orchestrated by the editor process (or a standalone watcher daemon during headless development):

```plaintext
Phase 1 — Detect
  FileWatcher monitors *.go files in the project directory.
  On change: debounce 200ms (batch rapid saves).
  Notify the reload orchestrator.

Phase 2 — Snapshot
  ReloadOrchestrator sends a HotReloadPrepare event to the running engine.
  The engine:
    1. Pauses the main loop (freezes at end of current frame).
    2. Serializes the World state to a snapshot file:
       - All serializable components (via TypeRegistry).
       - All serializable resources.
       - Entity hierarchy (parent-child relationships).
       - Current game state (FlowState, active schedules).
       - Camera transform and viewport configuration.
    3. Writes snapshot to a temp file: {project}/.hot-reload/snapshot.bin
    4. Writes a manifest listing non-serializable components that were dropped.
    5. Sends HotReloadReady signal to the orchestrator.
    6. Exits cleanly (Plugin.Cleanup in reverse order).

Phase 3 — Rebuild
  Orchestrator runs: go build -o {binary} ./cmd/{target}/
  If build fails:
    - Display compilation errors in the editor's error panel.
    - Do NOT kill the old process (it already exited in Phase 2).
    - Offer "Restart with old binary" fallback.
  If build succeeds: proceed to Phase 4.

Phase 4 — Restore
  Orchestrator launches the new binary with a flag:
    ./{binary} --hot-reload={project}/.hot-reload/snapshot.bin
  The new engine process:
    1. Initializes plugins normally (Build → Ready → Finish).
    2. Before entering the main loop, detects --hot-reload flag.
    3. Deserializes the snapshot into the World:
       - Entities are allocated with the SAME IDs as the snapshot.
       - Components are deserialized via TypeRegistry.
       - Resources are restored.
       - Hierarchy is rebuilt.
       - Game state is restored.
    4. Logs any components that existed in the snapshot but are no longer
       registered (type removed or renamed between builds).
    5. Fires a HotReloadComplete event.
    6. Resumes the main loop from the restored state.

Phase 5 — Cleanup
  Delete {project}/.hot-reload/snapshot.bin
  Editor reconnects to the new process (if using IPC).
```

### 4.3 State Snapshot Format

The snapshot uses the engine's binary serialization format (same as scene save/load) with additional metadata:

```plaintext
HotReloadSnapshot
  header:
    engine_version:   string        // must match on restore
    snapshot_version: uint32        // format version for forward compat
    timestamp:        int64         // unix nanos
    entity_count:     uint32
    component_types:  []string      // registered type names at snapshot time

  world_state:
    entities:         []EntitySnapshot
    resources:        []ResourceSnapshot
    hierarchy:        []ParentChildPair

  app_state:
    flow_state:       string        // current FlowState name
    active_schedules: []string      // which schedules were running
    camera_state:     CameraSnapshot
    time_state:       TimeSnapshot  // elapsed, frame_count (NOT delta)

EntitySnapshot
  id:          EntityID              // index + generation
  archetype:   string                // archetype signature for validation
  components:  map[string][]byte     // type_name -> serialized bytes

ResourceSnapshot
  type_name:   string
  data:        []byte

CameraSnapshot
  transform:   Transform
  projection:  ProjectionData
  viewport:    Rect
```

### 4.4 Entity ID Preservation

Entity ID stability across reloads is critical — systems that cache entity references (UI bindings, audio emitter links, scripting handles) must not break.

```plaintext
Restore strategy:
  1. Pre-allocate entity ID space to match snapshot entity_count.
  2. For each EntitySnapshot, allocate with the EXACT same index and generation.
  3. The EntityAllocator's free list is reconstructed from gaps in the snapshot.
  4. After restore, the allocator continues from max(snapshot_index) + 1.
```

**Type rename handling**: If a component type was renamed between builds, the snapshot contains the old name. The TypeRegistry supports optional migration aliases:

```plaintext
TypeRegistry
  RegisterAlias(old_name: string, new_type: reflect.Type)

// Example: component "PlayerHealth" renamed to "Health"
registry.RegisterAlias("PlayerHealth", reflect.TypeOf(Health{}))
```

Aliases are checked during deserialization. Without an alias, the component is dropped and logged.

### 4.5 Shader Hot-Swap

Shader hot-reload operates entirely in-process — no restart needed:

```plaintext
Phase 1 — Detect
  FileWatcher monitors *.glsl, *.vert, *.frag, *.comp, *.spv files.
  On change: debounce 100ms.

Phase 2 — Compile
  ShaderCompiler compiles the changed shader source.
  If compilation fails:
    - Log the error with file:line information.
    - Display error in the diagnostic overlay (shader error panel).
    - Keep the previous compiled shader active (no visual glitch).
    - Return early — do not proceed to swap.

Phase 3 — Swap
  If compilation succeeds:
    - The RenderPipeline replaces the shader program handle.
    - Materials referencing the old shader are rebound to the new program.
    - GPU resources for the old shader are released after the current frame completes
      (double-buffered swap to avoid mid-frame state change).

Phase 4 — Notify
  Fire ShaderReloaded event with the shader asset path.
  Diagnostic overlay shows "Shader reloaded: {path}" for 2 seconds.
```

**Uniform preservation**: When a shader is hot-swapped, existing uniform values are preserved if the uniform name and type match. New uniforms get default values. Removed uniforms are silently dropped.

### 4.6 Reload Orchestrator

The orchestrator is a lightweight coordinator that manages the reload lifecycle:

```plaintext
ReloadOrchestrator
  mode:            ReloadMode          // CodeRestart | ShaderSwap | DataReload
  state:           OrchestratorState   // Idle | Detecting | Snapshotting | Building | Restoring
  watcher:         FileWatcher
  build_command:   string              // configurable: "go build ./cmd/game/"
  snapshot_dir:    string              // default: {project}/.hot-reload/
  debounce_ms:     int                 // default: 200
  auto_reload:     bool                // default: true in editor mode

  // File pattern → reload mode mapping
  rules:
    "*.go"         -> CodeRestart
    "*.glsl"       -> ShaderSwap
    "*.vert"       -> ShaderSwap
    "*.frag"       -> ShaderSwap
    "*.json"       -> DataReload      // handled by Asset System
    "*.png"        -> DataReload
    "*.ogg"        -> DataReload
```

**Configuration**: The orchestrator reads settings from a project-level config file (`hot-reload.json` or engine settings resource). Developers can customize the build command, debounce timing, and file pattern rules.

**Manual trigger**: The editor exposes a "Reload Now" button (and keyboard shortcut) that bypasses the file watcher and immediately triggers a code hot-restart. Useful when the developer has made changes outside the watched directory or wants to force a reload.

### 4.7 Editor Integration

The editor process acts as the reload orchestrator's host:

```plaintext
Editor ←──IPC──→ Engine Process

Editor responsibilities:
  - Hosts the FileWatcher (survives engine restart).
  - Displays build errors, reload status, dropped component warnings.
  - Reconnects to the new engine process after code hot-restart.
  - Preserves editor-side state (inspector selection, viewport layout,
    breakpoints) independently of engine state.

IPC protocol:
  Editor → Engine:  HotReloadPrepare, HotReloadAbort
  Engine → Editor:  HotReloadReady, HotReloadFailed(reason)
  Engine → Editor:  ShaderError(file, line, message)
  Engine → Editor:  ReloadMetrics(snapshot_ms, build_ms, restore_ms)
```

**Headless development**: For developers not using the editor, a standalone `hot-reload-daemon` binary provides the same file watching and orchestration via a terminal UI. It displays build output, reload timing, and dropped component warnings in the console.

### 4.8 Incremental Build Optimization

Go's incremental compilation is already fast, but additional optimizations reduce reload latency:

```plaintext
Optimization                          Impact
--------------------------------------------------------------
go build -trimpath                    Reproducible builds, better cache hits
GOFLAGS=-count=1                      Avoid redundant test cache invalidation
Separate cmd/ entry points            Only rebuild changed packages
Build cache warming on editor start   First reload is as fast as subsequent ones
-overlay flag (future)                Replace single file without full rebuild
```

**Build cache**: The orchestrator runs `go build` once at editor startup to warm the build cache. Subsequent builds only recompile changed packages — typically under 500ms for a single-file change.

**Parallel build + snapshot**: The orchestrator can start `go build` as soon as the file change is detected (Phase 3), while the snapshot (Phase 2) happens in parallel. The engine only needs to pause when the build succeeds and the snapshot is requested.

```plaintext
Optimized timeline:
  t=0     File change detected
  t=0     go build starts (background)
  t=800ms Build succeeds → request snapshot from engine
  t=900ms Engine pauses, writes snapshot (100ms)
  t=1000ms New process launches with snapshot
  t=1200ms World restored, main loop resumes
  ─────────────────────────────────────────
  Total:  ~1.2s (vs ~2.5s sequential)
```

### 4.9 Failure Modes and Recovery

```plaintext
Failure                     Recovery
-------------------------------------------------------------------
Build error                 Display errors. Old process already exited →
                            offer "Restart with old binary" or
                            "Wait for fix and retry".

Snapshot serialization      Skip non-serializable components, log warnings.
  failure (partial)         Proceed with partial state.

Snapshot deserialization    Abort restore. Start with clean World.
  failure (corrupt)         Display error overlay with details.

Type mismatch               Drop components with changed struct layout.
  (field added/removed)     Log which entities lost which components.

Engine version mismatch     Reject snapshot. Clean start.

Shader compilation error    Keep old shader. Show error in overlay.
                            Auto-retry on next save.

IPC disconnect              Editor auto-reconnects with exponential backoff.
  (editor ↔ engine)         Engine continues running without editor.
```

### 4.10 Reload Scope Control

Not all code changes require a full World restore. The orchestrator classifies changes by scope:

```plaintext
ChangeScope
  SystemOnly     // only system function bodies changed → restore full state
  ComponentType  // component struct fields changed → drop affected components
  ResourceType   // resource struct changed → drop affected resource
  PluginAPI      // plugin interface changed → clean restart (no restore)
  CoreEngine     // world/entity/archetype internals → clean restart

Detection heuristic (best-effort):
  1. Parse changed *.go files for modified type declarations.
  2. If only func bodies changed → SystemOnly.
  3. If struct fields changed → check if struct is a component/resource type.
  4. If unsure → default to SystemOnly (safest common case).
```

**SystemOnly** is the fast path — the World snapshot is fully valid because no types changed. This covers the most common iteration pattern: tweaking system logic.

## 5. Open Questions

- Should the snapshot format be human-readable (JSON) for debugging, or binary-only for speed? *(resolved — L2 chose interned binary codec for speed; debug JSON dump is additive and deferred)*
- Should hot-reload support partial World snapshots (only serialize entities in the current scene)? *(non-blocking — deferred to future revision)*
- How should hot-reload interact with active network connections (multiplayer testing)? *(non-blocking — sockets are non-serializable; they are reconstructed post-reload by plugin Build/Ready)*
- Should the orchestrator support "rewind" — restoring a previous snapshot after a bad reload? *(non-blocking — deferred to future revision)*
- Should shader hot-reload support SPIR-V bytecode replacement, or only source recompilation? *(non-blocking — current implementation: source recompilation only; SPIR-V replacement deferred)*
- How should hot-reload handle changes to init/startup systems that ran once at launch? *(non-blocking — current behavior: clean restart; once-only init re-runs in the new process)*

## Canonical References

| Alias | Path | Purpose |
| :--- | :--- | :--- |
| snapshot | internal/hotreload/snapshot.go | World capture via DynamicScene codec |
| restore | internal/hotreload/restore.go | Transactional restore + IdentityRemapper entity-ID pinning |
| format | internal/hotreload/format.go | Snapshot header/AppState serialization structures |
| shader | internal/hotreload/shader.go | ShaderReloader in-process double-buffered swap |
| scope | internal/hotreload/scope.go | ClassifyChange AST heuristic (SystemOnly/ComponentAdded/TypeChanged) |
| orchestrator | internal/hotreload/orchestrator.go | RouteFile + build pipeline coordination |
| plugin | internal/hotreload/plugin.go | HotReloadPlugin — App integration entry point (`//go:build editor`) |
| daemon | cmd/hot-reload-daemon/main.go | Editor daemon binary |
| releaseguard | internal/releaseguard/hotreload_guard_test.go | INV-4 build-absence guard (verifies editor tag isolation) |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-03-28 | Initial draft: process-restart with state snapshot, shader hot-swap, orchestrator design |
| 0.2.0 | 2026-06-16 | Promoted Draft → Stable: Canonical References populated; open questions annotated (Q1 resolved: binary codec chosen; Q2–Q6 non-blocking). Implementation complete: `internal/hotreload` (snapshot/restore/shader/scope/orchestrator/plugin), `cmd/hot-reload-daemon`, `internal/releaseguard` INV-4 guard. |
