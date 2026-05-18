# Asset System

**Version:** 0.3.0
**Status:** Stable
**Layer:** concept

## Overview

The asset system manages loading, tracking, and lifetime of external resources (textures, meshes, audio, scenes). It provides asynchronous loading through the task system, reference-counted handles, hot-reload support, and an extensible loader architecture. All loaded assets are stored in typed ECS resources accessible to systems.

## Related Specifications

- [task-system.md](l1-task-system.md) — Async loading runs on the IO pool
- [scene-system.md](l1-scene-system.md) — Scenes are assets loaded through this system
- [event-system.md](l1-event-system.md) — Asset lifecycle events
- [definition-system.md](l1-definition-system.md) — JSON definitions loaded as assets with hot-reload

## 1. Motivation

Games reference thousands of external files. A robust asset system must:

- Load assets asynchronously to avoid frame stalls.
- Track load state so systems can react when assets become available.
- Support hot-reload for rapid iteration during development.
- Provide a stable handle abstraction that decouples users from load timing.
- Allow multiple IO backends (filesystem, embedded, network).

## 2. Constraints & Assumptions

- Asset loading is always asynchronous; there is no synchronous load API.
- Each asset type has at least one registered AssetLoader.
- Strong handles keep assets alive; when all strong handles drop, the asset is eligible for removal.
- Asset IDs come in two forms: runtime Index (generational) and persistent UUID.
- Meta files are optional; sensible defaults apply when absent.

## 3. Core Invariants

1. **Handle validity.** A strong handle always points to a valid slot (Loading, Loaded, or Failed).
2. **Reference counting.** Assets are freed only when the strong reference count reaches zero.
3. **Loader determinism.** Given the same bytes and settings, a loader produces the same asset.
4. **Event ordering.** Created fires before Modified; Removed fires only after all dependents are notified.

## 4. Detailed Design

### 4.1 AssetServer

The central coordinator for all asset operations:

```plaintext
AssetServer
  fn load(path: AssetPath) -> Handle[A]
  fn load_with_settings(path, settings) -> Handle[A]
  fn get_load_state(id: AssetID) -> LoadState
  fn reload(path: AssetPath)
  fn unload(id: AssetID)
```

Load states follow this progression:

```mermaid
stateDiagram-v2
    [*] --> NotLoaded
    NotLoaded --> Loading : load() called
    Loading --> Loaded : bytes decoded successfully
    Loading --> Failed : loader error
    Loaded --> Loading : reload triggered
    Failed --> Loading : retry / reload
    Loaded --> [*] : all handles dropped
```

### 4.2 AssetLoader Trait

```plaintext
trait AssetLoader:
    type Asset
    type Settings
    type Error
    fn load(reader: AssetReader, settings: Settings) -> Result[Asset, Error]
    fn extensions() -> []string
```

Loaders are registered at startup. When a path is loaded, the server selects a loader by file extension. If multiple loaders handle the same extension, the most recently registered one wins unless the meta file specifies otherwise.

### 4.3 Handles and Asset IDs

```plaintext
Handle[A]
├── Strong(AssetID)   -- ref-counted, keeps asset alive
└── Weak(AssetID)     -- does not prevent unload

AssetID
├── Index { index: u32, generation: u32 }   -- runtime, like Entity
└── UUID  { uuid: u128 }                    -- persistent across sessions
```

Strong handles are the default return from `load()`. A weak handle is obtained via `handle.downgrade()` and can be upgraded back if the asset still exists.

### 4.4 Assets Resource

Each asset type gets a typed resource inserted into the World:

```plaintext
Assets[A]
  fn get(id: AssetID) -> Option[&A]
  fn get_mut(id: AssetID) -> Option[&mut A]
  fn add(asset: A) -> Handle[A]           -- for programmatic creation
  fn remove(id: AssetID) -> Option[A]
  fn iter() -> Iterator[(&AssetID, &A)]
```

### 4.5 IO Abstraction

```plaintext
trait AssetReader:
    fn read(path) -> Result[bytes, IOError]
    fn read_directory(path) -> Result[[]string, IOError]
    fn is_directory(path) -> bool

trait AssetWriter:
    fn write(path, bytes) -> Result[(), IOError]
    fn delete(path) -> Result[(), IOError]
```

Built-in backends: FileSystem (default), Memory (testing), Embedded (compile-time bundled), HTTP (remote fetching).

### 4.6 Asset Path Format

```plaintext
"source://path/to/asset.ext#label"

source   -- named IO backend (default if omitted)
path     -- virtual path within that source
label    -- sub-asset identifier (e.g., a mesh within a GLTF file)
```

### 4.7 Asset Events

The system emits typed events each frame:

- **Created** — asset loaded for the first time.
- **Modified** — asset data replaced (hot-reload or manual update).
- **Removed** — asset unloaded, ID is no longer valid.
- **LoadedWithDependencies** — asset and all its transitive dependencies are loaded.

### 4.8 Hot-Reload

In development builds, a file watcher monitors asset directories. When a file changes:

1. The watcher enqueues the changed path.
2. AssetServer calls `reload(path)`, transitioning the asset back to Loading.
3. The loader re-processes the file; on success the asset slot is updated in-place.
4. A Modified event is emitted so dependent systems can react.

### 4.9 Asset Processor (Optional)

An offline pipeline that transforms source assets into optimized forms:

```plaintext
AssetProcessor
  fn process(source_path, dest_path, settings)
  fn register_processor(ProcessorDescriptor)
```

Processors run at build time or in a background tool, not at runtime. They produce processed assets plus updated meta files.

### 4.10 Virtual File System (VFS) with Mount Points

The asset system accesses files through a Virtual File System (VFS) with named mount points, providing a unified path abstraction (`vfs:///`) across all platforms:

```plaintext
VFS Structure:
  "/app/"       -> Local project directory (RW)
  "/engine/"    -> Core engine shaders and assets (RO)
  "/save/"      -> OS-specific persistent user data (RW)
  "/cache/"     -> Temporary, platform-managed cache (RW)
  "/pack/"      -> Read-only archive (.zip or proprietary pack)
```

**Key Features:**

- **Provider Abstraction**: Code refers to `vfs:///engine/shaders/pbr.glsl` regardless of whether it's an OS file, an embedded resource (`go:embed`), or a remote HTTP stream.
- **Priority Mounting**: Overlapping mount points (e.g., mounting a "Patch" pack over `/app/`) enable easy modding and hot-fixing without modifying the original installation.
- **Unified Path Syntax**: Forward slashes are used universally; the VFS provider handles OS-specific path separators and root directories.

### 4.11 Asset Compilation Pipeline

An offline pipeline transforms source assets (e.g., `.png`, `.fbx`) into optimized, platform-specific content formats (e.g., `.dds`, `.mesh`):

```plaintext
AssetCompiler (interface)
  GetInputFiles(item: AssetItem) -> []FilePath
  GetInputTypes(item: AssetItem) -> []TypeDependency
  Prepare(context: CompilerContext, item: AssetItem) -> CompilerResult
```

**Workflow:**

- **Dependency Tracking**: Compilers declare file and type dependencies for incremental builds.
- **Build Invalidation**: Changing a base material automatically triggers a re-compilation of all scenes referencing that material.
- **Parallel Execution**: Independent build steps are dispatched via the **Task System** (§4.8 in [task-system.md](l1-task-system.md)).

### 4.12 Content Manager & Reference Counting

To manage the runtime lifecycle of GPU resources, the engine separates **Assets** (metadata) from **Content** (runtime instances).

```plaintext
ContentManager
  loaded: map[ContentURL]*ResourceRecord

  Load[T](url) -> T:
    if record := loaded.get(url):
      record.ref_count++
      return record.instance

    instance = LoaderFor[T].Load(url)
    loaded.put(url, {instance, ref_count: 1})
    return instance

  Unload(url):
    if record := loaded.get(url):
      record.ref_count--
      if record.ref_count == 0:
        Destroy(record.instance)
        loaded.remove(url)
```

**Memory Management:**

- **Reference Counting**: Resources are shared; loading the same model twice just increments a counter.
- **Sub-Resource Hierarchy**: Loading a complex object (like a `Scene`) automatically manages reference counts for its dependent `Materials`, `Textures`, and `Models`.
- **Async Loading**: Non-blocking requests return a `Task` that provides the resource upon completion.

## 5. Open Questions

1. Should failed assets retry automatically, or require explicit reload?
2. How are circular asset dependencies detected and reported?
3. Should the asset database use a lightweight SQL (SQLite) or a simple JSON/Binary index?

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
| 0.1.0 | 2026-03-25 | Initial draft from architecture analysis |
| 0.2.0 | 2026-03-26 | Added VFS with mount points and asset compilation pipeline |
| 0.3.0 | 2026-03-26 | Refined VFS with mount points, added Content Manager with reference counting |
| — | — | Planned examples: `examples/asset/` |
