# Project Context

**Generated:** 2026-06-05

## Active Technologies

- Go

## Core Project Structure

```plaintext
.
├── .design/
│   ├── .cache/
│   ├── .graph-cache/
│   ├── .version
│   ├── INDEX.md
│   ├── RULES.md
│   ├── main/
│   ├── wiki/
│   └── workspace.json
├── cmd/
│   ├── apidiff/
│   ├── benchcompare/
│   ├── cli/
│   ├── codegen/
│   ├── examplecheck/
│   ├── hot-reload-daemon/
│   └── releasenotes/
├── examples/
│   ├── 2d/
│   ├── 3d/
│   ├── README.md
│   ├── _template/
│   ├── animation/
│   ├── app/
│   ├── asset/
│   ├── async/
│   ├── audio/
│   ├── camera/
│   ├── config/
│   ├── diagnostic/
│   ├── ecs/
│   ├── editor/
│   ├── gltf/
│   ├── goldens.json
│   ├── math/
│   ├── networking/
│   ├── physics/
│   ├── scene/
│   ├── shader/
│   ├── stress_test/
│   ├── tweening/
│   ├── ui/
│   ├── window/
│   └── world/
├── internal/
│   ├── animation/
│   ├── asset/
│   ├── audio/
│   ├── definition/
│   ├── diag/
│   ├── ecs/
│   ├── grapheditor/
│   ├── hotreload/
│   ├── platform/
│   ├── plugin/
│   ├── profiling/
│   ├── releaseguard/
│   ├── render/
│   ├── tween/
│   ├── ui/
│   └── window/
└── pkg/
    ├── animation/
    ├── app/
    ├── asset/
    ├── assistant/
    ├── audio/
    ├── definition/
    ├── diag/
    ├── ecs/
    ├── editor/
    ├── errs/
    ├── math/
    ├── platform/
    ├── plugin/
    ├── plugins/
    ├── protocol/
    ├── render/
    ├── scene/
    ├── task/
    ├── tween/
    ├── ui/
    ├── version/
    ├── visualgraph/
    └── window/
```

## Recent Changes


The editor-and-tooling phase. Closed with all activated tasks Done, 86/112 specs
Stable, and the full default test suite green (75 packages).

- **Definition system + hot-reload** — Kind-discriminated `Envelope`, total validation (`TypeResolver` + include-DAG 3-colour DFS), `Command`-only instantiation; the asset-event ECS bridge (`AssetEvent[A]` + `WatchAssetType`) drives despawn/re-instantiate hot-reload.
- **Visual graph system** — graph model + `NodeRegistry` + `ValidateGraph` (INV-3/4) + lazy-pull `Interpreter`; the editor bridge (`pkg/editor` contract-only, `pkg/protocol` IPC) with concrete `internal/grapheditor` impls + a `PostUpdate` graph-debug sync system + a non-invasive interpreter trace hook (`RunTraced`/`TraceRecorder`).
- **Window + platform** — headless-safe window-sync system (state↔backend diff, platform-event mirroring, exit policy); platform tier/capability matrix with build-tag-isolated backends.
- **Diagnostics** — `DiagnosticsStore` (reader-gated, INV-1) + built-in FPS/frame-time/entity-count metrics + immediate-mode gizmos + a UI-text debug overlay; structured `E-series` error taxonomy with localization + redaction.
- **UI** — single-line flexbox solver, widgets/styling, hit-test + focus interaction, and a runtime interaction stack (layout→hit-test ECS system); text rendering via an embedded **stdlib bitmap font** (the zero-dependency choice over `x/image/font`, C32).
- **CLI** — stdlib-`flag` router + command registry with availability gating, overwrite-safe scaffolding, `--json` output, and `plugin validate|list` over the SDK.
- **Plugin distribution + AI** — public `pkg/plugin` SDK, in-process (compile-time factory) + out-of-process (subprocess over `pkg/protocol`, failure isolation) loaders sharing one lifecycle; the AI-assistant transport/capability/context layer (stdio+http; WebSocket ADR-rejected, C32); the first-party `pkg/plugins/aiapi` plugin (OpenAI/Anthropic/Gemini/local, SSE, dispatch, rate-limit, diagnostics, parity).
- **Tooling** — examples framework + `cmd/examplecheck` golden gate; compatibility policy (`pkg/version`) + `cmd/apidiff` surface-snapshot gate; `cmd/benchcompare` regression gate wired into CI; `cmd/codegen/query` arity-ladder generator with a golden drift-gate; CI matrix + race gate + `live-ai`-gated provider smoke.
- **Asset formats** — glTF 2.0 loader (mesh/material/texture fan-out) extended with **animation** decoding (translation/rotation/scale channels, STEP/LINEAR/CUBICSPLINE); `.scene.json` decode + `SerializedScene → World` hydration/spawn.
- **Validation** — `examples/editor` App-integration round-trip (hash-stable C29); plugin-SDK contract tests (manifest fuzz, capability enforcement); aiapi per-method parity matrix; CLI golden output; codegen + bench gates.
- **Governance** — 5 editor-feature spec families promoted Draft→Stable (cli-tooling, plugin-distribution, ai-api-plugin, ai-assistant-system, visual-graph); added rule **C32 — Third-Party Dependency ADRs** formalizing the zero-dependency posture.

