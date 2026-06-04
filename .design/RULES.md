# Project Specification Rules

**Version:** 1.10.0
**Status:** Active

## Overview

Constitution of the specification system for this project.
Read by the agent before every operation. Updated only via explicit triggers.

## 1. Naming Conventions

- Spec files use lowercase kebab-case with layer prefixes:
  - `l1-{name}.md` for conceptual architectures (e.g., `l1-world-system.md`).
  - `l2-{name}.md` for technical implementations (e.g., `l2-world-system-go.md`).
- System files use uppercase: `INDEX.md`, `RULES.md`, `PLAN.md`, `TASKS.md`.
- Section names within specs are title-cased.

## 2. Status Rules

- **Draft → RFC**: all required sections filled, ready for review.
- **RFC → Stable**: reviewed, approved, no open questions.
- **RFC → Draft**: needs rework or significant revision.
- **Stable → RFC**: substantive amendment (minor/major bump) requires re-review.
- **Any → Deprecated**: explicitly superseded; replacement must be named.

## 3. Versioning Rules

- `patch` (0.0.X): typo fixes, clarifications — no structural change.
- `minor` (0.X.0): new section added or existing section extended.
- `major` (X.0.0): structural restructure or scope change.

## 4. Formatting Rules

- Use `plaintext` blocks for all directory trees.
- Use `mermaid` blocks for all flow and architecture diagrams.
- Do not use other diagram formats.

## 5. Content Rules

- No implementation code (no Rust, JS, Python, SQL, etc.).
- Pseudo-code and logic flows are permitted.
- Every spec must have: Overview, Motivation, Document History.

## 6. Relations Rules

- Every spec that depends on another must declare it in `Related Specifications`.
- Cross-file content duplication is not permitted — use a link instead.
- Circular dependencies must be flagged and resolved.

## 7. Project Conventions

### C1 — `.magic/` Engine Safety

`.magic/` is the active SDD engine. Any modification must follow this protocol:

1. **Read first** — open and fully read every file that will be affected.
2. **Analyse impact** — trace how the changed file is referenced by other engine files and workflow wrappers.
3. **Verify continuity** — confirm that after the change all workflows remain fully functional.
4. **Never edit blindly** — if the scope of impact is unclear, stop and ask before proceeding.
5. **Document the change** — record modifications in the relevant spec and commit message.
6. **Atomic Update** — apply changes simultaneously across all related files (scripts, workflows, and documentation) to maintain full engine consistency.
7. **No-Change, No-Bump** — NEVER trigger a version bump (C14) if no physical files in `.magic/` were modified (e.g., during dry runs or purely cognitive tasks).

### C2 — Workflow Minimalism

Limit the SDD workflow to the core command set to maximize automation and minimize cognitive overhead. Do not introduce new workflow commands unless strictly necessary and explicitly authorized as a C2 exception.

### C3 — Parallel Task Execution Mode

Task execution defaults to **Parallel mode**. A Manager Agent coordinates execution, reads status, unblocks tracks, and escalates conflicts. Tasks with no shared constraints are implemented in parallel tracks.

### C4 — Automate User Story Priorities

Skip the user story priority prompt. The agent must automatically assign default priorities (P2) to User Stories during task generation to maximize automation and avoid interrupting the user.

### C5 — [RESERVED]

This rule ID is reserved for future extensions.

### C6 — Selective Planning

During plan updates, specs are handled by their status:

- **Draft specs**: automatically moved to `## Backlog` in `PLAN.md` without user input.
- **RFC specs**: surfaced to user with a recommendation to backlog until Stable.
- **Stable specs**: agent asks which ones to pull into the active plan. All others go to Backlog.
- **Orphaned specs** (in INDEX.md but absent from both plan and backlog): flagged as critical blockers.

### C7 — Universal Script Executor

All automation scripts must be invoked via the cross-platform executor:
`node .magic/scripts/executor.js <script-name> [args]`

Direct calls to `.sh` or `.ps1` scripts are not permitted in workflow instructions. The executor detects the OS and delegates to the appropriate implementation.

### C8 — Phase Archival

On phase completion, the per-phase task file is moved from `$DESIGN_DIR/tasks/` to `$DESIGN_DIR/archives/tasks/`. The link in `TASKS.md` is updated to point to the archive location. This keeps the active workspace small while preserving full history.

### C9 — Zero-Prompt Automation

Once the user approves the plan and task breakdown, the agent proceeds through execution and conclusion workflows without further confirmation prompts. Silent operations include: retrospective Level 1, changelog Level 1, CONTEXT.md regeneration, and status updates. The single exception is changelog Level 2 (external release artifact) which requires one explicit user approval before writing.
**Phase Gates Exception**: C9 applies ONLY within a specific executing phase (e.g., executing atomic tasks within magic.run). Transitions across major workflow boundaries (Spec → Task → Run) constitute 'Phase Gates' and ALWAYS require explicit user approval (Hard Stop) before handing off.

### C10 — Nested Phase Architecture

Implementation plans in `PLAN.md` must follow a nested hierarchy: **Phase → Specification → Atomic Tasks**. Each specification is decomposed into 2–3 atomic checklist items using standardized notation:

- `[ ]` Todo
- `[/]` In Progress
- `[x]` Done
- `[~]` Cancelled
- `[!]` Blocked

### C11 — [RESERVED]

This rule ID is reserved for future extensions.

### C12 — Quarantine Cascade

If a Layer 1 (Concept) specification loses its `Stable` status or is removed, all dependent Layer 2/3 (Implementation) specifications must automatically and transparently be treated as demoted to `RFC` or moved to the Backlog by the Task workflow. The system must quarantine dependent specifications to prevent "orphaned" task scheduling without requiring manual status edits for every child in `INDEX.md`.

**C12.1 — Stabilization Exception**: Tasks explicitly intended to stabilize or fix mismatches to regain `Stable` status for the parent may bypass this quarantine.

### C13 — Agent Cognitive Discipline

All AI agents operating within the Magic SDD framework must adhere to strict cognitive discipline to prevent hallucinations and silent failures:

1. **Primary Source Principle**: Always read original `.magic/` and `.design/` files. Never rely on cached memory or interpretive assumptions.
2. **Anti-Truncation**: Execute checklists and multi-step processes literally. Do not skip, merge, or summarize steps.
3. **Zero Assumptions**: If an instruction is absent or ambiguous, halt and ask for clarification. Do not invent missing steps or scripts.
4. **Mandatory Self-Verification**: Cross-reference actions against original instructions before finalizing any task or presenting a completion checklist.
5. **Anti-Hallucination Audit**: All architectural conclusions, problem reports, and proposed changes must be directly traceable to specific statements within project specifications or engine rules.

### C14 — Engine Versioning Protocol

To ensure accurate engine state tracking and reliable updates, any modification to the core engine/kernel files (anything inside the `.magic/` directory, including workflows and templates) MUST be accompanied by an automated engine metadata update: `node .magic/scripts/executor.js update-engine-meta --workflow {workflow}`.

1. **Scope**: Applies to all `.md` workflows, `scripts/`, `templates/`, and `config.json` inside the engine directory.
2. **Automation**: This command automatically increments the patch version in `.magic/.version`, updates the relevant history file in `.magic/history/`, and regenerates `.magic/.checksums`. **Smart History**: Redundant automated entries are skipped if the last entry matches.
3. **Exclusion**: Modifications to `.design/` files (project content) do NOT trigger an engine version bump; they trigger project manifest bumps instead.
4. **Synchronization**: The version in `.magic/.version` should stay aligned with the latest meaningful change to the engine's functional logic.
5. **Cognitive Exemption**: Purely cognitive tasks, dry runs, or audit tasks that do not modify files MUST NOT trigger a C14 version bump to avoid metadata noise.

### C15 — Workspace Scope Isolation

All workspace-aware workflows (`magic.analyze`, `magic.task`, `magic.run`, `magic.rule`) MUST enforce strict scope isolation based on the active workspace entry in `.design/workspace.json`.

1. **Scope Source**: The active workspace `scope` array is the single source of truth for bounded scans and modifications.
2. **Read/Write Boundary**: During scoped operations, file discovery, analysis, and writes MUST stay inside the declared scope paths.
3. **Out-of-Scope Handling**: Files outside scope may be reported as `UNSCOPED` for visibility, but MUST NOT be treated as in-scope drift or blockers.
4. **Cross-Workspace Exception (Read-Only)**: Cross-workspace reads are allowed only for integrity checks that require parent references (for example L2→L1 status/header parity). This exception never grants write access outside scope.
5. **No Implicit Expansion**: The agent MUST NOT widen scope heuristically (by similarity, naming, or inferred ownership) without an explicit workspace change.
6. **Invalid Scope Contract**: If a declared scope path is missing on disk, report a `STRUCTURE` violation and halt only the affected scoped workflow.

### C16 — Micro-spec Convention

For minor features, simple bugfixes, or changes expected to be under 50 lines of documentation, the agent is authorized to use the lightweight `.magic/templates/micro-spec.md` instead of the full specification template. If a Micro-spec exceeds 50 lines or architectural complexity increases, it MUST be promoted to the full Standard template.

### C17 — Session Isolation (Phase Gates)

To prevent context bleed-over and hallucination loops, the SDD workflow strictly separates Brainstorming, Planning, and Execution phases into isolated context windows.

1. **Brainstorming & Spec Generation (Phase 1)**: Must be completed within a single, continuous chat session so the agent retains the context of the evolving idea. Do not break the session until specs are marked `Stable`.
2. **Phase Transition (Phase Gates)**: Once a major phase completes (e.g., Specs are `Stable`), the current chat MUST be closed. **Note**: giving a text command like "forget previous instructions" does NOT clear context memory reliably. You must physically click the "New Chat" (or equivalent) button in your IDE/interface.
3. **Execution (Phases 2 & 3)**: Planning (`/magic.task`) and Coding (`/magic.run`) MUST each be started in a brand-new, clean chat session. This forces the agent to read the committed files as the singular source of truth, eliminating reliance on ephemeral chat memory.

### C18 — Payload Security

The installers (Node/Python) must verify payload integrity (checksums) and prevent Path Traversal attacks during extraction. Deployment must be atomic to prevent partial engine states.

### C19 — Cross-Env CLI Parity

Node and Python installers must maintain strict CLI parity. Every command-line flag (e.g., `--yes`, `--update`, `--check`) must behave identically across both implementations to ensure a consistent user experience.

### C20 — Auto-Heal Recovery

The engine must proactively identify and repair its own metadata. If `executor.js` detects missing history files or corrupted checksums during non-critical operations, it should attempt to "Auto-Heal" (restore defaults or regenerate) before Proceeding or Halting.

### C21 — Project Ventilation (Analyze)

The command `/magic.analyze` (or `Analyze project`) triggers "Project Ventilation": a deep scan that treats the current codebase as the source of truth and compares it against `INDEX.md` and `RULES.md`. It must identify:

- **Registry Drift**: Specs in INDEX but missing on disk.
- **Coverage Gaps**: Code folders without corresponding specs.
- **Rule Violations**: Code patterns that contradict `RULES.md` (both global and workspace tiers).
- **Integrity Issues**: Mismatched checksums in `.magic/`.

### C22 — Workspace Rule Inheritance

Each workspace may maintain a local `RULES.md` at `.design/{workspace}/RULES.md`. These files:

1. Contain only workspace-specific §7 conventions, identified as `WC1`, `WC2`, … (workspace convention).
2. Inherit all §1–6 universal rules and global §7 conventions from `.design/RULES.md` — no re-declaration needed.
3. Must not contradict the global constitution (Constitutional Guard applies equally).
4. Are created on demand by `magic.rule` when the first workspace-scoped rule is requested.
5. Version independently from the global `RULES.md`.

### C23 — Context Economy & Validation Caching

To minimize redundant resource usage and improve performance, the agent may optimize `check-prerequisites` calls within a single task lifecycle:

1. **Turn-Aware Caching**: If `check-prerequisites` returned `ok: true` earlier in the current conversation turn or the immediately preceding turn, and the agent has NOT modified any files in `.magic/` or `.design/` since that check, the agent is authorized to skip the physical script execution and rely on the known "Clean State".
2. **External Drift Guard**: If a significant time has passed or the user has performed manual file operations (e.g. `git pull`, manual edits in terminal), the agent MUST perform a fresh `check-prerequisites` call.
3. **Halt Persistence**: If the previous check returned an error or warning (e.g. `checksums_mismatch`), the agent MUST re-run the check after any attempt to fix it. Never assume a "heal" without verification.
4. **Audit Exemption**: In `/magic.analyze` (Ventilation), caching is NOT permitted. These workflows must perform fresh, physical scans by definition to fulfill their audit purpose.

### C24 — Go Standard Library First

1. **Language Version**: The project targets **Go 1.26.3** or later. Code MUST utilize the latest language features, specifically:
    - **Generics & Self-referential Types**: for complex ECS relationships.
    - **Bitmask Matching**: Queries utilizes 128-bit bitmask-based archetype matching (O(1)).
    - **Range-over-func Iteration**: Using `iter.Seq2` for all entity and component traversals.
    - **Pure Data Components**: All ECS Components MUST be simple data structs (POD-like) without internal methods that maintain state or perform complex logic.
    - **Enhanced `new`**: `new(Struct{...})` for direct pointer initialization.
    - **SIMD**: Use `simd/archsimd` (where applicable) for performance-critical vector/math operations.
2. **Runtime & GC**: The project is optimized for the **Green Tea Garbage Collector**. Memory layouts MUST prioritize small-object locality and stack allocation.
3. **Storage Strategy**: The engine core MUST prioritize **Sparse-Set storage** for general-purpose component access to ensure O(1) removal and high cache-locality for fragmented entity sets. `Table` storage is reserved for high-density, uniform component sets.
4. **Stdlib Priority**: Always prefer Go standard library packages. Use modern additions like `unique`, `slices`, `maps`.
5. **Concurrency Safety**: All systems MUST be analyzed for data races. Use `go test -race` as the primary gate. Shared state between systems MUST be protected by the scheduler (via resource/component locks) or explicit synchronization.
6. **Zero-Dependency Goal**: Strive for a minimal dependency footprint. The engine core (ECS, scheduling, events) must have zero external Go dependencies.

### C25 — ECS Architecture Reference Skill

When creating, reviewing, or amending any specification (L1 or L2), the agent MUST:

1. **Load Skill**: Read `.agents/skills/neuengine-reference/SKILL.md` before starting spec work to ensure full architectural context is loaded.
2. **Cross-Reference**: Verify that new specs are consistent with the module map, dependency graph, and Go conventions defined in the skill.

### C26 — Specification-Example Correlation

1. **Mandatory Link**: Every Layer 1 (Concept) specification MUST include a direct link to its corresponding directory in `examples/` (if implemented or planned) within the `Document History` section.
2. **Reciprocal Updates**: When a new example is added to the codebase, the relevant specification's `Document History` must be updated to reflect this addition.
3. **Draft Context**: For specifications in `Draft` status where examples do not yet exist, a placeholder link to the intended `examples/` path should be provided.

### C27 — GC Compensation (sync.Pool)

All hot-path allocations (commands, events, temporary views) MUST utilize `sync.Pool` for object reuse to minimize GC pauses and overhead.

### C28 — Performance-First QA

All core engine modules MUST include:

1. **Table-driven tests**: Covering edge cases and common scenarios.
2. **Fuzzing**: For all input-parsing and data-transformation logic.
3. **Race Detection**: All concurrent tests MUST pass with `-race`.
4. **Baselines**: Benchmarks with regression thresholds (CI-gates).

### C29 — Code Validation Stop-Factor

No new Layer 1 (Concept) or Layer 2 (Go) specifications may be moved to `Stable` status without a corresponding validating implementation in the `examples/` directory (per C26).

### C30 — Reference Code Utilization

When generating project code, it is highly recommended to consult the provided references. Do not blindly copy code; instead, extract the best solutions and optimize them to fit within our specifications, architectural plans, and current tasks.

### C31 — sync-docs Safety Gate

Do not invoke `node .magic/scripts/executor.js sync-docs` (or any automation that calls it) to update `CONTRIBUTING.md` or `README.md`. These files contain project-specific content that `sync-docs` overwrites from a generic template, destroying customisations. Apply targeted manual edits instead — patch the specific line or badge that changed; do not regenerate the file.

Applies to all workflows that perform documentation repair or version-sync steps.

### C32 — Third-Party Dependency ADRs

Adding the engine's **first external Go dependency** is a consequential, near-irreversible decision (a precedent plus a supply-chain surface) under C24§6 (Zero-Dependency Goal). Every proposal to add a third-party dependency MUST be recorded here as a numbered ADR — rejected, or accepted-with-justification — **before** the dependency is introduced. Decisions to date:

1. **`golang.org/x/image/font` — REJECTED (2026-06-03).** UI text is realized via an embedded stdlib bitmap font (`internal/ui/bitmapfont.go`, T-6D04.2) feeding the existing `FontAtlas`, preserving zero dependencies; the diagnostic overlay (`internal/ui/overlay.go`) consumes it. A real TTF/proportional rasterizer remains a future ADR if extended-glyph typography is required.
2. **WebSocket library (e.g. `nhooyr.io/websocket`) — REJECTED (2026-06-03).** The AI-assistant transport keeps the stdlib `stdio` + `http` connections (`pkg/assistant`); a WebSocket transport is not pursued. A minimal stdlib RFC6455 implementation remains the path if a long-lived bidirectional transport is later required.
3. **Tracy profiler client (CGo C library) — REJECTED (2026-06-04).** The profiling protocol (`pkg/diag/profiling`, T-7A03) ships **stdlib-only exporters**: a `chrome://tracing` Trace-Event-Format writer (`encoding/json`) and a `runtime/pprof` span-timing aggregator. The Tracy real-time GUI named by the L1 spec requires CGo plus a vendored third-party C client — both a dependency and a non-pure-Go build — so it is not implemented. The `ProfileExporter` interface leaves it a drop-in for anyone who accepts the CGo dependency under their own ADR (INV-5); `chrome://tracing`/Perfetto provides the comparable timeline view with zero dependencies.

Rationale for the default-reject stance: rejecting a dependency is reversible (a later ADR can accept it), whereas adding one sets an irreversible precedent — so the conservative choice preserves C24§6 until a need is proven.

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 1.0.0 | 2026-03-25 | Initial constitution |
| 1.1.0 | 2026-03-25 | Added C24 — Go Standard Library First |
| 1.2.0 | 2026-03-25 | Added C25 — ECS Architecture Reference Skill |
| 1.3.0 | 2026-03-27 | Added C26 — Specification-Example Correlation |
| 1.4.0 | 2026-03-27 | Integrated Research Insights: Green Tea GC, Bitmasks, SIMD. |
| 1.5.0 | 2026-03-31 | Restored and formalized C15 — Workspace Scope Isolation. |
| 1.6.0 | 2026-03-31 | Narrowed C25 scope by removing the public-documentation branding restriction. |
| 1.7.0 | 2026-04-20 | Added C30 — Reference Code Utilization. |
| 1.7.1 | 2026-04-25 | Cleanup: removed stray `Sprint 3` heading; reordered §7 conventions monotonically (C1..C30). |
| 1.8.0 | 2026-04-26 | Added C31 — sync-docs Safety Gate. |
| 1.9.0 | 2026-06-03 | Added C32 — Third-Party Dependency ADRs (formalizes the `x/image/font` + WebSocket rejections under C24§6). |
| 1.10.0 | 2026-06-04 | C32 ADR #3: Tracy profiler CGo client REJECTED — profiling ships stdlib-only Chrome-TEF + pprof exporters (T-7A03). |
