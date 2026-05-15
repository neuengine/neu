# Code Documentation Convention — Go Implementation

**Version:** 0.1.0
**Status:** Draft
**Layer:** go
**Implements:** [code-documentation.md](l1-code-documentation.md)

## Overview

Realises the [Code Documentation Convention](l1-code-documentation.md) in Go
`godoc` comments. It defines the exact `AI-Meta:` block grammar, where it sits
relative to the idiomatic godoc summary, how it uses Go doc-link syntax for
relations, how it coexists with Go's `// Deprecated:` tooling marker, the
package scope tiers, and the CI detection mechanism.

## Related Specifications

- [code-documentation.md](l1-code-documentation.md) — L1 concept specification (parent)
- [l1-build-tooling.md](l1-build-tooling.md) — `ci test-doc` runs the presence and leakage checks
- [l1-examples-framework.md](l1-examples-framework.md) — Example packages are mandatory-tier surface

## 1. Motivation

A concrete Go binding is needed so the convention is enforceable and uniform:
`godoc`/`pkg.go.dev` render the full comment, so the block must be defined to
stay idiomatic for human readers while remaining deterministically parseable for
AI tooling and spec generation.

## 2. Constraints & Assumptions

- **Go 1.26.3+**; uses Go doc-link syntax (`[Pkg.Symbol]`) introduced in Go 1.19.
- The standard godoc contract (first sentence = summary, starts with the symbol
  name) MUST remain intact — the block is appended, never prepended.
- The block must survive `gofmt`/`goimports` unchanged (it is plain comment text).
- Detection MUST be runnable locally with the same tool/flags as CI (per
  [l1-build-tooling.md](l1-build-tooling.md) INV-1).
- No third-party dependency may be introduced for parsing (C24 / STATE [C-003]);
  detection uses `go/ast` + `go/doc` from the standard library, or a documented
  regex fallback.

## 3. Core Invariants

> [!NOTE]
> See [code-documentation.md §3](l1-code-documentation.md) for the
> technology-agnostic invariants.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1** Public symbols carry metadata | Every exported identifier in the mandatory/recommended tiers (§4.3) gets an `AI-Meta:` block in its doc comment. |
| **INV-2** Appended after summary | The block follows the existing godoc prose, separated by one blank comment line (`//`). The first sentence still leads and starts with the symbol name. |
| **INV-3** Fixed label, ordered fields, controlled vocab | Literal `AI-Meta:` label; fixed order `Purpose`, `Usage`, `Related`, `Stability`; `Stability` ∈ `{Stable, Experimental, Internal, Deprecated}` (§4.2). |
| **INV-4** Relations are code symbols only | `Related` uses Go doc-link syntax `[Symbol]` / `[Type.Method]` / `[pkg.Symbol]`; spec files, task IDs and `.design/` paths are rejected by the leakage scan (§4.4). |
| **INV-5** No workflow-artifact leakage | The leakage scan (§4.4) fails the build on task IDs, workflow names, spec filenames, or `.design/` paths anywhere in `*.go` comments and generated user docs. |
| **INV-6** Non-duplicative, consistent | Review gate: `Purpose` must not restate the summary verbatim; `Stability: Deprecated` requires a matching standard `// Deprecated:` line and a named replacement. |
| **INV-7** CI-checkable | Two `cmd/ci` sub-checks: presence (AST walk over exported decls) and leakage (regex over comment groups). Both wired into `ci test-doc`. |

## 5. Detailed Design

### 5.1 Block Grammar `[REFERENCE]`

The `AI-Meta` block is a comment-format contract (not executable Go). Shape:

```text
// {Symbol} {idiomatic godoc summary — unchanged, starts with the symbol name}.
// {optional additional human prose}
//
// AI-Meta:
//   - Purpose: {what it is for / why it exists — 1–2 lines, no fluff}
//   - Usage: {smallest realistic call form, or "n/a — {reason}"}
//   - Related: {[Symbol], [Type.Method], [pkg.Symbol] — comma-separated}
//   - Stability: {Stable | Experimental | Internal | Deprecated}
```

Rules:

- Exactly one blank `//` line separates the human prose from `AI-Meta:`.
- The label line is literally `// AI-Meta:` with no trailing text.
- Each field is one logical line, two-space indented, `- Field: value`.
- Field order is invariant. All four fields are always present.
- `Related` values use Go 1.19+ doc-link brackets so `pkg.go.dev` renders them
  as navigable links and AI tooling can graph-walk them.

Worked example `[REFERENCE]` (engine event bus):

```text
// EventBus is the double-buffered storage for broadcast events of type T.
// Two slices alternate roles each frame so a single integer cursor tracks
// reader progress across rotations.
//
// AI-Meta:
//   - Purpose: Per-world, type-parameterised broadcast channel; events live exactly two frames.
//   - Usage: bus := ecs.RegisterEvent[Damage](w); w := ecs.NewEventWriter[Damage](w); w.Send(Damage{...}).
//   - Related: [RegisterEvent], [EventWriter], [EventReader], [SwapAll].
//   - Stability: Stable.
type EventBus[T any] struct{ /* ... */ }
```

### 5.2 Stability Vocabulary Mapping

| Value | Meaning | Go binding |
| :--- | :--- | :--- |
| `Stable` | Supported public contract | Exported in `pkg/ecs` and intended for game code |
| `Experimental` | May change without major bump | Exported but flagged; safe to use, not frozen |
| `Internal` | No external compatibility promise | Symbols under `internal/` |
| `Deprecated` | Superseded; replacement named | MUST co-exist with a standard `// Deprecated: use [X]` line so `go vet`/IDE tooling also flags it |

### 5.3 Scope Tiers (Go binding)

| Tier | Location | Requirement |
| :--- | :--- | :--- |
| Mandatory | exported identifiers in `pkg/ecs/`, and `examples/**` public entry points | `AI-Meta:` required |
| Recommended | exported identifiers in `internal/ecs/**` that are conceptual surface (primary types, `New*` constructors, key methods) | `AI-Meta:` required unless trivial getter (then `Usage: n/a — trivial accessor`) |
| Optional | unexported identifiers | `AI-Meta:` only when intent is non-obvious |
| Excluded | `*_test.go`, generated files (`//go:generate` output, `*_gen.go`) | no requirement; leakage scan still applies to non-test shipped files |

`pkg/ecs/` is the highest-value target: it is the stable façade game code imports.

### 5.4 Detection (`cmd/ci`)

Wired into `ci test-doc` (see [l1-build-tooling.md](l1-build-tooling.md) §4.2):

1. **Presence check** — walk packages with `go/ast`; for every exported
   declaration in mandatory/recommended tiers, assert its doc group contains a
   line matching `^// AI-Meta:$` followed by the four ordered fields. Report
   `DOC_META_MISSING {pkg}.{symbol}` on failure.
2. **Leakage scan** — over every comment group in shipped `*.go` files (tiers
   except Excluded) and generated user docs, fail on a match of the forbidden
   pattern set (task identifiers, workflow command names, spec filenames,
   `.design/` path segments). Report `DOC_ARTIFACT_LEAK {file}:{line}`.

The convention's own `[REFERENCE]` examples in specs live under `.design/` and
are never scanned (the scan targets shipped code and user docs only).

### 5.5 Remediation of Existing Leakage

Existing shipped comments contain build-process task identifiers (observed in
multiple `internal/ecs/**` files). Bringing the tree into compliance is
implementation work scheduled by the planner, not part of this spec: the spec
defines the rule and the detector; the run phase performs the rewrite (move the
context into commit messages / specs, delete the inline reference).

## 6. Drawbacks & Alternatives

- **godoc/pkg.go.dev verbosity**: the block is visible in rendered docs.
  Accepted: it also helps human readers and is kept terse by INV-6. Alternative
  (a separate sidecar metadata file) was rejected — it desynchronises from code
  and is invisible to an AI reading the source.
- **AST detector cost**: a full AST walk is heavier than grep. Accepted: it runs
  in the existing CI doc stage and is precise; the regex fallback exists for
  fast local pre-checks.
- **Vocabulary rigidity**: a closed stability set may be too coarse. Tracked as
  an L1 Open Question; the set can be extended via an L1 amendment, not ad hoc.

## Canonical References

<!-- MANDATORY for Stable status. Populate with concrete authoritative files when
     implementation lands (the CI detector and the pkg/ecs façade). -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

<!-- Empty table = no canonical sources yet. Stable promotion requires ≥1 row. -->

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-15 | Initial draft: Go `AI-Meta:` godoc grammar, stability mapping, scope tiers, `cmd/ci` detection |
| — | — | Planned examples: [examples/ecs/poc/](../../../examples/ecs/poc/) |
