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
active_cohort: "engine-core (A/B/C/D/G/K) — definition, window, diagnostic, ui, platform, error-core. COMPLETE 17/17 (2026-05-30); C29 P6 gate closed by T-6T06. Deferred within this Active phase: tooling (E/F/I/J/L/M) + editor-layer (H/N/O/P) tracks."
cohort_key_files:
  created:
    - "pkg/errs/{error,severity,code,catalog,trace_debug,trace_release,redact}.go + locales/errors.en.json"
    - "pkg/platform/{caps,profile,plugin}.go + internal/platform/{detect,profile_default,profile_headless,plugin}.go"
    - "pkg/diag/{diagnostic,store,gizmos,log,span,span_noop,mathutil}.go + internal/diag/gizmofeature.go"
    - "pkg/window/{window,backend,events}.go + internal/window/{headless,diff,primary}.go"
    - "pkg/ui/{style,node,interaction,widgets}.go + internal/ui/{layout,atlas,interaction,feature}.go"
    - "pkg/definition/{envelope,content,errors,contracts,loader,validate,interp}.go"
    - "examples/{config,window,diagnostic,ui}/"
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
| A | Definition System (`pkg/definition/`) | l1-definition-system + l2-definition-system-go (+ l2-definition-integration-go: editor surface) | T-6A01..03 |
| B | Window System (`pkg/window/`) | l1-window-system + l2-window-system-go | T-6B01..02 |
| C | Diagnostic System (`pkg/diag/`) | l1-diagnostic-system + l2-diagnostic-system-go | T-6C01..03 |
| D | UI System (`pkg/ui/`) | l1-ui-system + l2-ui-system-go | T-6D01..03 |
| E | Build Tooling (`.github/`, `scripts/`) | l1-build-tooling | T-6E01..03 |
| F | CLI Tooling (`cmd/cli/`) | l1-cli-tooling + l2-cli-tooling-go | T-6F01..03 |
| G | Platform System (`pkg/platform/`) | l1-platform-system + l2-platform-system-go | T-6G01..02 |
| H | AI Assistant System (`pkg/assistant/`) | l1-ai-assistant-system + l2-ai-assistant-system-go | T-6H01..03 |
| I | Examples Framework (`examples/`) | l1-examples-framework | T-6I01..02 |
| J | Compatibility Policy | l1-compatibility-policy | T-6J01..02 |
| K | Error Core (`pkg/errs/`) | l1-error-core + l2-error-core-go | T-6K01..03 |
| L | Benchmark Spec (`bench/`) | l2-benchmark-spec | T-6L01..02 |
| M | Codegen Tools (`cmd/codegen/`) | l2-codegen-tools | T-6M01..02 |
| **N** | **Plugin Distribution (`pkg/plugin/`)** | **l1-plugin-distribution + l2-plugin-distribution-go** | **T-6N01..04** |
| **O** | **AI API Plugin (`pkg/plugins/aiapi/`)** | **l1-ai-api-plugin + l2-ai-api-plugin-go** | **T-6O01..05** |
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

- [x] [T-6A01] `Envelope` decode + `Kind` dispatch + `Decode`/`Load`; total validation = structure + `TypeResolver` type-refs (INV-4) + `CheckIncludeDAG` 3-colour DFS (INV-5) → instantiation infallible (INV-1). — `pkg/definition/{envelope,loader,validate,errors}.go` `[Bootstrap]`
- [x] [T-6A02] Four content models (ui/scene/flow/template) + `Command`-only `Instantiate` via `CommandSink` (INV-2: no `*World`); extensible `ActionRegistry`; `Action` verb+params decode. — `pkg/definition/{content,interp,contracts}.go` `[Bootstrap]`
- [x] [T-6A03] Hot-reload foundation: `Instantiate` is idempotent + infallible (validated → re-instantiable). Full `AssetEvent[Definition]::Modified` despawn+re-instantiate **system wiring deferred** to App integration (needs asset framework + Commands). — (deferred) `[Bootstrap]`
- **Verify:** validated definition always instantiates (property test, INV-1); include cycle ⇒ `ErrDefinitionCycle` (INV-5); unknown type ⇒ `ErrUnknownType` (INV-4); unknown action ⇒ `ErrUnknownAction`; `Instantiate` emits only CommandSink calls (INV-2); malformed JSON/sub-content ⇒ `ErrSchemaInvalid`.
- **Done (2026-05-30):** `pkg/definition` (Kind-discriminated `Envelope`, `Decode`/`Load`, kind-specific content models, total `validate` + `CheckIncludeDAG` 3-colour DFS, `Command`-only `Instantiate` over `CommandSink`/`TypeResolver` consumer interfaces, `ActionRegistry`, `Action` custom unmarshal). **88.5% cov**; build + modernize clean. Decoupled from TypeRegistry/World/Commands via consumer interfaces; ECS hot-reload system + scene-codec bridge land at App integration.

### Track B — Window System

> **L2 contract (authoritative):** [l2-window-system-go.md](../specifications/l2-window-system-go.md). Depends on Track G (platform selects the `WindowBackend`).

- [x] [T-6B01] `Window` component + `PrimaryWindow` marker + `WindowBackend` interface; `DiffWindow` field diff for change-driven sync (INV-4, Focused excluded as read-only); `PrimaryWindowRes` + `CheckSinglePrimary` (INV-1); `CausesAppExit` close→exit decision (INV-2). — `pkg/window/{window,backend,events}.go`, `internal/window/{diff,primary}.go` `[Bootstrap]`
- [x] [T-6B02] Headless `WindowBackend` (no OS windows, scripted event queue + call log for CI) + `WindowDescriptor`/`RawWindowHandle`/`WindowDiff` types. Native backend selection + full `windowSyncSystem`/`PollEvents` ECS wiring deferred to App integration. — `internal/window/headless.go` `[Bootstrap]`
- **Verify:** two primaries ⇒ `ErrMultiplePrimary`; close-event→exit matrix (OnPrimaryClosed/OnAllClosed/DontExit); headless backend records Create/Apply/Destroy only when called (deferred-semantics proxy); empty diff ⇒ no Apply; scripted event queue replays identically ×20; `Logical()` DPI scaling.
- **Done (2026-05-30):** `pkg/window` (Window component + enums + `WindowResolution.Logical`, `PrimaryWindow` marker, `WindowBackend` iface, `RawWindowHandle`/`WindowDescriptor`/`WindowDiff`, `PlatformEvent` + `CausesAppExit`, `ExitCondition`, `WindowPlugin` config) + `internal/window` (`HeadlessWindowBackend` deterministic, `DiffWindow` pure, `PrimaryWindowRes` INV-1). **pkg/window 89.3%, internal/window 82.6% cov**; build + modernize clean. Full sync system + native backend land with App wiring.

### Track C — Diagnostic System

> **L2 contract (authoritative):** [l2-diagnostic-system-go.md](../specifications/l2-diagnostic-system-go.md). **Reconciled 2026-05-30** — the prior "counters/gauges/histograms" Prometheus model was dropped: the L1/L2 design is a `DiagnosticsStore` of named diagnostics over fixed-cap `RingBuffer`s. Error codes are **not** redefined here — they defer to `l2-error-core-go` (Track K).

- [x] [T-6C01] `DiagnosticsStore` + named `Diagnostic` over fixed-capacity `RingBuffer` (0-alloc `Push`, deterministic sample-count averages); `HasAnyReader` run-condition for zero-cost-when-unread (INV-1). — `pkg/diag/{store,diagnostic}.go` `[Bootstrap]`
- [x] [T-6C02] Immediate-mode `Gizmos` + pooled per-frame `GizmoBuffer` drained by `GizmoFeature` (`RenderFeature`) after scene, before UI (INV-2, pure visual; headless Draw). — `pkg/diag/gizmos.go`, `internal/diag/gizmofeature.go` `[Bootstrap]`
- [x] [T-6C03] `log/slog` per-module `ModuleFilterHandler`; build-tag `profiling` spans with no-op release path (INV-4). Debug overlay via UI text **deferred** (soft-deps Track D — lands with ui). — `pkg/diag/{log,span,span_noop}.go` `[Bootstrap]`
- **Verify:** zero readers ⇒ `Push` no-op (gate test); `RingBuffer` wrap-around + deterministic `Average`/`Min`/`Max`; gizmo `Reset`+refill 0-alloc (`AllocsPerRun`); gizmo Draw mutates no world + Flush clears; module filter drops sub-threshold records; `-tags profiling` compiles, default = 0-cost no-op.
- **Done (2026-05-30):** `pkg/diag` (RingBuffer 0-alloc, Diagnostic stats, DiagnosticsStore reader-gate INV-1, GizmoBuffer 8-shape immediate-mode + 0-alloc Reset, ModuleFilterHandler over slog, profiling/noop span split) + `internal/diag/GizmoFeature` (RenderFeature, headless Draw + Flush reset, INV-2). **pkg/diag 94.6%, internal/diag 100% cov**; default+`-tags profiling` builds; modernize clean. Built-in metrics + overlay deferred with Track D wiring.

### Track D — UI System

> **L2 contract (authoritative):** [l2-ui-system-go.md](../specifications/l2-ui-system-go.md). Depends on Track B (window viewport) + Stable render/input/2d/hierarchy. **C29 closure needs the `x/image/font` ADR** (text shaping). Skeptic: 3 tasks is tight — expect a split.

- [x] [T-6D01] `Style` component + `Val` union + single-line flexbox solver (grow/shrink, justify-content, align-items, padding/gap, reverse, nested) with `SolveIfDirty` dirty-gate (INV-1: clean tree skipped). — `pkg/ui/{style,node}.go`, `internal/ui/layout.go` `[Bootstrap]`
- [x] [T-6D02] Widget bundles (Node/Text/TextSection/ImageNode) + visual styling (BackgroundColor/BorderColor); `FontAtlas` glyph cache over the render-core shelf-pack `DynamicAtlas` (INV-4: same (font,size,rune) rasterized once). — `pkg/ui/{widgets,node}.go`, `internal/ui/atlas.go` `[Bootstrap]`
- [x] [T-6D03] `Interaction` hit-test (reverse render order, MouseFilter Stop/Pass/Ignore) + `InteractionFor` state map (INV-3 logic); `UiFeature` compositing UI last via `RenderFeature` + `SortByZ` ZIndex resolve (INV-2); `Focused`/`TabIndex`/`FocusNeighbor`. — `pkg/ui/interaction.go`, `internal/ui/{interaction,feature}.go` `[Bootstrap]`
- **Verify:** flexbox golden rects (row-grow, column-justify-center, align-center+padding+gap, nested, shrink, reverse+space-between); dirty-gate clean→skipped + solve-count stable (INV-1); same glyph rasterized once (INV-4); top-most MouseStop consumes, MouseIgnore falls through; UiFeature z-sort ascending + Flush clears.
- **Done (2026-05-30):** `pkg/ui` (Style + Val union + UiRect, Node/LayoutRect/ZIndex/BackgroundColor, Text/TextSection/ImageNode/Font, Interaction/MouseFilter/Focused/TabIndex/FocusNeighbor) + `internal/ui` (flexbox `Solve`/`SolveIfDirty` dirty-gate, `FontAtlas` over DynamicAtlas, `HitTest`+`InteractionFor`, `UiFeature` RenderFeature + `SortByZ`). **pkg/ui 95.5%, internal/ui 89.8% cov**; build + modernize clean. Grid/wrap/AlignSelf/OffsetTransform + ECS PreUpdate-system wiring + real TTF rasterizer (x/image/font ADR) are deferred refinements.

### Track E — Build Tooling

- [x] [T-6E01] CI workflows: vet/lint/race/coverage gates; matrix over OS+Go versions per Track J. — `.github/workflows/ci.yml` `[Bootstrap]`
- [x] [T-6E02] Benchmark regression CI gate: parse `-benchmem` output, compare to baseline JSON, fail on >5% drift. — `.github/workflows/ci.yml` (bench job) `[Bootstrap]`
- [x] [T-6E03] Migration/release doc generators: changelog from spec `Document History`, breaking-change report. — `cmd/releasenotes/` `[Bootstrap]`
- **Verify:** `ci.yml` is well-formed (build/vet/test-cover OS matrix + `-race` job + golangci-lint + bench-gate jobs); `releasenotes` parses Document History (em-dash/separator rows skipped, pipes in description survive, missing-H1 → filename fallback), `-since` date filter, breaking detection matches `pkg/version` caret semantics (0.x minor = breaking, 1.x minor ≠ breaking, major = breaking), exit 0/2.
- **Done (2026-05-31):** **T-6E01/E02** — `.github/workflows/ci.yml`: `test` (build+vet+`-coverprofile` across ubuntu/macos/windows), `race` (ubuntu `go test -race` — finally satisfies C28/C-005 which can't run in the dep-free local env), `lint` (golangci-lint-action), `bench` (pipes `go test -bench -benchmem ./...` through `cmd/benchcompare` — advisory, since the seed `baseline.json` ns/op is dev-machine-specific while allocs/B are the portable signal). Go version pinned via `GO_VERSION` env tracking `pkg/version.MinGoToolchain`. Path reconciled: bench-gate is a CI job step over the tested Go tool, not an ad-hoc `scripts/bench-gate/` shell. **T-6E03** — `cmd/releasenotes` (Go, path reconciled from `scripts/release/` for testability): `parse.go` (spec title/version/status + Document History table parser, filename fallback), `report.go` (`ReleaseNotes` dated changelog + `BreakingChanges` dogfooding `pkg/version` caret-incompat), `main.go` (`-specs`/`-since`/`-breaking`/`-o`). **92.1% cov**; dogfooded on the real `.design/main/specifications` corpus (release notes + breaking report render correctly). `go test ./...` 67 pkgs green, build/modernize clean. **Track E core complete.**

### Track F — CLI Tooling

- [x] [T-6F01] CLI shell: stdlib-`flag` `Router` + `Command` iface + `Output` (text/`--json` INV-4) + global flags (`--json`/`--force`/`--dry-run`) + no-arg structured help (INV-2) + `doctor`. — `cmd/cli/{main,root,commands}.go` `[Bootstrap]`
- [x] [T-6F02] `scaffold <component|system|plugin> <path>` — overwrite-safe (skip unless `--force`, `--dry-run` preview, INV-1). Full `ecs new project` template tree deferred. — `cmd/cli/commands.go` `[Bootstrap]`
- [~] [T-6F03] `plugin validate <path>` + `plugin list <dir>` consuming the `pkg/plugin` SDK (ParseManifest+Validate). **Done.** `install|enable|disable|info|remove|doctor` + `asset import|list|build` **deferred** (need the plugin install flow + asset pipeline). — `cmd/cli/commands.go` `[Bootstrap]`
- **Verify:** no-arg help lists registered commands (INV-2); only registered commands shown (INV-3); `doctor --json` valid stable JSON, text mode no braces (INV-4); scaffold skips existing without `--force` + `--dry-run` writes nothing (INV-1); `plugin validate` good→ok / bad→exit 1; `plugin list` lists + empty-dir message; unknown command → exit 2.
- **Done (2026-05-31):** `cmd/cli` (Router/Command/Output, `doctor`, overwrite-safe `scaffold`, `plugin validate|list` over the `pkg/plugin` SDK; gated registry INV-3). **85.0% cov**; `go test ./...` 62 pkgs green; modernize clean. `new project`, `asset *`, full `plugin install/lifecycle` subcommands deferred (need templates + asset pipeline + plugin install flow).

### Track G — Platform System

> **L2 contract (authoritative):** [l2-platform-system-go.md](../specifications/l2-platform-system-go.md). Cohort root (no intra-cohort deps); selects the Track B `WindowBackend`.

- [x] [T-6G01] Immutable `PlatformProfile` resource + `PlatformCaps` branchless bitfield + `PlatformOS`/`Arch`/`Tier` enums; detection from `runtime.GOOS`/`GOARCH`; inserted in `PreStartup`, read-only thereafter (INV-2). — `pkg/platform/{profile,caps,plugin}.go`, `internal/platform/detect.go` `[Bootstrap]`
- [x] [T-6G02] Build-tag-split profile (`profile_default.go` `!headless` / `profile_headless.go` `headless`) — headless caps exclude `HasGPU`/`HasMultiWindow`/`HasSpatialAudio` (INV-4); `Plugin` inserts the profile resource (INV-3); pure `osFromGOOS`/`archFromGOARCH` mappings. `//go:build editor` scope is enforced by the existing `pkg/editor` guard tests (multi-repo), not duplicated. — `internal/platform/profile_{default,headless}.go`, `internal/platform/plugin.go` `[Bootstrap]`
- **Verify:** `Has`/`With` bitfield table (disjoint flags don't alias); `-tags headless` profile has `HasGPU/HasMultiWindow/HasSpatialAudio == false`; cross-compile smoke (`GOOS=windows/linux/darwin`, `js/wasm`) builds the core.
- **Done (2026-05-30):** `pkg/platform` (pure types: `PlatformProfile`, `PlatformCaps` bitfield, OS/Arch/Tier total-switch enums, `PlatformPlugin` iface) + `internal/platform` (GOOS/GOARCH pure mappings, `!headless`/`headless` profile split, profile-inserting `Plugin`). **pkg/platform 100%, internal/platform 100% coverage in both default and `-tags headless` builds**; headless build verified. Build + modernize clean.

### Track H — AI Assistant System

- [x] [T-6H01] `AssistantManager` (agent registry + per-agent `Capability` set + request log) — `//go:build editor`. — `pkg/assistant/manager.go` `[Bootstrap]`
- [~] [T-6H02] Transports: **stdio done** (`StdioConnection`, newline-delimited JSON) + `MemConnection` (deterministic test responder); **websocket/http deferred** (ws needs a dep ADR; http is a thin follow-up). — `pkg/assistant/transport.go` `[Bootstrap]`
- [x] [T-6H03] Standard method registry + required-capability table + `Dispatch` (INV-2 gate, INV-5 timeout, request-log tag) + `ApplyModifications` agent+request tagging for undo (INV-1/4). — `pkg/assistant/{methods,manager}.go` `[Bootstrap]`
- **Verify:** capability bitfield Has/With/String; method→cap table; `Dispatch` denies under-privileged method (INV-2) + logs; unknown agent → `ErrUnknownAgent`; slow agent → `ErrAgentUnavailable` within timeout (INV-5); modifications tagged agent+request (INV-4); stdio round-trip + close. Tested with `-tags editor`.
- **Done (2026-05-31):** `pkg/assistant` (`//go:build editor`): `AgentMessage` JSON protocol + Encode/Decode, `Capability` bitfield, standard-method/required-cap table, `Transport`/`Connection` + `StdioConnection` + `MemConnection`, `AssistantManager` Dispatch (gate INV-2, timeout/cancel INV-5, log) + `ApplyModifications` tagging (INV-1/4). **88.0% cov (`-tags editor`)**; default `go test ./...` 60 pkgs green (editor excluded); modernize clean. websocket/http transports + ContextProvider + editor UI deferred.

### Track I — Examples Framework

- [ ] [T-6I01] Example directory conventions: per-example `manifest.toml`, README, expected golden output. — `examples/README.md`, `examples/_template/` `[Bootstrap]`
- [ ] [T-6I02] Example lifecycle hooks + CI selective build (only changed examples). — `scripts/examples/` `[Bootstrap]`

### Track J — Compatibility Policy

- [x] [T-6J01] Engine SemVer policy doc + Go-toolchain compatibility matrix; `engine_version` constraint parser (consumed by Track N). — `pkg/version/{version,constraint,policy}.go` `[Bootstrap]`
- [ ] [T-6J02] Compatibility test harness: snapshot of `pkg/` public surface; CI fails on undocumented breaking change. — `scripts/api-diff/` `[Bootstrap]` *(deferred — CI/tooling deliverable)*
- **Verify:** SemVer Parse (v-prefix/pre-release strip, partial→0, invalid→`ErrInvalidVersion`); Compare ordering (major>minor>patch); Constraint matrix (^/~/>=/>/<=/</=/bare/range/empty, caret-on-0.x pins minor); bad clause → `ErrInvalidVersion`; `IsGoToolchainSupported` at/above/below `MinGoToolchain` + `go`-prefix + garbage; **`pkg/plugin` Track N tests still green** after the extraction (alias identity preserved).
- **Done (2026-05-31):** Extracted the canonical SemVer kernel to **`pkg/version`** (DRY — Track N's `pkg/plugin` and Track J's engine compat now share one implementation): `version.go` (`Version`, `Parse`, `Compare`, `String`), `constraint.go` (`Constraint`, `ParseConstraint`, caret/tilde/range/exact, `Matches`, `IsAny`), `policy.go` (engine SemVer policy doc + `MinGoToolchain` (1.26.3 from go.mod) + `GoToolchainConstraint`/`IsGoToolchainSupported` dogfooding the parser). Refactored `pkg/plugin/{semver,constraint}.go` to **type-alias re-exports** (`type Version = version.Version`, wrapper `ParseVersion`/`ParseConstraint`, aliased sentinels) — public SDK API + Track N tests unchanged. **pkg/version 98.9% cov**; pkg/plugin 86.1% (was 88.5% — logic moved out), internal/plugin 93.0%, cmd/cli 85.0% all green; `go test ./...` 65 pkgs, modernize clean. **T-6J02 deferred** (api-diff is a CI/scripts tool).

### Track K — Error Core

> **L2 contract (authoritative):** [l2-error-core-go.md](../specifications/l2-error-core-go.md). Cohort root (no intra-cohort deps); consumed by Track C (diagnostic codes) and the deferred editor/AI tracks.

- [x] [T-6K01] `EngineError` interface over stdlib `errors` (Is/As/Unwrap) + range-partitioned `Code` registry (duplicate ⇒ `ErrDuplicateCode`, out-of-range ⇒ `ErrCodeOutOfRange`) + `Severity`/`Audience` total-switch enums. — `pkg/errs/{error,severity,code}.go` `[Bootstrap]`
- [x] [T-6K02] `fs.FS` locale catalog (`errors.{lang}.json`) + `Localize` with default-template fallback (missing key never empty); malformed catalog keeps embedded defaults. — `pkg/errs/catalog.go`, `locales/errors.en.json` `[Bootstrap]`
- [x] [T-6K03] Build-tag debug traces (`runtime.Callers`; `!debug` = no-op, INV trace) + `MustSucceed` (panics **only** for Fatal+Developer) + structured-log adapter; redaction filter (consumed later by Track O). — `pkg/errs/{trace_debug,trace_release,redact}.go` `[Bootstrap]`
- **Verify:** `errors.As` recovers `EngineError` through a `%w` chain; severity `String()` total over all enum values; duplicate/out-of-range registration guarded; `Localize` falls back on missing key; `MustSucceed` panics only for Fatal+Developer.
- **Done (2026-05-30):** `pkg/errs` implemented — `EngineError`/`engineError`, `Code` registry with module-range + duplicate guards, `Severity`/`Audience` total-switch enums, `Catalog` over `fs.FS` (fmt-style templates + bare-code fallback), `//go:build debug` trace split, `MustSucceed` Fatal+Developer panic policy, `Redactor`. `locales/errors.en.json` shipped. **89.3% coverage**, build + modernize clean.

### Track L — Benchmark Spec

- [x] [T-6L01] Benchmark suite structure: per-subsystem `bench/{subsystem}/` packages, comparison harness, CSV/JSON output. — `bench/`, `cmd/benchcompare/` `[Bootstrap]`
- [~] [T-6L02] Baseline JSON format + CI drift gate (consumed by T-6E02). — `bench/baseline.json`, `scripts/bench-gate/` `[Bootstrap]` *(baseline format + JSON report done; CI shell wiring deferred to T-6E02)*
- **Verify:** parse `go test -bench -benchmem` output (proc-suffix strip, no-`benchmem` line, log-noise skip, multi-pkg); compare flags ns/op + B/op drift past threshold and **any** allocs/op increase (0→1 categorical); new/missing benchmarks listed; `-update` round-trips; `-json` valid (non-finite Δ → null); exit 0/1/2 for OK/regression/usage.
- **Done (2026-05-31):** `cmd/benchcompare` — stdlib-only parser (`parse.go`), `Baseline` JSON load/save (`baseline.go`), drift `Compare` (`compare.go`: ns/B by `-threshold` %, allocs strict-on-increase per the 0-alloc discipline), CLI (`main.go`: `-baseline`/`-current`/`-threshold`/`-update`/`-json`, exit 1 on regression). **85.4% cov**; `go test ./...` 66 pkgs green, build/modernize clean. Dogfooded: real `bench/baseline.json` (16 hot-path benchmarks captured via `-update`, self-compare OK) + `bench/README.md`. Per-subsystem `bench/{subsystem}/` packages: existing benchmarks stay colocated in `_test.go` (the harness consumes their piped output); a separate tree was unnecessary. T-6L02 CI gate (`scripts/bench-gate/`) deferred to Track E (`T-6E02`).

### Track M — Codegen Tools

- [x] [T-6M01] Query wrapper generator: typed `Query[N]` helpers from component declarations. — `cmd/codegen/query/` `[Bootstrap]`
- [ ] [T-6M02] Boilerplate generator: component registration stubs, plugin scaffolds (consumed by `ecs new plugin`). — `cmd/codegen/{component,plugin}/` `[Bootstrap]` *(deferred — Track F `scaffold` already emits basic component/system/plugin stubs)*
- **Verify:** `genSource` output parses as valid Go (go/format + re-parse); `genArity(4)` has 4 pointer-fields/4 `componentIDFor`/`ids[3]` fetch/`idC==idD` distinctness pair; CLI stdout + `-out` file modes; invalid range (min<2, max<min, max>26) + bad flag → exit 2. Generated `Query4/Query5` are functionally correct (spawn→`Count`==1, `All` tuple values, pointer mutation persists, same-type rejection).
- **Done (2026-05-31):** `cmd/codegen/query` — generates the typed Query/Tuple arity ladder (Go has no variadic type params, so each arity is a distinct generated type; the hand-written `Query1–Query3` set the pattern). `genArity`/`genSource` build the source and validate it through `go/format.Source`; CLI flags `-min`/`-max`/`-out`. **95.2% cov.** Ran it to generate `internal/ecs/query/query_gen.go` (`Query4`/`Query5`/`Query6` + `Tuple4-6`, "DO NOT EDIT" header, `//go:generate` directive in `query.go`) + added `pkg/ecs` re-exports (`Query4-6`/`NewQuery4-6`). Functional `query_gen_test.go` (Query4/5 spawn+query+mutation+distinct) green. `go test ./...` 68 pkgs, build/modernize clean. **Extends the query ladder 3 → 6.** T-6M02 (component/plugin boilerplate) deferred — the Track F CLI `scaffold` already covers basic stubs.

### Track N — Plugin Distribution (NEW)

- [x] [T-6N01] Public SDK surface: `Plugin`/`PluginGroup` re-exported from app framework; `PluginID`, `Mode`, `State`, `Capability`+`CapabilitySet`+tiers, `PluginContext`+`CommandIssuer`, `E-PLUGIN` errors. — `pkg/plugin/{plugin,capability,context,errors}.go` `[Bootstrap]`
- [x] [T-6N02] Manifest + SemVer: `Version`/`Constraint` (^/~/range, Cargo caret), `Manifest` struct + `Validate` + minimal stdlib TOML-subset `ParseManifest`, `CompatibleWith` (INV-2). — `pkg/plugin/{semver,constraint,manifest}.go` `[Bootstrap]`
- [~] [T-6N03] In-process loader: **partial** — `internal/plugin/Manager` (registry, INV-2 compat resolution, INV-5 duplicate guard, lifecycle states) + `capabilityProxy` (INV-3 runtime enforcement) + plugin-ID-tagged commands (INV-7) **done**; discovery (4 sources) + capability-prompt persistence + full Build/Ready/Finish wiring **deferred** to App integration. — `internal/plugin/manager.go` `[Bootstrap]`
- [ ] [T-6N04] Out-of-process loader: subprocess spawn (cwd-restricted), transport handshake (reuses Track H transports + `pkg/protocol`), host-side proxy `Plugin`, failure isolation per INV-8. — `internal/plugin/oop/` `[Bootstrap]` *(deferred — needs Track H transports + App wiring)*
- **Verify:** SemVer constraint matrix (^/~/range/exact, caret-on-0.x); manifest parse+validate (in/OOP); INV-2 incompatible → `ErrEngineIncompatible`; INV-5 duplicate → `ErrDuplicateID`; INV-1 invalid → `ErrManifestInvalid`; INV-3 ungranted cap → `CapabilityError`; INV-7 commands tagged.
- **Done (2026-05-31):** `pkg/plugin` SDK (re-exports, `Version`/`Constraint` Cargo-subset SemVer, `Manifest`+TOML-subset parser+`Validate`, `Capability`/`CapabilitySet`+tiers+wildcard, `PluginContext`/`CommandIssuer`, `E-PLUGIN` errors) + `internal/plugin/Manager`+`capabilityProxy`. **pkg/plugin 88.5%, internal/plugin 93.0% cov**; `go test ./...` 60 pkgs green; modernize clean. No Go `plugin` .so. Full loaders (discovery/spawn/lifecycle) deferred to App integration.

### Track O — AI API Plugin (NEW)

- [~] [T-6O01] Config struct + `DefaultConfig` (`//go:build editor`) done; embedded `plugin.toml` + full `New()`/Build/Ready/Finish/Cleanup lifecycle + ServiceRegistry registration **deferred** to App integration. — `pkg/plugins/aiapi/config.go` `[Bootstrap]`
- [x] [T-6O02] `Provider` abstraction + registry + `Select` + canonical request/response types + OpenAI-compatible provider (build/parse, httptest-validated); Anthropic/Gemini follow the same one-file pattern (deferred). — `pkg/plugins/aiapi/{provider,canonical,provider_openai}.go` `[Bootstrap]`
- [~] [T-6O03] Rate limiter (RPM+TPM token bucket per provider, INV-8) **done**; method dispatch (8 methods) + SSE streaming + cancellation map **deferred** (needs Track H method-routing + event bus wiring). — `pkg/plugins/aiapi/ratelimit.go` `[Bootstrap]`
- [x] [T-6O04] Credentials (env source; keyring/age ADR-gated) + redaction via `errs.Redactor` + secret zeroing (INV-1) + `E-PLUGIN-AIAPI-{NNN}` mapping incl. `MapHTTPStatus` (INV-5). Diagnostics/cost-budget events deferred (need diag wiring). — `pkg/plugins/aiapi/{credentials,errors}.go` `[Bootstrap]`
- [x] [T-6O05] `FakeProvider` deterministic responder for CI (INV-7 mode-parity foundation); full in-process/OOP parity harness deferred (needs Track N OOP loader). — `pkg/plugins/aiapi/fake.go` `[Bootstrap]`
- **Verify:** rate limiter RPM+TPM exhaustion + refill (INV-8); `MapHTTPStatus` 401/429/4xx/5xx (INV-5); OpenAI build/parse round-trip + `Complete` via httptest (success + 500); FakeProvider Complete/Stream/err; env secret resolve + redaction + zeroing (INV-1); `Select` + unknown-provider error. Tested `-tags editor`.
- **Done (2026-05-31):** `pkg/plugins/aiapi` (`//go:build editor`): canonical types, `Provider` iface + registry + `Select`, OpenAI-compatible provider (build/parse + httptest), `Config`/`DefaultConfig`, RPM+TPM `rateLimiter` (INV-8), `E-PLUGIN-AIAPI` codes + `MapHTTPStatus` (INV-5), env credentials + `errs.Redactor` reuse + zeroing (INV-1), `FakeProvider`. **82.3% cov (`-tags editor`)**; default `go test ./...` 60 pkgs green; `go build -tags editor ./...` clean; modernize clean. Deferred (App/event/diag/OOP wiring): SSE streaming, method dispatch, lifecycle, diagnostics, full mode-parity harness, Anthropic/Gemini/local providers.

### Track P — Visual Graph System (NEW)

- [x] [T-6P01] Graph Data Model: `GraphDefinition`, `Node`, `Pin` (Direction/Kind), `Connection`, `VariableDecl` + lookup helpers. Definition-System `"graph"` loader wiring deferred to App integration. — `pkg/visualgraph/model.go` `[Bootstrap]`
- [x] [T-6P02] `NodeRegistry` + `NodeDescriptor`/`PinDescriptor` (Register/Get/List/ListByCategory/Search, deterministic). TypeRegistry auto-generation deferred (needs the registry service wired). — `pkg/visualgraph/registry.go` `[Bootstrap]`
- [x] [T-6P03] `ValidateGraph` (INV-3 endpoint/direction/kind/type-compat + INV-4 data-dependency cycle detection) + `Interpreter` (execution chain + lazy memoized data pull, step-limit INV-2, `CommandSink` INV-1, deterministic INV-4) + built-in node set (event/math/logic/flow/action). — `pkg/visualgraph/{validate,interpreter,builtins}.go` `[Bootstrap]`
- [ ] [T-6P04] Editor Gateway: `GraphEditorPlugin`/`NodeRegistryQuery`/`GraphDebugger` in `pkg/editor/graph.go` + `pkg/protocol/graph.go` IPC. — `pkg/editor/graph.go` `[Bootstrap]` *(deferred — editor-bridge L2; needs pkg/editor extension + App integration)*
- **Verify:** registry CRUD + sorted List + Search; validate rejects unknown node/pin, direction, kind mismatch, type mismatch (INV-3), data cycle (INV-4); interpreter lazy data-eval (Add→Log sum), Branch routing, exec-cycle → `ErrStepLimit` (INV-2), Action → sink (INV-1), deterministic ×20 (INV-4).
- **Done (2026-05-31):** `pkg/visualgraph` (graph model + `NodeRegistry` + `ValidateGraph` INV-3/INV-4 + `Interpreter` lazy-pull execution engine INV-1/2/4 + built-in node set). **89.3% cov**; `go test ./...` 61 pkgs green; modernize clean. Editor gateway (`pkg/editor/graph.go`), TypeRegistry auto-gen, SubGraph/Query nodes, and the `"graph"` definition loader deferred to App/editor integration.

### Track T — Validation

- [ ] [T-6T01] Plugin SDK contract tests: manifest schema fuzz, capability enforcement (denial paths), in-process lifecycle proxy, in/out-of-process behavioural parity. — `pkg/plugin/contract_test.go`, `internal/plugin/testbench/` `[Bootstrap]`
- [ ] [T-6T02] AI API plugin parity matrix: every standard method exercised in both modes; identical canonical responses asserted. — `pkg/plugins/aiapi/parity_test.go` `[Bootstrap]`
- [ ] [T-6T03] CLI integration tests: `ecs plugin scaffold|validate|install|list|enable|disable|info|remove|doctor` golden output. — `cmd/cli/plugin/integration_test.go` `[Bootstrap]`
- [ ] [T-6T04] Codegen golden output + benchmark regression CI gate live (T-6E02 + T-6L02 wired). — `cmd/codegen/golden/`, `.github/workflows/bench.yml` `[Bootstrap]`
- [ ] [T-6T05] AI API plugin live-provider smoke test (gated by `live-ai` CI label, project-secret API keys); explicit cost budget. — `.github/workflows/ai-live.yml` `[Bootstrap]`
- [x] [T-6T06] **Engine-core C29 validation track** (this activation's gate): `examples/{config,window,diagnostic,ui}/` validate the 6 engine-core L2 contracts end-to-end (definition load+instantiate+INV-5 cycle reject, headless window lifecycle+event replay+INV-2 exit, diagnostics INV-1 reader-gate + gizmo geometry, flexbox layout + INV-1 dirty-gate); each hash-stable ×20; `go test ./...` 58 pkgs green + `modernize` clean. **Closes the C29 P6 gate** → engine-core L1+L2 Draft → Stable eligible. — `examples/{config,window,diagnostic,ui}/` `[Bootstrap]`
- **Done (2026-05-30):** `examples/{config,window,diagnostic,ui}/main.go` + `main_test.go` — each `run()` exercises its subsystem deterministically and asserts an identical FNV hash across ≥20 runs. All four green; full workspace `go build`/`go test` (58 pkgs) + `modernize` clean. **Engine-core cohort 17/17 complete; C29 P6 gate CLOSED.**

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
