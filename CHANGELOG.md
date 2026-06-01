# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Updated task plan and task index (main)
- Completed task `phase-1` (main)
- Added 2 specifications (main)
- Updated 7 specifications (main)
- Completed task `phase-2` (main)
- Updated 18 specifications (main)
- Updated 19 specifications (main)
- Updated 22 specifications (main)
- Updated global project rules (main)
- Completed task `phase-3` (main)
- Added 5 specifications (main)
- Completed task `phase-4` (main)
- Updated 4 specifications (main)
- Completed `phase-5` content systems (audio, animation, tweening, 2D, asset codecs) (main)
- Ratified multi-repo architecture l1/l2 → Stable (engine↔editor boundary) (main)
- Phase 6 engine-core cohort (17/17): added `pkg/errs` (structured error taxonomy), `pkg/platform` (platform profile + caps), `pkg/diag` (diagnostics + gizmos), `pkg/window` (entity windows + headless backend), `pkg/ui` (flexbox layout + font atlas + interaction), `pkg/definition` (JSON declarative layer) + `examples/{config,window,diagnostic,ui}` C29 validation (main)
- Authored Phase 6 editor-layer L2 contracts (plugin-distribution, ai-assistant, ai-api-plugin, cli, definition-integration) — full P6 L1+L2 parity (main)
- Phase 6 plugin SDK (Track N core): added `pkg/plugin` (public SDK — SemVer constraints, `plugin.toml` manifest, capability model, no Go `.so`) + `internal/plugin` (manager + capability-enforcing proxy) (main)
- Phase 6 AI assistant (Track H): added `pkg/assistant` (editor-only) — agent JSON protocol, capability-gated dispatch, stdio transport, timeout-bounded async, undo-tagged modifications (main)
- Phase 6 AI API plugin (Track O core): added `pkg/plugins/aiapi` (editor-only) — provider abstraction + canonical types, OpenAI-compatible provider, RPM/TPM rate limiter, E-PLUGIN-AIAPI error mapping, env credentials with redaction, FakeProvider (main)
- Phase 6 visual graph (Track P core): added `pkg/visualgraph` — node-based graph model, node registry, load-time validation (type-safety + data-cycle detection), bounded lazy-eval interpreter routing mutations through a command sink (main)
- Phase 6 CLI (Track F): replaced the `cmd/cli` stub with a working CLI — command router, `--json` output, `doctor`, overwrite-safe `scaffold`, and `plugin validate|list` over the plugin SDK (main)
- Phase 5 glTF loader (Track B follow-on): added `pkg/asset/formats/gltf` — stdlib-only `.gltf`/`.glb` decoder fanning a file out into meshes, PBR materials, embedded textures, and scenes addressed by a deterministic, reload-stable `GltfAssetLabel` (INV-4); full-asset-or-error decoding with a panic guard (INV-2); `examples/gltf` golden (main)
- Phase 6 versioning (Track J): added `pkg/version` — the canonical SemVer `Version` + Cargo-subset `Constraint` (caret/tilde/range) + engine Go-toolchain compatibility policy; `pkg/plugin` now re-exports these as type aliases instead of carrying its own copy (single SemVer implementation, no duplication) (main)
- Phase 6 benchmark gate (Track L): added `cmd/benchcompare` — parses `go test -bench -benchmem` output and fails on regression against a committed `bench/baseline.json` (ns/op + B/op past a threshold, any allocs/op increase), with text and `--json` reports (main)
- Phase 6 build tooling (Track E): added `.github/workflows/ci.yml` (build + vet + coverage matrix, `-race` gate, golangci-lint, benchmark drift gate) and `cmd/releasenotes` — generates a dated changelog and a breaking-change report from the specs' Document History (main)
- Phase 6 codegen (Track M): added `cmd/codegen/query` and used it to extend the ECS typed-query ladder from 3 to 6 components — `Query4`/`Query5`/`Query6` are now generated (Go has no variadic type parameters) and re-exported from `pkg/ecs` (main)
- Phase 6 examples framework (Track I): added `cmd/examplecheck` (runs the examples and gates them against a committed `examples/goldens.json`, with selective-build via `-changed`), an `examples/_template/` scaffold, and example conventions in `examples/README.md` (main)
- Phase 6 visual-graph editor bridge (Track P): added the editor↔engine graph contract — `pkg/editor/graph.go` (`GraphEditorPlugin`/`NodeRegistryQuery`/`GraphDebugger` interfaces, self-contained DTOs) and `pkg/protocol/graph.go` (breakpoint/trace/runtime-error/live-update IPC messages); engine-side wiring is deferred to App integration (main)
- Phase 6 API-stability gate (Track J): added `cmd/apidiff` — snapshots the exported `pkg/...` API (stdlib `go/ast`/`go/printer`) into a committed `api/snapshot.json` and fails on a breaking change (removed/changed exports), with a `-json` mode for CI; completes the compatibility tooling alongside `pkg/version` (main)
- Phase 5 scene loader (Track B follow-on): added `pkg/asset/formats/scene` — decodes `.scene.json` into the portable `scene.SerializedScene` (reusing the Phase 3 JSON codec) with full interning-index validation; spawning into a World remains the scene system's job (main)
