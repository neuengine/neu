# editor — App-integration validation (T-6T07)

Validates the Phase 6 App-integration cohort end-to-end. A single headless
`app.App` wires the cohort together and the example asserts deterministic facts
about the result, returning a stable hash (C29-style, checked by
`cmd/examplecheck` against `examples/goldens.json`).

What it exercises:

- **window** — `internal/window.SyncPlugin` composes and ticks under a headless
  backend (no OS window, CI-safe).
- **diagnostics** — `internal/diag.DiagnosticsPlugin` registers the built-in
  metrics and collects under a registered reader (INV-1 gate).
- **in-process plugin load** — a first-party plugin is registered via the
  compile-time factory registry, then `Build → Ready → Finish → Cleanup` are
  driven through `internal/plugin.LoadInProcess` / `FinishInProcess`.
- **definition hot-reload** — a scene definition is decoded, instantiated into
  the World, then replaced via `InstanceStore.Reload` (the operation the
  hot-reload system performs on an asset `Modified` event), proving stale
  entities are despawned.
- **ui** — a UI layout tree is solved against a fixed viewport (deterministic
  rects).
- **graph-debug round-trip** — a seeded debug trace is drained by the
  `PostUpdate` `grapheditor.DebugSync` system through one App frame, emitting
  `pkg/protocol` graph IPC messages; each message is asserted to survive an
  `Encode → Decode` wire round-trip.

Determinism: the loop runs on the real wall clock (gametime feeds the
diagnostics; the live trace recorder stamps `time.Now`), so the hash covers only
deterministic facts — registered diagnostic paths, entity counts, plugin
lifecycle state, UI rects, and graph frames seeded with fixed timestamps. The
live `Interpreter.RunTraced → grapheditor.Recorder` path is validated by
structure in `main_test.go`, not by hash.

Run it:

```sh
go run ./examples/editor    # prints: PASS: editor hash=<N>
go test ./examples/editor   # asserts the hash is stable across ≥20 runs
```
