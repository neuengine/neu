# Asset System — Go Implementation

**Version:** 0.1.0
**Status:** Stable
**Layer:** go
**Implements:** [l1-asset-system.md](l1-asset-system.md)

## Overview

Go-level design for asynchronous asset management: generic ref-counted
`Handle[A]`, typed `Assets[A]` ECS resources, an extension-keyed `AssetLoader`
registry, a `vfs:///` virtual file system over pluggable `fs.FS` providers
(filesystem / `go:embed` / memory / HTTP), a stdlib-only dev-mode polling hot
reloader, and a sub-resource ref-counting `ContentManager`. Async loading is
delegated to the task system's IOPool.

## Related Specifications

- [l1-asset-system.md](l1-asset-system.md) — L1 concept specification (parent)
- [l2-task-system-go.md](l2-task-system-go.md) — async loads run on the IOPool, return `TaskHandle`
- [l2-event-system-go.md](l2-event-system-go.md) — asset lifecycle events (Created/Modified/Removed)
- [l2-type-registry-go.md](l2-type-registry-go.md) — typed `Assets[A]` resource registration

## 1. Motivation

Games reference thousands of files. The Go implementation must give systems a
stable handle the moment `Load` is called (before bytes arrive), drive decoding
off-thread via the IOPool, surface load-state transitions as events, and keep
the whole thing dependency-free so the engine core honors C-003.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: generics for `Handle[A]`, `Assets[A]`, `AssetLoader`.
- **Async only**: no synchronous `Load` API; `Load` returns immediately with a
  `Loading` handle (L1 §2).
- **C-003 — stdlib only**: VFS providers use `io/fs`, embedding uses
  `embed.FS`, HTTP uses `net/http`. **No `fsnotify`.** The dev hot-reload
  watcher is a `time`-based mtime poller behind `//go:build dev`, *not* an OS
  inotify binding — this is the deliberate cost of zero-dependency (§4.8).
- **C-027**: decode scratch buffers are pooled via `sync.Pool`.
- **No `Option[T]` / `Result[T,E]`**: `Assets.Get` → `(*A, bool)`; loader
  returns `(A, error)`.
- **`AssetID` is a tagged value**: a struct with a `kind` discriminant rather
  than a Go interface, to keep it comparable and map-keyable.

## 3. Core Invariants

> [!NOTE]
> See [l1-asset-system.md §3](l1-asset-system.md) for technology-agnostic
> invariants. Go-specific compliance is tabulated in §4.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: Handle validity | `Load` allocates the slot in state `Loading` and returns a strong `Handle[A]` *synchronously*; the slot always resolves to `Loading`→`Loaded`/`Failed`. Slot is never freed while a strong handle exists. |
| **INV-2**: Reference counting | Strong `Handle[A]` holds an `*int64` refcount mutated with `sync/atomic`. `Clone` increments; finalizer/`Drop` decrements; slot freed only at zero (`ContentManager.Unload`). |
| **INV-3**: Loader determinism | `AssetLoader.Load(io.Reader, Settings) (A, error)` takes only the byte stream + settings — no ambient state — so identical input ⇒ identical asset. Enforced by a golden round-trip test. |
| **INV-4**: Event ordering | `flushAssetEvents` runs once per frame and emits in fixed order per asset: `Created` → `Modified` → (`Removed` only after dependents notified via the dependency graph walk). |

## Go Package

```
pkg/asset/
  server.go     // AssetServer, LoadState
  handle.go     // Handle[A], AssetID (Index|UUID tagged)
  loader.go     // AssetLoader[A,S], registry
  assets.go     // Assets[A] typed resource
  vfs.go        // VFS, mount table, fs.FS providers
  watch_dev.go  //go:build dev — mtime polling watcher
  content.go    // ContentManager (sub-resource refcount)
```

## Type Definitions

```go
type LoadState uint8
const (
    NotLoaded LoadState = iota
    Loading
    Loaded
    Failed
)

type AssetID struct {
    kind uint8  // 0 = index, 1 = uuid
    idx, gen uint32
    uuid [16]byte
}

type Handle[A any] struct {
    id   AssetID
    rc   *atomic.Int64 // nil ⇒ weak handle (no lifetime hold)
}

// AssetLoader is generic over asset type A and its settings S.
type AssetLoader[A any, S any] interface {
    Load(r io.Reader, settings S) (A, error)
    Extensions() []string
}

type Assets[A any] struct {
    slots map[AssetID]*assetSlot[A]
}

type VFS struct {
    mounts []mount // ordered; later mounts shadow earlier (priority)
}
type mount struct {
    prefix string  // "/engine/", "/app/", ...
    fsys   fs.FS   // os.DirFS | embed.FS | memFS | httpFS
    rw     bool
}

type ContentManager struct {
    loaded map[string]*resourceRecord // ContentURL → {instance, refCount}
    mu     sync.Mutex
}
```

## Key Methods

```
func NewAssetServer(vfs *VFS, io *task.IOPool) *AssetServer

func Load[A any](s *AssetServer, path string) Handle[A]              // INV-1: sync handle
func LoadWithSettings[A, S any](s *AssetServer, path string, set S) Handle[A]
func (s *AssetServer) GetLoadState(id AssetID) LoadState
func (s *AssetServer) Reload(path string)
func (s *AssetServer) Unload(id AssetID)

func RegisterLoader[A, S any](s *AssetServer, l AssetLoader[A, S])

func (a *Assets[A]) Get(id AssetID) (*A, bool)
func (a *Assets[A]) Add(asset A) Handle[A]                            // programmatic
func (a *Assets[A]) Remove(id AssetID) (A, bool)
func (a *Assets[A]) Iter() iter.Seq2[AssetID, *A]

func (h Handle[A]) Clone() Handle[A]      // INV-2: rc++
func (h Handle[A]) Downgrade() Handle[A]  // strong → weak
func (h Handle[A]) Upgrade() (Handle[A], bool)

func (v *VFS) Mount(prefix string, fsys fs.FS, rw bool)
func (v *VFS) Open(vpath string) (fs.File, error) // resolves vfs:///
```

`AssetPath` parsing handles the L1 `source://path#label` form: split scheme →
mount prefix, fragment → sub-asset label passed to the loader.

## Performance Strategy

- **Sync handle, async bytes**: `Load` is O(1) (slot alloc + IOPool submit);
  decoding never blocks the calling system.
- **`sync.Pool` decode buffers** (C-027): loaders borrow scratch `[]byte`.
- **Atomic refcount** (INV-2): no mutex on the handle hot path; only
  `ContentManager`'s map mutates under a `sync.Mutex` (cold path).
- **VFS priority shadowing**: mount lookup is a short ordered slice scan, not a
  per-open syscall probe.

## Error Handling

| Condition | Behavior |
| :--- | :--- |
| No loader for extension | `Load` returns a handle that resolves to `Failed`; `AssetEvent{Failed, reason}` emitted |
| Loader returns error | Slot → `Failed`; bytes retained for inspection; no panic |
| `vfs:///` unknown mount | `VFS.Open` returns `fs.ErrNotExist` wrapped with the unresolved prefix |
| Write to a read-only mount | `ErrReadOnlyMount` |
| Hot-reload of a `Failed` asset | Allowed — transitions `Failed`→`Loading` (retry path) |

```go
var (
    ErrReadOnlyMount = errors.New("asset: mount is read-only")
    ErrNoLoader      = errors.New("asset: no loader registered for extension")
)
```

## Testing Strategy

- **Handle lifetime (C-005)**: concurrent `Clone`/drop under `-race`; assert
  slot freed exactly at refcount zero, never before.
- **Loader determinism**: golden test — same bytes ⇒ byte-identical asset.
- **VFS shadowing**: mount a patch FS over `/app/`, assert priority resolution.
- **Event ordering**: assert `Created` precedes `Modified`; `Removed` only
  after dependent notification.
- **Dev watcher**: table-driven mtime-change simulation (no real filesystem
  race) verifying `Reload` is invoked once per change.

## 7. Drawbacks & Alternatives

- **Drawback**: stdlib mtime polling is coarser and busier than inotify.
  **Alternative**: `github.com/fsnotify/fsnotify`.
  **Decision**: C-003 forbids the dependency in engine core; the poller is
  dev-build-only (`//go:build dev`) and never ships in release binaries, so
  the cost is bounded to iteration-time. Revisit only if a stdlib OS-watch API
  lands.
- **Drawback**: `AssetID` as a tagged struct wastes 16 bytes for index-form IDs.
  **Alternative**: `interface{ assetID() }`.
  **Decision**: keep the struct — it stays `comparable` (map key) and
  allocation-free; 16 bytes is acceptable vs. interface boxing on the hot path.
- **Drawback**: asset DB index format undecided.
  **Decision**: L1 Open Question Q3 —
  <!-- TBD (L1 §5 Q3): JSON/binary index vs. SQLite. Lean JSON+binary
       (C-003: SQLite is cgo/external). Defer to processor implementation. -->

## Canonical References

<!-- MANDATORY for Stable. Stub — populate at implementation (P3). Stable
     blocked: (1) L1 parent Draft; (2) C29 — no validating examples/ yet. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-17 | Initial L2 draft — Go translation of l1-asset-system v0.3.0. Generic Handle[A]/Assets[A], fs.FS-based VFS, stdlib-only `//go:build dev` mtime watcher (C-003 trade-off documented), atomic refcount ContentManager. L1 Q3 carried as TBD. |
