# Compatibility and Versioning Policy

**Version:** 0.4.0
**Status:** Stable
**Layer:** concept

## Overview

This specification defines the versioning strategy for the Neu engine and its relationship with the Go toolchain. As a high-performance engine, we often utilize bleeding-edge compiler and runtime features, necessitating a clear policy on Go version support and backwards compatibility.

## Related Specifications

- [l1-build-tooling.md](l1-build-tooling.md) — CI matrix enforces the Go version support policy
- [l1-platform-system.md](l1-platform-system.md) — Build tags and platform capabilities gated by Go version
- [l1-error-core.md](l1-error-core.md) — Error-code stability guarantees tied to SemVer minor/major

## 1. Motivation

Game engines live at the intersection of two versioning pressures that usually conflict: the Go toolchain releases fast (new GC, new SIMD intrinsics, new `iter` primitives every 6 months), while production games expect a stable ABI across a multi-year content pipeline. Without a codified policy:

- **Bleeding-edge adoption becomes silent.** A maintainer pulls in a Go 1.27 feature; downstream consumers on Go 1.26 discover it only at compile time, inside a build agent.
- **Deprecation becomes emotional.** Dropping support for an old toolchain needs to be a scheduled, communicated event — not a side effect of a refactor.
- **Downstream planning is impossible.** Engine users need to know whether a `0.x` minor bump can silently break their code, and when a `1.0` contract becomes load-bearing.

This specification is the single source of truth for those answers. It makes the "Latest + 1" support window explicit, aligns the engine's SemVer semantics with the Go release cadence, and defines what counts as a breaking change at the API, runtime, and build-tag layers.

## 2. Engine Versioning (SemVer)

The engine follows [Semantic Versioning 2.0.0](https://semver.org/).

- **MAJOR**: Breaking API changes or fundamental architectural shifts (e.g., switching from Table to something else).
- **MINOR**: New features, new diagnostic metrics, or significant performance optimizations that maintain backward API compatibility.
- **PATCH**: Bug fixes, minor documentation updates, or internal refactoring.

## 3. Go Version Compatibility Matrix

The engine maintains a strict "Latest + 1" support policy.

| Engine Version | Min Go Version | Primary Features Used |
| :--- | :--- | :--- |
| **v0.1.x** | Go 1.26.3 | Green Tea GC, SIMD, Enhanced `new`, Iterators (Go 1.23+) |

### 3.1 Support Window

- **Active Development**: Targets the absolute latest stable Go release (currently Go 1.26.3).
- **Maintenance**: Supports the current (N) and previous (N-1) major Go releases.
- **Deprecation**: Support for a Go version is dropped when it becomes N-2 relative to the current Go release, unless otherwise specified.

## 4. Toolchain Enforcement

- **go.mod**: The `go` directive in `go.mod` MUST reflect the minimum supported Go version.
- **Build Tags**: Features dependent on specific Go versions (e.g., experimental SIMD) must be guarded by build tags (e.g., `//go:build go1.26`).

## 5. Breaking Changes Policy

### 5.1 API Stability

- During `0.x.y` (alpha/beta), breaking changes are permitted in MINOR versions.
- After `1.0.0`, breaking changes require a MAJOR version bump.

### 5.2 Go Runtime Dependencies

If a new Go version introduces a critical performance feature (like a new GC or memory allocator optimization) that requires a breaking change in our memory layout, it will trigger a MAJOR or MINOR version bump depending on the impact.

## 6. Development Workflow

- All PRs are tested against the current and previous Go version in CI.
- Benchmarks are run on the latest Go version to ensure performance gains from the new runtime are captured.

## Canonical References

<!-- Realized by the Track J tooling: pkg/version (policy) + cmd/apidiff (enforcement) + CI. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |
| version-policy | pkg/version/policy.go | Engine SemVer policy + `MinGoToolchain` (1.26.3) + `GoToolchainConstraint`/`IsGoToolchainSupported` (§2–§4) |
| version-core | pkg/version/version.go | `Version` + `Parse` + `Compare` — the SemVer value type (§2) |
| version-constraint | pkg/version/constraint.go | Cargo-subset `Constraint` (caret-on-0.x = the §5.1 0.x breaking rule) |
| apidiff | cmd/apidiff/ | Public-API snapshot + breaking-change gate enforcing §5.1 API stability |
| ci-matrix | .github/workflows/ci.yml | CI Go-version build/test + `-race` + benchmark gate enforcing §3/§6 |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-03-27 | Initial draft |
| 0.2.0 | 2026-03-30 | Added C26 example correlation placeholder for compatibility validation |
| 0.3.0 | 2026-04-19 | Added `## Related Specifications` and `## 1. Motivation` sections; renumbered §1–§5 → §2–§6 (RULES §5/§6 compliance) |
| 0.4.0 | 2026-06-01 | Promoted Draft → Stable (`/magic.task` Pre-Planning Stabilization). Fully realized by Track J: `pkg/version` (SemVer + Go-toolchain matrix — `MinGoToolchain` matches §3's v0.1.x → Go 1.26.3 row) + `cmd/apidiff` (the §5.1 breaking-change gate over a committed `api/snapshot.json`) + `.github/workflows/ci.yml` (§3/§6 Go-version + race + bench enforcement). Canonical References populated; no deferred headline. |
