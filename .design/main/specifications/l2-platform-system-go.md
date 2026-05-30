# Platform System — Go Implementation

**Version:** 0.1.0
**Status:** Stable
**Layer:** go
**Implements:** [l1-platform-system.md](l1-platform-system.md)

## Overview

Go-level design for the cross-platform abstraction. A `PlatformProfile` resource —
populated once during `PreStartup` from a build-tag-selected `NewPlatformProfile()` —
describes the current OS, architecture, support tier, and a `PlatformCaps` capability
bitfield. Platform-specific code lives exclusively behind `//go:build` tags in
`internal/platform`; the engine core and game code never reference an OS API. A
`PlatformPlugin` (chosen automatically by `DefaultPlugins`) wires the correct window,
render, audio, input, and file-IO backends for the target. A `headless` build tag
yields a window-less, GPU-less, audio-less binary on every platform for CI and servers.

## Related Specifications

- [l1-platform-system.md](l1-platform-system.md) — L1 concept specification (parent)
- [l2-app-framework-go.md](l2-app-framework-go.md) — `App`, `Plugin`, `DefaultPlugins`, startup levels that wire backends
- [l2-window-system-go.md](l2-window-system-go.md) — `WindowBackend` selected per platform
- [l2-render-core-go.md](l2-render-core-go.md) — `RenderBackend` selected per platform
- [l2-audio-system-go.md](l2-audio-system-go.md) — `AudioDriver`/`AudioBackend` selected per platform
- [l2-asset-system-go.md](l2-asset-system-go.md) — file-IO backend (`os.File` vs fetch/IndexedDB) per platform

## 1. Motivation

`runtime.GOOS` checks scattered across subsystems are the failure mode this spec
prevents. Centralizing platform detection into one resource and one plugin means a
capability query (`HasTouch`, `HasGPU`) is a branchless bitfield test available from
the first frame, and adding a platform is "implement the backend interfaces + add a
build tag" with zero edits to engine core (INV-5). The headless tag is the same
mechanism the render and audio subsystems already use for CI (nopBackend / headless
driver), unified here.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: `//go:build` constraints, `runtime.GOOS`/`GOARCH` for the shared fallback path.
- **C-003**: core is dep-free; the only permitted CGo is native mobile windowing, gated by `//go:build cgo && (android || ios)`.
- The engine core (ECS, scheduling, events, math) contains **zero** build-tagged platform files (INV-1).
- `PlatformProfile` is immutable after `PreStartup` — populated once, read everywhere (INV-2).
- Exactly one `PlatformPlugin` is linked per build; build tags guarantee mutual exclusivity (no two `platform_*.go` compile together).
- WASM (`js && wasm`) runs single-threaded: the platform profile reports no multi-thread capability and the task pool falls back to sequential.
- A `headless` build excludes window/render/audio backends entirely (INV-4).

## 3. Core Invariants

> [!NOTE]
> See [l1-platform-system.md §3](l1-platform-system.md) for the technology-agnostic
> invariants (INV-1…INV-5). Go-specific compliance is tabulated in §4.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: core compiles/tests on all Tier 1 platforms unmodified; platform code is isolated | Platform-specific files live only in `internal/platform/platform_{os}.go` behind `//go:build {os}`. `pkg/platform` (the public profile types) has no build tags and compiles everywhere. |
| **INV-2**: `PlatformProfile` available from frame 1, query without conditional compilation | `PlatformPlugin.Build` inserts the `PlatformProfile` resource during `PreStartup`; systems read it via `Res[PlatformProfile]`. `Capabilities.Has(cap)` is a runtime bitfield test — no `//go:build` in game code. |
| **INV-3**: `DefaultPlugins` selects correct backends automatically | A build-tag-selected `platformPlugin()` constructor returns the OS-appropriate `PlatformPlugin`; `DefaultPlugins` calls it. The chosen plugin registers window/render/audio/input/file-IO backends. No manual wiring in `main.go`. |
| **INV-4**: headless build (no window/GPU/audio) possible on every platform | `//go:build headless` selects `platform_headless.go`, whose plugin registers the existing headless backends (render `nopBackend`, audio `HeadlessBackend`/`HeadlessDriver`, window headless backend). `HasGPU`/`HasMultiWindow`/`HasSpatialAudio` are all false. |
| **INV-5**: new platform = implement interfaces + add build tags, no core changes | A new platform adds `platform_{os}.go` with `NewPlatformProfile()` + a `PlatformPlugin` that supplies backend implementations satisfying the existing window/render/audio/input interfaces. Engine core and existing games are untouched. |

## Go Package

```
pkg/platform/
  profile.go          // PlatformProfile, PlatformOS/Arch/Tier enums (shared, no build tags)
  caps.go             // PlatformCaps bitfield + Has/With; String()
  plugin.go           // PlatformPlugin interface (Build(*app.App))
internal/platform/
  detect.go           // shared GOOS/GOARCH → OS/Arch mapping (no tags)
  platform_windows.go // //go:build windows   — NewPlatformProfile + windowsPlugin
  platform_linux.go   // //go:build linux
  platform_darwin.go  // //go:build darwin
  platform_web.go     // //go:build js && wasm
  platform_android.go // //go:build android && cgo
  platform_ios.go     // //go:build ios && cgo
  platform_headless.go// //go:build headless — overrides all of the above
```

## Type Definitions

```go
type PlatformOS uint8
const (OSWindows PlatformOS = iota; OSLinux; OSMacOS; OSAndroid; OSIOS; OSWeb)

type PlatformArch uint8
const (ArchAMD64 PlatformArch = iota; ArchARM64; ArchWASM)

type PlatformTier uint8
const (Tier1 PlatformTier = iota + 1; Tier2; Tier3)

type PlatformCaps uint32
const (
    HasGPU PlatformCaps = 1 << iota
    HasTouch
    HasGamepad
    HasKeyboard
    HasMouse
    HasFileSystem
    HasMultiWindow
    HasClipboard
    HasVibration
    HasSpatialAudio
)

func (c PlatformCaps) Has(f PlatformCaps) bool { return c&f == f }
func (c PlatformCaps) With(f PlatformCaps) PlatformCaps { return c | f }

// PlatformProfile is an immutable ECS resource inserted during PreStartup.
type PlatformProfile struct {
    OS           PlatformOS
    Arch         PlatformArch
    Tier         PlatformTier
    Capabilities PlatformCaps
}

// PlatformPlugin wires all platform-specific backends.
type PlatformPlugin interface {
    app.Plugin // Build(*app.App)
}
```

## Key Methods

```go
// Build-tag-selected; exactly one compiles into the binary.
func NewPlatformProfile() PlatformProfile

// DefaultPlugins calls this to obtain the current platform's plugin.
func CurrentPlatformPlugin() PlatformPlugin

// Shared detection fallback (used by desktop profiles).
func detectOS() PlatformOS     // from runtime.GOOS
func detectArch() PlatformArch // from runtime.GOARCH
```

## Performance Strategy

- **Branchless capability checks**: `Has` is a single mask-and-compare; capability gates in hot systems cost one AND + one CMP.
- **Zero runtime platform branching**: backend selection is resolved at link time by build tags, not by per-frame `switch runtime.GOOS`.
- **Immutable profile, no locks**: the resource is written once in `PreStartup` and read-only thereafter — no synchronization on reads.
- **Headless excludes code**: the `headless` tag removes backend packages from the binary, shrinking server/CI builds.

## Error Handling

| Condition | Behavior |
| :--- | :--- |
| Two `platform_*.go` would compile together | Impossible by construction — build tags are mutually exclusive; CI builds each Tier 1 target to prove it. |
| Backend interface unimplemented for a platform | Compile error (missing method) — caught at build, not runtime. |
| Capability assumed but absent (e.g., spatial audio on web) | Caller uses feature negotiation (`Has`) and degrades gracefully (L1 §4.7); no error raised. |
| Headless build references a window/render backend | Compile error — those packages are excluded under `//go:build headless`. |

## Testing Strategy

- **Caps bitfield**: table test for `Has`/`With` across single and combined flags; assert disjoint flags do not alias.
- **Profile immutability**: a system mutating `PlatformProfile` is rejected by design (resource inserted, never re-inserted); test asserts the value is stable across frames.
- **Headless profile**: building with `-tags headless` yields a profile with `HasGPU==false`, `HasMultiWindow==false`, `HasSpatialAudio==false`; the registered render/audio backends are the headless stubs.
- **Detection mapping**: `detectOS`/`detectArch` map representative `GOOS`/`GOARCH` values to the correct enums.
- **Cross-compile smoke** (CI): `GOOS=windows/linux/darwin GOARCH=amd64/arm64 go build` + `GOOS=js GOARCH=wasm go build` all succeed for the engine core.

## 7. Drawbacks & Alternatives

- **Drawback**: build-tag isolation multiplies the per-OS file count.
  **Alternative**: a runtime `switch runtime.GOOS` in one file.
  **Decision**: L1 INV-1/INV-5 + C-003 require compile-time isolation so non-target platform code (and its deps) is never linked. Build tags are the idiomatic Go mechanism. Kept.
- **Drawback**: a `PlatformCaps` bitfield caps at 32 flags.
  **Alternative**: a `map[Capability]bool`.
  **Decision**: a bitfield is branchless, allocation-free, and 32 capabilities is ample; widen to `uint64` if exhausted. Kept.

## Canonical References

<!-- All paths verified on disk; 100% cov in both default and -tags headless
     builds; headless build verified (INV-4). C29 P6 gate closed by T-6T06. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |
| [CAPS] | `pkg/platform/caps.go` | `PlatformCaps` branchless bitfield + `Has`/`With`/`Without` + `String`. |
| [PROFILE] | `pkg/platform/profile.go` | `PlatformProfile`, `PlatformOS`/`Arch`/`Tier` total-switch enums. |
| [PLUGIN] | `pkg/platform/plugin.go` | `PlatformPlugin` interface (embeds `appface.Plugin`). |
| [DETECT] | `internal/platform/detect.go` | Pure `osFromGOOS`/`archFromGOARCH`/`tierFor` mappings (no build tags). |
| [DEFAULT] | `internal/platform/profile_default.go` | `//go:build !headless` profile + per-OS `defaultCaps`. |
| [HEADLESS] | `internal/platform/profile_headless.go` | `//go:build headless` profile excluding GPU/MultiWindow/SpatialAudio (INV-4). |
| [PLUGINIMPL] | `internal/platform/plugin.go` | Profile-inserting `Plugin` (INV-2/INV-3). |
| [TEST] | `pkg/platform/platform_test.go`, `internal/platform/*_test.go` | Caps bitfield, OS/Arch/Tier mapping, headless caps, plugin insertion (100% both builds). |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-28 | Initial L2 draft — Go translation of l1-platform-system v0.1.0. Immutable `PlatformProfile` resource, `PlatformCaps` bitfield with branchless `Has`, `//go:build`-isolated per-OS files, `headless` tag reusing existing nop/headless backends, `PlatformPlugin` wired by `DefaultPlugins`, CGo gated to mobile only. Authored ahead of Phase 6 (`/magic.spec`). Draft — L1 parent Draft + no implementation yet. |
