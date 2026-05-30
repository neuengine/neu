# AI API Plugin — Go Implementation

**Version:** 0.1.0
**Status:** Draft
**Layer:** go
**Implements:** [l1-ai-api-plugin.md](l1-ai-api-plugin.md)

## Overview

Go-level design for the first-party `pkg/plugins/aiapi/` plugin: a unified HTTP
client over OpenAI-compatible chat-completions providers (OpenAI, Anthropic,
Gemini, local), exposing the standard assistant methods. Provider wire formats
are translated to/from an internal **canonical** request/response type so only
canonical types cross plugin boundaries. Credentials are read (never written)
from env / OS keyring / age-encrypted file and redacted on every output path
(INV-1). All HTTP runs off the main loop on the worker pool with per-request
`context.Context` timeout + cancellation (INV-2/INV-6). Behaviour is identical
in-process and out-of-process (INV-7).

## Related Specifications

- [l1-ai-api-plugin.md](l1-ai-api-plugin.md) — L1 concept specification (parent)
- [l2-plugin-distribution-go.md](l2-plugin-distribution-go.md) — manifest, lifecycle, capability model, both delivery modes
- [l2-ai-assistant-system-go.md](l2-ai-assistant-system-go.md) — agent protocol + standard methods this plugin implements
- [l2-task-system-go.md](l2-task-system-go.md) — HTTP runs on the worker pool, off the main schedule
- [l2-error-core-go.md](l2-error-core-go.md) — provider/HTTP errors map to `E-PLUGIN-AIAPI-{NNN}` via `pkg/errs`
- [l2-event-system-go.md](l2-event-system-go.md) — streaming chat delivered as `AssistantStreamEvent`
- [l2-diagnostic-system-go.md](l2-diagnostic-system-go.md) — per-provider latency/token/cost diagnostics

## 1. Motivation

The L1 makes this the canonical exemplar of the plugin distribution model: a
non-trivial, shipped-but-optional plugin that uses only the public SDK, declares
capabilities, manages secrets, and runs in either mode unchanged. The Go binding
centers on the `Provider` interface + canonical types so adding a provider is one
new file, and on disciplined secret handling + redaction so keys never leak.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: `net/http` (stdlib client), `encoding/json`, `bufio` (SSE), `crypto/subtle` (secret zeroing); `filippo.io/age` for the encrypted-file source (ADR) — env/keyring paths are stdlib/OS.
- **`//go:build editor`** by default (L1 §2); a headless variant is out of scope.
- **C-003**: HTTP/JSON/SSE are stdlib; `age` + OS-keyring libs require ADRs; the env source is dependency-free and the default for CI/dev.
- HTTP/HTTPS only; streaming for `chat` only (whole responses for other methods).
- Non-deterministic model output ⇒ tests use a `FakeProvider` (canned `map[promptHash]response`).

## 3. Core Invariants

> [!NOTE]
> See [l1-ai-api-plugin.md §3](l1-ai-api-plugin.md) for INV-1…INV-9. Go-specific
> compliance is tabulated in §4.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: API keys never logged/leaked | `credentials.go` reads secrets into a `[]byte` zeroed via `crypto/subtle` after the request; a `redactingWriter` masks any configured key-source substring; tests assert no output contains the test key. |
| **INV-2**: every request has a `context` timeout; no goroutine leak | All `Provider` calls take `ctx context.Context` from the assistant manager under `context.WithTimeout`; cancellation closes the HTTP body and returns within one RTT (`-race` + leak check). |
| **INV-3**: concurrent requests are safe | Request bodies are built per-call from immutable `Config`; mutable state (rate-limit bucket, in-flight `map[RequestID]context.CancelFunc`) is `sync.Mutex`-guarded. |
| **INV-4**: honours capability declarations | Without `network.outbound` the plugin's `Ready` returns false / refuses load; methods needing `world.commands` return `CapabilityDenied` (from the SDK proxy) instead of executing. |
| **INV-5**: provider errors map to a finite `E-PLUGIN-AIAPI-{NNN}` set | `errors.go` maps HTTP status + provider shape to documented codes; unknown shapes ⇒ `E-PLUGIN-AIAPI-999` with the raw body in the diagnostic log only. |
| **INV-6**: streaming cancellable at any chunk boundary | `Stream` checks `ctx.Err()` between chunks; an `AssistantCancelEvent` for the request ID calls the stored `CancelFunc`, closing the connection + cleaning per-request state. |
| **INV-7**: identical in-process vs out-of-process | One test suite runs both modes via the Track-N OOP loader; canonical responses asserted equal. |
| **INV-8**: client-side per-provider rate limiting | `ratelimit.go` token bucket (RPM + TPM) per provider, in addition to the manager's global limit; exceeding either ⇒ `RateLimited` with retry-after. |
| **INV-9**: token/cost recorded per successful request | `usage` fields → diagnostics counters (`aiapi.tokens_*`, `aiapi.cost_usd`) on every success. |

## Go Package

```
pkg/plugins/aiapi/             // //go:build editor
  plugin.go                    // New(), Build/Ready/Finish/Cleanup lifecycle
  manifest.go plugin.toml      // embedded manifest (//go:embed)
  config.go                    // Config, ProviderConfig, JSON-Schema export
  provider.go                  // Provider interface, registry, Selector
  provider_openai.go           // + _anthropic, _gemini, _local
  canonical.go                 // CanonicalRequest/Message/Response/Usage
  stream.go                    // SSE chunk decoding → AssistantStreamEvent
  credentials.go               // env/keyring/age-file loaders + zeroing
  ratelimit.go                 // per-provider token bucket (RPM+TPM)
  errors.go                    // E-PLUGIN-AIAPI-{NNN} mapping
  methods/                     // one file per standard method (chat, generate_*, ...)
  testing/fake_provider.go     // canned in-memory provider for deterministic tests
```

## Type Definitions

```go
type Provider interface {
    Name() string
    Capabilities() ProviderCaps
    Complete(ctx context.Context, r CanonicalRequest) (CanonicalResponse, error)
    Stream(ctx context.Context, r CanonicalRequest, sink func(Chunk) error) error
    Embeddings(ctx context.Context, in []string) ([][]float32, error) // ErrUnsupported ok
}

type CanonicalRequest struct {
    Messages       []CanonicalMessage
    Tools          []CanonicalTool
    ResponseFormat CanonicalFormat
    Parameters     map[string]any // temperature, top_p, stop, ...
}
type CanonicalMessage struct {
    Role    string        // system | user | assistant | tool
    Content []ContentPart // text | image-url | tool_use | tool_result
}
type CanonicalResponse struct {
    Message   CanonicalMessage
    ToolCalls []CanonicalToolCall
    Finish    string // stop | length | tool_use | filter
    Usage     Usage  // InputTokens, OutputTokens, CostUSD
}

type Config struct {
    ActiveProvider string
    Providers      map[string]ProviderConfig
    DefaultTimeout time.Duration
    RateLimitRPM   int
    RateLimitTPM   int
    RedactLogs     bool
    CostBudgetUSD  float64
}

// RegisterProvider adds a provider factory at compile time (the one extension point).
func RegisterProvider(name string, p Provider)
```

## Performance Strategy

- **One stdlib `http.Client`** per provider with connection reuse (keep-alive); no per-request client allocation.
- **Off-main-loop**: every method runs on `task.IOPool`; the editor schedule never blocks on a provider RTT.
- **Streaming is incremental**: SSE chunks are decoded with a reused `bufio.Scanner` and emitted as events as they arrive — no full-buffer wait.
- **Secret hygiene without allocation churn**: the key `[]byte` is read once per request and zeroed; the redacting writer scans only on the error/log path, not the hot success path.

## Error Handling

| Condition | Behavior |
| :--- | :--- |
| Missing/invalid key | `E-PLUGIN-AIAPI-100` (credential resolution) |
| HTTP 401 / 429 / 4xx / 5xx | `-201` / `-202` (+retry-after) / `-200` / `-300` |
| Timeout / cancelled | `-400` / `-401`; goroutine released within one RTT |
| Canonical parse / tool-schema mismatch | `-500` / `-501` |
| Capability denied at runtime | `-600` |
| Unknown provider error shape | `-999`; raw body to diagnostic log only (never user-facing) |

## Testing Strategy

- **Per-provider golden**: request-build + response-parse against recorded fixtures (keys redacted).
- **FakeProvider integration**: deterministic method dispatch, streaming, cancellation, Command emission.
- **Redaction** (INV-1): every error path asserts the test key never appears in output.
- **Concurrency** (INV-3): parallel requests under `-race`; in-flight map + bucket are race-clean.
- **Streaming cancel** (INV-6): cancel mid-stream closes the connection + cleans state; no leak.
- **Mode parity** (INV-7): the suite runs in-process and OOP; canonical results asserted identical.
- **Fuzz**: response parsers fuzzed with malformed/truncated JSON streams.

## 7. Drawbacks & Alternatives

- **Drawback**: the OpenAI-compatible canonical schema can't express every provider feature.
  **Alternative**: per-provider native types end to end.
  **Decision**: L1 §2 — canonical covers ~80%; provider-only features use per-provider extension methods. Kept.
- **Drawback**: `age` + keyring are external deps.
  **Alternative**: env-var only.
  **Decision**: env is the dependency-free default (CI/dev); keyring/age are ADR-gated opt-ins for end-user/portable installs. Kept.

## Canonical References

<!-- MANDATORY for Stable status. Stub — populate when implementation lands
     (Phase 6 Track O). Blocked until: (1) l1-ai-api-plugin Stable + the plugin
     distribution SDK (Track N) lands; (2) providers + methods + FakeProvider
     implemented with mode-parity tests (T-6O01..05, T-6T02). -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-30 | Initial L2 draft — Go translation of l1-ai-api-plugin v0.1.0. `pkg/plugins/aiapi` (`//go:build editor`): `Provider` interface + canonical request/response types (one file per provider), embedded manifest, env/keyring/age credential sources with `crypto/subtle` zeroing + redaction (INV-1), worker-pool HTTP with per-request timeout/cancel (INV-2/INV-6), per-provider RPM+TPM token bucket (INV-8), `E-PLUGIN-AIAPI-{NNN}` mapping (INV-5), token/cost diagnostics (INV-9), `FakeProvider` for deterministic + mode-parity tests (INV-7), `RegisterProvider` extension point. Authored ahead of Phase 6 Track O (`/magic.spec`). Draft — L1 parent Draft + depends on Track N SDK + no implementation yet. |
