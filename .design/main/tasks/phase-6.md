---
phase: 6
name: "UI, Tooling & Quality"
status: Hold
subsystem: "pkg/ui, pkg/window, pkg/diag, cmd/cli, pkg/build, pkg/codegen, pkg/platform, pkg/plugin, pkg/plugins/aiapi, pkg/assistant, pkg/errs, pkg/definition, pkg/visualgraph, pkg/editor"
requires:
  - "Phase 1–3 Stable"
provides:
  - "Declarative definition layer (JSON / templates)"
  - "Visual programming node graph + external editor interfaces"
  - "Window + multi-window + platform abstraction"
  - "Diagnostics + profiling overlay + gizmos + error codes"
  - "UI layout, interaction, text, widgets, styling"
  - "Build pipeline + CI + golden testing"
  - "CLI scaffolding + asset management commands"
  - "Platform tier matrix + capabilities + build tags"
  - "AI assistant plugin protocol"
  - "Third-party plugin distribution: manifest, in/out-of-process, capability sandbox, public SDK"
  - "First-party AI API plugin (pkg/plugins/aiapi/) covering OpenAI/Anthropic/Gemini/local providers"
  - "Examples framework + lifecycle"
  - "Compatibility policy + Go toolchain matrix"
  - "Structured error taxonomy (E-series)"
  - "Standardized benchmark suite + regression CI gates"
  - "Codegen: query wrappers, boilerplate"
key_files:
  created: []
  modified: []
patterns_established: []
duration_minutes: ~
bootstrap: true
hold_reason: "Unfreezes after Phase 1–3 Stable."
---

# Stage 6 Tasks — UI, Tooling & Quality

**Phase:** 6
**Status:** Hold
**Strategic Goal:** Developer-experience surface — UI, CLI, build, diagnostics, AI assistant boundary, third-party plugin ecosystem, codegen, errors. Closes the loop between engine internals (Phases 1–3) and the editor/tooling consumer.

## Track Overview

| Track | Domain | Spec | Tasks |
| :--- | :--- | :--- | :--- |
| A | Definition System (`pkg/definition/`) | l1-definition-system | T-6A01..03 |
| B | Window System (`pkg/window/`) | l1-window-system | T-6B01..02 |
| C | Diagnostic System (`pkg/diag/`) | l1-diagnostic-system | T-6C01..03 |
| D | UI System (`pkg/ui/`) | l1-ui-system | T-6D01..03 |
| E | Build Tooling (`.github/`, `scripts/`) | l1-build-tooling | T-6E01..03 |
| F | CLI Tooling (`cmd/cli/`) | l1-cli-tooling | T-6F01..03 |
| G | Platform System (`pkg/platform/`) | l1-platform-system | T-6G01..02 |
| H | AI Assistant System (`pkg/assistant/`) | l1-ai-assistant-system | T-6H01..03 |
| I | Examples Framework (`examples/`) | l1-examples-framework | T-6I01..02 |
| J | Compatibility Policy | l1-compatibility-policy | T-6J01..02 |
| K | Error Core (`pkg/errs/`) | l1-error-core | T-6K01..03 |
| L | Benchmark Spec (`bench/`) | l2-benchmark-spec | T-6L01..02 |
| M | Codegen Tools (`cmd/codegen/`) | l2-codegen-tools | T-6M01..02 |
| **N** | **Plugin Distribution (`pkg/plugin/`)** | **l1-plugin-distribution** | **T-6N01..04** |
| **O** | **AI API Plugin (`pkg/plugins/aiapi/`)** | **l1-ai-api-plugin** | **T-6O01..05** |
| **P** | **Visual Graph System (`pkg/visualgraph/`, `pkg/editor/`)** | **l1-visual-graph-system** | **T-6P01..04** |
| T | Validation (cross-track) | — | T-6T01..05 |

**Hard dependencies inside Phase 6:**

- Track N (Plugin Distribution) → Track F (CLI for `ecs plugin …`), Track K (E-PLUGIN error codes), Track J (`engine_version` SemVer parser).
- Track O (AI API Plugin) → Track N (delivery + capability gating), Track H (AI Assistant protocol), Track C (diagnostics + cost metrics), Track K (E-PLUGIN-AIAPI codes).
- Track H (AI Assistant) → Track A (definitions for `generate_ui`/`generate_scene`), Track C (diagnostics).

**External dependencies (cross-phase):**

- Track O → Phase 3 Task System (HTTP off main loop), Phase 1 Event System (streaming events), Phase 1 Type Registry (component schema for `suggest_components`).
- Track N → Phase 2 App Framework (Plugin trait re-export from `pkg/plugin/`), Phase 2 Multi-Repo Architecture (pkg/ boundary contract).
- Track P → Phase 1 Type Registry (auto-registration of node definitions), Phase 1 Event System.

## Atomic Checklist

### Track A — Definition System

- [ ] [T-6A01] JSON schema parser + loader for UI/scene/flow/template definitions; AST + validation. — `pkg/definition/{schema,loader,ast}.go` `[Bootstrap]`
- [ ] [T-6A02] Template engine: variable substitution, includes, conditional sections. — `pkg/definition/template/` `[Bootstrap]`
- [ ] [T-6A03] Hot-reload watcher integration (asset server hook) + diff applier. — `pkg/definition/watch.go` `[Bootstrap]`

### Track B — Window System

- [ ] [T-6B01] Window abstraction interface + multi-window registry + WindowID lifecycle. — `pkg/window/{window,registry}.go` `[Bootstrap]`
- [ ] [T-6B02] Platform backend selection (build tags) + headless backend for tests. — `pkg/window/backend_*.go` `[Bootstrap]`

### Track C — Diagnostic System

- [ ] [T-6C01] Metric registry: counters, gauges, histograms, label sets; lock-free hot path. — `pkg/diag/{metric,registry}.go` `[Bootstrap]`
- [ ] [T-6C02] Profiling overlay + frame stats (delegates to UI system); pprof export endpoints. — `pkg/diag/{overlay,pprof}.go` `[Bootstrap]`
- [ ] [T-6C03] Gizmos API + error code surface for ad-hoc visual debug. — `pkg/diag/{gizmos,errors}.go` `[Bootstrap]`

### Track D — UI System

- [ ] [T-6D01] Layout engine (flex/grid) + measure/arrange pass. — `pkg/ui/layout/{flex,grid,box}.go` `[Bootstrap]`
- [ ] [T-6D02] Widget primitives: button, text, image, container, scroll. — `pkg/ui/widgets/` `[Bootstrap]`
- [ ] [T-6D03] Interaction handler (input routing) + style system + theming tokens. — `pkg/ui/{interaction,style}.go` `[Bootstrap]`

### Track E — Build Tooling

- [ ] [T-6E01] CI workflows: vet/lint/race/coverage gates; matrix over OS+Go versions per Track J. — `.github/workflows/ci.yml` `[Bootstrap]`
- [ ] [T-6E02] Benchmark regression CI gate: parse `-benchmem` output, compare to baseline JSON, fail on >5% drift. — `scripts/bench-gate/` `[Bootstrap]`
- [ ] [T-6E03] Migration/release doc generators: changelog from spec `Document History`, breaking-change report. — `scripts/release/` `[Bootstrap]`

### Track F — CLI Tooling

- [ ] [T-6F01] CLI shell: command dispatch, global flags, structured logging. — `cmd/cli/main.go`, `cmd/cli/root.go` `[Bootstrap]`
- [ ] [T-6F02] Scaffolding subcommands: `ecs new project|component|system|plugin`. — `cmd/cli/scaffold/` `[Bootstrap]`
- [ ] [T-6F03] Asset + plugin management subcommands: `ecs asset import|list|build`, `ecs plugin scaffold|validate|install|list|enable|disable|info|remove|doctor` (consumed by Track N). — `cmd/cli/{asset,plugin}/` `[Bootstrap]`

### Track G — Platform System

- [ ] [T-6G01] Capability registry + tier matrix (CPU features, GPU APIs, audio backends). — `pkg/platform/{capability,tier}.go` `[Bootstrap]`
- [ ] [T-6G02] Build-tag conventions + per-platform backend selection helpers; `//go:build editor` enforcement test. — `pkg/platform/build/` `[Bootstrap]`

### Track H — AI Assistant System

- [ ] [T-6H01] AssistantManager resource + AgentConnection registry + capability set persistence. — `pkg/assistant/{manager,agent}.go` `[Bootstrap]`
- [ ] [T-6H02] Transport implementations: stdio (subprocess), websocket (long-lived), http (request/response). — `pkg/assistant/transport/{stdio,websocket,http}.go` `[Bootstrap]`
- [ ] [T-6H03] Standard method dispatch (`chat`, `suggest_components`, `generate_scene`, `generate_ui`, `explain_entity`, `diagnose_issue`, `autocomplete`, `generate_code`); request tagging + undo grouping. — `pkg/assistant/methods/` `[Bootstrap]`

### Track I — Examples Framework

- [ ] [T-6I01] Example directory conventions: per-example `manifest.toml`, README, expected golden output. — `examples/README.md`, `examples/_template/` `[Bootstrap]`
- [ ] [T-6I02] Example lifecycle hooks + CI selective build (only changed examples). — `scripts/examples/` `[Bootstrap]`

### Track J — Compatibility Policy

- [ ] [T-6J01] Engine SemVer policy doc + Go-toolchain compatibility matrix; `engine_version` constraint parser (consumed by Track N). — `pkg/version/{semver,constraint}.go` `[Bootstrap]`
- [ ] [T-6J02] Compatibility test harness: snapshot of `pkg/` public surface; CI fails on undocumented breaking change. — `scripts/api-diff/` `[Bootstrap]`

### Track K — Error Core

- [ ] [T-6K01] E-series code registry + structured `Error` type (code, message, severity, fields, wrapped); chain via `errors.Is/As`. — `pkg/errs/{error,registry,code}.go` `[Bootstrap]`
- [ ] [T-6K02] Localization hooks (message templates per locale) + severity levels (Info/Warn/Error/Fatal). — `pkg/errs/{i18n,severity}.go` `[Bootstrap]`
- [ ] [T-6K03] Error formatter + redaction filter (used by Track O for API key redaction); structured-log adapter. — `pkg/errs/{format,redact}.go` `[Bootstrap]`

### Track L — Benchmark Spec

- [ ] [T-6L01] Benchmark suite structure: per-subsystem `bench/{subsystem}/` packages, comparison harness, CSV/JSON output. — `bench/`, `cmd/benchcompare/` `[Bootstrap]`
- [ ] [T-6L02] Baseline JSON format + CI drift gate (consumed by T-6E02). — `bench/baseline.json`, `scripts/bench-gate/` `[Bootstrap]`

### Track M — Codegen Tools

- [ ] [T-6M01] Query wrapper generator: typed `Query[N]` helpers from component declarations. — `cmd/codegen/query/` `[Bootstrap]`
- [ ] [T-6M02] Boilerplate generator: component registration stubs, plugin scaffolds (consumed by `ecs new plugin`). — `cmd/codegen/{component,plugin}/` `[Bootstrap]`

### Track N — Plugin Distribution (NEW)

- [ ] [T-6N01] Public SDK surface: re-export `Plugin`/`PluginGroup` from app framework; introduce `PluginContext`, `Capability`, `Manifest`, `CommandIssuer`, scoped logger. — `pkg/plugin/{plugin,manifest,capability,context,command,event,query,log,errors}.go` `[Bootstrap]`
- [ ] [T-6N02] Manifest schema (TOML) + parser + validator; in-process and out-of-process variants; `engine_version` constraint check via Track J. — `pkg/plugin/manifest.go`, `cmd/cli/plugin/validate.go` `[Bootstrap]`
- [ ] [T-6N03] In-process loader pipeline: discovery (4 sources per spec §4.4), compatibility resolution, capability prompt + persistence, lifecycle wiring (Build/Ready/Finish/Cleanup) with capability-enforcing proxy. — `internal/plugin/loader/` `[Bootstrap]`
- [ ] [T-6N04] Out-of-process loader: subprocess spawn (cwd-restricted), transport handshake (reuses Track H transports), host-side proxy `Plugin` translating lifecycle + commands + queries, failure isolation per INV-8 (graceful degrade, no host crash). — `internal/plugin/oop/` `[Bootstrap]`

### Track O — AI API Plugin (NEW)

- [ ] [T-6O01] Package skeleton + embedded manifest + lifecycle: `New()` factory, Build/Ready/Finish/Cleanup; config struct + JSON schema export; ServiceRegistry registration. — `pkg/plugins/aiapi/{plugin,config,manifest}.go`, `pkg/plugins/aiapi/plugin.toml` `[Bootstrap]`
- [ ] [T-6O02] Provider abstraction + canonical request/response types + four providers (OpenAI, Anthropic, Gemini, local OpenAI-compatible); request-build and response-parse golden tests per provider. — `pkg/plugins/aiapi/{provider,canonical,provider_*}.go` `[Bootstrap]`
- [ ] [T-6O03] Method dispatch (8 standard methods) + streaming chat via SSE + cancellation map (per-request `context.CancelFunc`); event emission for chunks; rate limiter (RPM+TPM token bucket per provider). — `pkg/plugins/aiapi/{methods/,stream.go,ratelimit.go}` `[Bootstrap]`
- [ ] [T-6O04] Credentials (env / OS keyring / age-encrypted file) + redaction writer + diagnostics (latency, token count, cost USD) + cost-budget event; error mapping to E-PLUGIN-AIAPI-{NNN} via Track K. — `pkg/plugins/aiapi/{credentials,redact,diag,errors}.go` `[Bootstrap]`
- [ ] [T-6O05] Mode-parity test harness: identical integration suite runs in-process AND out-of-process via Track N OOP loader; FakeProvider for deterministic CI; `-race` clean across both modes (INV-7). — `pkg/plugins/aiapi/testing/`, `internal/plugin/testbench/` `[Bootstrap]`

### Track P — Visual Graph System (NEW)

- [ ] [T-6P01] Graph Data Model: `GraphDefinition`, `Node`, `Pin`, `Connection` structs; integration with Definition System. — `pkg/visualgraph/{model,definition}.go` `[Bootstrap]`
- [ ] [T-6P02] Node Registry: Automatic generation of nodes from TypeRegistry components/events/states. — `pkg/visualgraph/registry.go` `[Bootstrap]`
- [ ] [T-6P03] Graph Interpreter: Execution engine (imperative chain + lazy data evaluation); cyclic dependency detection; context passing. — `pkg/visualgraph/{interpreter,execution}.go` `[Bootstrap]`
- [ ] [T-6P04] Editor Gateway: `GraphEditorPlugin`, `NodeRegistryQuery`, `GraphDebugger` interface implementations for external editor (`editor`) integration via `pkg/editor/`. — `pkg/editor/graph.go`, `pkg/visualgraph/debug.go` `[Bootstrap]`

### Track T — Validation

- [ ] [T-6T01] Plugin SDK contract tests: manifest schema fuzz, capability enforcement (denial paths), in-process lifecycle proxy, in/out-of-process behavioural parity. — `pkg/plugin/contract_test.go`, `internal/plugin/testbench/` `[Bootstrap]`
- [ ] [T-6T02] AI API plugin parity matrix: every standard method exercised in both modes; identical canonical responses asserted. — `pkg/plugins/aiapi/parity_test.go` `[Bootstrap]`
- [ ] [T-6T03] CLI integration tests: `ecs plugin scaffold|validate|install|list|enable|disable|info|remove|doctor` golden output. — `cmd/cli/plugin/integration_test.go` `[Bootstrap]`
- [ ] [T-6T04] Codegen golden output + benchmark regression CI gate live (T-6E02 + T-6L02 wired). — `cmd/codegen/golden/`, `.github/workflows/bench.yml` `[Bootstrap]`
- [ ] [T-6T05] AI API plugin live-provider smoke test (gated by `live-ai` CI label, project-secret API keys); explicit cost budget. — `.github/workflows/ai-live.yml` `[Bootstrap]`

## Detailed Tracking

### [T-6N01] Public SDK surface

- **Spec:** [l1-plugin-distribution.md](../specifications/l1-plugin-distribution.md) §4.7
- **Status:** Todo `[Bootstrap]`
- **Handoff:** Required by T-6N02..04 (loader implementations), T-6O01 (AI API Plugin imports SDK), T-6F03 (CLI plugin subcommands), T-6M02 (plugin scaffold generator).
- **Notes:** Re-export only — no reimplementation of `Plugin`/`PluginGroup`. Strict rule: nothing in `internal/` may be imported by code under `pkg/plugin/`. Snapshot of public surface goes through Track J's API-diff gate from day one.

### [T-6N03] In-process loader

- **Spec:** [l1-plugin-distribution.md](../specifications/l1-plugin-distribution.md) §§4.4–4.6, §4.12
- **Status:** Todo `[Bootstrap]`
- **Handoff:** Required by T-6O01 (plugin under test), T-6T01 (contract tests).
- **Notes:** Capability proxy mediates calls to engine API only — does NOT sandbox arbitrary Go code (per spec §4.12). Manifest checksum recorded at install; mismatch on next load demotes plugin to `Discovered` and re-prompts.

### [T-6N04] Out-of-process loader

- **Spec:** [l1-plugin-distribution.md](../specifications/l1-plugin-distribution.md) §4.6 (out-of-process branch), INV-8
- **Status:** Todo `[Bootstrap]`
- **Handoff:** Required by T-6O05 (parity harness), T-6T01 (parity contract tests).
- **Notes:** Uses Track H transports (stdio/websocket/http) — share infrastructure, do not reimplement. Failure isolation: subprocess crash MUST mark plugin `Failed` and continue host engine. Resource limits (cgroups/JobObjects) optional in v1.

### [T-6O02] Provider abstraction + four providers

- **Spec:** [l1-ai-api-plugin.md](../specifications/l1-ai-api-plugin.md) §4.4
- **Status:** Todo `[Bootstrap]`
- **Handoff:** Unblocks T-6O03 (method dispatch consumes providers), T-6O05 (parity harness).
- **Notes:** Canonical types are the single boundary — only `provider_*.go` files know wire format. Adding a 5th provider must touch exactly one new file plus its registration. Golden fixtures: redact API keys before commit; CI verifies redaction.

### [T-6O05] Mode-parity test harness

- **Spec:** [l1-ai-api-plugin.md](../specifications/l1-ai-api-plugin.md) INV-7
- **Status:** Todo `[Bootstrap]`
- **Handoff:** Closes Track O. Required by Phase 6 exit criteria.
- **Notes:** This is the **anchor task** for INV-7. If parity diverges by even one byte in canonical responses across modes, parity test fails. FakeProvider scripts canned responses; same script runs through in-process plugin and through out-of-process binary launched by Track N loader.

### [T-6T05] Live-provider smoke

- **Spec:** [l1-ai-api-plugin.md](../specifications/l1-ai-api-plugin.md) §4.9
- **Status:** Todo `[Bootstrap]`
- **Notes:** Gated by `live-ai` CI label. Default CI does NOT run this. Project secrets injected at job level. Cost budget caps spend per run.

### [T-6P04] Editor Gateway

- **Spec:** [l2-visual-graph-editor-bridge.md](../specifications/l2-visual-graph-editor-bridge.md) §4.1–§4.4 (extracted from l1-visual-graph-system §4.7–§4.8)
- **Status:** Todo `[Bootstrap]`
- **Handoff:** Closes Track P. Blocks multi-repo external editor integration logic.
- **Notes:** Must provide JSON/Protobuf-serializable boundary across `pkg/editor/`.

## Validation Strategy

- **Per-track local tests** (table-driven, `_test.go`) land alongside each implementation task; minimum 80% coverage per RULES.md C24/C28.
- **Cross-track integration** is gated by Track T (`T-6T*`).
- **Plugin SDK API stability**: every PR touching `pkg/plugin/` runs Track J's API-diff gate (T-6J02).
- **CI Gates** (mandatory before phase Done):
  - `go vet ./...`, `golangci-lint run`
  - `go test -race ./...`
  - `go test -bench=. -benchmem ./bench/...` with regression check (T-6E02)
  - Plugin contract suite (T-6T01) runs both in-process and out-of-process modes
  - AI API parity matrix (T-6T02) green in both modes

## Exit Criteria

Phase 6 is `Done` when **all** of:

1. Every atomic task above is `[x]`.
2. CI gates green on `master` (vet, lint, race, bench, contract, parity).
3. `examples/plugin/distribution/` and `examples/plugin/aiapi/` validate end-to-end (C29 unblock for `l1-plugin-distribution` + `l1-ai-api-plugin`).
4. `examples/ecs/poc/` visual graph validation POC verifies execution (C29 unblock for `l1-visual-graph-system`).
5. `magic.spec` promotes the Phase 6 spec cohort `Draft → Stable` (C29 unblocked).
6. STATE.md `Phase` advances to `7 — Networking & Hot-Reload` and `Status: Active` (subject to Phase Gate C9).

## Open Coordination Items

- **Track N ↔ Phase 2**: `pkg/plugin/` re-exports types from Phase 2's App Framework. T-6N01 cannot finalize until App Framework public types are fixed (Phase 2 stable).
- **Track O ↔ Phase 3**: HTTP off-loop relies on Phase 3 Task System. Schedule Track O after Phase 3 Stable.
- **Cross-workspace impact**: when `pkg/protocol/` (multi-repo split) is finalized, T-6H02 transports may move to a shared package — coordinate via `l1-multi-repo-architecture`.
