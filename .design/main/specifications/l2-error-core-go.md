# Error Core — Go Implementation

**Version:** 0.1.0
**Status:** Draft
**Layer:** go
**Implements:** [l1-error-core.md](l1-error-core.md)

## Overview

Go-level design for the engine's structured error taxonomy. Every engine error is
a value implementing the `EngineError` interface — a coded, categorized, localizable
error that composes cleanly with the Go standard `errors` package (`Is`/`As`/`Unwrap`).
Error text lives in an external `fs.FS`-backed catalog (`errors.{lang}.json`) so the
same E-series code drives a developer tooltip, a CI log line, and a localized
player-facing dialog. Stack traces are captured only in debug builds; panics are
reserved for `Fatal` developer errors.

## Related Specifications

- [l1-error-core.md](l1-error-core.md) — L1 concept specification (parent)
- [l2-asset-system-go.md](l2-asset-system-go.md) — `fs.FS` VFS reused to load the locale catalog
- [l2-diagnostic-system-go.md](l2-diagnostic-system-go.md) — diagnostics pipeline consumes `EngineError` codes (planned)
- [l1-compatibility-policy.md](l1-compatibility-policy.md) — E-series code stability across engine versions

## 1. Motivation

The engine returns errors across dozens of subsystems; without a single typed
contract each module invents its own string conventions. Binding the L1 taxonomy
to one Go interface gives every caller a uniform way to branch on `Severity`,
surface a `Solution`, and localize a message — while still inter-operating with the
billions of lines of code that expect a plain `error`. The catalog-over-`fs.FS`
choice reuses the existing asset VFS, keeping the core dep-free (C-003).

## 2. Constraints & Assumptions

- **Go 1.26.3+**: `errors.Is/As/Join`, `fmt.Errorf("%w")`, `text/template` for placeholder expansion — all stdlib.
- **C-003**: zero external deps. JSON via `encoding/json`; templating via `text/template`; trace via `runtime.Callers`.
- `EngineError` MUST satisfy `error` so it flows through existing `error`-typed return values unchanged.
- The locale catalog is loaded once at startup from an `fs.FS`; a missing catalog degrades to the embedded default-English template (never a crash).
- Trace capture is compiled out of release builds via a build tag (`//go:build debug`) — zero cost in production hot paths.
- Code → descriptor registration happens at `init()` per module; duplicate code registration is a developer error caught at startup.

## 3. Core Invariants

> [!NOTE]
> See [l1-error-core.md §2–§5](l1-error-core.md) for the technology-agnostic
> taxonomy and directives. Go-specific compliance is tabulated in §4.

## 4. Invariant Compliance

| L1 Directive | Implementation |
| :--- | :--- |
| `EngineError` interface (`Code`/`Severity`/`Module`/`Solution`) | A Go interface embedding `error` with exactly those four accessors; the concrete `engineError` struct is the only built-in implementation. `errors.As(err, &EngineError)` recovers it from any wrap chain. |
| **Severity** (Fatal/Recoverable/Warning/Debug) | `Severity uint8` iota enum; `String()` is a total switch (no default fall-through) so a new level cannot be silently mis-rendered. |
| **Target audience** (Developer/User/System) | `Audience uint8` iota enum carried on the `Descriptor`; lets diagnostics route developer errors to the editor and user errors to player dialogs. |
| **E-series codes** (`E[Category][ID]`, range-partitioned) | `Code string` newtype; a package-level `registry map[Code]Descriptor` guarded by `sync.RWMutex`; `Register` rejects a code outside its module's declared range and rejects duplicates (`ErrDuplicateCode`). |
| **Localization** (external `errors.{lang}.json`) | `Catalog` loads code→template maps from an `fs.FS`; `Localize(code, args)` expands the template with `text/template`. Missing key ⇒ fall back to the registered default template (never empty). |
| **No silencing** | The interface returns an `error` (not a bool); paired with the `errcheck` linter, a returned `EngineError` cannot be discarded with `_ =`. |
| **Trace context** (debug builds only) | `captureTrace()` has two build-tagged implementations: `//go:build debug` calls `runtime.Callers`; the default returns `nil` — release builds pay zero cost. |
| **Panic policy** (Fatal+Developer only) | `MustSucceed(err)` panics **iff** `Severity()==Fatal && Audience()==Developer`; all other severities are returned to the caller. |

## Go Package

```
pkg/errs/
  error.go         // EngineError interface, engineError struct, New/Wrap constructors
  severity.go      // Severity, Audience enums + total-switch String()
  code.go          // Code newtype, category ranges, Descriptor, Register/Lookup registry
  catalog.go       // Catalog (fs.FS loader), Localize, default-template fallback
  trace_debug.go   // //go:build debug — runtime.Callers capture
  trace_release.go // //go:build !debug — no-op capture (0 cost)
locales/
  errors.en.json   // canonical English catalog (code → template + solution)
```

## Type Definitions

```go
type Severity uint8
const (
    SeverityDebug Severity = iota
    SeverityWarning
    SeverityRecoverable
    SeverityFatal
)

type Audience uint8
const (
    AudienceDeveloper Audience = iota
    AudienceUser
    AudienceSystem
)

type Code string // "E0001"; E0000–E0999 ECS, E1000–E1999 scheduling, E2000–E2999 render/assets, E3000–E3999 physics

type Descriptor struct {
    Code     Code
    Severity Severity
    Audience Audience
    Module   string // "ecs", "render", "physics"
    Template string // default English message template
    Solution string // actionable developer advice
}

type EngineError interface {
    error
    Code() Code
    Severity() Severity
    Module() string
    Solution() string
}
```

## Key Methods

```go
// Construction — args feed the localized template.
func New(code Code, args ...any) EngineError
func Wrap(code Code, cause error, args ...any) EngineError // sets Unwrap() → cause

// Registry (called from each module's init()).
func Register(d Descriptor) error // ErrDuplicateCode / ErrCodeOutOfRange
func Lookup(code Code) (Descriptor, bool)

// Localization.
func (c *Catalog) Load(fsys fs.FS, lang string) error
func (c *Catalog) Localize(code Code, args ...any) string // template expand; default fallback

// Standard-library interop.
func (e *engineError) Unwrap() error          // wrap chain
func (e *engineError) Is(target error) bool    // matches by Code

// Panic policy (INV: Fatal+Developer only).
func MustSucceed(err error) // panics iff EngineError && Fatal && Developer
```

## Performance Strategy

- **Zero-cost release traces**: `trace_release.go` makes `captureTrace()` a no-op returning `nil`; the `runtime.Callers` path is excluded from release binaries entirely.
- **Construction allocates once**: `New` builds a single `engineError` value; the localized string is rendered lazily on `Error()`, so error paths that are only logged-by-code never run the template.
- **Registry is read-mostly**: populated at `init()`, read under `RWMutex.RLock` thereafter; the hot path (`Lookup`) takes only a read lock.
- **No reflection in the hot path**: `Code` comparison is a plain string compare; `Is` matches on `Code`, not deep equality.

## Error Handling

| Condition | Behavior |
| :--- | :--- |
| Duplicate `Register` of a code | `ErrDuplicateCode{Code}` at startup — fail fast |
| Code outside its module's declared range | `ErrCodeOutOfRange{Code, Module}` at `Register` |
| `Localize` of an unregistered/missing key | fall back to the registered default template; if none, return `"<code>: <untemplated args>"` — never empty |
| Catalog file missing or malformed | log a `Warning`-severity error, keep the embedded English defaults (no crash) |
| `MustSucceed` on a Fatal developer error | `panic(err)` — the only permitted panic path |

```go
type ErrDuplicateCode  struct{ Code Code }
type ErrCodeOutOfRange struct{ Code Code; Module string }
```

## Testing Strategy

- **Interface conformance**: `var _ EngineError = (*engineError)(nil)`; `errors.As` recovers `EngineError` through a `fmt.Errorf("%w")` chain.
- **Severity totality**: a table test asserts `String()` is defined for every enum value (guards against an unhandled new level).
- **Registry guards**: registering a duplicate code ⇒ `ErrDuplicateCode`; an out-of-range code ⇒ `ErrCodeOutOfRange`.
- **Localization fallback**: load a catalog missing a key ⇒ `Localize` returns the default template; load a malformed file ⇒ defaults retained, `Warning` logged.
- **Trace gating**: build with `-tags debug` ⇒ trace non-nil; default build ⇒ trace nil.
- **Panic policy**: `MustSucceed` panics only for Fatal+Developer; returns for every other (severity × audience) combination.

## 7. Drawbacks & Alternatives

- **Drawback**: an external catalog adds a load step and a failure mode.
  **Alternative**: embed all strings as Go string constants.
  **Decision**: L1 §4 requires localization; the `fs.FS` catalog reuses the asset VFS and degrades safely to embedded defaults. Kept.
- **Drawback**: a `Code string` newtype is stringly-typed vs. a generated enum.
  **Alternative**: code-generate a `Code` constant block per module.
  **Decision**: string codes are human-readable in logs and stable across versions (compatibility-policy); a future codegen pass (l2-codegen-tools) can emit the constants without changing the type. Kept.

## Canonical References

<!-- MANDATORY for Stable status. Stub — populate when implementation lands
     (Phase 6). Stable promotion blocked until: (1) l1-error-core is Stable
     (layer constraint); (2) implementation + tests land with a validating example. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-28 | Initial L2 draft — Go translation of l1-error-core v0.2.0. `EngineError` interface over stdlib `errors`, `Severity`/`Audience` total-switch enums, range-partitioned `Code` registry, `fs.FS` locale catalog with default fallback, build-tag-gated debug traces, Fatal+Developer-only panic policy. Authored ahead of Phase 6 (`/magic.spec`). Draft — L1 parent Draft + no implementation yet. |
