# Error Core Specification

**Version:** 0.2.0
**Status:** Stable
**Layer:** concept

## Overview

A game engine requires more than simple string-based errors. This specification defines a structured taxonomy for errors, categorizing them by severity, origin, and recoverability. It also establishes the foundation for error localization and developer-centric troubleshooting.

## Related Specifications

- [l1-diagnostic-system.md](l1-diagnostic-system.md) — Diagnostics pipeline consumes structured errors
- [l1-compatibility-policy.md](l1-compatibility-policy.md) — Error code stability across engine versions

## 1. Motivation

Unstructured error strings are the single largest source of production debugging friction: they are unlocalizable, unsearchable, un-triagable, and indistinguishable by severity. Every mature runtime (Go's `errors` package, Rust's `thiserror`, Java exceptions) converges on the same principle — errors must be typed, coded, and categorized.

For an ECS game engine the stakes are higher because:

- **Misuse is easy, detection is hard.** Querying a non-registered component, despawning a stale entity, or mutating an immutable component must surface as actionable developer errors — not mystery crashes deep in hot loops.
- **Fatal vs. recoverable has physical consequences.** A scheduling cycle is fatal; a missing asset is recoverable with a fallback. The engine must distinguish these unambiguously to decide whether to panic, log, or retry.
- **Localization matters for editor + end-user surfaces.** The same error code drives both a developer tooltip in the editor and a localized player-facing dialog.

A structured error taxonomy with codes, severity, and solution hints makes all three concerns tractable at the API boundary instead of deferring them to ad-hoc per-module conventions.

## 2. Error Taxonomy

All engine errors are categorized into the following dimensions:

### 2.1 Severity Levels

- **Fatal**: Unrecoverable engine state. Requires immediate process termination (Panic or os.Exit).
- **Recoverable**: Error that can be handled by the caller (e.g., entity not found).
- **Warning**: Potential issue that doesn't stop execution (e.g., duplicate system registration).
- **Debug**: Detailed trace info for development.

### 2.2 Target Audience

- **Developer Error**: Misuse of the API (e.g., querying for a component not added to the world).
- **User Error**: Runtime issues caused by end-user input or malformed assets (e.g., logic error in a script).
- **System Error**: OS-level or hardware issues (e.g., out of memory, file not found).

## 3. Structured Error Format

All errors MUST implement the `EngineError` interface:

```go
type EngineError interface {
    error
    Code() string        // E0001
    Severity() Severity  // Fatal, Recoverable, etc.
    Module() string      // "ecs", "render", "physics"
    Solution() string    // Actionable advice for the developer
}
```

### 3.1 Error Codes (E-Series)

Codes follow the format `E[Category][ID]`:

- `E0000-E0999`: Core ECS (Entity/Component/World)
- `E1000-E1999`: Scheduling & Systems
- `E2000-E2999`: Render & Assets
- `E3000-E3999`: Physics & Collision

## 4. Localization & UX

Error messages are stored externally to the code to allow for localization:

- **Registry**: A `locales/errors.en.json` file contains mapping from Error Codes to templates.
- **Templates**: Support placeholders (e.g., "System {name} has cyclic dependency").

## 5. Directives for Implementation

- **No Silencing**: Never use `_ = function()` for calls that return `EngineError`.
- **Trace Context**: Errors should automatically capture the stack trace when generated in `Debug` builds.
- **Panic Policy**: Panics are ONLY permitted for `Fatal` developer errors where continued execution would result in data corruption.

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
| 0.1.0 | 2026-03-27 | Initial draft |
| 0.2.0 | 2026-04-19 | Added `## Related Specifications` and `## 1. Motivation` sections; renumbered §1–§4 → §2–§5 (RULES §5/§6 compliance) |
| — | — | Planned examples: `examples/diagnostic/` |
