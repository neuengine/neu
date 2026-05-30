# Platform System

**Version:** 0.1.0
**Status:** Stable
**Layer:** concept

## Overview

The Platform System defines how the engine and games built with it run across multiple operating systems, architectures, and deployment targets. It provides a layered abstraction: a capability-based platform profile at the top, build tags and conditional compilation in the middle, and platform-specific backend implementations at the bottom. The goal is "write once, deploy everywhere" — game code and most engine code never touches platform-specific APIs directly.

## Related Specifications

- [app-framework.md](l1-app-framework.md) — Plugin system, DefaultPlugins vary per platform
- [window-system.md](l1-window-system.md) — WindowBackend per platform
- [render-core.md](l1-render-core.md) — RenderBackend per platform (OpenGL, Vulkan, WebGPU, Metal)
- [audio-system.md](l1-audio-system.md) — AudioDriver per platform
- [input-system.md](l1-input-system.md) — Input sources vary per platform (gamepad, touch, keyboard)
- [build-tooling.md](l1-build-tooling.md) — CI matrix for cross-platform builds
- [asset-system.md](l1-asset-system.md) — Asset packaging and IO backends per platform

## 1. Motivation

A game engine that only runs on one OS is not a serious engine. Games must ship to desktop (Windows, macOS, Linux), web (WASM), and mobile (Android, iOS) from a single codebase. Without a dedicated platform abstraction:

- Platform-specific code leaks into gameplay logic, creating maintenance nightmares.
- Each new platform requires touching dozens of files across unrelated subsystems.
- Feature availability (e.g., "does this device have a GPU?", "is touch input available?") is checked ad-hoc with `runtime.GOOS` scattered through the codebase.
- Build configuration for cross-compilation is undocumented and fragile.
- Games built on the engine inherit the engine's platform limitations — if the engine only runs on Linux, every game is Linux-only.

The Platform System centralizes all platform concerns into one specification, ensuring the engine and its games are portable by design.

## 2. Constraints & Assumptions

- Go's build tag system (`//go:build`) is the primary mechanism for conditional compilation. No preprocessor macros or code generation.
- The engine core (ECS, scheduling, events, math) has **zero platform-specific code**. Platform differences exist only in backend implementations (windowing, rendering, audio, IO).
- Each target platform maps to a well-defined set of build tags. Cross-compilation uses `GOOS` and `GOARCH` environment variables.
- Mobile platforms (Android, iOS) require CGo for native windowing and graphics context creation. This is the only permitted use of CGo in the engine.
- WebAssembly (WASM) target uses `GOOS=js GOARCH=wasm` with `syscall/js` for browser integration.
- Console platforms (PlayStation, Xbox, Nintendo Switch) are out of scope for the initial release but the architecture must not preclude them.

## 3. Core Invariants

- **INV-1**: Game code and engine core logic must compile and pass tests on all Tier 1 platforms without modification. Platform-specific code lives exclusively in backend packages.
- **INV-2**: The `PlatformProfile` resource is available from the first frame. Systems can query platform capabilities without conditional compilation.
- **INV-3**: `DefaultPlugins` automatically selects the correct backend implementations for the current platform. No manual platform wiring in game code.
- **INV-4**: A headless build (no window, no GPU, no audio) must be possible on every platform for testing, CI, and dedicated server deployments.
- **INV-5**: Adding support for a new platform requires only implementing backend interfaces and adding build tags — no changes to engine core or existing game code.

## 4. Detailed Design

### 4.1 Platform Tiers

Platforms are organized into support tiers:

| Tier | Platforms | Commitment |
| :--- | :--- | :--- |
| Tier 1 | Windows (amd64), Linux (amd64), macOS (amd64, arm64) | Full support, CI-tested every commit, release binaries provided |
| Tier 2 | Web/WASM, Android (arm64), iOS (arm64) | Supported, CI-tested on release branches, community-assisted |
| Tier 3 | Linux (arm64), FreeBSD, consoles | Best-effort, architecture does not preclude, no CI guarantee |

Tier 1 platforms are the primary development targets. All engine features must work on Tier 1 before a release is tagged.

### 4.2 Platform Profile

A runtime resource describing the current platform's capabilities:

```plaintext
PlatformProfile (Resource)
  OS:             PlatformOS       // Windows, Linux, MacOS, Android, iOS, Web
  Arch:           PlatformArch     // AMD64, ARM64, WASM
  Tier:           PlatformTier     // Tier1, Tier2, Tier3
  Capabilities:   PlatformCaps     // bitfield of available features

PlatformCaps (bitfield):
  HasGPU            // hardware-accelerated rendering available
  HasTouch          // touch input available
  HasGamepad        // gamepad input available
  HasKeyboard       // physical keyboard available
  HasMouse          // mouse/trackpad available
  HasFileSystem     // local filesystem access (false for sandboxed web)
  HasMultiWindow    // multiple OS windows supported
  HasClipboard      // system clipboard access
  HasVibration      // haptic feedback available
  HasSpatialAudio   // 3D audio positioning available
```

The profile is populated during `PreStartup` by the platform plugin and never changes during execution. Systems use it for runtime feature negotiation:

```plaintext
fn configure_input(profile: Res[PlatformProfile]) {
    if profile.Capabilities.Has(HasTouch) {
        // enable touch controls
    }
    if profile.Capabilities.Has(HasGamepad) {
        // enable gamepad controls
    }
}
```

### 4.3 Build Tag Architecture

Platform-specific code uses Go build tags organized in layers:

```plaintext
Build tag hierarchy:
  //go:build windows          // OS-level
  //go:build linux
  //go:build darwin
  //go:build js && wasm       // Web/WASM
  //go:build android
  //go:build ios

  //go:build !headless        // Feature-level
  //go:build editor           // Editor-only code
  //go:build cgo              // Native windowing (mobile)

Package structure:
  internal/platform/
    platform.go               // PlatformProfile, PlatformCaps (shared)
    platform_windows.go       // //go:build windows
    platform_linux.go         // //go:build linux
    platform_darwin.go        // //go:build darwin
    platform_web.go           // //go:build js && wasm
    platform_android.go       // //go:build android
    platform_ios.go           // //go:build ios
    platform_headless.go      // //go:build headless
```

Each platform file implements a `NewPlatformProfile() PlatformProfile` function that returns the correct capability set.

### 4.4 Backend Selection

`DefaultPlugins` varies per platform. The platform plugin selects backends automatically:

| Subsystem | Windows | Linux | macOS | Web/WASM | Android | iOS |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| Window | Win32 | X11/Wayland | Cocoa | Canvas | NativeActivity | UIKit |
| Render | Vulkan/D3D12 | Vulkan/OpenGL | Metal/OpenGL | WebGPU/WebGL2 | OpenGL ES/Vulkan | Metal |
| Audio | WASAPI | ALSA/PulseAudio | CoreAudio | WebAudio | AAudio/OpenSL | AVAudioEngine |
| Input | Win32 + XInput | evdev + SDL | IOKit | DOM Events | Android Input | UIKit Touch |
| FileIO | os.File | os.File | os.File | IndexedDB/Fetch | AssetManager | NSBundle |

Each row is an interface implementation. The platform plugin wires the correct implementations during `LEVEL_SERVERS` initialization (see app-framework.md §4.10).

### 4.5 Cross-Compilation Pipeline

Building for a target platform from any host:

```plaintext
Desktop (native):
  GOOS=windows GOARCH=amd64 go build -o game.exe ./cmd/game/
  GOOS=linux   GOARCH=amd64 go build -o game     ./cmd/game/
  GOOS=darwin  GOARCH=arm64 go build -o game      ./cmd/game/

Web/WASM:
  GOOS=js GOARCH=wasm go build -o game.wasm ./cmd/game/
  // Serve with wasm_exec.js + HTML wrapper

Mobile (requires CGo + NDK/Xcode):
  // Android: gomobile bind or custom NDK toolchain
  // iOS: gomobile bind or Xcode project with Go archive

Headless (any platform):
  go build -tags headless -o server ./cmd/server/
```

The `headless` build tag excludes all windowing, rendering, and audio code, producing a minimal binary suitable for dedicated game servers and CI testing.

### 4.6 Platform Plugin Architecture

Each platform provides a plugin that implements all platform-specific wiring:

```plaintext
PlatformPlugin interface:
  Build(app *App)
    // 1. Insert PlatformProfile resource
    // 2. Register WindowBackend implementation
    // 3. Register RenderBackend implementation
    // 4. Register AudioDriver implementation
    // 5. Register FileIO backend
    // 6. Register platform-specific input systems
```

`DefaultPlugins` includes the correct `PlatformPlugin` automatically based on build tags. A game's `main.go` never references platform-specific types:

```plaintext
// cmd/game/main.go — identical on all platforms
func main() {
    app := NewApp()
    app.AddPlugins(DefaultPlugins)
    app.AddPlugins(MyGamePlugin)
    app.Run()
}
```

### 4.7 Feature Negotiation

When a game requires a feature that may not be available on all platforms, it uses graceful degradation:

```plaintext
Strategy 1 — Runtime check:
  if profile.Capabilities.Has(HasSpatialAudio) {
      // use 3D audio
  } else {
      // fall back to stereo panning
  }

Strategy 2 — Plugin availability:
  // TouchInputPlugin is only registered on platforms with HasTouch
  // Systems that depend on touch simply don't run if the plugin is absent

Strategy 3 — Asset variants:
  // assets/textures/hero.png          (default, high-res)
  // assets/textures/hero.mobile.png   (low-res, mobile variant)
  // Asset system selects variant based on PlatformProfile
```

### 4.8 Mobile-Specific Concerns

Mobile platforms introduce unique constraints:

- **App lifecycle**: Android and iOS have background/foreground transitions, memory pressure events, and app suspension. The platform plugin translates these into engine events (`AppSuspended`, `AppResumed`, `LowMemory`).
- **Screen orientation**: The `Window` component on mobile includes `Orientation` (Portrait, Landscape, Auto). Changes trigger `WindowResized` events.
- **Touch as primary input**: On mobile, `HasMouse` is false and `HasTouch` is true. The pointer abstraction (see input-system.md §4.4) handles this transparently.
- **GPU constraints**: Mobile GPUs have lower fill rates, less VRAM, and different texture compression formats (ETC2, ASTC). The render backend adapts quality settings based on platform profile.
- **Battery awareness**: A `PowerState` resource reports battery level and charging status. Games can reduce frame rate or visual quality to conserve battery.

### 4.9 Web/WASM-Specific Concerns

The WASM target has unique constraints:

- **No filesystem**: `HasFileSystem` is false. Assets are loaded via HTTP fetch or embedded in the WASM binary. The `AssetReader` backend uses `fetch()` via `syscall/js`.
- **No threads**: Go's WASM target runs on a single OS thread. The parallel executor falls back to sequential mode. Web Workers are a future optimization.
- **Canvas integration**: The `Window` component's `Canvas` field specifies which HTML element to render into. The web platform plugin creates or attaches to the specified element.
- **Browser events**: Input events come from DOM event listeners, not OS-level polling. The platform plugin bridges DOM events to engine input events.
- **Audio autoplay restrictions**: Browsers require user interaction before playing audio. The audio system defers playback until the first user gesture event.

### 4.10 Asset Packaging Per Platform

Different platforms require different asset formats and packaging:

```plaintext
AssetPackager
  platform:   PlatformProfile
  rules:      []PackagingRule

PackagingRule
  source:      glob pattern         // e.g., "assets/textures/*.png"
  transform:   AssetTransform       // e.g., CompressTexture(ASTC) for mobile
  output:      string               // e.g., "assets/textures/{name}.astc"

Default rules per platform:
  Desktop:  PNG/JPEG textures, OGG audio, no compression
  Mobile:   ASTC/ETC2 textures, compressed audio, texture atlas packing
  Web:      WebP textures, compressed audio, maximum bundle splitting
```

The asset packager is a build-time tool (not runtime). It processes assets into platform-optimized formats before distribution.

## 5. Open Questions

- Should the engine provide a unified build command (`ci build --platform=web`) or rely on standard `GOOS`/`GOARCH` with documentation?
- How deep should mobile support go in the initial release — basic rendering only, or full lifecycle management?
- Should console platform support be structured as a separate closed-source plugin package to handle NDA constraints?
- Should WASM+Go target eventually be replaced with TinyGo for smaller binary sizes, or is standard Go sufficient?
- How should platform-specific shader variants be managed — preprocessor in shader files or separate files per backend?

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
| 0.1.0 | 2026-03-26 | Initial draft — cross-platform architecture for engine and games |
| — | — | Planned examples: `examples/window/` |
