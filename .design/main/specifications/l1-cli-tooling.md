# CLI Tooling

**Version:** 0.2.0
**Status:** Draft
**Layer:** concept

## Overview

Defines the structure, commands, and rollout of the internal command-line interface (`cmd/cli/`) used for scaffolding, diagnostics, and engine-facing workflows. The repository currently contains only a bootstrap stub, so this spec focuses on phased growth instead of implying a fully implemented command suite.

## Related Specifications

- [l1-build-tooling.md](l1-build-tooling.md) — CI pipeline commands vs. user-facing CLI
- [l2-codegen-tools.md](l2-codegen-tools.md) — Code generation commands executed via this CLI
- [l1-examples-framework.md](l1-examples-framework.md) — Example-oriented developer commands

## 1. Motivation

A dedicated CLI provides a unified entry point for developers using the Bolt engine. It replaces fragmented scripts by standardizing common project tasks.

At the current stage, the CLI also serves as a controlled expansion point: new commands should appear only when the underlying subsystems are real enough to support them.

## 2. Constraints & Assumptions

- The CLI uses standard Go `flag` or a lightweight router rather than heavy third-party frameworks unless required.
- Commands must be deterministic and support scripting, including optional `--json` output.
- During Phase 0, it is acceptable for commands to report `not implemented yet` as long as the help output is truthful and stable.

## 3. Core Invariants

- **INV-1**: All scaffolding commands must safely skip or prompt before overwriting existing files.
- **INV-2**: The CLI must print a structured help menu when invoked without arguments.
- **INV-3**: Phase 0 must not advertise commands that do not exist.
- **INV-4**: Machine-readable output is opt-in via explicit flags such as `--json`.

## 4. Detailed Design

### 4.1 Phase 0 Bootstrap

The current repository state is a bootstrap binary that confirms the CLI entry point exists. In this phase:

- `boltengine` without subcommands prints status/help.
- The binary may point the user to specs or contribution docs.
- No filesystem-mutating commands are required yet.

This keeps `cmd/cli/` real without pretending that scaffolding, asset management, or example runners already exist.

### 4.2 Planned Command Groups

The CLI expands in narrow, implementation-backed layers:

```plaintext
boltengine help
boltengine scaffold {target}
boltengine doctor [--json]
boltengine example list
boltengine example run {name}
boltengine codegen {target}
```

- `help` is always available.
- `scaffold` activates only after the corresponding project templates exist.
- `doctor` reports environment, version, and workspace health.
- `example` commands activate after runnable examples exist.
- `codegen` is a thin front-end over the code-generation subsystem, not a duplicate implementation.

### 4.3 Output Contracts

Human-oriented output is the default. Structured output is available only where automation needs it:

```plaintext
boltengine doctor --json
```

- Plain-text output prioritizes clarity and next steps.
- JSON output uses stable keys and avoids decorative text.
- Error messages remain actionable and reference the missing capability or prerequisite explicitly.

### 4.4 Activation Gates

Command groups are introduced only when their backing systems exist:

- No asset-management commands before the asset system exists.
- No example-runner commands before there are runnable examples.
- No codegen commands before the codegen toolchain is defined well enough to version.

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
| 0.1.0 | 2026-03-30 | Initial Draft |
| 0.2.0 | 2026-03-31 | Added phased rollout rules and aligned the spec with the current bootstrap CLI stub. |
| — | — | Planned examples: [examples/ecs/cli_stub/](../../../examples/ecs/cli_stub/) |
