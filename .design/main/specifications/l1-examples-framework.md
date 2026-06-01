# Examples Framework

**Version:** 0.5.0
**Status:** Stable
**Layer:** concept

## Overview

Defines the staged structure and lifecycle of the engine's `examples/` directory. The project is still in the specification phase, so the framework distinguishes between placeholder example paths, bootstrap validation examples, and fully runnable showcase coverage. This keeps `examples/` aligned with the current maturity of the engine instead of pretending that every planned subsystem already has executable demos.

## Related Specifications

- [l1-app-framework.md](l1-app-framework.md) — App entry points used by runnable examples
- [l1-build-tooling.md](l1-build-tooling.md) — CI and validation rules for examples
- [l1-entity-system.md](l1-entity-system.md) — First validating examples target the ECS core
- [l1-world-system.md](l1-world-system.md) — World-oriented examples expand after the POC

## 1. Motivation

Examples are the bridge between specifications and implementation. For this project they serve three separate purposes:

1. **Validation** — prove that a spec can be exercised end-to-end in code.
2. **Documentation** — show intended usage patterns once public APIs exist.
3. **Phase control** — prevent specifications from reaching `Stable` without a concrete validation target.

During the current spec-first phase, examples must be planned carefully. Over-specifying hundreds of demos before the engine core exists creates noise, inflates maintenance cost, and makes documentation drift look like implementation drift.

## 2. Constraints & Assumptions

- The repository currently contains placeholder paths in `examples/`; this is expected during Phase 0.
- A Draft specification may reserve an example path before runnable code exists.
- Runnable examples use Go and depend only on engine packages plus the standard library.
- Example categories must map cleanly to workspace scope and specification ownership.
- Render-, audio-, or physics-heavy example suites are deferred until the corresponding subsystems move beyond foundational design.

## 3. Core Invariants

- **INV-1**: No L1 or L2 specification may be promoted to `Stable` without at least one validating example path, per C26/C29.
- **INV-2**: Placeholder example directories are allowed in Draft, but they must clearly represent planned validation rather than fake implementation.
- **INV-3**: Once an example becomes runnable, it must have a stable invocation path from the repository root.
- **INV-4**: Example categories grow in the same order as subsystem implementation maturity: POC core first, expansion later.
- **INV-5**: Showcase, screenshot, and stress-test infrastructure activates only after there are enough runnable examples to justify dedicated tooling.

## 4. Detailed Design

### 4.1 Phase-Based Rollout

| Phase | Repository state | Example expectation |
| :--- | :--- | :--- |
| Phase 0 — Spec Drafting | Mostly placeholders and package skeletons | Reserve example paths and describe intended validation targets |
| Phase 1 — POC Validation | Core ECS packages start landing | Add minimal runnable examples for `ecs/`, `world/`, and `app/` |
| Phase 2 — Subsystem Expansion | Tooling and subsystem code grows | Expand category coverage for input, diagnostics, assets, and math |
| Phase 3 — Showcase Coverage | Multiple subsystems are operational | Add showcase, visual regression, and stress-test suites |

### 4.2 Directory Layout

The framework defines a small stable category set first. Additional categories are introduced only when the corresponding specs become implementation-relevant.

```plaintext
examples/
├── README.md                    # Future index once multiple runnable examples exist
├── ecs/                         # P1 validation target: entities, components, queries, scheduler
├── world/                       # P1/P2 validation target: resources, events, hierarchy
├── app/                         # App bootstrap and plugin lifecycle
├── diagnostic/                  # Profiling and debug validation, introduced later
├── stress_test/                 # Benchmarks once baseline implementation exists
└── {future-category}/           # Added only when backed by active implementation work
```

### 4.3 Example States

Each example path moves through one of three states:

1. **Placeholder** — directory and intent are reserved, but no runnable code is required yet.
2. **Bootstrap runnable** — minimal `main.go` exists and validates a narrow slice of the API.
3. **Validating example** — used as a CI or review gate for a spec that is approaching `Stable`.

Placeholder examples are acceptable only while the corresponding spec is still Draft or early RFC. They are planning artifacts, not substitutes for validation.

### 4.4 Per-Example Contract

Runnable examples follow the same minimal contract:

```plaintext
examples/{category}/{name}/
├── main.go                      # Executable validation entry point
├── README.md                    # Purpose, spec links, expected behavior
└── testdata/                    # Optional assets or golden files
```

The `README.md` for each runnable example documents:

- Which specifications it validates
- How to execute it
- What success looks like
- Which limitations are expected during the current phase

### 4.5 Validation Policy

- **Draft**: reserved example path is enough.
- **RFC**: at least one bootstrap runnable example is expected when underlying code exists.
- **Stable**: example must be runnable or otherwise mechanically validated by the active build tooling.

If a spec claims a mature example suite before the corresponding code exists, that is a documentation error and must be corrected before further expansion.

### 4.6 Showcase Runner Deferral

`cmd/showcase/` remains a planned tool, not a present repository guarantee. It becomes worthwhile only after the engine has enough runnable examples to justify:

- filtered execution
- headless example runs
- screenshot capture
- visual regression workflows

Until then, the framework prioritizes a small number of honest validation examples over a large imaginary catalog.

## 5. Implementation Notes

1. Start with `examples/ecs/` as the first validating lane for the POC specs.
2. Add `examples/world/` only after entity/component/query basics can execute.
3. Introduce `examples/stress_test/` once benchmarks have a meaningful runtime substrate.
4. Delay render-heavy categories until the render pipeline has executable code, not only concepts.

## 6. Drawbacks & Alternatives

**Drawback**: A staged framework is less visually impressive than a full example catalog.
**Mitigation**: It is substantially more honest and keeps documentation synchronized with the real implementation curve.

**Alternative considered**: Pre-list every planned example up front.
**Rejected**: It front-loads maintenance cost and creates false signals during project ventilation.

**Alternative considered**: Skip example planning until after implementation.
**Rejected**: That weakens C26/C29 traceability and makes spec validation ad hoc.

## Canonical References

<!-- Realized by the Track I tooling: cmd/examplecheck + conventions + scaffold + goldens. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |
| examplecheck | cmd/examplecheck/ | Mechanical validation harness — discovers/runs examples, gates against goldens, selective build (§4.5, INV-3) |
| conventions | examples/README.md | Per-example contract + golden-gate documentation (§4.4) |
| template | examples/_template/ | The runnable-example scaffold (`run()`/`main()`/`main_test.go` contract, §4.4) |
| goldens | examples/goldens.json | Committed golden registry — the Stable mechanical-validation gate (§4.5) |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-03-25 | Initial Draft |
| 0.2.0 | 2026-03-25 | Expanded: full example catalog from reference engine (280+ examples across 28 categories). Added categories: camera, gizmos, picking, time, ui, shader, showcase. Excluded language-specific examples. |
| 0.3.0 | 2026-03-26 | Added showcase runner section (bulk execution, visual regression, screenshot capture). Added build-tooling cross-reference. |
| 0.4.0 | 2026-03-31 | Reframed the spec around staged rollout, placeholder paths, and honest phase-based validation. |
| 0.5.0 | 2026-06-01 | Promoted Draft → Stable (`/magic.task` Pre-Planning Stabilization). Realized by Track I: `cmd/examplecheck` is the §4.5 mechanical-validation gate (golden drift + selective build), `examples/_template/` + `examples/README.md` codify the §4.4 per-example contract, `examples/goldens.json` is the committed gate. `cmd/showcase/` remains deferred **by the spec itself** (§4.6) — not an overclaim. Canonical References populated. |
