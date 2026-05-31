# Engine Examples (Track I)

Each subdirectory is a self-contained, runnable `package main` demonstrating one
engine subsystem. Examples double as end-to-end validation: they run headless
(no GPU), are deterministic, and are gated against committed goldens by
[`cmd/examplecheck`](../cmd/examplecheck).

Examples are introduced alongside the implementation phases (P1 ECS → P2
framework/asset/diagnostic → P3 scene/math → P4 render → P5 content). See the
[Examples Framework Specification](../.design/main/specifications/l1-examples-framework.md)
for the broader catalog and staged rollout.

## Conventions

- **`run() (uint64, error)`** does the deterministic work and returns a stable
  hash over its salient output.
- **`main()`** prints `PASS: <name> hash=<N>` on success, or `FAIL: <reason>`.
  Examples that validate by assertion rather than a single hash may print
  multiple `PASS ...` lines and no `hash=` token — these are run as smoke checks.
- **`main_test.go`** asserts `run()` is stable across ≥20 invocations.

Start a new example by copying [`_template/`](_template/) (the `_` prefix makes
the Go tool and `examplecheck` skip the scaffold itself).

## Golden gate

`examples/goldens.json` records the expected hash of every hash-emitting example.
The per-example `_test.go` proves *internal* determinism (the hash is stable);
the golden registry proves the *value itself* has not drifted — a behavioural
regression that is still deterministic would pass the unit test but fail here.

```sh
# Verify every example against the committed goldens (exit 1 on failure/drift):
go run ./cmd/examplecheck

# Rewrite the registry after an intentional behavioural change:
go run ./cmd/examplecheck -update

# List examples, or limit a CI run to those changed since a ref (selective build):
go run ./cmd/examplecheck -list
go run ./cmd/examplecheck -changed origin/master
```
