# Benchmark Drift Gate (Track L)

`cmd/benchcompare` parses `go test -bench -benchmem` output and compares it
against `baseline.json`, failing on performance regression. It is the engine's
guard for the 0-alloc hot-path discipline (C-004 / C-027).

## Baseline format

`baseline.json` maps each benchmark's canonical name (the GOMAXPROCS `-N` suffix
stripped) to its reference metrics:

```json
{
  "go_version": "go1.26.3",
  "updated": "2026-05-31T00:00:00Z",
  "results": {
    "BenchmarkLerpVec3": { "ns_per_op": 1.5, "bytes_per_op": 0, "allocs_per_op": 0 }
  }
}
```

## Workflow

Regenerate the baseline on a quiet machine:

```sh
go test -run=^$ -bench=. -benchmem ./... | go run ./cmd/benchcompare -update
```

Gate a change against it (exit 1 on regression):

```sh
go test -run=^$ -bench=. -benchmem ./... | go run ./cmd/benchcompare
```

## Regression policy

- **ns/op, B/op** — regress when they grow by more than `-threshold` percent
  (default 5%).
- **allocs/op** — regress on *any* increase. A hot path that went from 0 → 1
  allocation is a categorical break, not a percentage wobble.

`-json` emits a machine-readable report for the CI gate (Track E, `T-6E02`).
