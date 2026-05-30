---
phase: 6
name: "UI, Tooling & Quality"
status: Active
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
hold_reason: ""
active_cohort: "engine-core (A/B/C/D/G/K) — definition, window, diagnostic, ui, platform, error-core. All 6 L2 contracts authored (Draft [Bootstrap]). Deferred within this Active phase: tooling (E/F/I/J/L/M) + editor-layer (H/N/O/P) tracks."
---

# Stage 6 Tasks — UI, Tooling & Quality

**Phase:** 6
**Status:** Active (engine-core cohort)
**Strategic Goal:** Developer-experience surface — UI, CLI, build, diagnostics, AI assistant boundary, third-party plugin ecosystem, codegen, errors. Closes the loop between engine internals (Phases 1–3) and the editor/tooling consumer.

## Active Cohort (this activation, 2026-05-30)

Per user direction, **only the engine-core tracks are active this pass**: **A** (definition), **B** (window), **C** (diagnostic), **D** (ui), **G** (platform), **K** (error-core). Each has an authored L2 Go contract (`Draft [Bootstrap]`) that is the **authoritative design reference** — `/magic.run` MUST read the L2 spec's Type Definitions / Invariant Compliance / Testing Strategy before implementing a task (the task lines below are summaries; the L2 contract governs file layout + invariants).

**Build order (intra-cohort hard deps):** `{K error-core ‖ G platform}` → `{B window, C diagnostic}` → `D ui` → `A definition`.

**Deferred within this Active phase** (not yet started): tooling **E/F/I/J/L/M** + editor-layer **H/N/O/P** (the editor-layer L2 contracts are unblocked by multi-repo Stable but not yet authored). Validation **T** tasks track the editor/tooling tracks and stay deferred; an engine-core C29 validation track (`examples/{config,window,diagnostic,ui}/`) is added as **T-6T06** below.

**Skeptic flags:** (1) Track D (ui) at 3 tasks is optimistic for a flexbox solver + widgets + interaction + font atlas — expect a split during execution. (2) T-6C02 (diagnostic overlay) soft-depends on Track D (overlay renders via UI text); keep the overlay last in Track C so store/gizmos land first. (3) Track D C29 closure needs the `x/image/font` ADR (flagged in `l2-ui-system-go`).

## Track Overview

| Track | Domain | Spec | Tasks |
| :--- | :--- | :--- | :--- |
| A | Definition System (`pkg/definition/`) | l1-definition-system + l2-definition-system-go | T-6A01..03 |
| B | Window System (`pkg/window/`) | l1-window-system + l2-window-system-go | T-6B01..02 |
| C | Diagnostic System (`pkg/diag/`) | l1-diagnostic-system + l2-diagnostic-system-go | T-6C01..03 |
| D | UI System (`pkg/ui/`) | l1-ui-system + l2-ui-system-go | T-6D01..03 |
| E | Build Tooling (`.github/`, `scripts/`) | l1-build-tooling | T-6E01..03 |
| F | CLI Tooling (`cmd/cli/`) | l1-cli-tooling | T-6F01..03 |
| G | Platform System (`pkg/platform/`) | l1-platform-system + l2-platform-system-go | T-6G01..02 |
| H | AI Assistant System (`pkg/assistant/`) | l1-ai-assistant-system | T-6H01..03 |
| I | Examples Framework (`examples/`) | l1-examples-framework | T-6I01..02 |
| J | Compatibility Policy | l1-compatibility-policy | T-6J01..02 |
| K | Error Core (`pkg/errs/`) | l1-error-core + l2-error-core-go | T-6K01..03 |
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

> **L2 contract (authoritative):** [l2-definition-system-go.md](../specifications/l2-definition-system-go.md). Reconciled 2026-05-30 to the contract: `template` is an entity-blueprint (prefab), not text-substitution (expressions are a deferred open question).

- [ ] [T-6A01] `Envelope` decode + `Kind` dispatch + `DefinitionLoader` (decode → validate → typed asset); total validation = structure + `TypeRegistry` type-refs (INV-4) + include-DAG 3-colour DFS (INV-5) → instantiation infallible (INV-1). — `pkg/definition/{envelope,errors}.go`, `internal/definition/{loader,validate}.go` `[Bootstrap]`
- [ ] [T-6A02] Four `Command`-only interpreters (INV-2): `ui` → Node/Style trees, `scene` → DynamicScene codec reuse, `flow` → `NextState` + on_enter/exit, `template` → prefab + field overrides; extensible `ActionRegistry`. — `pkg/definition/{ui,scene,flow,template,action}.go`, `internal/definition/interp_*.go` `[Bootstrap]`
- [ ] [T-6A03] Hot-reload (INV-3): `AssetEvent[Definition]::Modified` → despawn `DefinitionInstance`-tagged entities + re-instantiate; flow preserves current state if it still exists. — `internal/definition/hotreload.go` `[Bootstrap]`
- **Verify:** validated definition always instantiates (property test); cycle ⇒ `ErrDefinitionCycle`; unknown type ⇒ `ErrUnknownType`; scene round-trips byte-stable via the codec; no-panic fuzz on garbage bytes.

### Track B — Window System

> **L2 contract (authoritative):** [l2-window-system-go.md](../specifications/l2-window-system-go.md). Depends on Track G (platform selects the `WindowBackend`).

- [ ] [T-6B01] `Window` component + `PrimaryWindow` marker + `WindowBackend` interface; diff-driven sync via change-detection ticks (INV-4); deferred create/destroy via commands (INV-3); `PrimaryWindowRes` single-primary assertion (INV-1) + `AppExit` on primary close (INV-2). — `pkg/window/{window,mode,cursor,markers,backend,plugin}.go`, `internal/window/{sync,primary}.go` `[Bootstrap]`
- [ ] [T-6B02] Headless `WindowBackend` (no OS windows, deterministic event queue for CI) + `MainThreadExecutor`-bound backend calls + `PollEvents` → engine events; native backend selected by build tags via the platform plugin. — `internal/window/{headless,poll}.go`, `pkg/window/events.go` `[Bootstrap]`
- **Verify:** two primaries ⇒ `ErrMultiplePrimary`; primary close ⇒ exactly one `AppExit` (OnPrimaryClosed); `SpawnWindow` emits no backend call until command flush; scripted PlatformEvent queue replays identically ×20.

### Track C — Diagnostic System

> **L2 contract (authoritative):** [l2-diagnostic-system-go.md](../specifications/l2-diagnostic-system-go.md). **Reconciled 2026-05-30** — the prior "counters/gauges/histograms" Prometheus model was dropped: the L1/L2 design is a `DiagnosticsStore` of named diagnostics over fixed-cap `RingBuffer`s. Error codes are **not** redefined here — they defer to `l2-error-core-go` (Track K).

- [ ] [T-6C01] `DiagnosticsStore` + named `Diagnostic` over fixed-capacity `RingBuffer` (0-alloc `Push`, deterministic sample-count averages); `hasAnyReader` run-condition for zero-cost-when-unread (INV-1); built-in metrics (fps/frame_time/entity_count) in the `Last` schedule (INV-3). — `pkg/diag/{store,diagnostic}.go`, `internal/diag/builtins.go` `[Bootstrap]`
- [ ] [T-6C02] Immediate-mode `Gizmos` + pooled per-frame vertex buffer drained by a `gizmoFeature` (`RenderFeature`) after scene, before UI (INV-2, pure visual); `GizmoConfigStore` + retained gizmos. — `pkg/diag/gizmos.go`, `internal/diag/gizmofeature.go` `[Bootstrap]`
- [ ] [T-6C03] `log/slog` per-module level filter; build-tag `profiling` spans with no-op release path (INV-4); debug overlay via the UI text path (**soft-deps Track D — sequence last**). — `pkg/diag/{log,span,span_noop}.go`, `internal/diag/overlay.go` `[Bootstrap]`
- **Verify:** zero readers ⇒ collection pass skipped + `Push` 0-alloc (benchmark); `RingBuffer` wrap-around + deterministic `Average`; gizmo draw performs no world mutation + clears each frame; `-tags profiling` records spans, default build = 0-cost no-op.

### Track D — UI System

> **L2 contract (authoritative):** [l2-ui-system-go.md](../specifications/l2-ui-system-go.md). Depends on Track B (window viewport) + Stable render/input/2d/hierarchy. **C29 closure needs the `x/image/font` ADR** (text shaping). Skeptic: 3 tasks is tight — expect a split.

- [ ] [T-6D01] `Style` component + `Val` union + flexbox solver (measure → layout) with **dirty-subtree relayout** via `Changed[Style]` (INV-1); cached minSize with upward dirty propagation + deferred child sort. — `pkg/ui/{style,node}.go`, `internal/ui/{layout,dirty}.go` `[Bootstrap]`
- [ ] [T-6D02] Widget bundles (Node/Text/ImageNode/Button/ScrollView) + visual styling (BackgroundColor/Border/Gradient); `FontAtlas` glyph cache over the render-core shelf-pack `DynamicAtlas` (INV-4) + text shaping. — `pkg/ui/{widgets,visual,font}.go`, `internal/ui/{atlas,shape}.go` `[Bootstrap]`
- [ ] [T-6D03] `Interaction` hit-test in `PreUpdate` + `MouseFilter` + event bubbling + focus neighbors (INV-3); `UiFeature` compositing UI last via `RenderFeature` + `ZIndex` resolve (INV-2); render-only `OffsetTransform`. — `pkg/ui/{interaction,transform}.go`, `internal/ui/{interaction,feature}.go` `[Bootstrap]`
- **Verify:** mutating one node re-solves only its subtree (sibling rects unchanged); same string renders glyphs once (atlas count stable on 2nd pass); top-most `MouseStop` consumes, `MousePass` propagates; batched draw topology hash stable ×20.

### Track E — Build Tooling

- [ ] [T-6E01] CI workflows: vet/lint/race/coverage gates; matrix over OS+Go versions per Track J. — `.github/workflows/ci.yml` `[Bootstrap]`
- [ ] [T-6E02] Benchmark regression CI gate: parse `-benchmem` output, compare to baseline JSON, fail on >5% drift. — `scripts/bench-gate/` `[Bootstrap]`
- [ ] [T-6E03] Migration/release doc generators: changelog from spec `Document History`, breaking-change report. — `scripts/release/` `[Bootstrap]`

### Track F — CLI Tooling

- [ ] [T-6F01] CLI shell: command dispatch, global flags, structured logging. — `cmd/cli/main.go`, `cmd/cli/root.go` `[Bootstrap]`
- [ ] [T-6F02] Scaffolding subcommands: `ecs new project|component|system|plugin`. — `cmd/cli/scaffold/` `[Bootstrap]`
- [ ] [T-6F03] Asset + plugin management subcommands: `ecs asset import|list|build`, `ecs plugin scaffold|validate|install|list|enable|disable|info|remove|doctor` (consumed by Track N). — `cmd/cli/{asset,plugin}/` `[Bootstrap]`

### Track G — Platform System

> **L2 contract (authoritative):** [l2-platform-system-go.md](../specifications/l2-platform-system-go.md). Cohort root (no intra-cohort deps); selects the Track B `WindowBackend`.

- [x] [T-6G01] Immutable `PlatformProfile` resource + `PlatformCaps` branchless bitfield + `PlatformOS`/`Arch`/`Tier` enums; detection from `runtime.GOOS`/`GOARCH`; inserted in `PreStartup`, read-only thereafter (INV-2). — `pkg/platform/{profile,caps,plugin}.go`, `internal/platform/detect.go` `[Bootstrap]`
- [x] [T-6G02] Build-tag-split profile (`profile_default.go` `!headless` / `profile_headless.go` `headless`) — headless caps exclude `HasGPU`/`HasMultiWindow`/`HasSpatialAudio` (INV-4); `Plugin` inserts the profile resource (INV-3); pure `osFromGOOS`/`archFromGOARCH` mappings. `//go:build editor` scope is enforced by the existing `pkg/editor` guard tests (multi-repo), not duplicated. — `internal/platform/profile_{default,headless}.go`, `internal/platform/plugin.go` `[Bootstrap]`
- **Verify:** `Has`/`With` bitfield table (disjoint flags don't alias); `-tags headless` profile has `HasGPU/HasMultiWindow/HasSpatialAudio == false`; cross-compile smoke (`GOOS=windows/linux/darwin`, `js/wasm`) builds the core.
- **Done (2026-05-30):** `pkg/platform` (pure types: `PlatformProfile`, `PlatformCaps` bitfield, OS/Arch/Tier total-switch enums, `PlatformPlugin` iface) + `internal/platform` (GOOS/GOARCH pure mappings, `!headless`/`headless` profile split, profile-inserting `Plugin`). **pkg/platform 100%, internal/platform 100% coverage in both default and `-tags headless` builds**; headless build verified. Build + modernize clean.

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

> **L2 contract (authoritative):** [l2-error-core-go.md](../specifications/l2-error-core-go.md). Cohort root (no intra-cohort deps); consumed by Track C (diagnostic codes) and the deferred editor/AI tracks.

- [x] [T-6K01] `EngineError` interface over stdlib `errors` (Is/As/Unwrap) + range-partitioned `Code` registry (duplicate ⇒ `ErrDuplicateCode`, out-of-range ⇒ `ErrCodeOutOfRange`) + `Severity`/`Audience` total-switch enums. — `pkg/errs/{error,severity,code}.go` `[Bootstrap]`
- [x] [T-6K02] `fs.FS` locale catalog (`errors.{lang}.json`) + `Localize` with default-template fallback (missing key never empty); malformed catalog keeps embedded defaults. — `pkg/errs/catalog.go`, `locales/errors.en.json` `[Bootstrap]`
- [x] [T-6K03] Build-tag debug traces (`runtime.Callers`; `!debug` = no-op, INV trace) + `MustSucceed` (panics **only** for Fatal+Developer) + structured-log adapter; redaction filter (consumed later by Track O). — `pkg/errs/{trace_debug,trace_release,redact}.go` `[Bootstrap]`
- **Verify:** `errors.As` recovers `EngineError` through a `%w` chain; severity `String()` total over all enum values; duplicate/out-of-range registration guarded; `Localize` falls back on missing key; `MustSucceed` panics only for Fatal+Developer.
- **Done (2026-05-30):** `pkg/errs` implemented — `EngineError`/`engineError`, `Code` registry with module-range + duplicate guards, `Severity`/`Audience` total-switch enums, `Catalog` over `fs.FS` (fmt-style templates + bare-code fallback), `//go:build debug` trace split, `MustSucceed` Fatal+Developer panic policy, `Redactor`. `locales/errors.en.json` shipped. **89.3% coverage**, build + modernize clean.

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
- [ ] [T-6T06] **Engine-core C29 validation track** (this activation's gate): `examples/{config,window,diagnostic,ui}/` validate the 6 engine-core L2 contracts end-to-end (definition load+instantiate, headless window event replay, diagnostics 0-alloc + gizmo topology, flexbox + atlas determinism); hash-stable ×20; `go test ./...` + `modernize` clean. Closes the C29 gate that promotes the engine-core cohort's L1+L2 Draft → Stable. — `examples/{config,window,diagnostic,ui}/` `[Bootstrap]`

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
