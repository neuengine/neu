# CLI Tooling — Go Implementation

**Version:** 0.1.0
**Status:** Draft
**Layer:** go
**Implements:** [l1-cli-tooling.md](l1-cli-tooling.md)

## Overview

Go-level design for the engine CLI at `cmd/cli/`. A stdlib-`flag`-based command
router dispatches subcommands; each subcommand is a small `Command` value with a
name, help, and `Run(ctx, args)`. Phase 0 ships only truthful, stable help +
`doctor`; further command groups activate only when their backing subsystem
exists (INV-3). Scaffolding commands never clobber files without prompt/skip
(INV-1); machine output is opt-in via `--json` (INV-4). Plugin subcommands
(`ecs plugin …`) front the `pkg/plugin` SDK; codegen fronts the codegen tool.

## Related Specifications

- [l1-cli-tooling.md](l1-cli-tooling.md) — L1 concept specification (parent)
- [l2-plugin-distribution-go.md](l2-plugin-distribution-go.md) — `ecs plugin scaffold|validate|install|list|…` front the plugin SDK
- [l2-codegen-tools.md](l2-codegen-tools.md) — `ecs codegen` is a thin front-end, not a duplicate implementation
- [l2-error-core-go.md](l2-error-core-go.md) — CLI errors use the structured taxonomy + actionable messages

## 1. Motivation

The L1 mandates a controlled-growth CLI: a unified entry point that never
advertises commands its subsystems can't back. The Go binding keeps this honest
with a registry of `Command` values whose availability is gated, stdlib `flag`
parsing (no heavy framework, C24), and a uniform `--json` output contract for
scripting.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: stdlib `flag` + a thin router; no Cobra/urfave unless an ADR justifies it (L1 §2 / C24).
- Commands are deterministic and scriptable; `--json` uses stable keys (INV-4).
- Phase 0 commands may print `not implemented yet` provided help stays truthful (INV-3).
- File-mutating commands prompt or skip before overwriting (INV-1); a `--force` flag is explicit.
- The binary name is `neuengine` (per L1 §4.2); `ecs` is the documented alias for plugin/asset subcommands.

## 3. Core Invariants

> [!NOTE]
> See [l1-cli-tooling.md §3](l1-cli-tooling.md) for INV-1…INV-4. Go-specific
> compliance is tabulated in §4.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: scaffolding never silently overwrites | `writeFile` checks `os.Stat`; an existing target prompts (or skips with `--force` explicit), never clobbers; `--dry-run` previews. |
| **INV-2**: no-arg invocation prints a structured help menu | `Router.Run(nil)` prints the registered command table (name + one-line help) + usage; exit 0. |
| **INV-3**: Phase 0 advertises only existing commands | The registry holds only implemented commands; a gated group registers via an `availableWhen()` predicate, so help never lists a command whose subsystem is absent. |
| **INV-4**: machine output opt-in via `--json` | Each command takes a shared `--json` flag; when set, output goes through a `jsonEncoder` with stable keys and no decorative text. |

## Go Package

```
cmd/cli/
  main.go                    // entry: build Router, register available commands, Run(os.Args)
  root.go                    // Router: command registry, global flags (--json, --force, --dry-run), help
  command.go                 // Command interface { Name, Help, Run(ctx, args, out) }
  doctor.go                  // `doctor`: environment + version + workspace health (--json)
  scaffold/                  // `scaffold {target}` — gated on templates existing
  asset/                     // `asset import|list|build` — gated on asset system
  plugin/                    // `plugin scaffold|validate|install|list|enable|disable|info|remove|doctor`
  output.go                  // text vs JSON writer, stable-key encoder
```

## Type Definitions

```go
// Command is one CLI verb. Run receives parsed args and an output sink that is
// either a human-text or a stable-JSON writer per the --json flag.
type Command interface {
    Name() string
    Help() string
    Run(ctx context.Context, args []string, out Output) error
}

// Router dispatches the first arg to a registered Command.
type Router struct {
    commands map[string]Command
    order    []string // registration order for help listing
}
func NewRouter() *Router
func (r *Router) Register(c Command)        // INV-3: only implemented commands
func (r *Router) Run(argv []string) int     // returns process exit code

// Output abstracts human text vs --json stable output (INV-4).
type Output interface {
    Text(format string, args ...any)
    JSON(v any) error
    Errorf(code, format string, args ...any) // actionable, references the missing prerequisite
}
```

## Performance Strategy

- **Negligible**: the CLI is a short-lived process; correctness and clear output dominate over speed. Command dispatch is a single map lookup.
- **Lazy subsystem init**: a command initializes only the engine subsystems it needs (e.g. `plugin list` builds the plugin manager, not a full `App`).
- **Streaming output**: long listings write incrementally to the `Output` sink rather than buffering whole result sets.

## Error Handling

| Condition | Behavior |
| :--- | :--- |
| Unknown subcommand | print help + the unknown name; exit 2 |
| Scaffold target exists | prompt / skip; `--force` overwrites explicitly (INV-1) |
| Command for an absent subsystem | not registered (INV-3); if invoked by alias, `Errorf` naming the missing prerequisite |
| `--json` + a runtime error | structured `{ "error": { "code", "message" } }` on stderr, non-zero exit |
| Plugin/asset op failure | surfaced via `pkg/errs` E-code + actionable next step |

## Testing Strategy

- **Help/no-arg** (INV-2): `Run(nil)` prints the registered table; golden output.
- **Truthful help** (INV-3): help lists only registered commands; a gated group absent ⇒ not shown.
- **Scaffold safety** (INV-1): writing over an existing file without `--force` prompts/skips (no clobber); `--dry-run` writes nothing.
- **JSON contract** (INV-4): `doctor --json` emits stable keys; schema asserted; no decorative text.
- **Exit codes**: unknown command ⇒ 2; success ⇒ 0; runtime error ⇒ non-zero.
- **Plugin subcommands**: `ecs plugin list|validate` golden output against a fixture plugin dir (integration, T-6T03).

## 7. Drawbacks & Alternatives

- **Drawback**: stdlib `flag` lacks subcommand niceties (nested help, completion).
  **Alternative**: Cobra.
  **Decision**: L1 §2 / C24 prefer stdlib; a thin router covers the documented command set. Revisit via ADR if completion/nested help become requirements. Kept.
- **Drawback**: gating commands on subsystem availability complicates the registry.
  **Alternative**: always register, error at runtime.
  **Decision**: INV-3 forbids advertising non-existent commands; the `availableWhen` predicate keeps help truthful. Kept.

## Canonical References

<!-- MANDATORY for Stable status. Stub — populate when implementation lands
     (Phase 6 Track F). Blocked until: (1) l1-cli-tooling Stable; (2) router +
     doctor + plugin subcommands implemented with integration tests (T-6F01..03,
     T-6T03). cmd/cli currently a Phase-0 bootstrap stub. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-30 | Initial L2 draft — Go translation of l1-cli-tooling v0.2.0. stdlib-`flag` `Router` + `Command` registry with availability gating (INV-3), no-arg structured help (INV-2), overwrite-safe scaffolding (INV-1), `--json` stable-key output via `Output` sink (INV-4), `ecs plugin …` subcommands fronting the `pkg/plugin` SDK, `codegen` front-end. Authored ahead of Phase 6 Track F (`/magic.spec`). Draft — L1 parent Draft + cmd/cli is a Phase-0 stub. |
