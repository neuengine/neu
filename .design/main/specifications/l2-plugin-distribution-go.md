# Plugin Distribution — Go Implementation

**Version:** 0.1.0
**Status:** Draft
**Layer:** go
**Implements:** [l1-plugin-distribution.md](l1-plugin-distribution.md)

## Overview

Go-level design for the third-party plugin ecosystem: the public `pkg/plugin/`
SDK (stable, `internal/`-free), the `plugin.toml` manifest parser, the
capability model + runtime-enforcing proxy, and the loading pipeline for both
in-process (compile-time Go module) and out-of-process (subprocess over the
`pkg/protocol` transport) plugins. The Go `plugin` stdlib package is deliberately
**not** used (§L1 2). Capability enforcement is trust-but-verify: in-process
plugins are mediated at the engine-API boundary; out-of-process plugins are
isolated by the OS process boundary and capability-gated on every message.

## Related Specifications

- [l1-plugin-distribution.md](l1-plugin-distribution.md) — L1 concept specification (parent)
- [l2-app-framework-go.md](l2-app-framework-go.md) — re-exported `Plugin`/`PluginGroup` lifecycle (Build/Ready/Finish/Cleanup)
- [l2-multi-repo-architecture-go.md](l2-multi-repo-architecture-go.md) — `pkg/editor`/`pkg/protocol` boundary the SDK respects; OOP transport reuses `pkg/protocol`
- [l2-command-system-go.md](l2-command-system-go.md) — all plugin world mutations flow through `CommandBuffer`, tagged with plugin ID (INV-7)
- [l2-error-core-go.md](l2-error-core-go.md) — load/runtime errors map to `E-PLUGIN-{NNN}` via `pkg/errs`
- [l2-ai-assistant-system-go.md](l2-ai-assistant-system-go.md) — the universal plugin transport is the assistant agent protocol generalised

## 1. Motivation

The L1 establishes that first-party `Plugin` linkage is insufficient for an open
ecosystem. The Go binding makes the boundary mechanical: a third-party author
imports only `pkg/plugin`, declares capabilities in `plugin.toml`, and the engine
enforces those capabilities at runtime through a proxy — without the Go `plugin`
`.so` mechanism (brittle, platform-limited, cgo-hostile). Delivery mode is a
manifest field, not a code difference (INV-4).

## 2. Constraints & Assumptions

- **Go 1.26.3+**: `encoding/json`, `os/exec` (OOP spawn), `golang.org/x/mod/semver` candidate for SemVer ranges (ADR — or a small stdlib parser); TOML via a single justified dep or a minimal hand-rolled subset (ADR).
- **C-003**: the manifest is TOML (L1 §4.2); a TOML dep requires an ADR, else a minimal stdlib-only parser for the documented subset. SemVer-range matching is hand-rolled over stdlib if no dep is approved.
- `pkg/plugin/` imports no `internal/` package (enforced by the multi-repo guard tests).
- OOP transport reuses `pkg/protocol` (newline-delimited JSON) — no second wire format.
- Capability checks are enforced at the engine-API boundary, not via Go code sandboxing (L1 §4.12).

## 3. Core Invariants

> [!NOTE]
> See [l1-plugin-distribution.md §3](l1-plugin-distribution.md) for INV-1…INV-9.
> Go-specific compliance is tabulated in §4.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: every plugin has a valid `plugin.toml`; invalid ⇒ rejected | `Manifest.Parse([]byte) (Manifest, error)` validates schema; a load with no/invalid manifest returns `ErrManifestInvalid` before any plugin code runs. |
| **INV-2**: `engine_version` SemVer constraint enforced pre-exec | `Constraint.Matches(engineVer)` (Cargo/npm `^`/`~`/range syntax); mismatch ⇒ `ErrEngineIncompatible` returned during compatibility resolution, before `New()`/spawn. |
| **INV-3**: capabilities granted only after approval; runtime checks reject + log | `CapabilitySet` (set of `Capability` strings); the `capabilityProxy` wraps `PluginContext` and returns `ErrCapabilityDenied` (logged with plugin ID + capability + call site) for any ungranted engine-API call. |
| **INV-4**: in/out-of-process share one lifecycle + capability model | Both paths drive the same `Plugin` interface (Build/Ready/Finish/Cleanup); OOP uses a host-side `proxyPlugin` translating lifecycle calls over `pkg/protocol`. The same `CapabilitySet` gates both. |
| **INV-5**: plugin IDs unique per engine instance | `PluginManager.registry map[PluginID]*PluginRecord`; a second `Register`/load of an existing ID ⇒ `ErrDuplicateID`. |
| **INV-6**: SDK follows engine SemVer | `pkg/plugin` is in the multi-repo `pkg/` tier; the API-diff gate (Track J) flags breaking changes ⇒ major bump. |
| **INV-7**: world mutations go through Commands, tagged with plugin ID | `PluginContext.Commands()` returns a `taggedCommands` wrapper that stamps every command with the plugin ID; there is no `*World` accessor. OOP plugins issue commands over the protocol; the host translates them to tagged `Command`s. |
| **INV-8**: OOP failure never crashes the host | The subprocess runs under a supervisor goroutine; crash/timeout/disconnect marks the record `Failed`, cancels in-flight requests, and returns; dependent systems use the service-registry optional-access pattern (graceful degrade). |
| **INV-9**: installed manifest immutable per version | `checksum_sha256` recorded at install; on next load a mismatch demotes the plugin to `Discovered` and re-prompts (no silent capability escalation). |

## Go Packages

```
pkg/plugin/                  // public SDK — interfaces + data types only (no internal/)
  plugin.go                  // Plugin, PluginGroup (re-exported from app public surface)
  manifest.go                // Manifest, Parse, Validate, Mode, EntryInProcess/OutOfProcess
  capability.go              // Capability consts, CapabilitySet, default-tier table
  constraint.go              // SemVer Constraint parse + Matches (^/~/range)
  context.go                 // PluginContext (capability-checked App handle; no *World)
  command.go                 // CommandIssuer (in-proc + OOP), plugin-ID tagging
  event.go query.go log.go   // capability-scoped event/query/log surfaces
  errors.go                  // E-PLUGIN-{NNN} helpers
internal/plugin/
  loader/loader.go           // discovery (4 sources), compatibility resolution, lifecycle
  loader/proxy.go            // capabilityProxy wrapping PluginContext (INV-3)
  oop/supervisor.go          // subprocess spawn, transport handshake, failure isolation (INV-8)
  oop/proxyplugin.go         // host-side Plugin translating lifecycle/commands over pkg/protocol
  manager.go                 // PluginManager resource: registry, states, audit
```

## Type Definitions

```go
type PluginID string // reverse-DNS, e.g. "com.example.aiapi"

type Mode uint8
const (ModeInProcess Mode = iota; ModeOutOfProcess)

type Manifest struct {
    ID, Version, Name, Description string
    Authors  []string
    License  string
    Mode     Mode
    Compatibility struct {
        EngineVersion string   // SemVer range
        Platforms     []string
    }
    RequiredCaps, OptionalCaps []Capability
    Entry    EntrySpec
    ChecksumSHA256 string
}

type Capability string // "world.commands", "network.outbound", ...
type CapabilitySet map[Capability]struct{}
func (s CapabilitySet) Has(c Capability) bool

type PluginState uint8
const (StateDiscovered PluginState = iota; StateApproved; StateLoading; StateActive; StateFailed; StateDisabled)

// PluginContext is the capability-checked handle passed to a plugin (no *World).
type PluginContext interface {
    Commands() CommandIssuer // tagged with plugin ID (INV-7)
    Config() []byte          // validated against manifest [config.schema]
    Capabilities() CapabilitySet
    Logger() *slog.Logger
}

var (
    ErrManifestInvalid    = errors.New("plugin: manifest invalid")
    ErrEngineIncompatible = errors.New("plugin: engine version constraint not satisfied")
    ErrCapabilityDenied   = errors.New("plugin: capability not granted")
    ErrDuplicateID        = errors.New("plugin: duplicate plugin id")
)
```

## Performance Strategy

- **Load is off the hot path**: discovery/parse/spawn happen at init levels, not per frame.
- **Capability check is a map lookup**: `CapabilitySet.Has` is O(1); the proxy adds one map probe per engine-API call, not per ECS op.
- **OOP I/O off the main loop**: protocol reads/writes run on dedicated goroutines via the task pool; the main schedule never blocks on a subprocess.
- **Zero cost when unused**: a build importing neither `pkg/plugin` nor the manager pays nothing (multi-repo INV-5).

## Error Handling

| Condition | Behavior |
| :--- | :--- |
| Malformed/absent manifest | `ErrManifestInvalid` (`E-PLUGIN-001`) at discovery |
| Engine-version mismatch | `ErrEngineIncompatible` (`E-PLUGIN-002`) before any plugin code |
| Ungranted capability at runtime | `ErrCapabilityDenied` (`E-PLUGIN-003`), logged with plugin ID + capability + site |
| Duplicate plugin ID | `ErrDuplicateID` (`E-PLUGIN-004`); second load rejected |
| OOP crash/timeout/disconnect | record → `Failed`; in-flight cancelled; host continues (INV-8) |
| Manifest checksum mismatch on reload | demote to `Discovered`, re-prompt approval (INV-9) |

## Testing Strategy

- **Manifest**: parse valid/invalid `plugin.toml`; missing required field ⇒ `ErrManifestInvalid`.
- **Constraint**: table test of `^`/`~`/range vs engine versions (INV-2).
- **Capability proxy**: an ungranted call ⇒ `ErrCapabilityDenied` + a log entry; a granted call passes (INV-3).
- **Duplicate ID** ⇒ `ErrDuplicateID` (INV-5).
- **Command tagging**: commands issued via `PluginContext.Commands()` carry the plugin ID (INV-7).
- **OOP isolation** (INV-8): a fake subprocess that crashes/times out marks the record `Failed` without panicking the host; mode-parity harness asserts in-proc ≡ OOP for a reference plugin.
- **Checksum**: tampered manifest on reload ⇒ demotion + re-prompt (INV-9).

## 7. Drawbacks & Alternatives

- **Drawback**: TOML + SemVer-range may need deps.
  **Alternative**: hand-rolled stdlib parsers.
  **Decision**: gate any dep behind an ADR (C24); ship a minimal stdlib parser for the documented manifest/constraint subset until then. Kept.
- **Drawback**: trust-but-verify does not sandbox in-process Go code.
  **Alternative**: WASM sandboxing of all plugins.
  **Decision**: L1 §4.12 scopes code sandboxing out for v1; OS isolation is offered via OOP mode. Kept.

## Canonical References

<!-- MANDATORY for Stable status. Stub — populate when implementation lands
     (Phase 6 Track N). Blocked until: (1) l1-plugin-distribution Stable; (2)
     pkg/plugin SDK + loader + OOP supervisor implemented with contract tests
     (T-6N01..04, T-6T01) + a validating example (examples/plugin/distribution/). -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-30 | Initial L2 draft — Go translation of l1-plugin-distribution v0.1.0. Public `pkg/plugin` SDK (no `internal/`), `plugin.toml` manifest parser, `CapabilitySet` + runtime-enforcing `capabilityProxy`, SemVer `Constraint`, in-process + OOP loaders sharing one lifecycle (INV-4), OOP failure isolation over `pkg/protocol` (INV-8), plugin-ID-tagged commands (INV-7), checksum-immutable manifests (INV-9). No Go `plugin` stdlib (.so). Authored ahead of Phase 6 Track N (`/magic.spec`). Draft — L1 parent Draft + no implementation yet. |
